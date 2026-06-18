package providers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/echayko/leadrula/backend/internal/integrations/googlemaps"
)

type GoogleMapsProvider struct{}

func (p *GoogleMapsProvider) Slug() string { return "google_maps" }

func (p *GoogleMapsProvider) Deliver(ctx context.Context, credentials []byte, payload DeliveryPayload) (*DeliveryResult, error) {
	return nil, fmt.Errorf("google_maps is not a delivery provider")
}

func (p *GoogleMapsProvider) ValidateCredentials(ctx context.Context, credentials []byte, config map[string]any) error {
	var creds struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(credentials, &creds); err != nil || creds.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}
	return googlemaps.ValidateAPIKey(ctx, creds.APIKey)
}
