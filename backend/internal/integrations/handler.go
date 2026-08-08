package integrations

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/permissions"
	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/webhooks"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type WebhookSunbaseService interface {
	ProvisionSunbaseWebhooks(ctx context.Context, accountID int64, connectionID int64, connectionPublicID, connectionName, schemaName, endpointURL string, outboundFieldMap json.RawMessage) (*webhooks.SunbaseWebhookIDs, error)
	SyncSunbaseInboundEvent(ctx context.Context, inboundWebhookID int64) error
	SyncSunbaseOutboundWebhooks(ctx context.Context, accountID int64, ids webhooks.SunbaseWebhookIDs, endpointURL string, outboundFieldMap json.RawMessage) error
	DeleteSunbaseWebhooks(ctx context.Context, accountID int64, ids webhooks.SunbaseWebhookIDs)
}

type WebhookGHLService interface {
	ProvisionGHLWebhooks(ctx context.Context, accountID int64, connectionID int64, connectionPublicID, connectionName string) (*webhooks.GHLWebhookIDs, error)
	SyncGHLInboundEvent(ctx context.Context, inboundWebhookID int64) error
	SyncGHLInboundFieldMaps(ctx context.Context, inboundWebhookID int64, config map[string]any) error
	DeleteGHLWebhooks(ctx context.Context, accountID int64, ids webhooks.GHLWebhookIDs)
}

type WebhookCRMService interface {
	ProvisionCRMInboundWebhook(ctx context.Context, accountID, connectionID int64, connectionPublicID, connectionName, providerSlug string) (*webhooks.CRMWebhookIDs, error)
}

type Handler struct {
	svc        *Service
	webhooks   WebhookSunbaseService
	ghlHooks   WebhookGHLService
	crmHooks   WebhookCRMService
	namespace  string
	appBaseURL string
	apiBaseURL string
}

