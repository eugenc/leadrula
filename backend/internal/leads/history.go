package leads

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/database"
)

const resolvedOwnerAccountSQL = `COALESCE(
  h.owner_account_id,
  (
    SELECT re.target_account_id
    FROM route_executions re
    WHERE re.lead_id = h.lead_id
      AND re.created_at <= h.created_at
      AND re.target_account_id IS NOT NULL
    ORDER BY re.created_at DESC, re.id DESC
    LIMIT 1
  ),
  l.publisher_id
)`

type stageHistoryParams struct {
	FromStage      *int64
	ToStage        int64
	OwnerAccountID int64
	UserID         int64
	ActionAt       *time.Time
	DisqReason     *int64
	Actor          HistoryActor
}

func (r *Repository) InsertStageHistory(ctx context.Context, q database.Querier, leadID int64, p stageHistoryParams) error {
	actorType := p.Actor.Type
	if actorType == "" {
		if p.UserID > 0 {
			actorType = "user"
		} else {
			actorType = "system"
		}
	}
	_, err := q.Exec(ctx,
		`INSERT INTO lead_stage_history(lead_id, from_stage_id, to_stage_id, owner_account_id, moved_by_user_id, action_at_captured, disqualification_reason_id, actor_type, actor_label)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		leadID, p.FromStage, p.ToStage, p.OwnerAccountID, p.UserID, p.ActionAt, p.DisqReason, actorType, nullIfEmpty(p.Actor.Label))
	return err
}

func (r *Repository) LeadHistory(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	collectors := []func(context.Context, int64) ([]LeadHistoryEntry, error){
		r.stageHistoryEntries,
		r.transferHistoryEntries,
		r.routeRunHistoryEntries,
		r.transactionHistoryEntries,
		r.disputeHistoryEntries,
		r.disputeMessageHistoryEntries,
		r.webhookHistoryEntries,
		r.outboundWebhookHistoryEntries,
		r.integrationHistoryEntries,
		r.changeLogHistoryEntries,
		r.createdHistoryEntry,
		r.noteHistoryEntries,
	}
	var out []LeadHistoryEntry
	for _, fn := range collectors {
		entries, err := fn(ctx, leadID)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	sortHistory(out)
	return dedupePipelinePlaced(dedupeLeadCreated(out)), nil
}

// dedupePipelinePlaced drops change_log pipeline_placed rows when a route_executions
// pipeline_placed row exists at the same second with the same summary (dual audit write).
func dedupePipelinePlaced(entries []LeadHistoryEntry) []LeadHistoryEntry {
	routeKeys := map[string]struct{}{}
	for _, e := range entries {
		if e.Kind != "pipeline_placed" || e.ActorType != "integration" {
			continue
		}
		key := pipelinePlacedDedupeKey(e)
		if key != "" {
			routeKeys[key] = struct{}{}
		}
	}
	if len(routeKeys) == 0 {
		return entries
	}
	out := make([]LeadHistoryEntry, 0, len(entries))
	for _, e := range entries {
		if e.Kind == "pipeline_placed" && e.ActorType == "route" {
			if _, dup := routeKeys[pipelinePlacedDedupeKey(e)]; dup {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

func pipelinePlacedDedupeKey(e LeadHistoryEntry) string {
	if e.Summary == "" {
		return ""
	}
	return e.CreatedAt.UTC().Truncate(time.Second).Format(time.RFC3339) + "|" + e.Summary
}

func dedupeLeadCreated(entries []LeadHistoryEntry) []LeadHistoryEntry {
	var keepID int64
	var keepAt time.Time
	n := 0
	for _, e := range entries {
		if e.Kind != "lead_created" {
			continue
		}
		n++
		if n == 1 || e.CreatedAt.Before(keepAt) || (e.CreatedAt.Equal(keepAt) && e.ID < keepID) {
			keepID = e.ID
			keepAt = e.CreatedAt
		}
	}
	if n <= 1 {
		return entries
	}
	out := make([]LeadHistoryEntry, 0, len(entries)-n+1)
	for _, e := range entries {
		if e.Kind == "lead_created" && e.ID != keepID {
			continue
		}
		out = append(out, e)
	}
	return out
}

func sortHistory(out []LeadHistoryEntry) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
}

func (r *Repository) stageHistoryEntries(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT h.id, h.from_stage_id, fs.name, h.to_stage_id, ts.name, u.full_name,
		        h.action_at_captured, dr.label, h.created_at, oa.name, oa.type,
		        `+resolvedOwnerAccountSQL+`, COALESCE(h.actor_type, 'user'), h.actor_label
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
		var actorType string
		var actorLabel *string
		if err := rows.Scan(&e.ID, &fromStageID, &e.FromStageName, &toStageID, &e.ToStageName,
			&e.MovedByName, &e.ActionAt, &e.DisqReason, &e.CreatedAt, &e.AccountName, &e.AccountType,
			&e.ownerAccountID, &actorType, &actorLabel); err != nil {
			return nil, err
		}
		e.Kind = "stage_change"
		e.ActorType = actorType
		if e.MovedByName != nil && *e.MovedByName != "" {
			e.ActorName = *e.MovedByName
		} else if actorLabel != nil && *actorLabel != "" {
			e.ActorName = *actorLabel
		} else {
			e.ActorName = "System"
			if e.ActorType == "" || e.ActorType == "user" {
				e.ActorType = "system"
			}
		}
		if fromStageID == nil && e.FromStageName == nil {
			from := "Created"
			e.FromStageName = &from
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) transferHistoryEntries(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	return r.queryRouteExecutions(ctx, leadID,
		`e.target_account_id IS NOT NULL AND (e.destination = 'contract' OR e.trigger_type IN ('return', 'redistribute'))`,
		"account_transfer")
}

func (r *Repository) routeRunHistoryEntries(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	return r.queryRouteExecutions(ctx, leadID,
		`NOT (e.target_account_id IS NOT NULL AND (e.destination = 'contract' OR e.trigger_type IN ('return', 'redistribute')))`,
		"")
}

func (r *Repository) queryRouteExecutions(ctx context.Context, leadID int64, extraWhere, forceKind string) ([]LeadHistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT e.id, e.created_at, e.trigger_type, e.route_name, e.trigger_label,
		        oa.name, ta.name, e.owner_account_id, e.target_account_id,
		        e.destination, e.status, e.error_message,
		        e.target_pipeline_name, e.target_stage_name, u.full_name
		 FROM route_executions e
		 LEFT JOIN accounts oa ON oa.id = e.owner_account_id
		 LEFT JOIN accounts ta ON ta.id = e.target_account_id
		 LEFT JOIN users u ON u.id = e.reviewed_by
		 WHERE e.lead_id = $1 AND `+extraWhere+`
		 ORDER BY e.created_at DESC`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeadHistoryEntry
	for rows.Next() {
		var e LeadHistoryEntry
		var triggerType, routeName, destination, status string
		var triggerLabel, errMsg, pipelineName, stageName, reviewerName *string
		var targetID *int64
		if err := rows.Scan(&e.ID, &e.CreatedAt, &triggerType, &routeName, &triggerLabel,
			&e.FromAccountName, &e.ToAccountName, &e.fromAccountID, &targetID,
			&destination, &status, &errMsg, &pipelineName, &stageName, &reviewerName); err != nil {
			return nil, err
		}
		if targetID != nil {
			e.toAccountID = *targetID
		}
		e.ownerAccountID = e.fromAccountID
		if forceKind != "" {
			e.Kind = forceKind
			kind := transferKindFromTrigger(triggerType)
			e.TransferKind = &kind
			e.ActorType = "route"
			e.ActorName = routeName
			if triggerLabel != nil && *triggerLabel != "" {
				e.TriggerLabel = triggerLabel
				e.ActorDetail = *triggerLabel
			}
			e.Summary = transferHeadline(e)
		} else if destination == "pipeline" {
			e.Kind = "pipeline_placed"
			e.ActorType = routeActorType(triggerType)
			e.ActorName = routeName
			if triggerLabel != nil && *triggerLabel != "" {
				e.ActorDetail = *triggerLabel
			}
			e.PipelineName = pipelineName
			e.StageName = stageName
			e.Summary = pipelinePlacedSummary(pipelineName, stageName)
		} else {
			e.Kind = "route_run"
			e.ActorType = routeActorType(triggerType)
			e.ActorName = routeName
			if triggerLabel != nil && *triggerLabel != "" {
				e.TriggerLabel = triggerLabel
				e.ActorDetail = *triggerLabel
			}
			e.Summary = fmt.Sprintf("Route · %s", routeName)
		}
		e.Status = status
		if errMsg != nil {
			e.ActorDetail = *errMsg
		}
		route := routeName
		e.RouteName = &route
		out = append(out, e)
	}
	return out, rows.Err()
}

func routeActorType(triggerType string) string {
	switch triggerType {
	case "webhook":
		return "webhook"
	case "integration":
		return "integration"
	default:
		return "route"
	}
}

func pipelinePlacedSummary(pipelineName, stageName *string) string {
	p, s := "Pipeline", "Stage"
	if pipelineName != nil && *pipelineName != "" {
		p = *pipelineName
	}
	if stageName != nil && *stageName != "" {
		s = *stageName
	}
	return fmt.Sprintf("Placed · %s → %s", p, s)
}

func transferHeadline(e LeadHistoryEntry) string {
	from, to := "Unknown", "Unknown"
	if e.FromAccountName != nil {
		from = *e.FromAccountName
	}
	if e.ToAccountName != nil {
		to = *e.ToAccountName
	}
	if e.TransferKind == nil {
		return fmt.Sprintf("%s → %s", from, to)
	}
	switch *e.TransferKind {
	case "returned":
		return fmt.Sprintf("Returned · %s → %s", from, to)
	case "redistributed":
		return fmt.Sprintf("Redistributed · %s → %s", from, to)
	default:
		return fmt.Sprintf("Sold · %s → %s", from, to)
	}
}

func (r *Repository) transactionHistoryEntries(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT t.id, t.created_at, t.type::text, ABS(t.amount::float8), t.description, t.buyer_id, a.name
		 FROM transactions t
		 JOIN accounts a ON a.id = t.buyer_id
		 WHERE t.lead_id = $1 AND t.type IN ('debit', 'credit', 'dispute_credit')
		 ORDER BY t.created_at DESC`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeadHistoryEntry
	for rows.Next() {
		var e LeadHistoryEntry
		var txnType, desc, buyerName string
		var amount float64
		if err := rows.Scan(&e.ID, &e.CreatedAt, &txnType, &amount, &desc, &e.buyerAccountID, &buyerName); err != nil {
			return nil, err
		}
		e.ActorType = "system"
		e.ActorName = "System"
		e.Amount = &amount
		switch txnType {
		case "debit":
			e.Kind = "purchase"
			e.Summary = fmt.Sprintf("Purchased · $%.2f", amount)
		case "dispute_credit", "credit":
			e.Kind = "refund"
			e.Summary = fmt.Sprintf("Refunded · $%.2f", amount)
		default:
			continue
		}
		if desc != "" {
			e.ActorDetail = desc
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) disputeHistoryEntries(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT d.id, d.created_at, d.status::text, d.reason, ABS(t.amount::float8), d.buyer_id,
		        ba.name, d.resolved_at, ru.full_name
		 FROM disputes d
		 JOIN transactions t ON t.id = d.transaction_id
		 JOIN accounts ba ON ba.id = d.buyer_id
		 LEFT JOIN users ru ON ru.id = d.resolved_by
		 WHERE t.lead_id = $1
		 ORDER BY d.created_at DESC`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeadHistoryEntry
	for rows.Next() {
		var disputeID int64
		var openedAt time.Time
		var status, reason, buyerName string
		var amount float64
		var buyerID int64
		var resolvedAt *time.Time
		var resolverName *string
		if err := rows.Scan(&disputeID, &openedAt, &status, &reason, &amount, &buyerID, &buyerName, &resolvedAt, &resolverName); err != nil {
			return nil, err
		}
		opened := LeadHistoryEntry{
			ID:             disputeID,
			Kind:           "dispute_opened",
			CreatedAt:      openedAt,
			ActorType:      "user",
			ActorName:      buyerName,
			Amount:         &amount,
			Summary:        fmt.Sprintf("Dispute opened · $%.2f", amount),
			ActorDetail:    reason,
			buyerAccountID: buyerID,
		}
		out = append(out, opened)
		if resolvedAt != nil {
			resolved := LeadHistoryEntry{
				ID:             disputeID*1_000_000 + 1,
				Kind:           "dispute_resolved",
				CreatedAt:      *resolvedAt,
				Status:         status,
				Amount:         &amount,
				buyerAccountID: buyerID,
			}
			if resolverName != nil && *resolverName != "" {
				resolved.ActorType = "user"
				resolved.ActorName = *resolverName
			} else {
				resolved.ActorType = "system"
				resolved.ActorName = "System"
			}
			if status == "accepted" {
				resolved.Summary = fmt.Sprintf("Dispute accepted · $%.2f refunded", amount)
			} else {
				resolved.Summary = "Dispute rejected"
			}
			out = append(out, resolved)
		}
	}
	return out, nil
}

func (r *Repository) disputeMessageHistoryEntries(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT m.id, m.created_at, m.author_party, COALESCE(u.full_name,''), m.kind, m.body, d.buyer_id,
		        COALESCE(string_agg(a.filename, ', '), '')
		 FROM dispute_messages m
		 JOIN disputes d ON d.id = m.dispute_id
		 JOIN transactions t ON t.id = d.transaction_id
		 LEFT JOIN users u ON u.id = m.user_id
		 LEFT JOIN dispute_message_attachments a ON a.message_id = m.id
		 WHERE COALESCE(d.lead_id, t.lead_id) = $1
		 GROUP BY m.id, m.created_at, m.author_party, u.full_name, m.kind, m.body, d.buyer_id
		 ORDER BY m.created_at`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeadHistoryEntry
	for rows.Next() {
		var id, buyerID int64
		var createdAt time.Time
		var party, authorName, kind, body, attachments string
		if err := rows.Scan(&id, &createdAt, &party, &authorName, &kind, &body, &buyerID, &attachments); err != nil {
			return nil, err
		}
		e := LeadHistoryEntry{
			ID:             id,
			Kind:           "dispute_message",
			CreatedAt:      createdAt,
			Summary:        body,
			ActorDetail:    attachments,
			buyerAccountID: buyerID,
		}
		if authorName != "" {
			e.ActorType = "user"
			e.ActorName = authorName
		} else {
			e.ActorType = "system"
			e.ActorName = strings.Title(party)
		}
		switch kind {
		case "open":
			e.Summary = "Dispute opened — " + body
		case "reject":
			if body == "" {
				e.Summary = "Dispute rejected"
			} else {
				e.Summary = "Dispute rejected — " + body
			}
		case "system":
			e.ActorType = "system"
			e.ActorName = "System"
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) webhookHistoryEntries(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT d.id, d.created_at, d.status::text, w.name, w.account_id, d.error_message
		 FROM webhook_deliveries d
		 JOIN webhooks w ON w.id = d.webhook_id
		 WHERE d.lead_id = $1
		 ORDER BY d.created_at DESC`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeadHistoryEntry
	for rows.Next() {
		var e LeadHistoryEntry
		var webhookName string
		var errMsg *string
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.Status, &webhookName, &e.ownerAccountID, &errMsg); err != nil {
			return nil, err
		}
		e.Kind = "webhook"
		e.ActorType = "webhook"
		e.ActorName = webhookName
		e.Summary = fmt.Sprintf("Inbound webhook · %s", webhookName)
		if errMsg != nil && *errMsg != "" {
			e.ActorDetail = *errMsg
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) outboundWebhookHistoryEntries(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT q.id, COALESCE(q.delivered_at, q.created_at), q.status::text, w.name, t.trigger_event::text, q.last_error, ic.account_id
		 FROM integration_delivery_queue q
		 JOIN webhook_outbound_triggers t ON t.id = q.webhook_trigger_id
		 JOIN webhooks w ON w.id = t.webhook_id
		 JOIN integration_connections ic ON ic.id = q.connection_id
		 WHERE q.lead_id = $1 AND q.webhook_trigger_id IS NOT NULL
		 ORDER BY q.created_at DESC`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeadHistoryEntry
	for rows.Next() {
		var e LeadHistoryEntry
		var webhookName, triggerEvent string
		var lastErr *string
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.Status, &webhookName, &triggerEvent, &lastErr, &e.ownerAccountID); err != nil {
			return nil, err
		}
		e.Kind = "outbound_webhook"
		e.ActorType = "webhook"
		e.ActorName = webhookName
		e.ActorDetail = triggerEvent
		e.Summary = fmt.Sprintf("Outbound webhook · %s", webhookName)
		if lastErr != nil && *lastErr != "" {
			e.ActorDetail = *lastErr
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) integrationHistoryEntries(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT q.id, COALESCE(q.delivered_at, q.created_at), q.status::text, ic.name, ip.slug, q.last_error, ic.account_id
		 FROM integration_delivery_queue q
		 JOIN integration_connections ic ON ic.id = q.connection_id
		 JOIN integration_providers ip ON ip.id = ic.provider_id
		 WHERE q.lead_id = $1 AND q.webhook_trigger_id IS NULL
		 ORDER BY q.created_at DESC`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeadHistoryEntry
	for rows.Next() {
		var e LeadHistoryEntry
		var connName, providerSlug string
		var lastErr *string
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.Status, &connName, &providerSlug, &lastErr, &e.ownerAccountID); err != nil {
			return nil, err
		}
		e.Kind = "integration"
		e.ActorType = "integration"
		e.ActorName = connName
		e.ActorDetail = providerSlug
		e.Summary = fmt.Sprintf("CRM sync · %s", connName)
		if lastErr != nil && *lastErr != "" {
			e.ActorDetail = *lastErr
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) changeLogHistoryEntries(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT c.id, c.created_at, c.change_kind, c.field_name, c.from_value, c.to_value,
		        c.actor_type, c.actor_label, u.full_name, c.owner_account_id
		 FROM lead_change_log c
		 LEFT JOIN users u ON u.id = c.actor_user_id
		 WHERE c.lead_id = $1
		 ORDER BY c.created_at DESC`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeadHistoryEntry
	for rows.Next() {
		var e LeadHistoryEntry
		var changeKind string
		var fieldName, fromVal, toVal, actorType, actorLabel *string
		var userName *string
		if err := rows.Scan(&e.ID, &e.CreatedAt, &changeKind, &fieldName, &fromVal, &toVal,
			&actorType, &actorLabel, &userName, &e.ownerAccountID); err != nil {
			return nil, err
		}
		e.Kind = changeLogKind(changeKind)
		if actorType != nil {
			e.ActorType = *actorType
		}
		if userName != nil && *userName != "" {
			e.ActorName = *userName
		} else if actorLabel != nil && *actorLabel != "" {
			e.ActorName = *actorLabel
		} else {
			e.ActorName = "System"
		}
		e.FieldName = fieldName
		e.FromValue = fromVal
		e.ToValue = toVal
		e.Summary = changeLogSummary(changeKind, fieldName, fromVal, toVal)
		out = append(out, e)
	}
	return out, rows.Err()
}

func changeLogKind(changeKind string) string {
	switch changeKind {
	case "field":
		return "field_change"
	case "assignee":
		return "assignee_change"
	case "tags":
		return "tag_change"
	case "action_at":
		return "calendar_event"
	case "status":
		return "status_change"
	case "pipeline_placed":
		return "pipeline_placed"
	case "pipeline_cleared":
		return "pipeline_cleared"
	case "follower_added":
		return "follower_added"
	case "follower_removed":
		return "follower_removed"
	case "lead_deleted":
		return "lead_deleted"
	case "imported":
		return "imported"
	case "lead_created":
		return "lead_created"
	case "preassigned_buyer":
		return "field_change"
	default:
		return "field_change"
	}
}

func changeLogSummary(changeKind string, fieldName, fromVal, toVal *string) string {
	field := "Field"
	if fieldName != nil && *fieldName != "" {
		field = *fieldName
	}
	from, to := formatLogVal(fromVal), formatLogVal(toVal)
	switch changeKind {
	case "assignee":
		return fmt.Sprintf("Assignee · %s → %s", from, to)
	case "tags":
		return fmt.Sprintf("Tags · %s → %s", from, to)
	case "action_at":
		return fmt.Sprintf("Action Date & Time · %s → %s", from, to)
	case "status":
		return fmt.Sprintf("Status · %s → %s", from, to)
	case "pipeline_placed":
		return fmt.Sprintf("Placed · %s → %s", from, to)
	case "pipeline_cleared":
		return fmt.Sprintf("Removed from pipeline · %s", from)
	case "follower_added":
		return fmt.Sprintf("Follower added · %s", to)
	case "follower_removed":
		return fmt.Sprintf("Follower removed · %s", from)
	case "lead_deleted":
		return "Lead deleted"
	case "imported":
		if toVal != nil && *toVal != "" {
			return fmt.Sprintf("Imported · %s", *toVal)
		}
		return "Imported"
	case "lead_created":
		if toVal != nil && *toVal != "" {
			return fmt.Sprintf("Created · %s", *toVal)
		}
		return "Created"
	default:
		return fmt.Sprintf("%s · %s → %s", field, from, to)
	}
}

func formatLogVal(v *string) string {
	if v == nil || *v == "" {
		return "None"
	}
	return *v
}

func (r *Repository) createdHistoryEntry(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	var hasLogged bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM lead_change_log WHERE lead_id=$1 AND change_kind='lead_created')`, leadID).
		Scan(&hasLogged); err == nil && hasLogged {
		return nil, nil
	}
	var createdAt time.Time
	var source *string
	var publisherID int64
	err := r.pool.QueryRow(ctx,
		`SELECT created_at, source, publisher_id FROM leads WHERE id = $1`, leadID).
		Scan(&createdAt, &source, &publisherID)
	if err != nil {
		return nil, err
	}
	e := LeadHistoryEntry{
		ID:             -leadID,
		Kind:           "lead_created",
		CreatedAt:      createdAt,
		ActorType:      "system",
		ActorName:      "System",
		ownerAccountID: publisherID,
	}
	if source != nil && *source != "" {
		e.Summary = fmt.Sprintf("Created · %s", *source)
		to := *source
		e.ToValue = &to
	} else {
		e.Summary = "Created"
	}
	return []LeadHistoryEntry{e}, nil
}

func (r *Repository) noteHistoryEntries(ctx context.Context, leadID int64) ([]LeadHistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT n.id, n.created_at, n.body, n.user_id, n.author_name, u.full_name
		 FROM lead_notes n
		 LEFT JOIN users u ON u.id = n.user_id
		 WHERE n.lead_id = $1`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeadHistoryEntry
	for rows.Next() {
		var e LeadHistoryEntry
		var body string
		var userID *int64
		var authorName, userName *string
		if err := rows.Scan(&e.ID, &e.CreatedAt, &body, &userID, &authorName, &userName); err != nil {
			return nil, err
		}
		e.Kind = "note_added"
		e.Summary = strings.TrimSpace(body)
		to := body
		e.ToValue = &to
		if userID == nil && authorName != nil && *authorName != "" {
			e.ActorType = "webhook"
			e.ActorName = *authorName
		} else {
			e.ActorType = "user"
			if authorName != nil && *authorName != "" {
				e.ActorName = *authorName
			} else if userName != nil && *userName != "" {
				e.ActorName = *userName
			} else {
				e.ActorName = "System"
			}
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

func buyerOwnershipWindow(entries []LeadHistoryEntry, buyerID int64) (start time.Time, end time.Time, ok bool) {
	var soldAt, returnedAt *time.Time
	for _, e := range entries {
		if e.Kind != "account_transfer" || e.TransferKind == nil {
			continue
		}
		switch *e.TransferKind {
		case "sold", "redistributed":
			if e.toAccountID == buyerID {
				t := e.CreatedAt
				if soldAt == nil || t.Before(*soldAt) {
					soldAt = &t
				}
			}
		case "returned":
			if e.fromAccountID == buyerID {
				t := e.CreatedAt
				if returnedAt == nil || t.After(*returnedAt) {
					returnedAt = &t
				}
			}
		}
	}
	if soldAt == nil {
		return time.Time{}, time.Time{}, false
	}
	start = *soldAt
	if returnedAt != nil && !returnedAt.Before(start) {
		end = *returnedAt
	}
	return start, end, true
}

func FilterLeadHistory(p *auth.Principal, entries []LeadHistoryEntry) []LeadHistoryEntry {
	if !buyerScopedView(p) {
		return entries
	}
	start, end, hasWindow := buyerOwnershipWindow(entries, p.AccountID)
	var out []LeadHistoryEntry
	for _, e := range entries {
		if e.Kind == "lead_created" {
			continue
		}
		if hasWindow {
			if e.CreatedAt.Before(start) {
				continue
			}
			if !end.IsZero() && e.CreatedAt.After(end) {
				continue
			}
		}
		if includeHistoryForBuyer(p.AccountID, e) {
			out = append(out, e)
		}
	}
	return out
}

func includeHistoryForBuyer(buyerID int64, e LeadHistoryEntry) bool {
	switch e.Kind {
	case "stage_change", "field_change", "assignee_change", "tag_change", "calendar_event",
		"status_change", "pipeline_cleared", "follower_added", "follower_removed", "imported":
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
	case "purchase", "refund", "dispute_opened", "dispute_resolved", "dispute_message":
		return e.buyerAccountID == buyerID
	case "pipeline_placed":
		return e.ownerAccountID == buyerID || e.toAccountID == buyerID
	case "route_run", "webhook", "outbound_webhook", "integration":
		return e.ownerAccountID == buyerID || e.toAccountID == buyerID
	case "note_added":
		return true
	case "lead_deleted":
		return e.ownerAccountID == buyerID
	}
	return false
}

func LogChangesFromUpdate(ctx context.Context, q database.Querier, repo *Repository, leadID, ownerAccountID int64, actor HistoryActor, before, after *Lead, in leadUpdateInput, fieldNames map[string]string) error {
	changes := diffLeadUpdate(before, after, in, fieldNames)
	for _, c := range changes {
		kind := "field"
		switch c.Field {
		case "Assignee":
			kind = "assignee"
		case "Tags":
			kind = "tags"
		case "Pre-assigned buyer":
			kind = "preassigned_buyer"
		}
		if err := repo.InsertChangeLog(ctx, q, leadID, ownerAccountID, actor, kind, c.Field, c.From, c.To); err != nil {
			return err
		}
	}
	return nil
}

func LogActionAtChange(ctx context.Context, q database.Querier, repo *Repository, leadID, ownerAccountID int64, actor HistoryActor, from, to *time.Time) error {
	changes := actionAtChange(from, to)
	for _, c := range changes {
		if err := repo.InsertChangeLog(ctx, q, leadID, ownerAccountID, actor, "action_at", c.Field, c.From, c.To); err != nil {
			return err
		}
	}
	return nil
}

func userDisplayName(ctx context.Context, q database.Querier, userID int64) string {
	if userID == 0 {
		return ""
	}
	var name string
	_ = q.QueryRow(ctx, `SELECT full_name FROM users WHERE id=$1`, userID).Scan(&name)
	return strings.TrimSpace(name)
}
