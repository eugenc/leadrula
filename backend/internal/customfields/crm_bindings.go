package customfields

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type CRMBinding struct {
	ID               int64  `json:"id"`
	AccountID        int64  `json:"-"`
	CustomFieldID    int64  `json:"custom_field_id"`
	ConnectionID     int64  `json:"connection_id"`
	CRMFieldID       string `json:"crm_field_id"`
	CRMFieldKey      string `json:"crm_field_key"`
	CRMObject        string `json:"crm_object"`
	InboundSourceKey string `json:"inbound_source_key"`
}

type CRMBindingSyncer interface {
	SyncCRMBindingFieldMaps(ctx context.Context, connectionID int64) error
}

type ImportFromCRMFieldInput struct {
	CRMFieldID       string   `json:"crm_field_id"`
	CRMFieldKey      string   `json:"crm_field_key"`
	Name             string   `json:"name"`
	DataType         string   `json:"data_type"`
	Object           string   `json:"object"`
	Options          []string `json:"options"`
	LeadType         string   `json:"lead_type"`
	InboundSourceKey string   `json:"inbound_source_key"`
}

type ImportFromCRMInput struct {
	ConnectionID int64                     `json:"connection_id"`
	Fields       []ImportFromCRMFieldInput `json:"fields"`
}

type ImportFromCRMResult struct {
	Created int              `json:"created"`
	Skipped int              `json:"skipped"`
	Errors  []ImportRowError `json:"errors"`
}

func (s *Service) SetCRMBindingSyncer(syncer CRMBindingSyncer) {
	s.crmBindingSyncer = syncer
}

func (s *Service) bindingExists(ctx context.Context, connectionID int64, crmFieldID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM custom_field_crm_bindings WHERE connection_id=$1 AND crm_field_id=$2)`,
		connectionID, crmFieldID).Scan(&exists)
	return exists, err
}

func (s *Service) ImportedCRMFieldIDs(ctx context.Context, connectionID int64) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT crm_field_id FROM custom_field_crm_bindings WHERE connection_id=$1`, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

func (s *Service) ListBindingsByConnection(ctx context.Context, accountID, connectionID int64) ([]CRMBinding, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, account_id, custom_field_id, connection_id, crm_field_id, crm_field_key, crm_object, inbound_source_key
		 FROM custom_field_crm_bindings WHERE account_id=$1 AND connection_id=$2 ORDER BY id`,
		accountID, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CRMBinding
	for rows.Next() {
		var b CRMBinding
		if err := rows.Scan(&b.ID, &b.AccountID, &b.CustomFieldID, &b.ConnectionID,
			&b.CRMFieldID, &b.CRMFieldKey, &b.CRMObject, &b.InboundSourceKey); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Service) ImportFromCRM(ctx context.Context, accountID int64, in ImportFromCRMInput) (*ImportFromCRMResult, error) {
	if in.ConnectionID <= 0 {
		return nil, httpx.Validation("connection_id required")
	}
	if len(in.Fields) == 0 {
		return nil, httpx.Validation("no fields to import")
	}
	if len(in.Fields) > maxImportRows {
		return nil, httpx.Validation(fmt.Sprintf("maximum %d fields per import (got %d)", maxImportRows, len(in.Fields)))
	}

	var connAccountID int64
	err := s.pool.QueryRow(ctx,
		`SELECT account_id FROM integration_connections WHERE id=$1`, in.ConnectionID).Scan(&connAccountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("connection not found")
	}
	if err != nil {
		return nil, err
	}
	if connAccountID != accountID {
		return nil, httpx.NotFound("connection not found")
	}

	result := &ImportFromCRMResult{Errors: []ImportRowError{}}
	for i, f := range in.Fields {
		crmFieldID := strings.TrimSpace(f.CRMFieldID)
		name := strings.TrimSpace(f.Name)
		if crmFieldID == "" || name == "" {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: "crm_field_id and name required"})
			continue
		}

		exists, err := s.bindingExists(ctx, in.ConnectionID, crmFieldID)
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: err.Error()})
			continue
		}
		if exists {
			result.Skipped++
			continue
		}

		crmFieldKey := strings.TrimSpace(f.CRMFieldKey)
		if crmFieldKey == "" {
			crmFieldKey = crmFieldID
		}
		crmObject := strings.TrimSpace(f.Object)
		if crmObject == "" {
			crmObject = "contact"
		}
		leadType := strings.ToLower(strings.TrimSpace(f.LeadType))
		if leadType == "" {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: "lead_type is required"})
			continue
		}
		if !validType(leadType) {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: "invalid lead_type"})
			continue
		}
		inboundKey := strings.TrimSpace(f.InboundSourceKey)
		if inboundKey == "" {
			inboundKey = crmFieldKey
		}

		var options json.RawMessage
		if leadType == "dropdown" && len(f.Options) > 0 {
			options, _ = json.Marshal(f.Options)
		} else {
			options = json.RawMessage("[]")
		}

		fieldKey, err := s.uniqueFieldKey(ctx, accountID, slugFieldKey(name))
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: err.Error()})
			continue
		}

		created, err := s.CreateField(ctx, accountID, name, fieldKey, leadType, options, nil)
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: err.Error()})
			continue
		}

		_, err = s.pool.Exec(ctx,
			`INSERT INTO custom_field_crm_bindings(account_id, custom_field_id, connection_id, crm_field_id, crm_field_key, crm_object, inbound_source_key)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			accountID, created.ID, in.ConnectionID, crmFieldID, crmFieldKey, crmObject, inboundKey)
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: err.Error()})
			continue
		}
		result.Created++
	}

	if result.Created > 0 && s.crmBindingSyncer != nil {
		if err := s.crmBindingSyncer.SyncCRMBindingFieldMaps(ctx, in.ConnectionID); err != nil {
			return result, err
		}
	}

	return result, nil
}

func (s *Service) uniqueFieldKey(ctx context.Context, accountID int64, base string) (string, error) {
	if base == "" {
		base = "field"
	}
	candidate := base
	for i := 0; i < 100; i++ {
		if i > 0 {
			candidate = fmt.Sprintf("%s_%d", base, i)
		}
		existing, err := s.fieldByKey(ctx, accountID, candidate)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return candidate, nil
		}
	}
	return "", httpx.Conflict("could not generate unique field_key")
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugFieldKey(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugNonAlnum.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	return s
}

func (s *Service) UpsertLeadCustomValue(ctx context.Context, leadID, customFieldID int64, valJSON []byte) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO lead_custom_values(lead_id, custom_field_id, value) VALUES ($1,$2,$3)
		 ON CONFLICT (lead_id, custom_field_id) DO UPDATE SET value=EXCLUDED.value`,
		leadID, customFieldID, valJSON)
	return err
}
