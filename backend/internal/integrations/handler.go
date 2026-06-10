package integrations

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc        *Service
	namespace  string
	appBaseURL string
}

func NewHandler(svc *Service, namespace, appBaseURL string) *Handler {
	return &Handler{svc: svc, namespace: namespace, appBaseURL: appBaseURL}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/integrations/providers", h.listProviders)
	r.Get("/integrations/connections", h.listConnections)
	r.Get("/integrations/routes/{routeID}", h.listRouteIntegrations)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole("admin"))
		r.Post("/integrations/connections", h.createConnection)
		r.Delete("/integrations/connections/{id}", h.deleteConnection)
		r.Post("/integrations/routes/{routeID}/attach", h.attachToRoute)
		r.Delete("/integrations/route-integrations/{id}", h.detachFromRoute)
		r.Post("/integrations/oauth/{provider}/start", h.oauthStartJSON)
		r.Get("/integrations/oauth/{provider}/callback", h.oauthCallback)
	})
}

func (h *Handler) listProviders(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListProviders(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) listConnections(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListConnections(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) createConnection(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		ProviderSlug string          `json:"provider_slug"`
		Name         string          `json:"name"`
		Credentials  json.RawMessage `json:"credentials"`
		Config       map[string]any  `json:"config"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	conn, err := h.svc.CreateConnection(r.Context(), p.AccountID, body.ProviderSlug, body.Name, body.Credentials, body.Config)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, conn)
}

func (h *Handler) deleteConnection(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := h.svc.DeleteConnection(r.Context(), p.AccountID, id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) attachToRoute(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	routeID, _ := strconv.ParseInt(chi.URLParam(r, "routeID"), 10, 64)
	var body struct {
		ConnectionID   int64          `json:"connection_id"`
		DeliveryConfig map[string]any `json:"delivery_config"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.AttachToRoute(r.Context(), p.AccountID, p.AccountType, routeID, body.ConnectionID, body.DeliveryConfig); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) detachFromRoute(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := h.svc.DetachFromRoute(r.Context(), p.AccountID, p.AccountType, id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) listRouteIntegrations(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	routeID, _ := strconv.ParseInt(chi.URLParam(r, "routeID"), 10, 64)
	items, err := h.svc.ListRouteIntegrations(r.Context(), p.AccountID, p.AccountType, routeID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) oauthStartJSON(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	provider := chi.URLParam(r, "provider")
	var body struct {
		Name      string         `json:"name"`
		Config    map[string]any `json:"config"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	name := body.Name
	if name == "" {
		name = provider + " connection"
	}
	url, _, err := h.svc.OAuthStartURL(r.Context(), p.AccountID, provider, name, body.Config, h.namespace)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"url": url})
}

func (h *Handler) oauthCallback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		httpx.WriteError(w, httpx.Validation("missing code or state"))
		return
	}
	if err := h.svc.OAuthCallback(r.Context(), h.namespace, provider, code, state); err != nil {
		httpx.WriteError(w, err)
		return
	}
	redirect := strings.TrimRight(h.appBaseURL, "/")
	if redirect == "" {
		redirect = "http://localhost:5173"
	}
	http.Redirect(w, r, redirect+"/"+shortPrefix(h.namespace)+"/integrations?connected="+provider, http.StatusFound)
}

func shortPrefix(ns string) string {
	if ns == "publisher" {
		return "p"
	}
	return "b"
}
