package integrations

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/webhooks"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type WebhookSunbaseService interface {
	ProvisionSunbaseWebhooks(ctx context.Context, accountID int64, connectionID int64, connectionPublicID, connectionName, schemaName, endpointURL string, outboundFieldMap json.RawMessage) (*webhooks.SunbaseWebhookIDs, error)
	SyncSunbaseOutboundWebhooks(ctx context.Context, accountID int64, ids webhooks.SunbaseWebhookIDs, endpointURL string, outboundFieldMap json.RawMessage) error
	DeleteSunbaseWebhooks(ctx context.Context, accountID int64, ids webhooks.SunbaseWebhookIDs)
}

type Handler struct {
	svc        *Service
	webhooks   WebhookSunbaseService
	namespace  string
	appBaseURL string
	apiBaseURL string
}

func NewHandler(svc *Service, webhooksSvc WebhookSunbaseService, namespace, appBaseURL, apiBaseURL string) *Handler {
	return &Handler{
		svc:        svc,
		webhooks:   webhooksSvc,
		namespace:  namespace,
		appBaseURL: appBaseURL,
		apiBaseURL: apiBaseURL,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/integrations/providers", h.listProviders)
	r.Get("/integrations/connections", h.listConnections)
	r.Get("/integrations/routes/{routeID}", h.listRouteIntegrations)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole("admin"))
		r.Post("/integrations/connections", h.createConnection)
		r.Post("/integrations/connections/test", h.testConnection)
		r.Patch("/integrations/connections/{id}", h.patchConnection)
		r.Get("/integrations/connections/{id}/sunbase", h.getSunbaseDetail)
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
	if body.ProviderSlug == "sunbase" {
		h.createSunbaseConnection(w, r, p.AccountID, body.Name, body.Credentials, body.Config)
		return
	}
	conn, err := h.svc.CreateConnection(r.Context(), p.AccountID, body.ProviderSlug, body.Name, body.Credentials, body.Config)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, conn)
}

func (h *Handler) createSunbaseConnection(w http.ResponseWriter, r *http.Request, accountID int64, name string, credentials json.RawMessage, config map[string]any) {
	var err error
	name, err = h.svc.ResolveSunbaseConnectionName(r.Context(), accountID, name)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if config == nil {
		config = map[string]any{}
	}
	if _, ok := config["endpoint_url"]; !ok {
		config["endpoint_url"] = sunbaseEndpointFromConfig(config)
	}
	schemaName := schemaNameFromCredentials(credentials, config)
	fieldMapJSON, _ := json.Marshal(config["outbound_field_map"])
	if len(fieldMapJSON) == 0 || string(fieldMapJSON) == "null" {
		fieldMapJSON = defaultOutboundFieldMapForSchema(schemaName)
	}

	conn, err := h.svc.CreateConnection(r.Context(), accountID, "sunbase", name, credentials, config)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if h.webhooks == nil {
		httpx.JSON(w, http.StatusCreated, h.sunbaseResponse(conn, nil))
		return
	}

	endpointURL := sunbaseEndpointFromConfig(config)
	ids, err := h.webhooks.ProvisionSunbaseWebhooks(
		r.Context(), accountID, conn.ID, conn.PublicID, conn.Name,
		schemaName, endpointURL, fieldMapJSON,
	)
	if err != nil {
		_ = h.svc.DeleteConnection(r.Context(), accountID, conn.ID)
		httpx.WriteError(w, wrapSunbaseProvisionErr("provision webhooks", err))
		return
	}
	merged := webhooks.MergeSunbaseConfig(config, ids, endpointURL, fieldMapJSON)
	if err := h.svc.FinalizeSunbaseConnection(r.Context(), conn.ID, merged); err != nil {
		h.webhooks.DeleteSunbaseWebhooks(r.Context(), accountID, *ids)
		_ = h.svc.DeleteConnection(r.Context(), accountID, conn.ID)
		httpx.WriteError(w, wrapSunbaseProvisionErr("finalize connection", err))
		return
	}
	conn.Config = merged
	httpx.JSON(w, http.StatusCreated, h.sunbaseResponse(conn, ids))
}

