package intake

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type IntegrationDeliveryDetail struct {
	ID             int64                           `json:"id"`
	Status         string                          `json:"status"`
	ConnectionName string                          `json:"connection_name"`
	ProviderSlug   string                          `json:"provider_slug"`
	LeadPublicID   string                          `json:"lead_public_id,omitempty"`
	ExternalID     string                          `json:"external_id,omitempty"`
	Payload        json.RawMessage                 `json:"payload"`
	LastError      *string                         `json:"last_error,omitempty"`
	Attempts       []IntegrationDeliveryAttemptLog `json:"attempts"`
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
