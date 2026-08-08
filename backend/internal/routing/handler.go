package routing

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/permissions"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterBuyer mounts buyer route list + admin CRUD for buyer-owned routes.
func (h *Handler) RegisterBuyer(r chi.Router) {
	r.Get("/routes", h.buyerListRoutes)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(permissions.ActionPipelinesRouting))
		r.Post("/routes", h.buyerCreateRoute)
		r.Patch("/routes/{id}", h.buyerUpdateRoute)
		r.Delete("/routes/{id}", h.buyerDeleteRoute)
	})
}

// RegisterRoutes mounts sources, routes, and field maps (publisher admin).
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/sources", h.listSources)
	r.Get("/sources/{id}/field-map", h.listSourceFieldMap)
	r.Get("/sources/{id}/sample-payload", h.getSourceSamplePayload)
	r.Get("/routes", h.listRoutes)
	r.Get("/routes/{id}/field-map", h.listRouteFieldMap)
	r.Get("/routes/{id}/field-map/options", h.getRouteFieldMapOptions)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(permissions.ActionPipelinesRouting))
		r.Post("/sources", h.createSource)
		r.Patch("/sources/{id}", h.updateSource)
		r.Delete("/sources/{id}", h.deleteSource)
		r.Post("/sources/{id}/field-map", h.addSourceFieldMap)
		r.Delete("/source-field-map/{mapId}", h.deleteSourceFieldMap)

		r.Post("/routes", h.createRoute)
		r.Patch("/routes/{id}", h.updateRoute)
		r.Delete("/routes/{id}", h.deleteRoute)
		r.Post("/routes/{id}/field-map", h.addRouteFieldMap)
		r.Post("/routes/{id}/buyer-custom-fields", h.createBuyerCustomField)
		r.Delete("/route-field-map/{mapId}", h.deleteRouteFieldMap)
	})
}

func (h *Handler) listSources(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListSources(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) createSource(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name           string                     `json:"name"`
		Slug           string                     `json:"slug"`
		Type           string                     `json:"type"`
		APIKeyRequired *bool                      `json:"api_key_required"`
		Call           *CallSourceParams          `json:"call"`
		Appointment    *AppointmentSourceParams   `json:"appointment"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	src, err := h.svc.CreateSource(r.Context(), p.AccountID, body.Name, body.Slug, body.Type, body.APIKeyRequired, body.Call, body.Appointment)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, src)
}

func (h *Handler) updateSource(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name           *string                    `json:"name"`
		Slug           *string                    `json:"slug"`
		IsActive       *bool                      `json:"is_active"`
		APIKeyRequired *bool                      `json:"api_key_required"`
		Call           *CallSourceParams          `json:"call"`
		Appointment    *AppointmentSourceParams   `json:"appointment"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	src, err := h.svc.UpdateSource(r.Context(), p.AccountID, idp(r, "id"), body.Name, body.Slug, body.IsActive, body.APIKeyRequired, body.Call, body.Appointment)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, src)
}

