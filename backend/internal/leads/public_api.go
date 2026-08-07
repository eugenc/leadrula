package leads

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/apikeys"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

// RegisterPublicRoutes mounts API-key-authenticated read endpoints under /api/v1.
func (h *Handler) RegisterPublicRoutes(r chi.Router, apikeysSvc *apikeys.Service) {
	r.With(apikeysSvc.RequireLeadsRead).Get("/api/v1/leads", h.publicList)
	r.With(apikeysSvc.RequireLeadsRead).Get("/api/v1/leads/{public_id}", h.publicGet)
	r.With(apikeysSvc.RequireLeadsWrite).Patch("/api/v1/leads/{public_id}", h.publicPatch)
}

func apiKeyPrincipal(r *http.Request) *auth.Principal {
	acct := auth.APIKeyAccountFromContext(r.Context())
	if acct == nil {
		return nil
	}
	return &auth.Principal{
		AccountID:   acct.AccountID,
		AccountType: acct.AccountType,
		Role:        "admin",
	}
}

func (h *Handler) publicList(w http.ResponseWriter, r *http.Request) {
	p := apiKeyPrincipal(r)
	if p == nil {
		httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	q := r.URL.Query()
	if phone := strings.TrimSpace(q.Get("phone")); phone != "" {
		h.publicGetByField(w, r, p, "phone", phone)
		return
	}
	if email := strings.TrimSpace(q.Get("email")); email != "" {
		h.publicGetByField(w, r, p, "email", email)
		return
	}
	if externalID := strings.TrimSpace(q.Get("external_id")); externalID != "" {
		h.publicGetByExternalID(w, r, p, externalID)
		return
	}

	src := q.Get("source")
	if src == "" {
		src = q.Get("campaign")
	}
	f := ListFilters{
		Status:     q.Get("status"),
		Source:     src,
		PipelineID: parseInt(q.Get("pipeline_id")),
		StageID:    parseInt(q.Get("stage_id")),
		Assigned:   parseInt(q.Get("assigned")),
		Tag:        q.Get("tag"),
		Search:     strings.TrimSpace(q.Get("q")),
		ExternalID: strings.TrimSpace(q.Get("external_id")),
	}
	if updatedSince := strings.TrimSpace(q.Get("updated_since")); updatedSince != "" {
		t, err := time.Parse(time.RFC3339, updatedSince)
		if err != nil {
			httpx.WriteError(w, httpx.Validation("invalid updated_since"))
			return
		}
		f.UpdatedSince = &t
	}
	tz := h.svc.AccountTimezone(r.Context(), p.AccountID)
	f.FilterTZ = tz

	actionOn := q.Get("action_on")
	if actionOn != "" {
		if actionOn == "today" {
			actionOn = todayInTZ(tz)
		}
		f.ActionOn = actionOn
		f.ActionTZ = tz
	}
	if q.Get("action_overdue") == "1" {
		f.ActionOverdue = true
	}

	opts := ListOptions{
		ListFilters:         f,
		Page:                int(parseInt(q.Get("page"))),
		Limit:               int(parseInt(q.Get("limit"))),
		Sort:                q.Get("sort"),
		SortDir:             q.Get("sort_dir"),
		All:                 q.Get("all") == "1",
		IncludeEconomics:    parseIncludeFlag(q.Get("include_economics"), true),
		IncludeStageHistory: parseIncludeFlag(q.Get("include_stage_history"), true),
	}

	result, err := h.svc.repo.List(r.Context(), p, opts)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) publicGetByField(w http.ResponseWriter, r *http.Request, p *auth.Principal, field, value string) {
	var (
		l   *Lead
		err error
	)
	switch field {
	case "phone":
		l, err = h.svc.repo.GetByPhone(r.Context(), h.svc.repo.pool, p.AccountID, value)
	case "email":
		l, err = h.svc.repo.GetByEmail(r.Context(), h.svc.repo.pool, p.AccountID, value)
	default:
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "unsupported lookup field")
		return
	}
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.publicRespondLead(w, r, p, l)
}

func (h *Handler) publicGetByExternalID(w http.ResponseWriter, r *http.Request, p *auth.Principal, externalID string) {
	l, err := h.svc.repo.GetByExternalID(r.Context(), h.svc.repo.pool, p.AccountID, externalID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.publicRespondLead(w, r, p, l)
}

func (h *Handler) publicGet(w http.ResponseWriter, r *http.Request) {
	p := apiKeyPrincipal(r)
	if p == nil {
		httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	if externalID := strings.TrimSpace(r.URL.Query().Get("external_id")); externalID != "" {
		h.publicGetByExternalID(w, r, p, externalID)
		return
	}
	publicID := chi.URLParam(r, "public_id")
	l, err := h.svc.repo.GetByPublicID(r.Context(), h.svc.repo.pool, p.AccountID, publicID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.publicRespondLead(w, r, p, l)
}

func (h *Handler) publicPatch(w http.ResponseWriter, r *http.Request) {
	p := apiKeyPrincipal(r)
	if p == nil {
		httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	if p.AccountType != "publisher" {
		httpx.Err(w, http.StatusForbidden, httpx.CodeForbidden, "publisher API key required")
		return
	}
	publicID := chi.URLParam(r, "public_id")
	externalID := strings.TrimSpace(r.URL.Query().Get("external_id"))
	if externalID != "" {
		publicID = ""
	}
	leadID, err := h.svc.ResolvePublicLeadID(r.Context(), p, publicID, externalID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	var body struct {
		FirstName *string                    `json:"first_name"`
		LastName  *string                    `json:"last_name"`
		Phone     *string                    `json:"phone"`
		Email     *string                    `json:"email"`
		Address   *string                    `json:"address"`
		City      *string                    `json:"city"`
		State     *string                    `json:"state"`
		Zip       *string                    `json:"zip"`
		Country   *string                    `json:"country"`
		Source    *string                    `json:"source"`
		Tags      *[]string                  `json:"tags"`
		StageID   *int64                     `json:"stage_id"`
		Custom    map[string]json.RawMessage `json:"custom"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	l, err := h.svc.PublicPatch(r.Context(), p, leadID, PublicPatchParams{
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Phone:     body.Phone,
		Email:     body.Email,
		Address:   body.Address,
		City:      body.City,
		State:     body.State,
		Zip:       body.Zip,
		Country:   body.Country,
		Source:    body.Source,
		Tags:      body.Tags,
		StageID:   body.StageID,
		Custom:    body.Custom,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.repo.attachCustomValues(r.Context(), l); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, l)
}

func (h *Handler) publicRespondLead(w http.ResponseWriter, r *http.Request, p *auth.Principal, l *Lead) {
	if err := h.svc.repo.attachCustomValues(r.Context(), l); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.repo.attachLeadNames(r.Context(), l); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.repo.EnrichLeadEconomics(r.Context(), p.AccountType, l); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, l)
}
