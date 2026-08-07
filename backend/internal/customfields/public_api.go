package customfields

import (
	"net/http"

	"github.com/echayko/leadrula/backend/internal/apikeys"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type publicFieldItem struct {
	ID       int64  `json:"id"`
	FieldKey string `json:"field_key"`
	Name     string `json:"name"`
	Type     string `json:"type"`
}

// RegisterPublicRoutes mounts API-key custom field reads under /api/v1.
func (h *Handler) RegisterPublicRoutes(r chi.Router, apikeysSvc *apikeys.Service) {
	r.With(apikeysSvc.RequireLeadsRead).Get("/api/v1/custom-fields", h.publicListFields)
}

func (h *Handler) publicListFields(w http.ResponseWriter, r *http.Request) {
	acct := auth.APIKeyAccountFromContext(r.Context())
	if acct == nil {
		httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	fields, err := h.svc.ListFields(r.Context(), acct.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	items := make([]publicFieldItem, 0, len(fields))
	for _, f := range fields {
		if !f.IsActive {
			continue
		}
		items = append(items, publicFieldItem{
			ID: f.ID, FieldKey: f.FieldKey, Name: f.Name, Type: f.Type,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": items}})
}
