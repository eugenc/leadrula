package leads

import (
	"context"
	"encoding/json"

	"github.com/echayko/leadrula/backend/internal/database"
)

// BuildDeliveryPayload serialises lead data for integration delivery (post field mapping).
func BuildDeliveryPayload(lead *Lead) ([]byte, error) {
	p := map[string]any{
		"lead_id":    lead.PublicID,
		"first_name": lead.FirstName,
		"last_name":  lead.LastName,
	}
	if lead.Phone != nil {
		p["phone"] = *lead.Phone
	}
	if lead.Email != nil {
		p["email"] = *lead.Email
	}
	if lead.Address != nil {
		p["address"] = *lead.Address
	}
	if lead.City != nil {
		p["city"] = *lead.City
	}
	if lead.State != nil {
		p["state"] = *lead.State
	}
	if lead.Zip != nil {
		p["zip"] = *lead.Zip
	}
	if lead.Source != nil {
		p["source"] = *lead.Source
	}
	customs := map[string]any{}
	for k, v := range lead.CustomValues {
		var decoded any
		if err := json.Unmarshal(v, &decoded); err == nil {
			customs[k] = decoded
		} else {
			customs[k] = string(v)
		}
	}
	p["custom_fields"] = customs
	return json.Marshal(p)
}

// MergeDeliveryConfig copies route delivery_config (_config) from an old payload into a rebuilt one.
func MergeDeliveryConfig(rebuilt, old []byte) []byte {
	var oldMap, newMap map[string]any
	if json.Unmarshal(old, &oldMap) != nil || json.Unmarshal(rebuilt, &newMap) != nil {
		return rebuilt
	}
	if cfg, ok := oldMap["_config"]; ok {
		newMap["_config"] = cfg
	}
	b, err := json.Marshal(newMap)
	if err != nil {
		return rebuilt
	}
	return b
}

// RefreshDeliveryPayload reloads lead + custom values and rebuilds the integration delivery payload.
func RefreshDeliveryPayload(ctx context.Context, q database.Querier, repo *Repository, leadID int64, oldPayload []byte) ([]byte, error) {
	lead, err := repo.GetByID(ctx, q, leadID)
	if err != nil {
		return nil, err
	}
	if err := LoadCustomValues(ctx, q, lead); err != nil {
		return nil, err
	}
	rebuilt, err := BuildDeliveryPayload(lead)
	if err != nil {
		return nil, err
	}
	return MergeDeliveryConfig(rebuilt, oldPayload), nil
}

// TryEnqueueIntegrations runs after a route apply transaction commits.
func TryEnqueueIntegrations(ctx context.Context, q database.Querier, repo *Repository, enq IntegrationEnqueuer, routeID, leadID int64, branchPosition int) {
	if enq == nil || routeID == 0 {
		return
	}
	lead, err := repo.GetByID(ctx, q, leadID)
	if err != nil {
		return
	}
	if err := LoadCustomValues(ctx, q, lead); err != nil {
		return
	}
	payloadJSON, err := BuildDeliveryPayload(lead)
	if err != nil {
		return
	}
	_ = enq.EnqueueDelivery(ctx, routeID, leadID, branchPosition, payloadJSON)
}