func (h *Handler) deleteSource(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteSource(r.Context(), p.AccountID, idp(r, "id")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listSourceFieldMap(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	sid := idp(r, "id")
	ok, err := h.svc.SourceOwnedBy(r.Context(), p.AccountID, sid)
	if err != nil || !ok {
		httpx.Err(w, http.StatusNotFound, httpx.CodeNotFound, "source not found")
		return
	}
	items, err := h.svc.ListSourceFieldMap(r.Context(), sid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) getSourceSamplePayload(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	sid := idp(r, "id")
	ok, err := h.svc.SourceOwnedBy(r.Context(), p.AccountID, sid)
	if err != nil || !ok {
		httpx.Err(w, http.StatusNotFound, httpx.CodeNotFound, "source not found")
		return
	}
	out, err := h.svc.LatestSourceSamplePayload(r.Context(), p.AccountID, sid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) addSourceFieldMap(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	sid := idp(r, "id")
	ok, err := h.svc.SourceOwnedBy(r.Context(), p.AccountID, sid)
	if err != nil || !ok {
		httpx.Err(w, http.StatusNotFound, httpx.CodeNotFound, "source not found")
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
	e, err := h.svc.AddSourceFieldMap(r.Context(), p.AccountID, sid, body.SourceKey, body.TargetType, body.BuiltinField, body.CustomFieldID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, e)
}

func (h *Handler) deleteSourceFieldMap(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteSourceFieldMap(r.Context(), idp(r, "mapId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listRoutes(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListRoutes(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) buyerListRoutes(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListRoutesForBuyer(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) createRoute(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body CreateRouteParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	rt, err := h.svc.CreateRoute(r.Context(), p.AccountID, body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, rt)
}

func (h *Handler) buyerCreateRoute(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body CreateRouteParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	rt, err := h.svc.CreateBuyerRoute(r.Context(), p.AccountID, body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, rt)
}

func (h *Handler) updateRoute(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body UpdateRouteParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	rt, err := h.svc.UpdateRoute(r.Context(), p.AccountID, idp(r, "id"), body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rt)
}

func (h *Handler) buyerUpdateRoute(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rid := idp(r, "id")
	ok, err := h.svc.RouteOwnedByBuyer(r.Context(), p.AccountID, rid)
	if err != nil || !ok {
		httpx.Err(w, http.StatusNotFound, httpx.CodeNotFound, "route not found")
		return
	}
	var body UpdateRouteParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	rt, err := h.svc.UpdateBuyerRoute(r.Context(), p.AccountID, rid, body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rt)
}

func (h *Handler) deleteRoute(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteRoute(r.Context(), p.AccountID, idp(r, "id")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) buyerDeleteRoute(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteBuyerRoute(r.Context(), p.AccountID, idp(r, "id")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listRouteFieldMap(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rid := idp(r, "id")
	ok, err := h.svc.RouteOwnedBy(r.Context(), p.AccountID, rid)
	if err != nil || !ok {
		httpx.Err(w, http.StatusNotFound, httpx.CodeNotFound, "route not found")
		return
	}
	items, err := h.svc.ListRouteFieldMap(r.Context(), rid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) getRouteFieldMapOptions(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rid := idp(r, "id")
	ok, err := h.svc.RouteOwnedBy(r.Context(), p.AccountID, rid)
	if err != nil || !ok {
		httpx.Err(w, http.StatusNotFound, httpx.CodeNotFound, "route not found")
		return
	}
	opts, err := h.svc.RouteFieldMapOptions(r.Context(), p.AccountID, rid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, opts)
}

func (h *Handler) addRouteFieldMap(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rid := idp(r, "id")
	ok, err := h.svc.RouteOwnedBy(r.Context(), p.AccountID, rid)
	if err != nil || !ok {
		httpx.Err(w, http.StatusNotFound, httpx.CodeNotFound, "route not found")
		return
	}
	var body struct {
		SrcType          string  `json:"src_type"`
		SrcBuiltin       *string `json:"src_builtin"`
		SrcCustomFieldID *int64  `json:"src_custom_field_id"`
		DstType          string  `json:"dst_type"`
		DstBuiltin       *string `json:"dst_builtin"`
		DstCustomFieldID *int64  `json:"dst_custom_field_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	e, err := h.svc.AddRouteFieldMap(r.Context(), p.AccountID, rid, body.SrcType, body.SrcBuiltin, body.SrcCustomFieldID,
		body.DstType, body.DstBuiltin, body.DstCustomFieldID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, e)
}

func (h *Handler) deleteRouteFieldMap(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteRouteFieldMap(r.Context(), idp(r, "mapId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) createBuyerCustomField(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rid := idp(r, "id")
	ok, err := h.svc.RouteOwnedBy(r.Context(), p.AccountID, rid)
	if err != nil || !ok {
		httpx.Err(w, http.StatusNotFound, httpx.CodeNotFound, "route not found")
		return
	}
	var body struct {
		Name     string          `json:"name"`
		FieldKey string          `json:"field_key"`
		Type     string          `json:"type"`
		Options  json.RawMessage `json:"options"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	f, err := h.svc.CreateBuyerCustomField(r.Context(), p.AccountID, rid, body.Name, body.FieldKey, body.Type, body.Options)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, f)
}

func idp(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	return id
}
