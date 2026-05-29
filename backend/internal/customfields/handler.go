package customfields

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/custom-fields", h.listFields)
	r.Get("/disqualification-reasons", h.listReasons)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole("admin"))
		r.Post("/custom-fields", h.createField)
		r.Patch("/custom-fields/{id}", h.updateField)
		r.Delete("/custom-fields/{id}", h.deleteField)
		r.Post("/disqualification-reasons", h.createReason)
		r.Patch("/disqualification-reasons/{id}", h.updateReason)
		r.Delete("/disqualification-reasons/{id}", h.deleteReason)
	})
}

func (h *Handler) listFields(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListFields(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) createField(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name     string          `json:"name"`
		FieldKey string          `json:"field_key"`
		Type     string          `json:"type"`
		Options  json.RawMessage `json:"options"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	f, err := h.svc.CreateField(r.Context(), p.AccountID, body.Name, body.FieldKey, body.Type, body.Options)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, f)
}

func (h *Handler) updateField(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name     *string         `json:"name"`
		Options  json.RawMessage `json:"options"`
		Position *int            `json:"position"`
		IsActive *bool           `json:"is_active"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	f, err := h.svc.UpdateField(r.Context(), p.AccountID, idParam(r), body.Name, body.Options, body.Position, body.IsActive)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, f)
}

func (h *Handler) deleteField(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteField(r.Context(), p.AccountID, idParam(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listReasons(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListReasons(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) createReason(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Label string `json:"label"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	d, err := h.svc.CreateReason(r.Context(), p.AccountID, body.Label)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, d)
}

func (h *Handler) updateReason(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Label    *string `json:"label"`
		Position *int    `json:"position"`
		IsActive *bool   `json:"is_active"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	d, err := h.svc.UpdateReason(r.Context(), p.AccountID, idParam(r), body.Label, body.Position, body.IsActive)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

func (h *Handler) deleteReason(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteReason(r.Context(), p.AccountID, idParam(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func idParam(r *http.Request) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id
}
