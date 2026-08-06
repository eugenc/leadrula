package integrations

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
)

// TryEnqueueGHLOnContactUpdate enqueues contact-only GHL deliveries when a lead's contact fields change.
func (s *Service) TryEnqueueGHLOnContactUpdate(ctx context.Context, ownerAccountID, leadID int64, beforePayload, afterPayload []byte) error {
	if s == nil || s.pool == nil || ownerAccountID == 0 || leadID == 0 {
		return nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT c.id, c.config
		 FROM integration_connections c
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE c.account_id = $1 AND c.status = 'active' AND p.slug = 'ghl'`, ownerAccountID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var connID int64
		var connConfig json.RawMessage
		if err := rows.Scan(&connID, &connConfig); err != nil {
			return err
		}
		if !ghlContactUpdateShouldEnqueue(connConfig) {
			continue
		}
		if !GHLContactPayloadChangedForConnection(connConfig, beforePayload, afterPayload) {
			continue
		}
		var pending bool
		if err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM integration_delivery_queue
				WHERE lead_id = $1 AND connection_id = $2 AND status = 'pending'
			)`, leadID, connID).Scan(&pending); err != nil {
			return err
		}
		if pending {
			continue
		}
		payload := setSkipOpportunityStage(afterPayload)
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO integration_delivery_queue (lead_id, connection_id, payload)
			 VALUES ($1, $2, $3)`,
			leadID, connID, payload); err != nil {
			return err
		}
	}
	return rows.Err()
}

func ghlContactUpdateShouldEnqueue(connConfig json.RawMessage) bool {
	cfg, err := ghlConfigFromJSON(connConfig)
	if err != nil {
		return false
	}
	return cfg.SyncContactUpdatesEnabled && cfg.DeliveryMode != "webhook"
}

// GHLContactPayloadChangedForConnection reports whether contact-relevant delivery fields differ for a connection.
func GHLContactPayloadChangedForConnection(connConfig json.RawMessage, before, after []byte) bool {
	cfg, err := ghlConfigFromJSON(connConfig)
	if err != nil {
		return false
	}
	return providers.GHLContactPayloadChanged(cfg, before, after)
}

func payloadHasSkipOpportunityStage(payload []byte) bool {
	var m map[string]any
	if json.Unmarshal(payload, &m) != nil {
		return false
	}
	cfg, _ := m["_config"].(map[string]any)
	if cfg == nil {
		return false
	}
	switch v := cfg["skip_opportunity_stage"].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}