func (h *Handler) testConnection(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProviderSlug string          `json:"provider_slug"`
		Credentials  json.RawMessage `json:"credentials"`
		Config       map[string]any  `json:"config"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.TestConnection(r.Context(), body.ProviderSlug, body.Credentials, body.Config); err != nil {
		httpx.JSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) patchConnection(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var body struct {
		Credentials json.RawMessage `json:"credentials"`
		Config      map[string]any  `json:"config"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	existing, err := h.svc.GetConnection(r.Context(), p.AccountID, id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if existing.ProviderSlug != "sunbase" {
		httpx.WriteError(w, httpx.Validation("patch only supported for sunbase connections"))
		return
	}
	var sync func(ctx context.Context, ids webhooks.SunbaseWebhookIDs, endpointURL string, fieldMap json.RawMessage) error
	if h.webhooks != nil {
		sync = func(ctx context.Context, ids webhooks.SunbaseWebhookIDs, endpointURL string, fieldMap json.RawMessage) error {
			return h.webhooks.SyncSunbaseOutboundWebhooks(ctx, p.AccountID, ids, endpointURL, fieldMap)
		}
	}
	conn, err := h.svc.UpdateSunbaseConnection(r.Context(), p.AccountID, id, body.Credentials, body.Config, sync)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	ids := webhooks.ParseSunbaseWebhookIDs(conn.Config)
	httpx.JSON(w, http.StatusOK, h.sunbaseResponse(conn, &ids))
}

func (h *Handler) getSunbaseDetail(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	conn, err := h.svc.GetConnection(r.Context(), p.AccountID, id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if conn.ProviderSlug != "sunbase" {
		httpx.WriteError(w, httpx.Validation("not a sunbase connection"))
		return
	}
	detail := SunbaseDetailFromConnection(conn, h.apiBaseURL)
	httpx.JSON(w, http.StatusOK, detail)
}

func (h *Handler) deleteConnection(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if h.webhooks != nil {
		if conn, err := h.svc.GetConnection(r.Context(), p.AccountID, id); err == nil && conn.ProviderSlug == "sunbase" {
			ids := webhooks.ParseSunbaseWebhookIDs(conn.Config)
			h.webhooks.DeleteSunbaseWebhooks(r.Context(), p.AccountID, ids)
		}
	}
	if err := h.svc.DeleteConnection(r.Context(), p.AccountID, id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) sunbaseResponse(conn *Connection, ids *webhooks.SunbaseWebhookIDs) SunbaseConnectionResponse {
	resp := SunbaseConnectionResponse{Connection: *conn}
	if ids != nil && conn.Status == "active" {
		resp.InboundWebhook = BuildInboundWebhookInfo(h.apiBaseURL, ids.Inbound, ids.InboundSlug)
	} else if conn.Status == "active" {
		parsed := webhooks.ParseSunbaseWebhookIDs(conn.Config)
		if parsed.InboundSlug != "" {
			resp.InboundWebhook = BuildInboundWebhookInfo(h.apiBaseURL, parsed.Inbound, parsed.InboundSlug)
		}
	}
	return resp
}

func defaultOutboundFieldMapForSchema(schemaName string) json.RawMessage {
	entries := []map[string]any{
		{"dest_key": "schema_name", "source_type": "static", "static_value": schemaName},
		{"dest_key": "last_name", "source_type": "builtin", "builtin_field": "last_name"},
		{"dest_key": "first_name", "source_type": "builtin", "builtin_field": "first_name"},
		{"dest_key": "address1", "source_type": "builtin", "builtin_field": "address"},
		{"dest_key": "city", "source_type": "builtin", "builtin_field": "city"},
		{"dest_key": "state", "source_type": "builtin", "builtin_field": "state"},
		{"dest_key": "zip_code", "source_type": "builtin", "builtin_field": "zip"},
		{"dest_key": "email", "source_type": "builtin", "builtin_field": "email"},
		{"dest_key": "phone", "source_type": "builtin", "builtin_field": "phone"},
		{"dest_key": "lead_source", "source_type": "builtin", "builtin_field": "source"},
		{"dest_key": "lead_other", "source_type": "builtin", "builtin_field": "external_id"},
	}
	b, _ := json.Marshal(entries)
	return b
}

func (h *Handler) attachToRoute(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	routeID, _ := strconv.ParseInt(chi.URLParam(r, "routeID"), 10, 64)
	var body struct {
		ConnectionID   int64          `json:"connection_id"`
		BranchPosition int            `json:"branch_position"`
		DeliveryConfig map[string]any `json:"delivery_config"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.AttachToRoute(r.Context(), p.AccountID, p.AccountType, routeID, body.ConnectionID, body.BranchPosition, body.DeliveryConfig); err != nil {
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
		Name   string         `json:"name"`
		Config map[string]any `json:"config"`
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
