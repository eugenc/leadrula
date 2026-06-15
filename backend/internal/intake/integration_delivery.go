package intake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type LabeledCustomField struct {
	ID       int64  `json:"id"`
	FieldKey string `json:"field_key"`
	Name     string `json:"name"`
	Value    any    `json:"value"`
}

type IntegrationDeliveryDetail struct {
	ID                  int64                           `json:"id"`
	Status              string                          `json:"status"`
	ConnectionName      string                          `json:"connection_name"`
	ProviderSlug        string                          `json:"provider_slug"`
	LeadPublicID        string                          `json:"lead_public_id,omitempty"`
	ExternalID          string                          `json:"external_id,omitempty"`
	Payload             json.RawMessage                 `json:"payload"`
	CustomFieldsLabeled []LabeledCustomField            `json:"custom_fields_labeled,omitempty"`
	LastError           *string                         `json:"last_error,omitempty"`
	Attempts            []IntegrationDeliveryAttemptLog `json:"attempts"`
}

type IntegrationDeliveryAttemptLog struct {
	AttemptNumber int             `json:"attempt_number"`
	Status        string          `json:"status"`
	HTTPStatus    *int            `json:"http_status,omitempty"`
	RequestBody   json.RawMessage `json:"request_body,omitempty"`
	ResponseBody  string          `json:"response_body,omitempty"`
	DurationMs    *int            `json:"duration_ms,omitempty"`
	Error         *string         `json:"error,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

func (s *Service) GetIntegrationDelivery(ctx context.Context, accountID, deliveryID int64) (*IntegrationDeliveryDetail, error) {
	detail := &IntegrationDeliveryDetail{Payload: json.RawMessage("{}"), Attempts: []IntegrationDeliveryAttemptLog{}}
	var payload []byte
	var externalID *string
	err := s.pool.QueryRow(ctx,
		`SELECT q.id, q.status::text, c.name, p.slug, COALESCE(l.public_id::text, ''), q.external_id, q.payload, q.last_error
		 FROM integration_delivery_queue q
		 JOIN integration_connections c ON c.id = q.connection_id
		 JOIN integration_providers p ON p.id = c.provider_id
		 LEFT JOIN leads l ON l.id = q.lead_id
		 WHERE q.id = $1 AND c.account_id = $2 AND q.webhook_trigger_id IS NULL`,
		deliveryID, accountID,
	).Scan(
		&detail.ID, &detail.Status, &detail.ConnectionName, &detail.ProviderSlug,
		&detail.LeadPublicID, &externalID, &payload, &detail.LastError,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("integration delivery not found")
		}
		return nil, err
	}
	if len(payload) > 0 {
		detail.Payload = payload
	}
	if externalID != nil {
		detail.ExternalID = *externalID
	}

	labeled, err := s.labelPayloadCustomFields(ctx, accountID, payload)
	if err != nil {
		return nil, err
	}
	detail.CustomFieldsLabeled = labeled

	rows, err := s.pool.Query(ctx,
		`SELECT attempt_number, status, http_status, request_body, response_body, duration_ms, error, created_at
		 FROM integration_delivery_logs
		 WHERE queue_item_id = $1
		 ORDER BY attempt_number ASC`,
		deliveryID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var a IntegrationDeliveryAttemptLog
		var reqBody []byte
		var respBody *string
		if err := rows.Scan(
			&a.AttemptNumber, &a.Status, &a.HTTPStatus, &reqBody, &respBody, &a.DurationMs, &a.Error, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		if len(reqBody) > 0 {
			a.RequestBody = reqBody
		}
		if respBody != nil {
			a.ResponseBody = *respBody
		}
		detail.Attempts = append(detail.Attempts, a)
	}
	return detail, rows.Err()
}

func (s *Service) RetryIntegrationDelivery(ctx context.Context, accountID, deliveryID int64) error {
	var id int64
	err := s.pool.QueryRow(ctx,
		`UPDATE integration_delivery_queue q
		 SET status = 'pending', attempts = 0, next_attempt_at = now(), last_error = NULL, updated_at = now()
		 FROM integration_connections c
		 WHERE q.id = $1 AND q.connection_id = c.id AND c.account_id = $2 AND q.webhook_trigger_id IS NULL
		 RETURNING q.id`,
		deliveryID, accountID,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.NotFound("integration delivery not found")
		}
		return err
	}
	return nil
}

func (s *Service) labelPayloadCustomFields(ctx context.Context, accountID int64, payload []byte) ([]LabeledCustomField, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	var root map[string]any
	if err := json.Unmarshal(payload, &root); err != nil {
		return nil, nil
	}
	rawCustom, ok := root["custom_fields"].(map[string]any)
	if !ok || len(rawCustom) == 0 {
		return nil, nil
	}

	ids := make([]int64, 0, len(rawCustom))
	for key := range rawCustom {
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	meta, err := s.lookupCustomFieldMeta(ctx, accountID, ids)
	if err != nil {
		return nil, err
	}

	out := make([]LabeledCustomField, 0, len(ids))
	for _, id := range ids {
		key := fmt.Sprintf("%d", id)
		m, ok := meta[id]
		if !ok {
			continue
		}
		out = append(out, LabeledCustomField{
			ID:       id,
			FieldKey: m.FieldKey,
			Name:     m.Name,
			Value:    rawCustom[key],
		})
	}
	return out, nil
}

type customFieldMeta struct {
	FieldKey string
	Name     string
}

func (s *Service) lookupCustomFieldMeta(ctx context.Context, accountID int64, ids []int64) (map[int64]customFieldMeta, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, field_key, name FROM custom_fields WHERE account_id = $1 AND id = ANY($2)`,
		accountID, ids,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]customFieldMeta{}
	for rows.Next() {
		var id int64
		var m customFieldMeta
		if err := rows.Scan(&id, &m.FieldKey, &m.Name); err != nil {
			return nil, err
		}
		out[id] = m
	}
	return out, rows.Err()
}
