package leads

import (
	"context"
	"fmt"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/database"
)

type HistoryActor struct {
	Type   string
	UserID int64
	Label  string
}

func ActorFromPrincipal(p *auth.Principal) HistoryActor {
	if p == nil {
		return HistoryActor{Type: "system", Label: "System"}
	}
	a := HistoryActor{Type: "user", UserID: p.UserID}
	if p.Impersonator != nil {
		a.Label = fmt.Sprintf("via %s", p.Impersonator.AccountPublicID)
	}
	return a
}

func ActorSystem(label string) HistoryActor {
	if label == "" {
		label = "System"
	}
	return HistoryActor{Type: "system", Label: label}
}

func ActorWebhook(name string) HistoryActor {
	return HistoryActor{Type: "webhook", Label: name}
}

func ActorRoute(name string) HistoryActor {
	return HistoryActor{Type: "route", Label: name}
}

func (r *Repository) InsertChangeLog(ctx context.Context, q database.Querier, leadID, ownerAccountID int64, actor HistoryActor, changeKind, fieldName, fromValue, toValue string) error {
	_, err := q.Exec(ctx,
		`INSERT INTO lead_change_log(lead_id, owner_account_id, actor_type, actor_user_id, actor_label, change_kind, field_name, from_value, to_value)
		 VALUES ($1,$2,$3,NULLIF($4,0),$5,$6,NULLIF($7,''),$8,$9)`,
		leadID, ownerAccountID, actor.Type, actor.UserID, nullIfEmpty(actor.Label), changeKind, fieldName, fromValue, toValue)
	return err
}

func (r *Repository) SetStatusWithLog(ctx context.Context, q database.Querier, leadID int64, actor HistoryActor, toStatus string) error {
	var fromStatus string
	var ownerAccountID int64
	if err := q.QueryRow(ctx, `SELECT status::text, owner_account_id FROM leads WHERE id=$1`, leadID).Scan(&fromStatus, &ownerAccountID); err != nil {
		return err
	}
	if fromStatus == toStatus {
		return r.SetStatus(ctx, q, leadID, toStatus)
	}
	if err := r.InsertChangeLog(ctx, q, leadID, ownerAccountID, actor, "status", "Status", fromStatus, toStatus); err != nil {
		return err
	}
	return r.SetStatus(ctx, q, leadID, toStatus)
}

func (r *Repository) LogPipelinePlacement(ctx context.Context, q database.Querier, leadID int64, actor HistoryActor, pipelineID, stageID int64) error {
	var ownerAccountID int64
	var pipelineName, stageName string
	if err := q.QueryRow(ctx,
		`SELECT l.owner_account_id, p.name, ps.name
		 FROM leads l
		 JOIN pipelines p ON p.id = $2
		 JOIN pipeline_stages ps ON ps.id = $3
		 WHERE l.id = $1`, leadID, pipelineID, stageID).
		Scan(&ownerAccountID, &pipelineName, &stageName); err != nil {
		return err
	}
	return r.InsertChangeLog(ctx, q, leadID, ownerAccountID, actor, "pipeline_placed", "Pipeline", pipelineName, stageName)
}

func (r *Repository) LogLeadCreated(ctx context.Context, q database.Querier, leadID, ownerAccountID int64, actor HistoryActor, source string) error {
	return r.InsertChangeLog(ctx, q, leadID, ownerAccountID, actor, "lead_created", "Created", "", source)
}

func (r *Repository) LogCRMSyncSkipped(ctx context.Context, q database.Querier, leadID, ownerAccountID int64, actor HistoryActor, reason string) error {
	if reason == "" {
		reason = "Skipped — no integration linked"
	}
	return r.InsertChangeLog(ctx, q, leadID, ownerAccountID, actor, "crm_sync_skipped", "CRM sync", "", reason)
}

func (r *Repository) BuyerHasActiveCRMConnection(ctx context.Context, q database.Querier, buyerID int64) (bool, error) {
	var exists bool
	err := q.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM integration_connections ic
		   JOIN integration_providers p ON p.id = ic.provider_id
		   WHERE ic.account_id = $1 AND ic.status = 'active'
		     AND p.slug IN ('pipedrive', 'ghl', 'hubspot', 'zoho_crm', 'salesforce', 'sunbase')
		 )`, buyerID).Scan(&exists)
	return exists, err
}
