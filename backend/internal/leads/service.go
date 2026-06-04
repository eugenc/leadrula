package leads

import (
	"context"
	"time"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/billing"
	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/internal/pipelines"
	"github.com/echayko/leadrula/backend/internal/routing"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

type Service struct {
	repo         *Repository
	notif        *notifications.Service
	accounts     *accounts.Repository
	pipelines    *pipelines.Service
	integrations IntegrationEnqueuer
}

func NewService(repo *Repository, notif *notifications.Service, acc *accounts.Repository, pipes *pipelines.Service, integrations IntegrationEnqueuer) *Service {
	return &Service{repo: repo, notif: notif, accounts: acc, pipelines: pipes, integrations: integrations}
}

func (s *Service) Repo() *Repository { return s.repo }

// ChangeStage moves a lead to a new stage, enforcing destination prompts and
// applying any matching return rule. Atomic per DB spec §4.2.
func (s *Service) ChangeStage(ctx context.Context, p *auth.Principal, leadID, newStageID int64, actionAt *time.Time, disqReasonID *int64) (*Lead, []auth.ImpersonationChange, error) {
	tx, err := s.repo.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	lead, err := s.repo.GetByID(ctx, tx, leadID)
	if err != nil {
		return nil, nil, err
	}
	if lead.OwnerAccountID != p.AccountID {
		return nil, nil, httpx.NotFound("lead not found")
	}
	if err := assertCanEdit(p, lead); err != nil {
		return nil, nil, err
	}

	stage, err := s.repo.GetStage(ctx, tx, newStageID)
	if err != nil {
		return nil, nil, err
	}
	if stage.AccountID != p.AccountID {
		return nil, nil, httpx.BusinessRule("stage does not belong to this account")
	}

	switch stage.StageType {
	case pipelines.StageTypeAction:
		if actionAt == nil {
			return nil, nil, httpx.BusinessRule("Action Date & Time is required for this stage")
		}
	case pipelines.StageTypeDisqualification:
		if disqReasonID == nil {
			return nil, nil, httpx.BusinessRule("a disqualification reason is required for this stage")
		}
	}

	if actionAt != nil {
		if err := s.repo.SetActionAt(ctx, tx, leadID, actionAt); err != nil {
			return nil, nil, err
		}
	}
	if disqReasonID != nil {
		ok, err := s.repo.ReasonBelongsToAccount(ctx, tx, p.AccountID, *disqReasonID)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, httpx.Validation("invalid disqualification reason")
		}
		if err := s.repo.SetDisqReason(ctx, tx, leadID, *disqReasonID); err != nil {
			return nil, nil, err
		}
	}

	fromStage := lead.StageID
	if err := s.repo.UpdateStage(ctx, tx, leadID, newStageID); err != nil {
		return nil, nil, err
	}
	if stage.StageType == pipelines.StageTypeWon {
		if err := s.repo.SetStatus(ctx, tx, leadID, "closed"); err != nil {
			return nil, nil, err
		}
	}
	if err := s.repo.InsertStageHistory(ctx, tx, leadID, fromStage, newStageID, p.UserID, actionAt, disqReasonID); err != nil {
		return nil, nil, err
	}

	if err := s.pipelines.EvaluateStageRules(ctx, tx, p.AccountID, p.UserID, leadID, newStageID, fromStage); err != nil {
		return nil, nil, err
	}

	updated, err := s.repo.GetByID(ctx, tx, leadID)
	if err != nil {
		return nil, nil, err
	}
	finalStageID := updated.StageID
	if finalStageID == nil {
		return nil, nil, httpx.BusinessRule("lead has no stage after move")
	}

	var enqueueRouteID int64
	// pipeline-origin route: publisher-owned lead reached a trigger stage
	if lead.ContractID == nil && lead.OwnerAccountID == lead.PublisherID {
		rt, err := routing.MatchRouteByStage(ctx, tx, lead.PublisherID, *finalStageID)
		if err != nil {
			return nil, nil, err
		}
		if rt != nil {
			deps := RouteApplyDeps{Repo: s.repo, Accounts: s.accounts, Notif: s.notif, Integrations: s.integrations}
			if err := ApplyRoute(ctx, tx, deps, rt, lead.PublisherID, leadID); err != nil {
				return nil, nil, err
			}
			enqueueRouteID = rt.ID
			updated, err = s.repo.GetByID(ctx, tx, leadID)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	// return rule?
	if lead.ContractID != nil {
		ri, err := contracts.FindReturnRule(ctx, tx, *lead.ContractID, *finalStageID)
		if err != nil {
			return nil, nil, err
		}
		if ri != nil {
			if err := s.repo.MoveToPublisher(ctx, tx, leadID, ri.PublisherID, ri.SourcePipelineID, ri.ReturnStageID); err != nil {
				return nil, nil, err
			}
			adminIDs, err := s.accounts.AdminUserIDs(ctx, tx, ri.PublisherID)
			if err != nil {
				return nil, nil, err
			}
			if err := s.notif.Enqueue(ctx, tx, adminIDs, "lead_returned", map[string]any{"lead_id": leadID}); err != nil {
				return nil, nil, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	if enqueueRouteID != 0 {
		TryEnqueueIntegrations(ctx, s.repo.Pool(), s.repo, s.integrations, enqueueRouteID, leadID)
	}
	_ = s.repo.attachCustomValues(ctx, updated)
	var auditChanges []auth.ImpersonationChange
	if p.Impersonator != nil {
		fromName := s.repo.StageName(ctx, s.repo.pool, fromStage)
		toName := s.repo.StageName(ctx, s.repo.pool, &newStageID)
		auditChanges = stageChange(fromName, toName)
	}
	return updated, auditChanges, nil
}

// Redistribute reassigns a publisher-held (returned) lead to another buyer and
// charges that buyer. DB spec §4.3.
func (s *Service) Redistribute(ctx context.Context, p *auth.Principal, leadID, contractID int64) (*Lead, error) {
	tx, err := s.repo.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	lead, err := s.repo.GetByID(ctx, tx, leadID)
	if err != nil {
		return nil, err
	}
	if lead.OwnerAccountID != p.AccountID {
		return nil, httpx.NotFound("lead not found")
	}

	target, err := contracts.GetTarget(ctx, tx, contractID)
	if err != nil {
		return nil, err
	}
	// land in the buyer pipeline's first stage
	var firstStage int64
	if err := tx.QueryRow(ctx,
		`SELECT id FROM pipeline_stages WHERE pipeline_id=$1 ORDER BY position, id LIMIT 1`,
		target.BuyerPipelineID).Scan(&firstStage); err != nil {
		return nil, httpx.BusinessRule("target pipeline has no stages")
	}
	if err := s.repo.PlaceInPipeline(ctx, tx, leadID, target.BuyerID, target.BuyerPipelineID, firstStage, &target.ID); err != nil {
		return nil, err
	}
	if err := s.repo.SetStatus(ctx, tx, leadID, "distributed"); err != nil {
		return nil, err
	}
	if err := billing.Debit(ctx, tx, target.BuyerID, target.RatePerLead, leadID, target.ID, "lead re-distributed"); err != nil {
		return nil, err
	}
	adminIDs, err := s.accounts.AdminUserIDs(ctx, tx, target.BuyerID)
	if err != nil {
		return nil, err
	}
	if err := s.notif.Enqueue(ctx, tx, adminIDs, "new_lead", map[string]any{"lead_id": leadID}); err != nil {
		return nil, err
	}

	updated, err := s.repo.GetByID(ctx, tx, leadID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return updated, nil
}

type BulkAction string

const (
	BulkDelete      BulkAction = "delete"
	BulkAssignUser  BulkAction = "assign_user"
	BulkAddFollower BulkAction = "add_follower"
	BulkAssignBuyer BulkAction = "assign_buyer"
)

type BulkParams struct {
	Action     BulkAction
	LeadIDs    []int64
	UserID     int64
	ContractID int64
}

type BulkResult struct {
	Affected int `json:"affected"`
}

func (s *Service) Bulk(ctx context.Context, p *auth.Principal, bp BulkParams) (*BulkResult, []auth.ImpersonationChange, error) {
	if len(bp.LeadIDs) == 0 {
		return nil, nil, httpx.Validation("no leads selected")
	}
	switch bp.Action {
	case BulkDelete:
		n, err := s.repo.Delete(ctx, p.AccountID, bp.LeadIDs)
		if err != nil {
			return nil, nil, err
		}
		return &BulkResult{Affected: int(n)}, nil, nil
	case BulkAssignUser:
		if bp.UserID == 0 {
			return nil, nil, httpx.Validation("user_id required")
		}
		uid := bp.UserID
		n, err := s.repo.BulkSetAssignee(ctx, p.AccountID, bp.LeadIDs, &uid)
		if err != nil {
			return nil, nil, err
		}
		var auditChanges []auth.ImpersonationChange
		if p.Impersonator != nil {
			name := ""
			if user, err := s.accounts.GetUser(ctx, bp.UserID); err == nil && user != nil {
				name = user.FullName
			}
			auditChanges = bulkAssignChange(name, int(n))
		}
		return &BulkResult{Affected: int(n)}, auditChanges, nil
	case BulkAddFollower:
		if bp.UserID == 0 {
			return nil, nil, httpx.Validation("user_id required")
		}
		if err := s.repo.BulkAddFollowers(ctx, bp.LeadIDs, bp.UserID); err != nil {
			return nil, nil, err
		}
		return &BulkResult{Affected: len(bp.LeadIDs)}, nil, nil
	case BulkAssignBuyer:
		if bp.ContractID == 0 {
			return nil, nil, httpx.Validation("contract_id required")
		}
		affected := 0
		for _, id := range bp.LeadIDs {
			if _, err := s.Redistribute(ctx, p, id, bp.ContractID); err != nil {
				continue
			}
			affected++
		}
		if affected == 0 {
			return nil, nil, httpx.BusinessRule("no leads could be assigned to buyer")
		}
		return &BulkResult{Affected: affected}, nil, nil
	default:
		return nil, nil, httpx.Validation("unknown bulk action")
	}
}

func assertCanEdit(p *auth.Principal, l *Lead) error {
	switch p.Role {
	case "admin":
		return nil
	case "user":
		if l.AssignedUserID != nil && *l.AssignedUserID == p.UserID {
			return nil
		}
		return httpx.Forbidden("you can only edit leads assigned to you")
	default:
		return httpx.Forbidden("read-only role")
	}
}
