package leads

import (
	"context"
	"fmt"
	"log"

	"github.com/echayko/leadrula/backend/internal/billing"
	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/notifications"
)

// ReturnDeps holds collaborators for contract return evaluation.
type ReturnDeps struct {
	Repo  *Repository
	Notif *notifications.Service
}

// ReturnOutcome is the result of TryReturnLead.
type ReturnOutcome struct {
	Lead      *Lead
	Emails    []notifications.EmailJob
	Returned  bool
	Scheduled bool
}

// resolveLeadContractID returns the lead's contract_id, or the contract from its latest distribute debit.
func resolveLeadContractID(ctx context.Context, q database.Querier, lead *Lead) (*int64, error) {
	if lead.ContractID != nil && *lead.ContractID != 0 {
		return lead.ContractID, nil
	}
	if lead.OwnerAccountID == lead.PublisherID {
		return nil, nil
	}
	cid, err := billing.LatestDistributeContractID(ctx, q, lead.OwnerAccountID, lead.ID)
	if err != nil {
		return nil, err
	}
	if cid == 0 {
		return nil, nil
	}
	return &cid, nil
}

func backfillSoldLeadContract(ctx context.Context, q database.Querier, leadID, contractID int64) error {
	_, err := q.Exec(ctx,
		`UPDATE leads SET contract_id=$2, status='distributed'::lead_status
		 WHERE id=$1 AND contract_id IS NULL`,
		leadID, contractID)
	return err
}

// TryReturnLead checks whether the lead's current stage triggers a contract return
// and moves it back to the publisher when matched (or schedules a delayed return).
func TryReturnLead(ctx context.Context, q database.Querier, deps ReturnDeps, leadID int64) (*ReturnOutcome, error) {
	lead, err := deps.Repo.GetByID(ctx, q, leadID)
	if err != nil {
		return nil, err
	}
	if lead.Status == "disputed" {
		log.Printf("TryReturnLead lead=%d: skip disputed", leadID)
		return &ReturnOutcome{Lead: lead}, nil
	}
	contractID, err := resolveLeadContractID(ctx, q, lead)
	if err != nil {
		return nil, err
	}
	if contractID == nil || lead.StageID == nil {
		if contractID == nil {
			log.Printf("TryReturnLead lead=%d: skip no contract", leadID)
		} else {
			log.Printf("TryReturnLead lead=%d contract=%d: skip no stage", leadID, *contractID)
		}
		return &ReturnOutcome{Lead: lead}, nil
	}
	if lead.ContractID == nil {
		if err := backfillSoldLeadContract(ctx, q, leadID, *contractID); err != nil {
			return nil, err
		}
		lead.ContractID = contractID
	}

	match, err := contracts.FindReturnMatch(ctx, q, *lead.ContractID, lead.OwnerAccountID, *lead.StageID)
	if err != nil {
		return nil, err
	}
	if match == nil {
		log.Printf("TryReturnLead lead=%d contract=%d stage=%d: no matching return rule", leadID, *lead.ContractID, *lead.StageID)
		return &ReturnOutcome{Lead: lead}, nil
	}

	mode := match.ReturnScheduleMode
	if mode == "" {
		mode = contracts.ReturnScheduleImmediate
	}
	if mode != contracts.ReturnScheduleImmediate {
		if err := ScheduleReturn(ctx, q, deps.Repo, leadID, *contractID, *lead.StageID, match); err != nil {
			return nil, fmt.Errorf("schedule return: %w", err)
		}
		log.Printf("TryReturnLead lead=%d contract=%d: scheduled return mode=%s", leadID, *contractID, mode)
		return &ReturnOutcome{Lead: lead, Scheduled: true}, nil
	}

	return executeReturnMove(ctx, q, deps, leadID, &match.ReturnInfo, *contractID, lead)
}
