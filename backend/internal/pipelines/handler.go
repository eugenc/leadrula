package pipelines

import (
	"net/http"
	"strconv"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterRoutes mounts pipeline + stage routes. Mutations require admin.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/pipelines", h.list)
	r.Get("/pipelines/{id}/stages", h.listStages)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole("admin"))
		r.Post("/pipelines", h.create)
		r.Patch("/pipelines/{id}", h.update)
		r.Delete("/pipelines/{id}", h.delete)
		r.Post("/pipelines/{id}/stages", h.createStage)
		r.Post("/pipelines/{id}/stages/reorder", h.reorder)
		r.Patch("/stages/{id}", h.updateStage)
		r.Delete("/stages/{id}", h.deleteStage)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.List(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name string `json:"name"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	pl, err := h.svc.Create(r.Context(), p.AccountID, body.Name)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, pl)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id := idParam(r, "id")
	var body struct {
		Name     *string `json:"name"`
		Position *int    `json:"position"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	pl, err := h.svc.Update(r.Context(), p.AccountID, id, body.Name, body.Position)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, pl)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.Delete(r.Context(), p.AccountID, idParam(r, "id")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listStages(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListStages(r.Context(), p.AccountID, idParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) createStage(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name                   string `json:"name"`
		PromptActionDatetime   *bool  `json:"prompt_action_datetime"`
		PromptDisqualification *bool  `json:"prompt_disqualification"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	promptAction := true
	if body.PromptActionDatetime != nil {
		promptAction = *body.PromptActionDatetime
	}
	promptDisq := false
	if body.PromptDisqualification != nil {
		promptDisq = *body.PromptDisqualification
	}
	st, err := h.svc.CreateStage(r.Context(), p.AccountID, idParam(r, "id"), body.Name, promptAction, promptDisq)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, st)
}

func (h *Handler) updateStage(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name                   *string `json:"name"`
		PromptActionDatetime   *bool   `json:"prompt_action_datetime"`
		PromptDisqualification *bool   `json:"prompt_disqualification"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	st, err := h.svc.UpdateStage(r.Context(), p.AccountID, idParam(r, "id"), body.Name, body.PromptActionDatetime, body.PromptDisqualification)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, st)
}

func (h *Handler) deleteStage(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteStage(r.Context(), p.AccountID, idParam(r, "id")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) reorder(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		OrderedStageIDs []int64 `json:"ordered_stage_ids"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.Reorder(r.Context(), p.AccountID, idParam(r, "id"), body.OrderedStageIDs); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func idParam(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	return id
}
