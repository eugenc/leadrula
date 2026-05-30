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

const maxImportRows = 500

type CreateLeadInput struct {
	FirstName      string                     `json:"first_name"`
	LastName       string                     `json:"last_name"`
	Phone          string                     `json:"phone"`
	Email          string                     `json:"email"`
	Address        string                     `json:"address"`
	City           string                     `json:"city"`
	State          string                     `json:"state"`
	Zip            string                     `json:"zip"`
	CampaignName   string                     `json:"campaign_name"`
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

type ImportLeadsInput struct {
	Destination string              `json:"destination"`
	PipelineID  int64               `json:"pipeline_id"`
	StageID     int64               `json:"stage_id"`
	Mapping     []ColumnMapping     `json:"mapping"`
	Rows        []map[string]string `json:"rows"`
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
		return nil, httpx.Validation(fmt.Sprintf("maximum %d rows per import", maxImportRows))
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

	result := &ImportLeadsResult{Errors: []ImportRowError{}}
	for i, row := range in.Rows {
		input, err := mapImportRow(row, in.Mapping, in.PipelineID, in.StageID, dest == "intake")
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: i + 1, Message: err.Error()})
			continue
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

func mapImportRow(row map[string]string, mapping []ColumnMapping, pipelineID, stageID int64, toIntake bool) (CreateLeadInput, error) {
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
			in.Phone = val
		case "email":
			in.Email = val
		case "address":
			in.Address = val
		case "city":
			in.City = val
		case "state":
			in.State = val
		case "zip":
			in.Zip = val
		case "campaign_name":
			in.CampaignName = val
		case "tags":
			in.Tags = append(in.Tags, val)
		}
	}
	if err := validateCreateInput(&in); err != nil {
		return in, err
	}
	return in, nil
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
	leadID, _, err := s.repo.InsertLead(ctx, tx, p.AccountID, p.AccountID, in.CampaignName, raw)
	if err != nil {
		return 0, err
	}

	builtins := map[string]string{
		"first_name": in.FirstName, "last_name": in.LastName,
		"phone": in.Phone, "email": in.Email,
		"address": in.Address, "city": in.City, "state": in.State, "zip": in.Zip,
	}
	if in.CampaignName != "" {
		builtins["campaign_name"] = in.CampaignName
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
		if err := s.repo.EnqueueIntake(ctx, tx, leadID, in.CampaignName, raw); err != nil {
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
	return leadID, nil
}

// EnqueueIntake adds a lead to the publisher intake queue.
func (r *Repository) EnqueueIntake(ctx context.Context, q database.Querier, leadID int64, campaignName string, rawPayload []byte) error {
	if len(rawPayload) == 0 {
		rawPayload = []byte("{}")
	}
	var cn interface{}
	if campaignName != "" {
		cn = campaignName
	}
	_, err := q.Exec(ctx,
		`INSERT INTO lead_intake_queue(lead_id, raw_payload, campaign_name) VALUES ($1,$2,$3)`,
		leadID, rawPayload, cn)
	return err
}
