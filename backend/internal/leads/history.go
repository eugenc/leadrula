package leads

import (
	"context"
	"sort"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/database"
)

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
		        h.action_at_captured, dr.label, h.created_at, oa.name, oa.type
		 FROM lead_stage_history h
		 LEFT JOIN pipeline_stages fs ON fs.id = h.from_stage_id
		 LEFT JOIN pipeline_stages ts ON ts.id = h.to_stage_id
		 LEFT JOIN users u ON u.id = h.moved_by_user_id
		 LEFT JOIN disqualification_reasons dr ON dr.id = h.disqualification_reason_id
		 LEFT JOIN accounts oa ON oa.id = h.owner_account_id
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
			&e.MovedByName, &e.ActionAt, &e.DisqReason, &e.CreatedAt, &e.AccountName, &e.AccountType); err != nil {
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
		        oa.name, ta.name
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
		if err := rows.Scan(&e.ID, &e.CreatedAt, &triggerType, &routeName, &triggerLabel,
			&e.FromAccountName, &e.ToAccountName); err != nil {
			return nil, err
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

const buyerPublisherLabel = "Publisher"

func FilterLeadHistory(p *auth.Principal, entries []LeadHistoryEntry) []LeadHistoryEntry {
	if p == nil || p.AccountType != "buyer" {
		return entries
	}
	out := make([]LeadHistoryEntry, len(entries))
	copy(out, entries)
	for i := range out {
		switch out[i].Kind {
		case "stage_change":
			if out[i].AccountType != nil && *out[i].AccountType == "publisher" {
				label := buyerPublisherLabel
				out[i].AccountName = &label
			}
		case "account_transfer":
			sanitizeTransferForBuyer(&out[i])
		}
	}
	return out
}

func sanitizeTransferForBuyer(e *LeadHistoryEntry) {
	if e.TransferKind == nil {
		return
	}
	switch *e.TransferKind {
	case "returned":
		label := buyerPublisherLabel
		e.ToAccountName = &label
	case "sold", "redistributed":
		label := buyerPublisherLabel
		e.FromAccountName = &label
	}
}
