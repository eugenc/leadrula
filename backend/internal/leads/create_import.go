package leads

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

const maxImportRows = 1000

// flexInt64 accepts JSON numbers or numeric strings (some clients stringify IDs).
type flexInt64 int64

func (n *flexInt64) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*n = 0
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case float64:
		*n = flexInt64(x)
		return nil
	case string:
		if strings.TrimSpace(x) == "" {
			*n = 0
			return nil
		}
		i, err := strconv.ParseInt(x, 10, 64)
		if err != nil {
			return err
		}
		*n = flexInt64(i)
		return nil
	default:
		return fmt.Errorf("invalid int64")
	}
}

type CreateLeadInput struct {
	FirstName      string                     `json:"first_name"`
	LastName       string                     `json:"last_name"`
	Phone          string                     `json:"phone"`
	Email          string                     `json:"email"`
	Address        string                     `json:"address"`
	City           string                     `json:"city"`
	State          string                     `json:"state"`
	Zip            string                     `json:"zip"`
	Source         string                     `json:"source"`
	ExternalID     string                     `json:"external_id"`
	CampaignName   string                     `json:"campaign_name"` // deprecated: use source
	PipelineID     int64                      `json:"pipeline_id"`
	StageID        int64                      `json:"stage_id"`
	AssignedUserID *int64                     `json:"assigned_user_id"`
	Tags           []string                   `json:"tags"`
	CustomValues   map[string]json.RawMessage `json:"custom_values"`
	ToIntake       bool                       `json:"-"`
}

type ColumnMapping struct {
	CSVColumn string `json:"csv_column"`
	Target    string `json:"target"`
}

// importRow accepts string, number, or bool cell values from JSON clients.
// Arrays (e.g. Papa Parse __parsed_extra) are skipped.
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

type ImportLeadsInput struct {
	Destination string          `json:"destination"`
	PipelineID  flexInt64       `json:"pipeline_id"`
	StageID     flexInt64       `json:"stage_id"`
	DefaultTags []string        `json:"default_tags"`
	Mapping     []ColumnMapping `json:"mapping"`
	Rows        []importRow     `json:"rows"`
}

type ImportRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

type ImportLeadsResult struct {
	Created int              `json:"created"`
	Skipped int              `json:"skipped"`
	Errors  []ImportRowError `json:"errors"`
}

func assertCanCreate(p *auth.Principal) error {
	switch p.Role {
	case "admin", "user":
		return nil
	default:
		return httpx.Forbidden("insufficient role to create leads")
	}
}

func validateCreateInput(in *CreateLeadInput) error {
	in.FirstName = strings.TrimSpace(in.FirstName)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Email = strings.TrimSpace(in.Email)
	if in.FirstName == "" {
		return httpx.Validation("first_name is required")
	}
	if in.Phone == "" && in.Email == "" {
		return httpx.Validation("phone or email is required")
	}
	return nil
}

func (s *Service) CreateLead(ctx context.Context, p *auth.Principal, in CreateLeadInput) (*Lead, error) {
	if err := assertCanCreate(p); err != nil {
		return nil, err
	}
	if err := validateCreateInput(&in); err != nil {
		return nil, err
	}
	leadID, err := s.insertLead(ctx, p, in)
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, p, leadID)
}

func (s *Service) ImportLeads(ctx context.Context, p *auth.Principal, in ImportLeadsInput) (*ImportLeadsResult, error) {
	if err := assertCanCreate(p); err != nil {
		return nil, err
	}
	if len(in.Rows) == 0 {
		return nil, httpx.Validation("no rows to import")
	}
	if len(in.Rows) > maxImportRows {
		return nil, httpx.Validation(fmt.Sprintf("maximum %d rows per import (got %d)", maxImportRows, len(in.Rows)))
	}
	dest := strings.TrimSpace(in.Destination)
	if dest != "pipeline" && dest != "intake" {
		return nil, httpx.Validation("destination must be pipeline or intake")
	}
	if dest == "intake" && p.AccountType != "publisher" {
		return nil, httpx.Validation("intake queue is only available for publisher accounts")
	}
	if dest == "pipeline" && (in.PipelineID == 0 || in.StageID == 0) {
		return nil, httpx.Validation("pipeline_id and stage_id required for pipeline import")
	}

	pipelineID := int64(in.PipelineID)
	stageID := int64(in.StageID)

	result := &ImportLeadsResult{Errors: []ImportRowError{}}
	for i, row := range in.Rows {
		input, err := mapImportRow(row, in.Mapping, pipelineID, stageID, dest == "intake")
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: err.Error()})
			continue
		}
		if len(in.DefaultTags) > 0 {
			input.Tags = append(append([]string{}, in.DefaultTags...), input.Tags...)
		}
		if _, err := s.insertLead(ctx, p, input); err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: err.Error()})
			continue
		}
		result.Created++
	}
	return result, nil
}

