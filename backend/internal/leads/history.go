package leads

import (
	"context"
	"sort"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/database"
)

const resolvedOwnerAccountSQL = `COALESCE(
  h.owner_account_id,
  (
    SELECT CASE
      WHEN re.trigger_type = 'return' THEN re.target_account_id
      ELSE re.target_account_id
    END
    FROM route_executions re
    WHERE re.lead_id = h.lead_id
      AND re.created_at <= h.created_at
      AND re.target_account_id IS NOT NULL
    ORDER BY re.created_at DESC, re.id DESC
    LIMIT 1
  ),
  l.publisher_id
)`

func (r *Repository) InsertStageHistory(ctx context.Context, q database.Querier, leadID int64, fromStage *int64, toStage, ownerAccountID, userID int64, actionAt *time.Time, disqReason *int64) error {
	_, err := q.Exec(ctx,
		`INSERT INTO lead_stage_history(lead_id, from_stage_id, to_stage_id, owner_account_id, moved_by_user_id, action_at_captured, disqualification_reason_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		leadID, fromStage, toStage, ownerAccountID, userID, actionAt, disqReason)
	return err
}

func (r *Repository) LeadHistory(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	stages, err := r.stageHistoryEntries(ctx, leadID)
	if err != nil {
		return nil, err
	}
	transfers, err := r.transferHistoryEntries(ctx, leadID)
	if err != nil {
		return nil, err
	}
	out := append(stages, transfers...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			if out[i].Kind != out[j].Kind {
				return out[i].Kind == "account_transfer"
			}
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (r *Repository) stageHistoryEntries(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT h.id, h.from_stage_id, fs.name, h.to_stage_id, ts.name, u.full_name,
		        h.action_at_captured, dr.label, h.created_at, oa.name, oa.type,
		        `+resolvedOwnerAccountSQL+`
		 FROM lead_stage_history h
		 JOIN leads l ON l.id = h.lead_id
		 LEFT JOIN pipeline_stages fs ON fs.id = h.from_stage_id
		 LEFT JOIN pipeline_stages ts ON ts.id = h.to_stage_id
		 LEFT JOIN users u ON u.id = h.moved_by_user_id
		 LEFT JOIN disqualification_reasons dr ON dr.id = h.disqualification_reason_id
		 LEFT JOIN accounts oa ON oa.id = `+resolvedOwnerAccountSQL+`
		 WHERE h.lead_id = $1`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeadHistoryEntry
	for rows.Next() {
		var e LeadHistoryEntry
		var fromStageID *int64
		var toStageID int64
		if err := rows.Scan(&e.ID, &fromStageID, &e.FromStageName, &toStageID, &e.ToStageName,
			&e.MovedByName, &e.ActionAt, &e.DisqReason, &e.CreatedAt, &e.AccountName, &e.AccountType,
			&e.ownerAccountID); err != nil {
			return nil, err
		}
		e.Kind = "stage_change"
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) transferHistoryEntries(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT e.id, e.created_at, e.trigger_type, e.route_name, e.trigger_label,
		        oa.name, ta.name, e.owner_account_id, e.target_account_id
		 FROM route_executions e
		 LEFT JOIN accounts oa ON oa.id = e.owner_account_id
		 LEFT JOIN accounts ta ON ta.id = e.target_account_id
		 WHERE e.lead_id = $1
		   AND e.target_account_id IS NOT NULL
		   AND (e.destination = 'contract' OR e.trigger_type IN ('return', 'redistribute'))
		 ORDER BY e.created_at DESC`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeadHistoryEntry
	for rows.Next() {
		var e LeadHistoryEntry
		var triggerType, routeName string
		var triggerLabel *string
		var targetID *int64
		if err := rows.Scan(&e.ID, &e.CreatedAt, &triggerType, &routeName, &triggerLabel,
			&e.FromAccountName, &e.ToAccountName, &e.fromAccountID, &targetID); err != nil {
			return nil, err
		}
		if targetID != nil {
			e.toAccountID = *targetID
		}
		e.Kind = "account_transfer"
		kind := transferKindFromTrigger(triggerType)
		e.TransferKind = &kind
		if triggerLabel != nil && *triggerLabel != "" {
			e.TriggerLabel = triggerLabel
		} else if routeName != "" {
			e.TriggerLabel = &routeName
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func transferKindFromTrigger(triggerType string) string {
	switch triggerType {
	case "return":
		return "returned"
	case "redistribute":
		return "redistributed"
	default:
		return "sold"
	}
}

func buyerScopedView(p *auth.Principal) bool {
	if p == nil || p.AccountType != "buyer" {
		return false
	}
	_, oversight := p.OversightPublisherID()
	return !oversight
}

func FilterLeadHistory(p *auth.Principal, entries []LeadHistoryEntry) []LeadHistoryEntry {
	if !buyerScopedView(p) {
		return entries
	}
	var out []LeadHistoryEntry
	for _, e := range entries {
		if includeHistoryForBuyer(p.AccountID, e) {
			out = append(out, e)
		}
	}
	return out
}

func includeHistoryForBuyer(buyerID int64, e LeadHistoryEntry) bool {
	switch e.Kind {
	case "stage_change":
		return e.ownerAccountID == buyerID
	case "account_transfer":
		if e.TransferKind == nil {
			return false
		}
		switch *e.TransferKind {
		case "sold", "redistributed":
			return e.toAccountID == buyerID
		case "returned":
			return e.fromAccountID == buyerID
		}
	}
	return false
}
