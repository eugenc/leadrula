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
	webhooks     WebhookFirer
}

func NewService(repo *Repository, notif *notifications.Service, acc *accounts.Repository, pipes *pipelines.Service, integrations IntegrationEnqueuer) *Service {
	return &Service{repo: repo, notif: notif, accounts: acc, pipelines: pipes, integrations: integrations}
}

// SetWebhookFirer wires outbound webhook firing after construction (avoids import cycle).
func (s *Service) SetWebhookFirer(wf WebhookFirer) { s.webhooks = wf }

func (s *Service) Repo() *Repository { return s.repo }

// ChangeStage moves a lead to a new stage, enforcing destination prompts and
// applying any matching return rule. Atomic per DB spec §4.2.
func (s *Service) ChangeStage(ctx context.Context, p *auth.Principal, leadID, newStageID int64, actionAt *time.Time, disqReasonID *int64) (*Lead, []auth.ImpersonationChange, error) {
	var pendingEmails []notifications.EmailJob
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
	if !s.repo.CollaborationLeadAllowed(ctx, p, lead) {
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
	if err := s.pipelines.CheckStageAccess(ctx, p, newStageID); err != nil {
		return nil, nil, err
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
		hasActive, err := s.pipelines.HasActiveStageReasons(ctx, newStageID)
		if err != nil {
			return nil, nil, err
		}
		if !hasActive {
			return nil, nil, httpx.BusinessRule("no disqualification reasons configured for this stage")
		}
	}

	if actionAt != nil {
		if err := s.repo.SetActionAt(ctx, tx, leadID, actionAt); err != nil {
			return nil, nil, err
		}
	}
	if disqReasonID != nil {
		ok, err := s.pipelines.ReasonBelongsToStage(ctx, newStageID, *disqReasonID)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, httpx.Validation("invalid disqualification reason for this stage")
		}
		if err := s.repo.SetDisqReason(ctx, tx, leadID, *disqReasonID); err != nil {
			return nil, nil, err
		}
	}

	fromStage := lead.StageID
	if err := s.repo.UpdateStage(ctx, tx, leadID, stage.PipelineID, newStageID); err != nil {
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
	var enqueueBranchPos int
	deps := RouteApplyDeps{Repo: s.repo, Accounts: s.accounts, Notif: s.notif, Integrations: s.integrations}

	// pipeline-origin route: publisher-owned lead reached a trigger stage
	if lead.ContractID == nil && lead.OwnerAccountID == lead.PublisherID {
		rt, err := routing.MatchRouteByStage(ctx, tx, lead.PublisherID, *finalStageID, leadID, nil)
		if err != nil {
			return nil, nil, err
		}
		if rt != nil {
			if lead.PreassignedBuyerID != nil {
				emails, err := TryApplyPreassignedBuyer(ctx, tx, deps, lead, leadID)
				if err != nil {
					return nil, nil, err
				}
				pendingEmails = append(pendingEmails, emails...)
			} else {
				enqueue, emails, err := TryApplyMatchedRoute(ctx, tx, deps, rt, leadID)
				if err != nil {
					return nil, nil, err
				}
				pendingEmails = append(pendingEmails, emails...)
				if enqueue {
					enqueueRouteID = rt.ID
					enqueueBranchPos = rt.MatchedBranchPosition
				}
			}
			updated, err = s.repo.GetByID(ctx, tx, leadID)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	// buyer-owned pipeline-origin routes
	if p.AccountType == "buyer" && lead.OwnerAccountID == p.AccountID {
		if rt, err := routing.MatchBuyerRouteByStage(ctx, tx, p.AccountID, *finalStageID, leadID, nil); err != nil {
			return nil, nil, err
		} else if rt != nil && enqueueRouteID == 0 {
			enqueue, emails, err := TryApplyMatchedRoute(ctx, tx, deps, rt, leadID)
			if err != nil {
				return nil, nil, err
			}
			pendingEmails = append(pendingEmails, emails...)
			if enqueue {
				enqueueRouteID = rt.ID
				enqueueBranchPos = rt.MatchedBranchPosition
			}
			updated, err = s.repo.GetByID(ctx, tx, leadID)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	if lead.ContractID != nil && finalStageID != nil {
		if lead.OwnerAccountID != lead.PublisherID {
			if err := contracts.SyncPublisherStageWithRebuild(ctx, tx, *lead.ContractID, leadID, lead.OwnerAccountID, *finalStageID); err != nil {
				return nil, nil, err
			}
		}
		if err := contracts.TryAccrueOnBuyerStage(ctx, tx, *lead.ContractID, leadID, *finalStageID); err != nil {
			return nil, nil, err
		}
	}

	// return rule?
	if lead.ContractID != nil {
		ri, err := contracts.FindReturnRule(ctx, tx, *lead.ContractID, lead.OwnerAccountID, *finalStageID)
		if err != nil {
			return nil, nil, err
		}
		if ri != nil {
			if err := contracts.RecordEarningReturn(ctx, tx, leadID, lead.ContractID); err != nil {
				return nil, nil, err
			}
			if err := s.repo.MoveToPublisher(ctx, tx, leadID, ri.PublisherID, ri.SourcePipelineID, ri.ReturnStageID); err != nil {
				return nil, nil, err
			}
			returned, err := s.repo.GetByID(ctx, tx, leadID)
			if err != nil {
				return nil, nil, err
			}
			emails, err := s.notif.Deliver(ctx, tx, notifications.DeliverParams{
				AccountID: ri.PublisherID,
				UserIDs:   notifications.AssigneeIDs(returned.AssignedUserID),
				EventType: "lead_returned",
				Payload:   map[string]any{"lead_id": leadID},
			})
			if err != nil {
				return nil, nil, err
			}
			pendingEmails = append(pendingEmails, emails...)
			updated = returned
			finalStageID = returned.StageID
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	s.notif.SendEmails(pendingEmails)
	if enqueueRouteID != 0 {
		TryEnqueueIntegrations(ctx, s.repo.Pool(), s.repo, s.integrations, enqueueRouteID, leadID, enqueueBranchPos)
	}
	_ = s.repo.attachCustomValues(ctx, updated)
	var auditChanges []auth.ImpersonationChange
	if p.Impersonator != nil {
		fromName := s.repo.StageName(ctx, s.repo.pool, fromStage)
		toName := s.repo.StageName(ctx, s.repo.pool, &newStageID)
		auditChanges = stageChange(fromName, toName)
	}
	// Fire outbound webhook triggers for stage move.
	s.fireOutbound(ctx, p.AccountID, "pipeline.move_stage", updated, PipelineContext{
		PipelineID:  updated.PipelineID,
		StageID:     finalStageID,
		PrevStageID: fromStage,
	})
	return updated, auditChanges, nil
}

// ClearFromPipeline removes a lead from its pipeline without triggering stage rules or return flows.
func (s *Service) ClearFromPipeline(ctx context.Context, p *auth.Principal, leadID int64) (*Lead, error) {
	lead, err := s.repo.Get(ctx, p, leadID)
	if err != nil {
		return nil, err
	}
	if err := assertCanEdit(p, lead); err != nil {
		return nil, err
	}
	if lead.PipelineID == nil && lead.StageID == nil {
		return lead, nil
	}

	tx, err := s.repo.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := s.repo.ClearFromPipeline(ctx, tx, leadID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	updated, err := s.repo.Get(ctx, p, leadID)
	if err != nil {
		return nil, err
	}
	s.fireOutbound(ctx, p.AccountID, "lead.update", updated, PipelineContext{
		PipelineID: updated.PipelineID,
		StageID:    updated.StageID,
	})
	return updated, nil
}

// ChangeStageByWebhook moves a lead without user permission checks (inbound webhook).
func (s *Service) ChangeStageByWebhook(ctx context.Context, accountID, leadID, newStageID int64, actionAt *time.Time, disqReasonID *int64) (*Lead, error) {
	p := &auth.Principal{AccountID: accountID, Role: "admin", UserID: 0}
	lead, _, err := s.ChangeStage(ctx, p, leadID, newStageID, actionAt, disqReasonID)
	return lead, err
}

// fireOutbound fires outbound webhooks if a firer is wired. Runs best-effort (no error return).
func (s *Service) fireOutbound(ctx context.Context, accountID int64, event string, lead *Lead, pctx PipelineContext) {
	if s.webhooks == nil {
		return
	}
	s.webhooks.FireOutbound(ctx, accountID, event, lead, pctx)
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

	target, err := contracts.GetTargetByContract(ctx, tx, contractID)
	if err != nil {
		return nil, err
	}
	if err := CheckDuplicate(ctx, tx, target.BuyerID, lead.Phone, lead.Email, leadID); err != nil {
		return nil, err
	}
	buyer, err := s.accounts.GetAccount(ctx, target.BuyerID)
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
	if err := contracts.InitPublisherTracking(ctx, tx, target.ID, leadID, target.BuyerID, firstStage); err != nil {
		return nil, err
	}
	if err := contracts.CheckCap(ctx, tx, target.ID, target.CompensationID); err != nil {
		return nil, err
	}
	if err := s.repo.SetStatus(ctx, tx, leadID, "distributed"); err != nil {
		return nil, err
	}
	if err := billing.Debit(ctx, tx, target.BuyerID, target.RatePerLead, leadID, target.ID, "lead re-distributed"); err != nil {
		return nil, err
	}
	costBasis := costBasisFromLead(lead)
	if err := contracts.RecordEarningDistribute(ctx, tx, target.CompensationID, leadID, target.RatePerLead, costBasis); err != nil {
		return nil, err
	}
	if err := s.repo.SetCostAfterBuyerDistribution(ctx, tx, leadID, buyer.Type, target.RatePerLead); err != nil {
		return nil, err
	}
	updated, err := s.repo.GetByID(ctx, tx, leadID)
	if err != nil {
		return nil, err
	}
	emails, err := s.notif.Deliver(ctx, tx, notifications.DeliverParams{
		AccountID: target.BuyerID,
		UserIDs:   notifications.AssigneeIDs(updated.AssignedUserID),
		EventType: "new_lead",
		Payload:   map[string]any{"lead_id": leadID},
	})
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.notif.SendEmails(emails)
	_ = s.repo.attachCustomValues(ctx, updated)
	s.fireOutbound(ctx, target.BuyerID, "pipeline.place", updated, PipelineContext{
		PipelineID: updated.PipelineID,
		StageID:    updated.StageID,
	})
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
		n, err := s.repo.Delete(ctx, p, bp.LeadIDs)
		if err != nil {
			return nil, nil, err
		}
		if err := rejectPartialBulk(p, len(bp.LeadIDs), int(n)); err != nil {
			return nil, nil, err
		}
		return &BulkResult{Affected: int(n)}, nil, nil
	case BulkAssignUser:
		if bp.UserID == 0 {
			return nil, nil, httpx.Validation("user_id required")
		}
		uid := bp.UserID
		n, err := s.repo.BulkSetAssignee(ctx, p, bp.LeadIDs, &uid)
		if err != nil {
			return nil, nil, err
		}
		if err := rejectPartialBulk(p, len(bp.LeadIDs), int(n)); err != nil {
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
		if err := s.repo.BulkAddFollowers(ctx, p, bp.LeadIDs, bp.UserID); err != nil {
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

func rejectPartialBulk(p *auth.Principal, requested, affected int) error {
	if _, scoped := p.CollaborationPublisherID(); scoped && affected != requested {
		return httpx.Forbidden("one or more leads are not accessible")
	}
	return nil
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