func NewHandler(svc *Service, webhooksSvc WebhookSunbaseService, namespace, appBaseURL, apiBaseURL string) *Handler {
	h := &Handler{
		svc:        svc,
		webhooks:   webhooksSvc,
		namespace:  namespace,
		appBaseURL: appBaseURL,
		apiBaseURL: apiBaseURL,
	}
	if ghl, ok := webhooksSvc.(WebhookGHLService); ok {
		h.ghlHooks = ghl
	}
	if crm, ok := webhooksSvc.(WebhookCRMService); ok {
		h.crmHooks = crm
	}
	return h
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/integrations/providers", h.listProviders)
	r.Get("/integrations/connections", h.listConnections)
	r.Get("/integrations/routes/{routeID}", h.listRouteIntegrations)
	r.Get("/google-maps/status", h.googleMapsStatus)
	r.Get("/google-maps/autocomplete", h.googleMapsAutocomplete)
	r.Post("/google-maps/place-details", h.googleMapsPlaceDetails)
	r.Get("/google-maps/satellite-map", h.googleMapsSatelliteMap)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(permissions.ActionSettingsAdmin))
		r.Post("/integrations/connections", h.createConnection)
		r.Post("/integrations/connections/test", h.testConnection)
		r.Post("/integrations/connections/{id}/test", h.testStoredConnection)
		r.Post("/integrations/ghl/metadata", h.postGHLMetadata)
		r.Patch("/integrations/connections/{id}", h.patchConnection)
		r.Get("/integrations/connections/{id}/sunbase", h.getSunbaseDetail)
		r.Get("/integrations/connections/{id}/ghl", h.getGHLDetail)
		r.Get("/integrations/connections/{id}/voiceuni", h.getVoiceUniDetail)
		r.Get("/integrations/connections/{id}/crm", h.getCRMDetail)
		r.Get("/integrations/connections/{id}/crm/pipelines", h.getCRMPipelines)
		r.Get("/integrations/connections/{id}/ghl/pipelines", h.getGHLPipelines)
		r.Get("/integrations/connections/{id}/ghl/calendars", h.getGHLCalendars)
		r.Get("/integrations/connections/{id}/ghl/custom-fields", h.getGHLCustomFields)
		r.Get("/integrations/connections/{id}/crm/custom-fields", h.getCRMCustomFields)
		r.Get("/integrations/connections/{id}/twilio/phone-numbers", h.getTwilioPhoneNumbers)
		r.Get("/integrations/connections/{id}/twilio/available-numbers", h.getTwilioAvailableNumbers)
		r.Get("/integrations/connections/{id}/twilio/pricing", h.getTwilioPricing)
		r.Post("/integrations/connections/{id}/twilio/phone-numbers", h.postTwilioPhoneNumber)
		r.Delete("/integrations/connections/{id}/twilio/phone-numbers/{sid}", h.deleteTwilioPhoneNumber)
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
	if body.ProviderSlug == "ghl" {
		h.createGHLConnection(w, r, p.AccountID, body.Name, body.Credentials, body.Config)
		return
	}
	if body.ProviderSlug == "voiceuni" {
		h.createVoiceUniConnection(w, r, p.AccountID, body.Name, body.Config)
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

func (h *Handler) postGHLMetadata(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Credentials json.RawMessage `json:"credentials"`
		Config      map[string]any  `json:"config"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.Config != nil && providers.ParseGHLDeliveryModeFromConfig(body.Config) == "webhook" {
		httpx.WriteError(w, httpx.Validation("metadata not available in webhook delivery mode"))
		return
	}
	data, err := h.svc.GHLMetadataFromCredentials(r.Context(), body.Credentials, body.Config)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, data)
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
	if existing.ProviderSlug == "sunbase" {
		var syncOutbound func(ctx context.Context, ids webhooks.SunbaseWebhookIDs, endpointURL string, fieldMap json.RawMessage) error
		var syncInbound func(ctx context.Context, ids webhooks.SunbaseWebhookIDs) error
		if h.webhooks != nil {
			syncOutbound = func(ctx context.Context, ids webhooks.SunbaseWebhookIDs, endpointURL string, fieldMap json.RawMessage) error {
				return h.webhooks.SyncSunbaseOutboundWebhooks(ctx, p.AccountID, ids, endpointURL, fieldMap)
			}
			syncInbound = func(ctx context.Context, ids webhooks.SunbaseWebhookIDs) error {
				return h.webhooks.SyncSunbaseInboundEvent(ctx, ids.Inbound)
			}
		}
		conn, err := h.svc.UpdateSunbaseConnection(r.Context(), p.AccountID, id, body.Credentials, body.Config, syncOutbound, syncInbound)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		ids := webhooks.ParseSunbaseWebhookIDs(conn.Config)
		httpx.JSON(w, http.StatusOK, h.sunbaseResponse(conn, &ids))
		return
	}
	if existing.ProviderSlug == "ghl" {
		conn, err := h.svc.UpdateGHLConnection(r.Context(), p.AccountID, id, body.Credentials, body.Config)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		if h.ghlHooks != nil {
			ids := webhooks.ParseGHLWebhookIDs(conn.Config)
			if ids.Inbound > 0 {
				if syncErr := h.ghlHooks.SyncGHLInboundFieldMaps(r.Context(), ids.Inbound, configMap(conn.Config)); syncErr != nil {
					httpx.WriteError(w, syncErr)
					return
				}
			}
		}
		httpx.JSON(w, http.StatusOK, h.ghlResponse(conn))
		return
	}
	if existing.ProviderSlug == "voiceuni" {
		conn, err := h.svc.UpdateVoiceUniConnection(r.Context(), p.AccountID, id, body.Config)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, h.voiceuniResponse(conn))
		return
	}
	if CRMConnectionConfigurable(existing.ProviderSlug) {
		conn, err := h.svc.UpdateCRMConnection(r.Context(), p.AccountID, id, body.Config)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, crmResponse(conn, h.apiBaseURL))
		return
	}
	httpx.WriteError(w, httpx.Validation("patch only supported for sunbase, ghl, voiceuni, and configurable crm connections"))
}

func (h *Handler) createVoiceUniConnection(w http.ResponseWriter, r *http.Request, accountID int64, name string, config map[string]any) {
	if config == nil {
		config = map[string]any{}
	}
	config = providers.MergeVoiceUniConfigDefaults(config)
	if name == "" {
		name = "VoiceUni"
	}
	conn, err := h.svc.CreateConnection(r.Context(), accountID, "voiceuni", name, json.RawMessage(`{}`), config)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	sourceSlug, callSourceSlug, sourceID, err := h.svc.provisionVoiceUniSources(r.Context(), accountID, conn.ID, conn.PublicID, conn.Name)
	if err != nil {
		_ = h.svc.DeleteConnection(r.Context(), accountID, conn.ID)
		httpx.WriteError(w, err)
		return
	}
	config["source_slug"] = sourceSlug
	config["source_id"] = sourceID
	config["call_source_slug"] = callSourceSlug
	if err := h.svc.syncVoiceUniSourceFieldMaps(r.Context(), accountID, sourceID, config); err != nil {
		_ = h.svc.DeleteConnection(r.Context(), accountID, conn.ID)
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.FinalizeVoiceUniConnection(r.Context(), conn.ID, config); err != nil {
		_ = h.svc.DeleteConnection(r.Context(), accountID, conn.ID)
		httpx.WriteError(w, err)
		return
	}
	conn.Config = config
	httpx.JSON(w, http.StatusCreated, h.voiceuniResponse(conn))
}

func (h *Handler) getVoiceUniDetail(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	detail, err := h.svc.VoiceUniConnectionDetail(r.Context(), p.AccountID, id, h.apiBaseURL)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, detail)
}

func (h *Handler) voiceuniResponse(conn *Connection) VoiceUniConnectionResponse {
	resp := VoiceUniConnectionResponse{Connection: *conn}
	cfg := configMap(conn.Config)
	resp.SourceSlug = providers.VoiceUniSourceSlug(cfg)
	resp.IngestEndpoint = strings.TrimRight(h.apiBaseURL, "/") + "/api/v1/integrations/voiceuni/ingest"
	return resp
}

func (h *Handler) createGHLConnection(w http.ResponseWriter, r *http.Request, accountID int64, name string, credentials json.RawMessage, config map[string]any) {
	if config == nil {
		config = map[string]any{}
	}
	config = providers.MergeGHLConfigDefaults(config)
	if err := providers.ValidateGHLConfigJSON(config); err != nil {
		httpx.WriteError(w, httpx.Validation(err.Error()))
		return
	}
	if name == "" {
		name = "GoHighLevel"
	}
	conn, err := h.svc.CreateConnection(r.Context(), accountID, "ghl", name, credentials, config)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if h.ghlHooks == nil {
		httpx.JSON(w, http.StatusCreated, h.ghlResponse(conn))
		return
	}
	ids, err := h.ghlHooks.ProvisionGHLWebhooks(
		r.Context(), accountID, conn.ID, conn.PublicID, conn.Name,
	)
	if err != nil {
		_ = h.svc.DeleteConnection(r.Context(), accountID, conn.ID)
		httpx.WriteError(w, wrapGHLProvisionErr("provision webhooks", err))
		return
	}
	merged := webhooks.MergeGHLConfig(config, ids)
	if err := h.ghlHooks.SyncGHLInboundFieldMaps(r.Context(), ids.Inbound, merged); err != nil {
		h.ghlHooks.DeleteGHLWebhooks(r.Context(), accountID, *ids)
		_ = h.svc.DeleteConnection(r.Context(), accountID, conn.ID)
		httpx.WriteError(w, wrapGHLProvisionErr("sync inbound webhook", err))
		return
	}
	if err := h.svc.FinalizeGHLConnection(r.Context(), conn.ID, merged); err != nil {
		h.ghlHooks.DeleteGHLWebhooks(r.Context(), accountID, *ids)
		_ = h.svc.DeleteConnection(r.Context(), accountID, conn.ID)
		httpx.WriteError(w, wrapGHLProvisionErr("finalize connection", err))
		return
	}
	conn.Config = merged
	httpx.JSON(w, http.StatusCreated, h.ghlResponse(conn))
}

func (h *Handler) getGHLDetail(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	detail, err := h.svc.GHLConnectionDetail(r.Context(), p.AccountID, id, h.apiBaseURL)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, detail)
}

func (h *Handler) testStoredConnection(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := h.svc.TestStoredConnection(r.Context(), p.AccountID, id); err != nil {
		httpx.JSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) getCRMDetail(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	detail, err := h.svc.CRMConnectionDetail(r.Context(), p.AccountID, id, h.apiBaseURL)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, detail)
}

func (h *Handler) getCRMPipelines(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	pipelines, slug, err := h.svc.FetchCRMPipelines(r.Context(), p.AccountID, id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"pipelines":     pipelines,
		"provider_slug": slug,
	})
}

func (h *Handler) getGHLPipelines(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	data, err := h.svc.GHLMetadata(r.Context(), p.AccountID, id, "pipelines")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, data)
}

func (h *Handler) getGHLCalendars(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	data, err := h.svc.GHLMetadata(r.Context(), p.AccountID, id, "calendars")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, data)
}

func (h *Handler) getGHLCustomFields(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	data, err := h.svc.GHLMetadata(r.Context(), p.AccountID, id, "custom_fields")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, data)
}

func (h *Handler) getCRMCustomFields(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	data, err := h.svc.CRMCustomFields(r.Context(), p.AccountID, id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, data)
}

func (h *Handler) getTwilioPhoneNumbers(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	numbers, err := h.svc.ListTwilioPhoneNumbers(r.Context(), p.AccountID, id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"phone_numbers": numbers})
}

func (h *Handler) getTwilioAvailableNumbers(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	q := r.URL.Query()
	numbers, err := h.svc.SearchTwilioAvailableNumbers(r.Context(), p.AccountID, id,
		q.Get("type"), q.Get("area_code"), q.Get("prefix"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"phone_numbers": numbers})
}

func (h *Handler) getTwilioPricing(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	price, err := h.svc.TwilioMonthlyPrice(r.Context(), p.AccountID, id, r.URL.Query().Get("type"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := map[string]any{"currency": "USD"}
	if price != nil {
		out["monthly_price"] = *price
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) postTwilioPhoneNumber(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var body struct {
		PhoneNumber string `json:"phone_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, httpx.Validation("invalid json"))
		return
	}
	purchased, err := h.svc.PurchaseTwilioPhoneNumber(r.Context(), p.AccountID, id, body.PhoneNumber)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, purchased)
}

