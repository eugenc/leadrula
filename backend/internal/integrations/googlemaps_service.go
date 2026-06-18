package integrations

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

const googleMapsProviderSlug = "google_maps"

func (s *Service) HasGoogleMapsConnection(ctx context.Context, accountID int64) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM integration_connections c
			JOIN integration_providers p ON p.id = c.provider_id
			WHERE c.account_id = $1 AND p.slug = $2 AND c.status = 'active'
		)`, accountID, googleMapsProviderSlug).Scan(&exists)
	return exists, err
}

func (s *Service) GoogleMapsAPIKey(ctx context.Context, accountID int64) (string, error) {
	var encCredentials []byte
	var status string
	err := s.pool.QueryRow(ctx,
		`SELECT c.credentials, c.status
		 FROM integration_connections c
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE c.account_id = $1 AND p.slug = $2
		 ORDER BY c.created_at DESC
		 LIMIT 1`, accountID, googleMapsProviderSlug).Scan(&encCredentials, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", httpx.NotFound("google maps is not connected")
	}
	if err != nil {
		return "", err
	}
	if status != "active" {
		return "", httpx.Validation("google maps connection is not active")
	}
	raw, err := decrypt(s.encKey, encCredentials)
	if err != nil {
		return "", err
	}
	var creds struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(raw, &creds); err != nil || creds.APIKey == "" {
		return "", httpx.Validation("google maps credentials are invalid")
	}
	return creds.APIKey, nil
}