func mapImportRow(row importRow, mapping []ColumnMapping, pipelineID, stageID int64, toIntake bool) (CreateLeadInput, error) {
	in := CreateLeadInput{
		PipelineID: pipelineID,
		StageID:    stageID,
		ToIntake:   toIntake,
		CustomValues: map[string]json.RawMessage{},
	}
	for _, m := range mapping {
		if m.Target == "" || m.Target == "skip" {
			continue
		}
		val := strings.TrimSpace(row[m.CSVColumn])
		if val == "" {
			continue
		}
		if strings.HasPrefix(m.Target, "custom_") {
			fid := strings.TrimPrefix(m.Target, "custom_")
			in.CustomValues[fid] = json.RawMessage(strconv.Quote(val))
			continue
		}
		switch m.Target {
		case "first_name":
			in.FirstName = val
		case "last_name":
			in.LastName = val
		case "phone":
			if val != "" && in.Phone == "" {
				in.Phone = val
			}
		case "email":
			if val != "" && in.Email == "" {
				in.Email = val
			}
		case "address":
			in.Address = val
		case "city":
			in.City = val
		case "state":
			in.State = val
		case "zip":
			in.Zip = val
		case "source", "campaign_name":
			in.Source = val
		case "tags":
			for _, part := range strings.Split(val, ",") {
				if t := strings.TrimSpace(part); t != "" {
					in.Tags = append(in.Tags, t)
				}
			}
		}
	}
	if err := validateCreateInput(&in); err != nil {
		return in, err
	}
	return in, nil
}

func (in *CreateLeadInput) resolvedSource() string {
	if s := strings.TrimSpace(in.Source); s != "" {
		return s
	}
	return strings.TrimSpace(in.CampaignName)
}

func (s *Service) insertLead(ctx context.Context, p *auth.Principal, in CreateLeadInput) (int64, error) {
	tx, err := s.repo.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	if in.PipelineID != 0 && in.StageID != 0 {
		stage, err := s.repo.GetStage(ctx, tx, in.StageID)
		if err != nil {
			return 0, err
		}
		if stage.AccountID != p.AccountID {
			return 0, httpx.Validation("stage does not belong to this account")
		}
		if stage.PipelineID != in.PipelineID {
			return 0, httpx.Validation("stage does not belong to the selected pipeline")
		}
	}

	raw, _ := json.Marshal(in)
	source := in.resolvedSource()
	leadID, _, err := s.repo.InsertLead(ctx, tx, p.AccountID, p.AccountID, source, raw)
	if err != nil {
		return 0, err
	}

	builtins := map[string]string{
		"first_name": in.FirstName, "last_name": in.LastName,
		"phone": in.Phone, "email": in.Email,
		"address": in.Address, "city": in.City, "state": in.State, "zip": in.Zip,
	}
	if source != "" {
		builtins["source"] = source
	}
	if ext := strings.TrimSpace(in.ExternalID); ext != "" {
		builtins["external_id"] = ext
	}
	for field, val := range builtins {
		if val == "" {
			continue
		}
		if err := s.repo.SetBuiltinField(ctx, tx, leadID, field, val); err != nil {
			return 0, err
		}
	}

	for fidStr, val := range in.CustomValues {
		fid, err := strconv.ParseInt(fidStr, 10, 64)
		if err != nil || fid == 0 {
			continue
		}
		if err := s.repo.UpsertCustomValue(ctx, tx, leadID, fid, val); err != nil {
			return 0, err
		}
	}

	if len(in.Tags) > 0 {
		if err := s.repo.setTags(ctx, tx, p.AccountID, leadID, in.Tags); err != nil {
			return 0, err
		}
	}

	if in.AssignedUserID != nil && p.Role == "admin" {
		if err := s.repo.setAssignee(ctx, tx, p.AccountID, leadID, in.AssignedUserID); err != nil {
			return 0, err
		}
	}

	if in.ToIntake {
		if err := s.repo.EnqueueIntake(ctx, tx, leadID, source, raw); err != nil {
			return 0, err
		}
	} else if in.PipelineID != 0 && in.StageID != 0 {
		if err := s.repo.PlaceInPipeline(ctx, tx, leadID, p.AccountID, in.PipelineID, in.StageID, nil); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	// Fire outbound webhook trigger for lead creation (best-effort, post-commit).
	if s.webhooks != nil {
		lead, err := s.repo.GetByID(ctx, s.repo.Pool(), leadID)
		if err == nil {
			_ = LoadCustomValues(ctx, s.repo.Pool(), lead)
			s.fireOutbound(ctx, p.AccountID, "lead.create", lead, PipelineContext{
				PipelineID: lead.PipelineID,
				StageID:    lead.StageID,
			})
		}
	}
	return leadID, nil
}

// EnqueueIntake adds a lead to the publisher intake queue.
func (r *Repository) EnqueueIntake(ctx context.Context, q database.Querier, leadID int64, source string, rawPayload []byte) error {
	if len(rawPayload) == 0 {
		rawPayload = []byte("{}")
	}
	var src interface{}
	if source != "" {
		src = source
	}
	_, err := q.Exec(ctx,
		`INSERT INTO lead_intake_queue(lead_id, raw_payload, source) VALUES ($1,$2,$3)`,
		leadID, rawPayload, src)
	return err
}
