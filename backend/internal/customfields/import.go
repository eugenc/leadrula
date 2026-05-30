package customfields

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

const maxImportRows = 1000

type ColumnMapping struct {
	CSVColumn string `json:"csv_column"`
	Target    string `json:"target"`
}

type importRow map[string]string

func (r *importRow) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := make(importRow)
	for k, v := range raw {
		if k == "__parsed_extra" {
			continue
		}
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			out[k] = s
			continue
		}
		var f float64
		if err := json.Unmarshal(v, &f); err == nil {
			out[k] = strconv.FormatFloat(f, 'f', -1, 64)
			continue
		}
		var bval bool
		if err := json.Unmarshal(v, &bval); err == nil {
			out[k] = strconv.FormatBool(bval)
			continue
		}
		var arr []json.RawMessage
		if err := json.Unmarshal(v, &arr); err == nil {
			var parts []string
			for _, item := range arr {
				var s string
				if json.Unmarshal(item, &s) == nil {
					s = strings.TrimSpace(s)
					if s != "" {
						parts = append(parts, s)
					}
				}
			}
			if len(parts) > 0 {
				out[k] = strings.Join(parts, ", ")
			}
		}
	}
	*r = out
	return nil
}

type ImportFieldsInput struct {
	Mapping []ColumnMapping `json:"mapping"`
	Rows    []importRow     `json:"rows"`
}

type ImportRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

type ImportFieldsResult struct {
	Created int              `json:"created"`
	Updated int              `json:"updated"`
	Skipped int              `json:"skipped"`
	Errors  []ImportRowError `json:"errors"`
}

type importFieldRow struct {
	Name     string
	FieldKey string
	Type     string
	Options  json.RawMessage
	IsActive *bool
}

func (s *Service) ImportFields(ctx context.Context, accountID int64, in ImportFieldsInput) (*ImportFieldsResult, error) {
	if len(in.Rows) == 0 {
		return nil, httpx.Validation("no rows to import")
	}
	if len(in.Rows) > maxImportRows {
		return nil, httpx.Validation(fmt.Sprintf("maximum %d rows per import (got %d)", maxImportRows, len(in.Rows)))
	}

	result := &ImportFieldsResult{Errors: []ImportRowError{}}
	seenKeys := make(map[string]int)

	for i, row := range in.Rows {
		parsed, err := mapImportFieldRow(row, in.Mapping)
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: err.Error()})
			continue
		}

		if prev, ok := seenKeys[parsed.FieldKey]; ok {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{
				Row:     i + 1,
				Message: fmt.Sprintf("duplicate field_key %q (also on row %d)", parsed.FieldKey, prev),
			})
			continue
		}
		seenKeys[parsed.FieldKey] = i + 1

		existing, err := s.fieldByKey(ctx, accountID, parsed.FieldKey)
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: err.Error()})
			continue
		}

		if existing != nil {
			if parsed.Type != "" && !strings.EqualFold(parsed.Type, existing.Type) {
				result.Skipped++
				result.Errors = append(result.Errors, ImportRowError{
					Row:     i + 1,
					Message: fmt.Sprintf("type cannot be changed (existing %q, got %q)", existing.Type, parsed.Type),
				})
				continue
			}
			name := parsed.Name
			opts := parsed.Options
			if _, err := s.UpdateField(ctx, accountID, existing.ID, &name, nil, opts, nil, parsed.IsActive); err != nil {
				result.Skipped++
				result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: err.Error()})
				continue
			}
			result.Updated++
			continue
		}

		if parsed.Type == "" {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: "type is required for new fields"})
			continue
		}
		if !validType(parsed.Type) {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: "invalid field type"})
			continue
		}
		if parsed.Type == "dropdown" && len(parsed.Options) <= 2 {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: "dropdown fields require options"})
			continue
		}

		f, err := s.CreateField(ctx, accountID, parsed.Name, parsed.FieldKey, parsed.Type, parsed.Options)
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: err.Error()})
			continue
		}
		if parsed.IsActive != nil && !*parsed.IsActive {
			active := false
			if _, err := s.UpdateField(ctx, accountID, f.ID, nil, nil, nil, nil, &active); err != nil {
				result.Skipped++
				result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: err.Error()})
				continue
			}
		}
		result.Created++
	}

	return result, nil
}

func (s *Service) fieldByKey(ctx context.Context, accountID int64, fieldKey string) (*CustomField, error) {
	f := &CustomField{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, public_id, account_id, name, field_key, type, options, position, is_active, created_at
		 FROM custom_fields WHERE account_id = $1 AND field_key = $2`,
		accountID, fieldKey).Scan(
		&f.ID, &f.PublicID, &f.AccountID, &f.Name, &f.FieldKey, &f.Type, &f.Options, &f.Position, &f.IsActive, &f.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

func mapImportFieldRow(row importRow, mapping []ColumnMapping) (importFieldRow, error) {
	out := importFieldRow{}
	hasIsActive := false

	for _, m := range mapping {
		if m.Target == "" || m.Target == "skip" {
			continue
		}
		val := strings.TrimSpace(row[m.CSVColumn])
		switch m.Target {
		case "name":
			out.Name = val
		case "field_key":
			out.FieldKey = val
		case "type":
			out.Type = strings.ToLower(val)
		case "options":
			if val != "" {
				opts, err := parseOptionsCSV(val)
				if err != nil {
					return out, err
				}
				out.Options = opts
			}
		case "is_active":
			active, err := parseBool(val)
			if err != nil {
				return out, err
			}
			out.IsActive = &active
			hasIsActive = true
		}
	}

	out.Name = strings.TrimSpace(out.Name)
	out.FieldKey = strings.TrimSpace(out.FieldKey)
	if out.Name == "" {
		return out, httpx.Validation("name is required")
	}
	if out.FieldKey == "" {
		return out, httpx.Validation("field_key is required")
	}
	if !hasIsActive {
		active := true
		out.IsActive = &active
	}
	return out, nil
}

func parseOptionsCSV(val string) (json.RawMessage, error) {
	var opts []string
	for _, part := range strings.Split(val, ",") {
		if t := strings.TrimSpace(part); t != "" {
			opts = append(opts, t)
		}
	}
	if len(opts) == 0 {
		return nil, httpx.Validation("options cannot be empty")
	}
	b, err := json.Marshal(opts)
	return b, err
}

func parseBool(val string) (bool, error) {
	s := strings.ToLower(strings.TrimSpace(val))
	if s == "" {
		return true, nil
	}
	switch s {
	case "true", "1", "yes", "y":
		return true, nil
	case "false", "0", "no", "n":
		return false, nil
	default:
		return false, httpx.Validation("is_active must be true or false")
	}
}
