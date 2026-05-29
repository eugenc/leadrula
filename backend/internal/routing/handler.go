package routing

import (
	"net/http"
	"strconv"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterRoutes mounts routing campaigns + field map (publisher admin).
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/routing-campaigns", h.list)
	r.Get("/routing-campaigns/{id}/field-map", h.listFieldMap)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole("admin"))
		r.Post("/routing-campaigns", h.create)
		r.Patch("/routing-campaigns/{id}", h.update)
		r.Delete("/routing-campaigns/{id}", h.delete)
		r.Post("/routing-campaigns/{id}/field-map", h.addFieldMap)
		r.Delete("/field-map/{mapId}", h.deleteFieldMap)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListCampaigns(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		CampaignName     string `json:"campaign_name"`
		TargetPipelineID int64  `json:"target_pipeline_id"`
		TargetStageID    int64  `json:"target_stage_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	c, err := h.svc.CreateCampaign(r.Context(), p.AccountID, body.CampaignName, body.TargetPipelineID, body.TargetStageID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		CampaignName     *string `json:"campaign_name"`
		TargetPipelineID *int64  `json:"target_pipeline_id"`
		TargetStageID    *int64  `json:"target_stage_id"`
		IsActive         *bool   `json:"is_active"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	c, err := h.svc.UpdateCampaign(r.Context(), p.AccountID, idp(r, "id"), body.CampaignName, body.TargetPipelineID, body.TargetStageID, body.IsActive)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteCampaign(r.Context(), p.AccountID, idp(r, "id")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listFieldMap(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	cid := idp(r, "id")
	ok, err := h.svc.CampaignOwnedBy(r.Context(), p.AccountID, cid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if !ok {
		httpx.Err(w, http.StatusNotFound, httpx.CodeNotFound, "campaign not found")
		return
	}
	items, err := h.svc.ListFieldMap(r.Context(), cid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) addFieldMap(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	cid := idp(r, "id")
	ok, err := h.svc.CampaignOwnedBy(r.Context(), p.AccountID, cid)
	if err != nil || !ok {
		httpx.Err(w, http.StatusNotFound, httpx.CodeNotFound, "campaign not found")
		return
	}
	var body struct {
		SourceKey     string  `json:"source_key"`
		TargetType    string  `json:"target_type"`
		BuiltinField  *string `json:"builtin_field"`
		CustomFieldID *int64  `json:"custom_field_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	e, err := h.svc.AddFieldMap(r.Context(), cid, body.SourceKey, body.TargetType, body.BuiltinField, body.CustomFieldID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, e)
}

func (h *Handler) deleteFieldMap(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteFieldMap(r.Context(), idp(r, "mapId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func idp(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	return id
}
