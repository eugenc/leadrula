package dashboard

import (
	"net/http"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/dashboard/views", h.listViews)
	r.With(auth.RequireRole("admin")).Post("/dashboard/views", h.createView)
	r.With(auth.RequireRole("admin")).Patch("/dashboard/views/{viewId}", h.updateView)
	r.With(auth.RequireRole("admin")).Delete("/dashboard/views/{viewId}", h.deleteView)
}

func (h *Handler) listViews(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	views, err := h.svc.ListViews(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, views)
}

func (h *Handler) createView(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body CreateInput
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	v, err := h.svc.CreateView(r.Context(), p, body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, v)
}

func (h *Handler) updateView(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body UpdateInput
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	v, err := h.svc.UpdateView(r.Context(), p, chi.URLParam(r, "viewId"), body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

func (h *Handler) deleteView(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteView(r.Context(), p, chi.URLParam(r, "viewId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
