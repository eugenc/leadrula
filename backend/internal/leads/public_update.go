package leads

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

// PublicPatchParams is the VoiceUni-friendly flat update body for PATCH /api/v1/leads.
type PublicPatchParams struct {
	FirstName *string
	LastName  *string
	Phone     *string
	Email     *string
	Address   *string
	City      *string
	State     *string
	Zip       *string
	Country   *string
	Source    *string
	Tags      *[]string
	StageID   *int64
	Custom    map[string]json.RawMessage
}

func (s *Service) PublicPatch(ctx context.Context, p *auth.Principal, leadID int64, params PublicPatchParams) (*Lead, error) {
	if p.AccountType != "publisher" {
		return nil, httpx.Forbidden("publisher API key required")
	}
	if _, err := s.repo.Get(ctx, p, leadID); err != nil {
		return nil, err
	}

	fields := map[string]*string{}
	setField := func(key string, val *string) {
		if val != nil {
			fields[key] = val
		}
	}
	setField("first_name", params.FirstName)
	setField("last_name", params.LastName)
	setField("phone", params.Phone)
	setField("email", params.Email)
	setField("address", params.Address)
	setField("city", params.City)
	setField("state", params.State)
	setField("zip", params.Zip)
	setField("country", params.Country)
	setField("source", params.Source)

	for k := range fields {
		if IsMoneyBuiltin(k) {
			return nil, httpx.Validation("cost and revenue cannot be edited manually")
		}
	}
	if len(fields) > 0 {
		if err := s.repo.UpdateBuiltins(ctx, p.AccountID, leadID, fields); err != nil {
			return nil, err
		}
	}
	for fieldKey, val := range params.Custom {
		fieldKey = strings.TrimSpace(fieldKey)
		if fieldKey == "" {
			continue
		}
		fid, err := s.repo.CustomFieldIDByKey(ctx, p.AccountID, fieldKey)
		if err != nil {
			return nil, err
		}
		if err := s.repo.UpsertCustomValue(ctx, s.repo.pool, leadID, fid, val); err != nil {
			return nil, err
		}
	}
	if params.Tags != nil {
		if err := s.repo.SetTags(ctx, p.AccountID, leadID, *params.Tags); err != nil {
			return nil, err
		}
	}
	if params.StageID != nil && *params.StageID > 0 {
		if _, _, err := s.ChangeStage(ctx, p, leadID, *params.StageID, nil, nil); err != nil {
			return nil, err
		}
	}
	return s.repo.Get(ctx, p, leadID)
}

func (s *Service) ResolvePublicLeadID(ctx context.Context, p *auth.Principal, publicID, externalID string) (int64, error) {
	publicID = strings.TrimSpace(publicID)
	externalID = strings.TrimSpace(externalID)
	if publicID != "" {
		l, err := s.repo.GetByPublicID(ctx, s.repo.pool, p.AccountID, publicID)
		if err != nil {
			return 0, err
		}
		return l.ID, nil
	}
	if externalID != "" {
		l, err := s.repo.GetByExternalID(ctx, s.repo.pool, p.AccountID, externalID)
		if err != nil {
			return 0, err
		}
		return l.ID, nil
	}
	return 0, httpx.Validation("public_id or external_id required")
}
