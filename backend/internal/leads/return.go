package leads

import (
	"context"
	"fmt"

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
	Lead     *Lead
	Emails   []notifications.EmailJob
	Returned bool
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
// and moves it back to the publisher when matched.
func TryReturnLead(ctx context.Context, q database.Querier, deps ReturnDeps, leadID int64) (*ReturnOutcome, error) {
	lead, err := deps.Repo.GetByID(ctx, q, leadID)
	if err != nil {
		return nil, err
	}
	contractID, err := resolveLeadContractID(ctx, q, lead)
	if err != nil {
		return nil, err
	}
	if contractID == nil || lead.StageID == nil {
		return &ReturnOutcome{Lead: lead}, nil
	}
	if lead.ContractID == nil {
		if err := backfillSoldLeadContract(ctx, q, leadID, *contractID); err != nil {
			return nil, err
		}
		lead.ContractID = contractID
	}

	returnInfo, err := contracts.FindReturnRule(ctx, q, *lead.ContractID, lead.OwnerAccountID, *lead.StageID)
	if err != nil {
		return nil, err
	}
	if returnInfo == nil {
		return &ReturnOutcome{Lead: lead}, nil
	}

	if err := contracts.ValidateReturnDestination(ctx, q, returnInfo.SourcePipelineID, returnInfo.ReturnStageID); err != nil {
		return nil, err
	}
	refunded, err := billing.ReturnCreditExists(ctx, q, lead.OwnerAccountID, leadID, *lead.ContractID)
	if err != nil {
		return nil, fmt.Errorf("return rule check refund: %w", err)
	}
	if !refunded {
		amt, err := billing.DistributeDebitAmount(ctx, q, lead.OwnerAccountID, leadID, *lead.ContractID)
		if err != nil {
			return nil, fmt.Errorf("return rule lookup debit: %w", err)
		}
		if amt > 0 {
			if err := billing.Credit(ctx, q, lead.OwnerAccountID, amt, leadID, *lead.ContractID, "lead returned"); err != nil {
				return nil, fmt.Errorf("return rule refund buyer: %w", err)
			}
		}
	}
	if err := contracts.RecordEarningReturn(ctx, q, leadID, lead.ContractID); err != nil {
		return nil, fmt.Errorf("return rule record earning: %w", err)
	}
	if err := deps.Repo.MoveToPublisher(ctx, q, leadID, returnInfo.PublisherID, returnInfo.SourcePipelineID, returnInfo.ReturnStageID); err != nil {
		return nil, fmt.Errorf("return rule move to publisher: %w", err)
	}
	buyerID := lead.OwnerAccountID
	pubID := returnInfo.PublisherID
	if err := RecordRouteExecution(ctx, q, RecordRouteExecutionParams{
		RouteName:       "Lead returned",
		LeadID:          leadID,
		OwnerAccountID:  buyerID,
		TargetAccountID: &pubID,
		Destination:     "return",
		TriggerType:     "return",
	}); err != nil {
		return nil, fmt.Errorf("return rule record execution: %w", err)
	}
	returned, err := deps.Repo.GetByID(ctx, q, leadID)
	if err != nil {
		return nil, fmt.Errorf("return rule reload lead: %w", err)
	}
	emails, err := deps.Notif.Deliver(ctx, q, notifications.DeliverParams{
		AccountID: returnInfo.PublisherID,
		UserIDs:   notifications.AssigneeIDs(returned.AssignedUserID),
		EventType: "lead_returned",
		Payload:   map[string]any{"lead_id": leadID},
	})
	if err != nil {
		return nil, fmt.Errorf("return rule notify: %w", err)
	}
	return &ReturnOutcome{Lead: returned, Emails: emails, Returned: true}, nil
}
