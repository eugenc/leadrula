package leads

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/billing"
	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/internal/routing"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

// IntegrationEnqueuer enqueues outbound integration deliveries after routing.
type IntegrationEnqueuer interface {
	EnqueueDelivery(ctx context.Context, routeID, leadID int64, payloadJSON []byte) error
}

// PipelineContext carries the pipeline/stage snapshot that outbound webhook triggers receive.
type PipelineContext struct {
	PipelineID    *int64
	PipelineName  *string
	StageID       *int64
	StageName     *string
	PrevStageID   *int64
	PrevStageName *string
}

// WebhookFirer fires outbound webhook events after lead/pipeline mutations.
// The event string mirrors the outbound_trigger_event enum values.
type WebhookFirer interface {
	FireOutbound(ctx context.Context, accountID int64, event string, lead *Lead, pctx PipelineContext)
}

// RouteApplyDeps holds collaborators for ApplyRoute.
type RouteApplyDeps struct {
	Repo          *Repository
	Accounts      *accounts.Repository
	Notif         *notifications.Service
	Integrations  IntegrationEnqueuer
}

// ApplyRoute moves a lead according to route destination and delivery.
func ApplyRoute(ctx context.Context, q database.Querier, deps RouteApplyDeps, route *routing.Route, publisherID, leadID int64) error {
	if route.Destination == "publisher" {
		return applyPublisherRoute(ctx, q, deps.Repo, route, publisherID, leadID)
	}
	return applyBuyerRoute(ctx, q, deps, route, leadID)
}

func applyPublisherRoute(ctx context.Context, q database.Querier, repo *Repository, route *routing.Route, publisherID, leadID int64) error {
	if route.Delivery == "leads" {
		return repo.SetStatus(ctx, q, leadID, "review")
	}
	if route.TargetPipelineID == nil || route.TargetStageID == nil {
		return httpx.BusinessRule("route missing publisher pipeline target")
	}
	if err := repo.PlaceInPipeline(ctx, q, leadID, publisherID, *route.TargetPipelineID, *route.TargetStageID, nil); err != nil {
		return err
	}
	return repo.SetStatus(ctx, q, leadID, "distributed")
}

func applyBuyerRoute(ctx context.Context, q database.Querier, deps RouteApplyDeps, route *routing.Route, leadID int64) error {
	if route.ContractID == nil {
		return httpx.BusinessRule("route missing contract")
	}
	target, err := contracts.GetTarget(ctx, q, *route.ContractID, route.CompensationID)
	if err != nil {
		return err
	}

	lead, err := deps.Repo.GetByID(ctx, q, leadID)
	if err != nil {
		return err
	}
	if err := LoadCustomValues(ctx, q, lead); err != nil {
		return err
	}
	maps, err := routing.RouteFieldMap(ctx, q, route.ID)
	if err != nil {
		return err
	}
	if err := ApplyRouteFieldMap(ctx, q, deps.Repo, lead, maps); err != nil {
		return err
	}

	contractID := target.ID
	if route.Delivery == "leads" {
		if err := deps.Repo.TransferOwner(ctx, q, leadID, target.BuyerID, &contractID); err != nil {
			return err
		}
		return deps.Repo.SetStatus(ctx, q, leadID, "review")
	}

	var destStage int64
	if route.TargetStageID != nil && *route.TargetStageID != 0 {
		destStage = *route.TargetStageID
	} else {
		if err := q.QueryRow(ctx,
			`SELECT id FROM pipeline_stages WHERE pipeline_id=$1 ORDER BY position, id LIMIT 1`,
			target.BuyerPipelineID).Scan(&destStage); err != nil {
			return httpx.BusinessRule("target pipeline has no stages")
		}
	}
	if err := deps.Repo.PlaceInPipeline(ctx, q, leadID, target.BuyerID, target.BuyerPipelineID, destStage, &contractID); err != nil {
		return err
	}
	if err := contracts.CheckCap(ctx, q, target.ID, target.CompensationID); err != nil {
		return err
	}
	if err := billing.Debit(ctx, q, target.BuyerID, target.RatePerLead, leadID, target.ID, "lead routed: "+route.Name); err != nil {
		return err
	}
	if err := deps.Repo.SetStatus(ctx, q, leadID, "distributed"); err != nil {
		return err
	}
	adminIDs, err := deps.Accounts.AdminUserIDs(ctx, q, target.BuyerID)
	if err != nil {
		return err
	}
	return deps.Notif.Enqueue(ctx, q, adminIDs, "new_lead", map[string]any{"lead_id": leadID})
}