func (h *Handler) deleteTwilioPhoneNumber(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	sid := chi.URLParam(r, "sid")
	if err := h.svc.ReleaseTwilioPhoneNumber(r.Context(), p.AccountID, id, sid); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) ghlResponse(conn *Connection) GHLConnectionResponse {
	resp := GHLConnectionResponse{Connection: *conn}
	if conn.Status == "active" {
		ids := webhooks.ParseGHLWebhookIDs(conn.Config)
		if ids.InboundSlug != "" {
			resp.InboundWebhook = BuildGHLInboundWebhookInfo(h.apiBaseURL, ids.Inbound, ids.InboundSlug)
		}
	}
	return resp
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
		if conn, err := h.svc.GetConnection(r.Context(), p.AccountID, id); err == nil {
			switch conn.ProviderSlug {
			case "sunbase":
				ids := webhooks.ParseSunbaseWebhookIDs(conn.Config)
				h.webhooks.DeleteSunbaseWebhooks(r.Context(), p.AccountID, ids)
			case "ghl":
				if h.ghlHooks != nil {
					ids := webhooks.ParseGHLWebhookIDs(conn.Config)
					h.ghlHooks.DeleteGHLWebhooks(r.Context(), p.AccountID, ids)
				}
			}
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
	connID, err := h.svc.OAuthCallback(r.Context(), h.namespace, provider, code, state)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if h.crmHooks != nil && providers.CRMPipelineImportSupported(provider) && provider != "salesforce" {
		accountID, publicID, name, slug, cfg, loadErr := h.svc.GetConnectionAccount(r.Context(), connID)
		if loadErr == nil {
			if ids, provErr := h.crmHooks.ProvisionCRMInboundWebhook(r.Context(), accountID, connID, publicID, name, slug); provErr == nil {
				merged := webhooks.MergeCRMConfig(cfg, ids)
				_ = h.svc.FinalizeCRMConnection(r.Context(), connID, merged)
			}
		}
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
