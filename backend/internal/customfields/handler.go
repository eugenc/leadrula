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
	r.Get("/custom-field-folders", h.listFolders)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole("admin"))
		r.Post("/custom-fields", h.createField)
		r.Post("/custom-fields/import", h.importFields)
		r.Post("/custom-fields/layout", h.saveLayout)
		r.Patch("/custom-fields/{id}", h.updateField)
		r.Delete("/custom-fields/{id}", h.deleteField)
		r.Post("/custom-field-folders", h.createFolder)
		r.Patch("/custom-field-folders/{id}", h.updateFolder)
		r.Delete("/custom-field-folders/{id}", h.deleteFolder)
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
		Format   *string         `json:"format"`
		Options  json.RawMessage `json:"options"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	f, err := h.svc.CreateField(r.Context(), p.AccountID, body.Name, body.FieldKey, body.Type, body.Options, body.Format)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, f)
}

func (h *Handler) importFields(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body ImportFieldsInput
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	result, err := h.svc.ImportFields(r.Context(), p.AccountID, body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) updateField(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name     *string         `json:"name"`
		FieldKey *string         `json:"field_key"`
		Format   *string         `json:"format"`
		Options  json.RawMessage `json:"options"`
		Position *int            `json:"position"`
		IsActive *bool           `json:"is_active"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	f, err := h.svc.UpdateField(r.Context(), p.AccountID, idParam(r), body.Name, body.FieldKey, body.Options, body.Format, body.Position, body.IsActive)
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

func (h *Handler) listFolders(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListFolders(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) createFolder(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name string `json:"name"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	f, err := h.svc.CreateFolder(r.Context(), p.AccountID, body.Name)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, f)
}

func (h *Handler) updateFolder(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name     *string `json:"name"`
		Position *int    `json:"position"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	f, err := h.svc.UpdateFolder(r.Context(), p.AccountID, idParam(r), body.Name, body.Position)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, f)
}

func (h *Handler) deleteFolder(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteFolder(r.Context(), p.AccountID, idParam(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) saveLayout(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body Layout
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.SaveLayout(r.Context(), p.AccountID, body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func idParam(r *http.Request) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id
}
