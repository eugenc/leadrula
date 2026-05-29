package leads

import (
	"context"
	"time"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/billing"
	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

type Service struct {
	repo     *Repository
	notif    *notifications.Service
	accounts *accounts.Repository
}

func NewService(repo *Repository, notif *notifications.Service, acc *accounts.Repository) *Service {
	return &Service{repo: repo, notif: notif, accounts: acc}
}

func (s *Service) Repo() *Repository { return s.repo }

// ChangeStage moves a lead to a new stage, enforcing destination prompts and
// applying any matching return rule. Atomic per DB spec §4.2.
func (s *Service) ChangeStage(ctx context.Context, p *auth.Principal, leadID, newStageID int64, actionAt *time.Time, disqReasonID *int64) (*Lead, error) {
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
	if err := assertCanEdit(p, lead); err != nil {
		return nil, err
	}

	stage, err := s.repo.GetStage(ctx, tx, newStageID)
	if err != nil {
		return nil, err
	}
	if stage.AccountID != p.AccountID {
		return nil, httpx.BusinessRule("stage does not belong to this account")
	}

	if stage.PromptActionDatetime && actionAt == nil {
		return nil, httpx.BusinessRule("Action Date & Time is required for this stage")
	}
	if stage.PromptDisqualification && disqReasonID == nil {
		return nil, httpx.BusinessRule("a disqualification reason is required for this stage")
	}

	if actionAt != nil {
		if err := s.repo.SetActionAt(ctx, tx, leadID, actionAt); err != nil {
			return nil, err
		}
	}
	if disqReasonID != nil {
		ok, err := s.repo.ReasonBelongsToAccount(ctx, tx, p.AccountID, *disqReasonID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, httpx.Validation("invalid disqualification reason")
		}
		if err := s.repo.SetDisqReason(ctx, tx, leadID, *disqReasonID); err != nil {
			return nil, err
		}
	}

	fromStage := lead.StageID
	if err := s.repo.UpdateStage(ctx, tx, leadID, newStageID); err != nil {
		return nil, err
	}
	if err := s.repo.InsertStageHistory(ctx, tx, leadID, fromStage, newStageID, p.UserID, actionAt, disqReasonID); err != nil {
		return nil, err
	}

	// return rule?
	if lead.ContractID != nil {
		ri, err := contracts.FindReturnRule(ctx, tx, *lead.ContractID, newStageID)
		if err != nil {
			return nil, err
		}
		if ri != nil {
			if err := s.repo.MoveToPublisher(ctx, tx, leadID, ri.PublisherID, ri.SourcePipelineID, ri.ReturnStageID); err != nil {
				return nil, err
			}
			adminIDs, err := s.accounts.AdminUserIDs(ctx, tx, ri.PublisherID)
			if err != nil {
				return nil, err
			}
			if err := s.notif.Enqueue(ctx, tx, adminIDs, "lead_returned", map[string]any{"lead_id": leadID}); err != nil {
				return nil, err
			}
		}
	}

	updated, err := s.repo.GetByID(ctx, tx, leadID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	_ = s.repo.attachCustomValues(ctx, updated)
	return updated, nil
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
	if err := s.repo.PlaceInPipeline(ctx, tx, leadID, target.BuyerID, target.BuyerPipelineID, firstStage, target.ID); err != nil {
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
