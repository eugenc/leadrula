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
	EnqueueDelivery(ctx context.Context, routeID, leadID int64, branchPosition int, payloadJSON []byte) error
	EnqueueConnectionDelivery(ctx context.Context, connectionID, leadID int64, payloadJSON []byte) error
	EnqueueParticipationWebhook(ctx context.Context, webhookID, leadID int64, payloadJSON []byte) error
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

// ApplyRoute moves a lead according to route destination.
func ApplyRoute(ctx context.Context, q database.Querier, deps RouteApplyDeps, route *routing.Route, leadID int64) ([]notifications.EmailJob, error) {
	switch route.Destination {
	case "pipeline":
		return applyPipelineRoute(ctx, q, deps.Repo, route, route.OwnerAccountID(), leadID)
	case "contract":
		return applyContractRoute(ctx, q, deps, route, leadID)
	case "webhook":
		if err := applyWebhookDestRoute(ctx, q, deps, route, leadID); err != nil {
			return nil, err
		}
		return nil, nil
	case "integration":
		return nil, nil
	default:
		return nil, httpx.BusinessRule("unsupported route destination")
	}
}

func applyPipelineRoute(ctx context.Context, q database.Querier, repo *Repository, route *routing.Route, ownerAccountID, leadID int64) ([]notifications.EmailJob, error) {
	if route.Delivery == "leads" {
		return nil, repo.SetStatus(ctx, q, leadID, "review")
	}
	if route.TargetPipelineID == nil || route.TargetStageID == nil {
		return nil, httpx.BusinessRule("route missing pipeline target")
	}
	if err := repo.PlaceInPipeline(ctx, q, leadID, ownerAccountID, *route.TargetPipelineID, *route.TargetStageID, nil); err != nil {
		return nil, err
	}
	return nil, repo.SetStatus(ctx, q, leadID, "review")
}

func applyWebhookDestRoute(ctx context.Context, q database.Querier, deps RouteApplyDeps, route *routing.Route, leadID int64) error {
	if deps.Integrations == nil || route.DestWebhookID == nil {
		return nil
	}
	lead, err := deps.Repo.GetByID(ctx, q, leadID)
	if err != nil {
		return err
	}
	if err := LoadCustomValues(ctx, q, lead); err != nil {
		return err
	}
	payloadJSON, err := BuildDeliveryPayload(lead)
	if err != nil {
		return err
	}
	return deps.Integrations.EnqueueParticipationWebhook(ctx, *route.DestWebhookID, leadID, payloadJSON)
}

// TryApplyMatchedRoute applies a matched route and returns whether integrations should enqueue.
func TryApplyMatchedRoute(ctx context.Context, q database.Querier, deps RouteApplyDeps, route *routing.Route, leadID int64) (enqueueIntegrations bool, emails []notifications.EmailJob, err error) {
	if route == nil {
		return false, nil, nil
	}
	emails, err = ApplyRoute(ctx, q, deps, route, leadID)
	if err != nil {
		return false, nil, err
	}
	return route.Destination == "integration", emails, nil
}

func applyContractRoute(ctx context.Context, q database.Querier, deps RouteApplyDeps, route *routing.Route, leadID int64) ([]notifications.EmailJob, error) {
	if route.ContractID == nil {
		return nil, httpx.BusinessRule("route missing contract")
	}
	maps, err := routing.RouteFieldMap(ctx, q, route.ID)
	if err != nil {
		return nil, err
	}
	return ApplyContractDistribution(ctx, q, deps, contractDistributionParams{
		ContractID:       *route.ContractID,
		CompensationID:   route.CompensationID,
		TargetStageID:    route.TargetStageID,
		Delivery:         route.Delivery,
		RouteFieldMaps:   maps,
		BillingLabel:     "lead routed: " + route.Name,
		ClearPreassigned: false,
	}, leadID)
}

