package notifications

import (
	"net/http"
	"strconv"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/notifications", h.list)
	r.Get("/notifications/settings", h.getSettings)
	r.Patch("/notifications/settings", h.patchSettings)
	r.Patch("/notifications/{id}/read", h.markRead)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.List(r.Context(), p.UserID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	out, err := h.svc.GetSettings(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) patchSettings(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body SettingsPatch
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	out, err := h.svc.PatchSettings(r.Context(), p, body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "invalid id")
		return
	}
	if err := h.svc.MarkRead(r.Context(), p.UserID, id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
