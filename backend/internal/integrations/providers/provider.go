package providers

import (
	"context"
	"strings"
)

type DeliveryPayload struct {
	LeadID       string         `json:"lead_id"`
	FirstName    string         `json:"first_name"`
	LastName     string         `json:"last_name"`
	Phone        string         `json:"phone"`
	Email        string         `json:"email"`
	Address      string         `json:"address"`
	City         string         `json:"city"`
	State        string         `json:"state"`
	Zip          string         `json:"zip"`
	Source       string         `json:"source"`
	ActionAt     string         `json:"action_at,omitempty"`
	PipelineID   int64          `json:"pipeline_id,omitempty"`
	StageID      int64          `json:"stage_id,omitempty"`
	CustomFields map[string]any `json:"custom_fields"`
	Config       map[string]any `json:"-"`
}

type DeliveryResult struct {
	ExternalID string
	Raw        []byte
	Request    []byte // JSON-marshaled DeliveryRequestLog
	HTTPStatus int
}

type Provider interface {
	Slug() string
	Deliver(ctx context.Context, credentials []byte, payload DeliveryPayload) (*DeliveryResult, error)
	ValidateCredentials(ctx context.Context, credentials []byte, config map[string]any) error
}

func appendLeadID(body map[string]any, payload DeliveryPayload) {
	if id := strings.TrimSpace(payload.LeadID); id != "" {
		body["lead_id"] = id
	}
}
