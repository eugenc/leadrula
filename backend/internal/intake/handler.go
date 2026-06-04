package intake

import (
	"net/http"
	"strconv"
	"time"

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

func resolvePublisherID(r *http.Request) (int64, bool) {
	if a := auth.APIKeyAccountFromContext(r.Context()); a != nil && a.AccountType == "publisher" {
		return a.AccountID, true
	}
	if p := auth.FromContext(r.Context()); p != nil && p.AccountType == "publisher" {
		return p.AccountID, true
	}
	return 0, false
}

// RegisterPublicRoutes mounts the API-key-authenticated ingest endpoints.
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/api/v1/leads", h.ingest)
	r.Post("/api/v1/sources/{slug}", h.ingestSource)
	r.Post("/api/v1/leads/{id}/action", h.action)
}

// RegisterQueueRoutes mounts the publisher admin intake queue routes.
func (h *Handler) RegisterQueueRoutes(r chi.Router) {
	r.Get("/intake-queue", h.listQueue)
	r.With(auth.RequireRole("admin")).Post("/intake-queue/{id}/route", h.route)
	r.With(auth.RequireRole("admin")).Post("/intake-queue/{id}/reject", h.reject)
	r.With(auth.RequireRole("admin")).Post("/intake-queue/{id}/map-field", h.mapField)
}

// RegisterBuyerRoutes mounts buyer read-only contract routing log routes.
func (h *Handler) RegisterBuyerRoutes(r chi.Router) {
	r.With(auth.RequireRole("admin")).Get("/routing-log", h.listRoutingLog)
}

func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	publisherID, ok := resolvePublisherID(r)
	if !ok {
		httpx.Err(w, http.StatusForbidden, httpx.CodeForbidden, "publisher account required")
		return
	}
	var raw map[string]any
	if !httpx.DecodeJSON(w, r, &raw) {
		return
	}
	res, err := h.svc.Ingest(r.Context(), publisherID, raw)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, res)
}

func (h *Handler) ingestSource(w http.ResponseWriter, r *http.Request) {
	publisherID, ok := resolvePublisherID(r)
	if !ok {
		httpx.Err(w, http.StatusForbidden, httpx.CodeForbidden, "publisher account required")
		return
	}
	slug := chi.URLParam(r, "slug")
	var raw map[string]any
	if !httpx.DecodeJSON(w, r, &raw) {
		return
	}
	res, err := h.svc.IngestFromSource(r.Context(), publisherID, slug, raw)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, res)
}

func (h *Handler) action(w http.ResponseWriter, r *http.Request) {
	publicID := chi.URLParam(r, "id")
	var body struct {
		ActionAt *time.Time `json:"action_at"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.SetActionByPublicID(r.Context(), publicID, body.ActionAt); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listQueue(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	q := r.URL.Query()
	result, err := h.svc.ListQueue(r.Context(), p.AccountID, ListQueueParams{
		Status: q.Get("status"),
		Page:   int(parseInt(q.Get("page"))),
		Limit:  int(parseInt(q.Get("limit"))),
		Search: q.Get("q"),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) listRoutingLog(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	q := r.URL.Query()
	result, err := h.svc.ListRoutingLogForBuyer(r.Context(), p.AccountID, ListQueueParams{
		Status: q.Get("status"),
		Page:   int(parseInt(q.Get("page"))),
		Limit:  int(parseInt(q.Get("limit"))),
		Search: q.Get("q"),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) route(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		PipelineID int64 `json:"pipeline_id"`
		StageID    int64 `json:"stage_id"`
		BuyerID    int64 `json:"buyer_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.RouteFromQueue(r.Context(), idp(r), body.PipelineID, body.StageID, body.BuyerID, p.UserID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.Reject(r.Context(), idp(r), p.UserID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) mapField(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		SourceKey     string  `json:"source_key"`
		TargetType    string  `json:"target_type"`
		BuiltinField  *string `json:"builtin_field"`
		CustomFieldID *int64  `json:"custom_field_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	item, err := h.svc.MapField(r.Context(), p.AccountID, idp(r), body.SourceKey, body.TargetType, body.BuiltinField, body.CustomFieldID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func idp(r *http.Request) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id
}

func parseInt(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
