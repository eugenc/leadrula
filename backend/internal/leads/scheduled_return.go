package leads

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/echayko/leadrula/backend/internal/billing"
	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/jackc/pgx/v5"
)

type scheduledReturnRow struct {
	ID            int64
	LeadID        int64
	ContractID    int64
	ReturnRuleID  int64
	BuyerStageID  int64
	ReturnStageID int64
	ExecuteAt     time.Time
}

func scheduleReturnLabel(executeAt time.Time, tz string) string {
	loc, err := time.LoadLocation(contracts.NormalizeTimezoneForDisplay(tz))
	if err != nil {
		return executeAt.UTC().Format("Jan 2 3:04 PM")
	}
	return executeAt.In(loc).Format("Mon Jan 2 3:04 PM")
}

func ScheduleReturn(ctx context.Context, q database.Querier, repo *Repository, leadID, contractID int64, buyerStageID int64, match *contracts.ReturnMatch) error {
	rule := contracts.ReturnRule{
		ReturnScheduleMode: match.ReturnScheduleMode,
		ReturnDelaySeconds: match.ReturnDelaySeconds,
		ReturnTime:         match.ReturnTime,
		ReturnWeekdays:     match.ReturnWeekdays,
	}
	executeAt, err := contracts.NextReturnExecuteAt(time.Now().UTC(), match.ScheduleTimezone, rule)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx,
		`UPDATE scheduled_lead_returns SET status = 'cancelled'
		 WHERE lead_id = $1 AND status = 'pending'`, leadID)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx,
		`INSERT INTO scheduled_lead_returns(
		   lead_id, contract_id, return_rule_id, buyer_stage_id, return_stage_id, execute_at, status)
		 VALUES ($1,$2,$3,$4,$5,$6,'pending')`,
		leadID, contractID, match.RuleID, buyerStageID, match.ReturnInfo.ReturnStageID, executeAt)
	if err != nil {
		return err
	}
	var ownerAccountID int64
	if err := q.QueryRow(ctx, `SELECT owner_account_id FROM leads WHERE id = $1`, leadID).Scan(&ownerAccountID); err != nil {
		return err
	}
	label := scheduleReturnLabel(executeAt, match.ScheduleTimezone)
	return repo.InsertChangeLog(ctx, q, leadID, ownerAccountID, ActorSystem("Return schedule"), "return_scheduled", "", "", label)
}

func CancelPendingReturns(ctx context.Context, q database.Querier, leadID int64) error {
	_, err := q.Exec(ctx,
		`UPDATE scheduled_lead_returns SET status = 'cancelled'
		 WHERE lead_id = $1 AND status = 'pending'`, leadID)
	return err
}

