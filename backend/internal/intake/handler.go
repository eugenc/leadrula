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
	svc         *Service
	publisherID int64
}

func NewHandler(svc *Service, publisherID int64) *Handler {
	return &Handler{svc: svc, publisherID: publisherID}
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
}

func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	acct := auth.APIKeyAccountFromContext(r.Context())
	publisherID := h.publisherID
	if acct != nil && acct.AccountType == "publisher" {
		publisherID = acct.AccountID
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
	acct := auth.APIKeyAccountFromContext(r.Context())
	publisherID := h.publisherID
	if acct != nil && acct.AccountType == "publisher" {
		publisherID = acct.AccountID
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
	items, err := h.svc.ListQueue(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
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

func idp(r *http.Request) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id
}
