package leads

import (
	"net/http"
	"strings"

	"github.com/echayko/leadrula/backend/internal/apikeys"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

// RegisterPublicRoutes mounts API-key-authenticated read endpoints under /api/v1.
func (h *Handler) RegisterPublicRoutes(r chi.Router, apikeysSvc *apikeys.Service) {
	r.With(apikeysSvc.RequireLeadsRead).Get("/api/v1/leads", h.publicList)
	r.With(apikeysSvc.RequireLeadsRead).Get("/api/v1/leads/{public_id}", h.publicGet)
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
		ListFilters: f,
		Page:        int(parseInt(q.Get("page"))),
		Limit:       int(parseInt(q.Get("limit"))),
		Sort:        q.Get("sort"),
		SortDir:     q.Get("sort_dir"),
		All:         q.Get("all") == "1",
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
	httpx.JSON(w, http.StatusOK, &ListResult{
		Items: []Lead{*l},
		Total: 1,
		Page:  1,
		Limit: 1,
	})
}

func (h *Handler) publicGet(w http.ResponseWriter, r *http.Request) {
	p := apiKeyPrincipal(r)
	if p == nil {
		httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	publicID := chi.URLParam(r, "public_id")
	l, err := h.svc.repo.GetByPublicID(r.Context(), h.svc.repo.pool, p.AccountID, publicID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
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