type contractDistributionParams struct {
	ContractID         int64
	CompensationID     *int64
	PreassignedBuyerID *int64
	TargetStageID      *int64
	Delivery           string
	RouteFieldMaps     []routing.RouteFieldMapEntry
	BillingLabel       string
	ClearPreassigned   bool
}

// ApplyContractDistribution moves a lead to a buyer via contract.
func ApplyContractDistribution(ctx context.Context, q database.Querier, deps RouteApplyDeps, p contractDistributionParams, leadID int64) ([]notifications.EmailJob, error) {
	if err := contracts.RequireActiveContract(ctx, q, p.ContractID); err != nil {
		return nil, err
	}
	lead, err := deps.Repo.GetByID(ctx, q, leadID)
	if err != nil {
		return nil, err
	}
	leadCost := float64(0)
	if cb := costBasisFromLead(lead); cb != nil {
		leadCost = *cb
	}
	var target *contracts.Target
	if p.PreassignedBuyerID != nil {
		target, err = contracts.GetTargetForPreassignedBuyer(ctx, q, p.ContractID, *p.PreassignedBuyerID)
	} else {
		target, err = contracts.GetTargetForRoute(ctx, q, p.ContractID, p.CompensationID, leadCost)
	}
	if err != nil {
		return nil, err
	}
	if p.PreassignedBuyerID != nil && target.BuyerID != *p.PreassignedBuyerID {
		return nil, httpx.BusinessRule("pre-assigned buyer does not match contract participation")
	}
	if err := contracts.RequireFieldMappingComplete(ctx, q, p.ContractID, target.BuyerID, target.ParticipationID); err != nil {
		return nil, err
	}
	if err := CheckDuplicate(ctx, q, target.BuyerID, lead.Phone, lead.Email, leadID); err != nil {
		return nil, err
	}
	if err := LoadCustomValues(ctx, q, lead); err != nil {
		return nil, err
	}
	if len(p.RouteFieldMaps) > 0 {
		if err := ApplyRouteFieldMap(ctx, q, deps.Repo, lead, p.RouteFieldMaps); err != nil {
			return nil, err
		}
	}
	contractMaps, err := contracts.ContractFieldMapForRoute(ctx, q, p.ContractID, target.ParticipationID)
	if err != nil {
		return nil, err
	}
	if len(contractMaps) > 0 {
		if err := ApplyRouteFieldMap(ctx, q, deps.Repo, lead, contractMaps); err != nil {
			return nil, err
		}
	}

	buyer, err := deps.Accounts.GetAccount(ctx, target.BuyerID)
	if err != nil {
		return nil, err
	}

	contractID := target.ID
	delivery := p.Delivery
	if target.Delivery != "" {
		delivery = target.Delivery
	}
	if delivery == "leads" || delivery == "webhook" {
		if err := deps.Repo.TransferOwner(ctx, q, leadID, target.BuyerID, &contractID); err != nil {
			return nil, err
		}
		if err := deps.Repo.SetStatus(ctx, q, leadID, "review"); err != nil {
			return nil, err
		}
		if p.ClearPreassigned {
			if err := deps.Repo.ClearPreassignedBuyer(ctx, q, leadID); err != nil {
				return nil, err
			}
		}
		return nil, enqueueParticipationIntegration(ctx, deps, target, lead)
	}

	var destStage int64
	if target.BuyerStageID != 0 {
		destStage = target.BuyerStageID
	} else if p.TargetStageID != nil && *p.TargetStageID != 0 {
		destStage = *p.TargetStageID
	} else {
		if err := q.QueryRow(ctx,
			`SELECT id FROM pipeline_stages WHERE pipeline_id=$1 ORDER BY position, id LIMIT 1`,
			target.BuyerPipelineID).Scan(&destStage); err != nil {
			return nil, httpx.BusinessRule("target pipeline has no stages")
		}
	}
	if err := deps.Repo.PlaceInPipeline(ctx, q, leadID, target.BuyerID, target.BuyerPipelineID, destStage, &contractID); err != nil {
		return nil, err
	}
	if err := contracts.InitPublisherTracking(ctx, q, contractID, leadID, target.BuyerID, destStage); err != nil {
		return nil, err
	}
	if err := contracts.CheckCap(ctx, q, target.ID, target.CompensationID); err != nil {
		return nil, err
	}
	costBasis := costBasisFromLead(lead)
	if err := billing.Debit(ctx, q, target.BuyerID, target.RatePerLead, leadID, target.ID, p.BillingLabel); err != nil {
		return nil, err
	}
	if err := contracts.RecordEarningDistribute(ctx, q, target.CompensationID, leadID, target.RatePerLead, costBasis); err != nil {
		return nil, err
	}
	if err := deps.Repo.SetCostAfterBuyerDistribution(ctx, q, leadID, buyer.Type, target.RatePerLead); err != nil {
		return nil, err
	}
	if err := deps.Repo.SetStatus(ctx, q, leadID, "distributed"); err != nil {
		return nil, err
	}
	if p.ClearPreassigned {
		if err := deps.Repo.ClearPreassignedBuyer(ctx, q, leadID); err != nil {
			return nil, err
		}
	}
	updated, err := deps.Repo.GetByID(ctx, q, leadID)
	if err != nil {
		return nil, err
	}
	emails, err := deps.Notif.Deliver(ctx, q, notifications.DeliverParams{
		AccountID: target.BuyerID,
		UserIDs:   notifications.AssigneeIDs(updated.AssignedUserID),
		EventType: "new_lead",
		Payload:   map[string]any{"lead_id": leadID},
	})
	if err != nil {
		return nil, err
	}
	if err := enqueueParticipationIntegration(ctx, deps, target, updated); err != nil {
		return nil, err
	}
	return emails, nil
}