// LoadCustomValues fills lead.CustomValues from lead_custom_values.
func LoadCustomValues(ctx context.Context, q database.Querier, l *Lead) error {
	rows, err := q.Query(ctx, `SELECT custom_field_id, value FROM lead_custom_values WHERE lead_id=$1`, l.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	l.CustomValues = map[string]json.RawMessage{}
	for rows.Next() {
		var fid int64
		var val json.RawMessage
		if err := rows.Scan(&fid, &val); err != nil {
			return err
		}
		l.CustomValues[fmt.Sprintf("%d", fid)] = val
	}
	return rows.Err()
}

// ApplyRouteFieldMap copies publisher lead field values onto the lead per route_field_map rows.
func ApplyRouteFieldMap(ctx context.Context, q database.Querier, repo *Repository, lead *Lead, maps []routing.RouteFieldMapEntry) error {
	for _, m := range maps {
		if m.DstType == "builtin" && m.DstBuiltin != nil {
			val, ok := readLeadBuiltinForRoute(lead, m.SrcType, m.SrcBuiltin, m.SrcCustomFieldID)
			if !ok {
				continue
			}
			if err := repo.SetBuiltinField(ctx, q, lead.ID, *m.DstBuiltin, val); err != nil {
				return err
			}
			continue
		}
		if m.DstType == "custom" && m.DstCustomFieldID != nil {
			raw, ok := readLeadFieldRawForRoute(lead, m.SrcType, m.SrcBuiltin, m.SrcCustomFieldID)
			if !ok {
				continue
			}
			if err := repo.UpsertCustomValue(ctx, q, lead.ID, *m.DstCustomFieldID, raw); err != nil {
				return err
			}
		}
	}
	return nil
}

func readLeadBuiltinForRoute(l *Lead, srcType string, srcBuiltin *string, srcCustomID *int64) (string, bool) {
	if srcType == "builtin" && srcBuiltin != nil {
		v := builtinValue(l, *srcBuiltin)
		return v, v != ""
	}
	if srcType == "custom" && srcCustomID != nil && l.CustomValues != nil {
		raw, ok := l.CustomValues[fmt.Sprintf("%d", *srcCustomID)]
		if !ok {
			return "", false
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s, true
		}
		return string(raw), true
	}
	return "", false
}

func readLeadFieldRawForRoute(l *Lead, srcType string, srcBuiltin *string, srcCustomID *int64) (json.RawMessage, bool) {
	if srcType == "custom" && srcCustomID != nil && l.CustomValues != nil {
		raw, ok := l.CustomValues[fmt.Sprintf("%d", *srcCustomID)]
		return raw, ok && len(raw) > 0
	}
	if srcType == "builtin" && srcBuiltin != nil {
		v := builtinValue(l, *srcBuiltin)
		if v == "" {
			return nil, false
		}
		b, _ := json.Marshal(v)
		return b, true
	}
	return nil, false
}

func builtinValue(l *Lead, field string) string {
	switch field {
	case "first_name":
		return l.FirstName
	case "last_name":
		return l.LastName
	case "phone":
		if l.Phone != nil {
			return *l.Phone
		}
	case "email":
		if l.Email != nil {
			return *l.Email
		}
	case "address":
		if l.Address != nil {
			return *l.Address
		}
	case "city":
		if l.City != nil {
			return *l.City
		}
	case "state":
		if l.State != nil {
			return *l.State
		}
	case "zip":
		if l.Zip != nil {
			return *l.Zip
		}
	case "source", "campaign_name":
		if l.Source != nil {
			return *l.Source
		}
	}
	return ""
}
