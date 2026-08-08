package integrations

import (
	"context"
	"encoding/json"

	"github.com/echayko/leadrula/backend/internal/customfields"
	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

type CRMCustomFieldResponse struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	FieldKey         string   `json:"field_key"`
	Object           string   `json:"object"`
	DataType         string   `json:"data_type"`
	LeadType         string   `json:"lead_type"`
	InboundSourceKey string   `json:"inbound_source_key"`
	Options          []string `json:"options,omitempty"`
	AlreadyImported  bool     `json:"already_imported"`
}

func (s *Service) CRMCustomFields(ctx context.Context, accountID, connectionID int64) (map[string]any, error) {
	conn, err := s.GetConnection(ctx, accountID, connectionID)
	if err != nil {
		return nil, err
	}
	if !providers.CRMCustomFieldsSupported(conn.ProviderSlug) {
		return nil, httpx.Validation("custom fields not supported for " + conn.ProviderSlug)
	}

	cfg := configMap(conn.Config)
	credentials, err := s.decryptedCredentials(ctx, connectionID, conn.ProviderSlug, cfg)
	if err != nil {
		return nil, httpx.Validation(err.Error())
	}

	fields, err := providers.FetchCRMCustomFields(ctx, conn.ProviderSlug, credentials, cfg)
	if err != nil {
		return nil, httpx.Validation(err.Error())
	}

	cfSvc := customfields.NewService(s.pool)
	imported, err := cfSvc.ImportedCRMFieldIDs(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	out := make([]CRMCustomFieldResponse, 0, len(fields))
	for _, f := range fields {
		out = append(out, CRMCustomFieldResponse{
			ID:               f.ID,
			Name:             f.Name,
			FieldKey:         f.FieldKey,
			Object:           f.Object,
			DataType:         f.DataType,
			LeadType:         providers.MapCRMFieldType(conn.ProviderSlug, f.DataType),
			InboundSourceKey: providers.CRMInboundSourceKey(conn.ProviderSlug, f),
			Options:          f.Options,
			AlreadyImported:  imported[f.ID],
		})
	}
	return map[string]any{
		"connection_id":   connectionID,
		"provider_slug":   conn.ProviderSlug,
		"custom_fields":   out,
	}, nil
}

func (s *Service) decryptedCredentials(ctx context.Context, connectionID int64, providerSlug string, cfg map[string]any) ([]byte, error) {
	if _, oauthOK := oauthProviders[providerSlug]; oauthOK {
		return s.refreshOAuthToken(ctx, connectionID, providerSlug, cfg)
	}
	var encCredentials []byte
	if err := s.pool.QueryRow(ctx,
		`SELECT credentials FROM integration_connections WHERE id=$1`, connectionID).Scan(&encCredentials); err != nil {
		return nil, err
	}
	if len(encCredentials) == 0 {
		return nil, httpx.Validation("connection has no credentials")
	}
	return decrypt(s.encKey, encCredentials)
}

func (s *Service) ImportCustomFieldsFromCRM(ctx context.Context, accountID int64, in customfields.ImportFromCRMInput) (*customfields.ImportFromCRMResult, error) {
	cfSvc := customfields.NewService(s.pool)
	if s.webhookSyncer != nil {
		cfSvc.SetCRMBindingSyncer(s.webhookSyncer)
	}
	return cfSvc.ImportFromCRM(ctx, accountID, in)
}

// WebhookCRMBindingSyncer syncs CRM bindings into inbound webhook field maps.
type WebhookCRMBindingSyncer interface {
	SyncCRMBindingFieldMaps(ctx context.Context, connectionID int64) error
}

func (s *Service) SetWebhookCRMBindingSyncer(syncer WebhookCRMBindingSyncer) {
	s.webhookSyncer = syncer
}

func (s *Service) applySalesforceCRMBindings(ctx context.Context, connectionID, leadID int64, externalID string, credentials []byte, config map[string]any) {
	var accountID int64
	if err := s.pool.QueryRow(ctx,
		`SELECT account_id FROM integration_connections WHERE id=$1`, connectionID).Scan(&accountID); err != nil {
		return
	}
	cfSvc := customfields.NewService(s.pool)
	bindings, err := cfSvc.ListBindingsByConnection(ctx, accountID, connectionID)
	if err != nil || len(bindings) == 0 {
		return
	}
	keys := make([]string, 0, len(bindings))
	keyToField := map[string]int64{}
	for _, b := range bindings {
		keys = append(keys, b.CRMFieldKey)
		keyToField[b.CRMFieldKey] = b.CustomFieldID
	}
	values, err := providers.FetchSalesforceLeadCustomValues(ctx, credentials, config, externalID, keys)
	if err != nil {
		return
	}
	for key, val := range values {
		fid, ok := keyToField[key]
		if !ok {
			continue
		}
		valJSON, _ := json.Marshal(val)
		_ = cfSvc.UpsertLeadCustomValue(ctx, leadID, fid, valJSON)
	}
}