// TryApplyPreassignedBuyer distributes a lead to its pre-assigned buyer when a trigger stage is reached.
func TryApplyPreassignedBuyer(ctx context.Context, q database.Querier, deps RouteApplyDeps, lead *Lead, leadID int64) ([]notifications.EmailJob, error) {
	if lead.PreassignedBuyerID == nil {
		return nil, httpx.BusinessRule("no pre-assigned buyer")
	}
	if lead.PipelineID == nil || *lead.PipelineID == 0 {
		return nil, httpx.BusinessRule("lead has no pipeline for contract lookup")
	}
	contractID, err := contracts.FindActiveContractByBuyerPipeline(ctx, q, lead.PublisherID, *lead.PreassignedBuyerID, *lead.PipelineID)
	if err != nil {
		return nil, err
	}
	buyerID := *lead.PreassignedBuyerID
	return ApplyContractDistribution(ctx, q, deps, contractDistributionParams{
		ContractID:         contractID,
		PreassignedBuyerID: &buyerID,
		BillingLabel:       "lead pre-assigned",
		ClearPreassigned:   true,
	}, leadID)
}

func enqueueParticipationIntegration(ctx context.Context, deps RouteApplyDeps, target *contracts.Target, lead *Lead) error {
	if deps.Integrations == nil {
		return nil
	}
	if err := LoadCustomValues(ctx, deps.Repo.Pool(), lead); err != nil {
		return err
	}
	payloadJSON, err := BuildDeliveryPayload(lead)
	if err != nil {
		return err
	}
	if target.IntegrationID != 0 {
		if err := deps.Integrations.EnqueueConnectionDelivery(ctx, target.IntegrationID, lead.ID, payloadJSON); err != nil {
			return err
		}
	}
	if target.Delivery == "webhook" && target.WebhookID != 0 {
		return deps.Integrations.EnqueueParticipationWebhook(ctx, target.WebhookID, lead.ID, payloadJSON)
	}
	return nil
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
		if m.DstType == "builtin" && m.DstBuiltin != nil && IsMoneyBuiltin(*m.DstBuiltin) {
			continue
		}
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