func CancelPendingReturnsIfStageChanged(ctx context.Context, q database.Querier, repo *Repository, leadID int64, newStageID *int64) error {
	var rows pgx.Rows
	var err error
	if newStageID == nil {
		rows, err = q.Query(ctx,
			`UPDATE scheduled_lead_returns SET status = 'cancelled', updated_at = now()
			 WHERE lead_id = $1 AND status = 'pending'
			 RETURNING execute_at, contract_id`, leadID)
	} else {
		rows, err = q.Query(ctx,
			`UPDATE scheduled_lead_returns SET status = 'cancelled', updated_at = now()
			 WHERE lead_id = $1 AND status = 'pending' AND buyer_stage_id IS DISTINCT FROM $2
			 RETURNING execute_at, contract_id`, leadID, *newStageID)
	}
	if err != nil {
		return err
	}
	defer rows.Close()

	type cancelledReturn struct {
		executeAt  time.Time
		contractID int64
	}
	var cancelled []cancelledReturn
	for rows.Next() {
		var row cancelledReturn
		if err := rows.Scan(&row.executeAt, &row.contractID); err != nil {
			return err
		}
		cancelled = append(cancelled, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(cancelled) == 0 {
		return nil
	}

	var ownerAccountID int64
	if err := q.QueryRow(ctx, `SELECT owner_account_id FROM leads WHERE id = $1`, leadID).Scan(&ownerAccountID); err != nil {
		return err
	}
	actor := ActorSystem("Return schedule")
	for _, row := range cancelled {
		var tz string
		if err := q.QueryRow(ctx, `SELECT schedule_timezone FROM contracts WHERE id = $1`, row.contractID).Scan(&tz); err != nil {
			return err
		}
		label := scheduleReturnLabel(row.executeAt, tz)
		if err := repo.InsertChangeLog(ctx, q, leadID, ownerAccountID, actor, "return_cancelled", "", "", label); err != nil {
			return err
		}
	}
	return nil
}

func ExecuteScheduledReturn(ctx context.Context, q database.Querier, deps ReturnDeps, scheduledID int64) (*ReturnOutcome, error) {
	row, err := loadScheduledReturn(ctx, q, scheduledID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return &ReturnOutcome{}, nil
	}
	lead, err := deps.Repo.GetByID(ctx, q, row.LeadID)
	if err != nil {
		return nil, err
	}
	if lead.Status == "disputed" || lead.StageID == nil || *lead.StageID != row.BuyerStageID {
		_, _ = q.Exec(ctx, `UPDATE scheduled_lead_returns SET status = 'cancelled' WHERE id = $1`, scheduledID)
		return &ReturnOutcome{Lead: lead}, nil
	}
	contractID, err := resolveLeadContractID(ctx, q, lead)
	if err != nil {
		return nil, err
	}
	if contractID == nil {
		_, _ = q.Exec(ctx, `UPDATE scheduled_lead_returns SET status = 'cancelled' WHERE id = $1`, scheduledID)
		return &ReturnOutcome{Lead: lead}, nil
	}
	match, err := contracts.FindReturnMatch(ctx, q, *contractID, lead.OwnerAccountID, *lead.StageID)
	if err != nil {
		return nil, err
	}
	if match == nil || match.RuleID != row.ReturnRuleID {
		_, _ = q.Exec(ctx, `UPDATE scheduled_lead_returns SET status = 'cancelled' WHERE id = $1`, scheduledID)
		return &ReturnOutcome{Lead: lead}, nil
	}
	out, err := executeReturnMove(ctx, q, deps, row.LeadID, &match.ReturnInfo, *contractID, lead)
	if err != nil {
		return nil, err
	}
	_, _ = q.Exec(ctx, `UPDATE scheduled_lead_returns SET status = 'completed' WHERE id = $1`, scheduledID)
	return out, nil
}

func loadScheduledReturn(ctx context.Context, q database.Querier, scheduledID int64) (*scheduledReturnRow, error) {
	var row scheduledReturnRow
	err := q.QueryRow(ctx,
		`SELECT id, lead_id, contract_id, return_rule_id, buyer_stage_id, return_stage_id, execute_at
		 FROM scheduled_lead_returns
		 WHERE id = $1 AND status IN ('pending', 'processing')`, scheduledID).
		Scan(&row.ID, &row.LeadID, &row.ContractID, &row.ReturnRuleID, &row.BuyerStageID, &row.ReturnStageID, &row.ExecuteAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func executeReturnMove(ctx context.Context, q database.Querier, deps ReturnDeps, leadID int64, returnInfo *contracts.ReturnInfo, contractID int64, lead *Lead) (*ReturnOutcome, error) {
	if err := contracts.ValidateReturnDestination(ctx, q, returnInfo.SourcePipelineID, returnInfo.ReturnStageID); err != nil {
		return nil, err
	}
	refunded, err := billing.ReturnCreditExists(ctx, q, lead.OwnerAccountID, leadID, contractID)
	if err != nil {
		return nil, fmt.Errorf("return rule check refund: %w", err)
	}
	if !refunded {
		amt, err := billing.DistributeDebitAmount(ctx, q, lead.OwnerAccountID, leadID, contractID)
		if err != nil {
			return nil, fmt.Errorf("return rule lookup debit: %w", err)
		}
		if amt > 0 {
			if err := billing.Credit(ctx, q, lead.OwnerAccountID, amt, leadID, contractID, "lead returned"); err != nil {
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
