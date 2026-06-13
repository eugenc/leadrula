package pipelines

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

// RegisterRoutes mounts pipeline + stage routes. Mutations require admin.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/pipelines", h.list)
	r.Get("/pipelines/{id}/stages", h.listStages)
	r.Get("/pipelines/{id}/disqualification-reasons", h.listPipelineReasons)
	r.Get("/stages/{id}/rules", h.listRules)
	r.Get("/stages/{id}/disqualification-reasons", h.listStageReasons)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole("admin"))
		r.Post("/pipelines", h.create)
		r.Patch("/pipelines/{id}", h.update)
		r.Delete("/pipelines/{id}", h.delete)
		r.Post("/pipelines/{id}/stages", h.createStage)
		r.Post("/pipelines/{id}/stages/reorder", h.reorder)
		r.Patch("/stages/{id}", h.updateStage)
		r.Delete("/stages/{id}", h.deleteStage)
		r.Post("/stages/{id}/disqualification-reasons", h.createStageReason)
		r.Patch("/disqualification-reasons/{id}", h.updateStageReason)
		r.Delete("/disqualification-reasons/{id}", h.deleteStageReason)
		r.Post("/stages/{id}/rules", h.createRule)
		r.Patch("/stage-rules/{id}", h.updateRule)
		r.Delete("/stage-rules/{id}", h.deleteRule)
		r.Post("/stages/{id}/rules/reorder", h.reorderRules)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.List(r.Context(), p)
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
	pl, err := h.svc.Create(r.Context(), p, body.Name)
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
	pl, err := h.svc.Update(r.Context(), p, id, body.Name, body.Position)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, pl)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.Delete(r.Context(), p, idParam(r, "id")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listStages(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListStages(r.Context(), p, idParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) createStage(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name      string `json:"name"`
		Color     string `json:"color"`
		StageType string `json:"stage_type"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	st, err := h.svc.CreateStage(r.Context(), p, idParam(r, "id"), body.Name, body.Color, body.StageType)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, st)
}

func (h *Handler) updateStage(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name      *string `json:"name"`
		Color     *string `json:"color"`
		StageType *string `json:"stage_type"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	st, err := h.svc.UpdateStage(r.Context(), p, idParam(r, "id"), body.Name, body.Color, body.StageType)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, st)
}

func (h *Handler) deleteStage(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteStage(r.Context(), p, idParam(r, "id")); err != nil {
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
	if err := h.svc.Reorder(r.Context(), p, idParam(r, "id"), body.OrderedStageIDs); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func idParam(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	return id
}

func (h *Handler) listRules(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListRules(r.Context(), p, idParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		ConditionLogic string          `json:"condition_logic"`
		Conditions     json.RawMessage `json:"conditions"`
		Actions        json.RawMessage `json:"actions"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	rule, err := h.svc.CreateRule(r.Context(), p, idParam(r, "id"), body.ConditionLogic, body.Conditions, body.Actions)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, rule)
}

func (h *Handler) updateRule(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		ConditionLogic *string         `json:"condition_logic"`
		Conditions     json.RawMessage `json:"conditions"`
		Actions        json.RawMessage `json:"actions"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	var conds, acts json.RawMessage
	if body.Conditions != nil {
		conds = body.Conditions
	}
	if body.Actions != nil {
		acts = body.Actions
	}
	rule, err := h.svc.UpdateRule(r.Context(), p, idParam(r, "id"), body.ConditionLogic, conds, acts)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rule)
}

func (h *Handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteRule(r.Context(), p, idParam(r, "id")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) reorderRules(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		OrderedRuleIDs []int64 `json:"ordered_rule_ids"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.ReorderRules(r.Context(), p, idParam(r, "id"), body.OrderedRuleIDs); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listStageReasons(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListStageReasons(r.Context(), p, idParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) listPipelineReasons(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListPipelineReasons(r.Context(), p, idParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) createStageReason(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Label string `json:"label"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	d, err := h.svc.CreateStageReason(r.Context(), p, idParam(r, "id"), body.Label)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, d)
}

func (h *Handler) updateStageReason(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Label    *string `json:"label"`
		Position *int    `json:"position"`
		IsActive *bool   `json:"is_active"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	d, err := h.svc.UpdateStageReason(r.Context(), p, idParam(r, "id"), body.Label, body.Position, body.IsActive)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

func (h *Handler) deleteStageReason(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteStageReason(r.Context(), p, idParam(r, "id")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
