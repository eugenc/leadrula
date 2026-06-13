package webhooks

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

// RegisterPublicRoutes mounts the webhook ingest endpoint.
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.With(h.AuthenticateWebhook).Post("/api/v1/webhooks/{slug}", h.ingest)
}

// RegisterRoutes mounts admin CRUD for publisher or buyer namespace.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/webhook-deliveries", h.listAccountDeliveries)
	r.Get("/webhooks", h.list)
	r.Get("/webhooks/{id}/events", h.listEvents)
	r.Get("/webhooks/{id}/events/{eventId}/field-map", h.listFieldMap)
	r.Get("/webhooks/{id}/sample-payload", h.samplePayload)
	r.Get("/webhooks/{id}/deliveries", h.listDeliveries)
	r.Get("/webhooks/{id}/deliveries/{deliveryId}", h.getDelivery)
	r.Get("/webhooks/{id}/outbound-triggers", h.listTriggers)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole("admin"))
		r.Post("/webhooks", h.create)
		r.Patch("/webhooks/{id}", h.update)
		r.Delete("/webhooks/{id}", h.delete)
		r.Post("/webhooks/{id}/rotate-secret", h.rotateSecret)
		r.Post("/webhooks/{id}/rotate-outbound-secret", h.rotateOutboundSecret)
		r.Post("/webhooks/{id}/events", h.createEvent)
		r.Patch("/webhooks/{id}/events/{eventId}", h.updateEvent)
		r.Delete("/webhooks/{id}/events/{eventId}", h.deleteEvent)
		r.Post("/webhooks/{id}/events/{eventId}/field-map", h.addFieldMap)
		r.Delete("/webhook-field-map/{mapId}", h.deleteFieldMap)
		r.Post("/webhooks/{id}/outbound-triggers", h.createTrigger)
		r.Patch("/webhooks/{id}/outbound-triggers/{triggerId}", h.updateTrigger)
		r.Delete("/webhooks/{id}/outbound-triggers/{triggerId}", h.deleteTrigger)
		r.Post("/webhooks/{id}/deliveries/{deliveryId}/replay", h.replayDelivery)
	})
}

// AuthenticateWebhook resolves the webhook by slug and optionally verifies a Bearer secret.
func (h *Handler) AuthenticateWebhook(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		wh, err := h.svc.ResolveBySlug(r.Context(), slug)
		if err != nil {
			httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid webhook")
			return
		}
		if wh.InboundSecretRequired {
			token := auth.Bearer(r)
			if token == "" {
				httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "missing webhook secret")
				return
			}
			if !verifySecretForWebhook(wh, token) {
				httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid webhook secret")
				return
			}
		}
		ctx := auth.WithWebhookAuth(r.Context(), &auth.WebhookAuth{
			WebhookID: wh.ID,
			AccountID: wh.AccountID,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	wa := auth.WebhookAuthFromContext(r.Context())
	if wa == nil {
		httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	var raw map[string]any
	if !httpx.DecodeJSON(w, r, &raw) {
		return
	}
	res, err := h.svc.Ingest(r.Context(), &WebhookAuth{WebhookID: wa.WebhookID, AccountID: wa.AccountID}, chi.URLParam(r, "slug"), raw)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if res.Status == "captured" {
		httpx.JSON(w, http.StatusOK, res)
		return
	}
	httpx.JSON(w, http.StatusAccepted, res)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.List(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []Webhook{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name                  string  `json:"name"`
		Slug                  string  `json:"slug"`
		InboundEnabled        *bool   `json:"inbound_enabled"`
		InboundSecretRequired *bool   `json:"inbound_secret_required"`
		OutboundEnabled       *bool   `json:"outbound_enabled"`
		OutboundSignEnabled   *bool   `json:"outbound_sign_enabled"`
		OutboundURL           *string `json:"outbound_url"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	wb, secret, err := h.svc.Create(r.Context(), p.AccountID, CreateWebhookInput{
		Name:                  body.Name,
		Slug:                  body.Slug,
		InboundEnabled:        body.InboundEnabled,
		InboundSecretRequired: body.InboundSecretRequired,
		OutboundEnabled:       body.OutboundEnabled,
		OutboundSignEnabled:   body.OutboundSignEnabled,
		OutboundURL:           body.OutboundURL,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"webhook": wb, "secret": secret})
}

func (h *Handler) guardUserEditable(w http.ResponseWriter, r *http.Request, accountID, webhookID int64) bool {
	if err := h.svc.AssertUserEditableWebhook(r.Context(), accountID, webhookID); err != nil {
		httpx.WriteError(w, err)
		return false
	}
	return true
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if !h.guardUserEditable(w, r, p.AccountID, wid) {
		return
	}
	var body struct {
		Name                    *string         `json:"name"`
		Slug                    *string         `json:"slug"`
		IsActive                *bool           `json:"is_active"`
		InboundEnabled          *bool           `json:"inbound_enabled"`
		InboundSecretRequired   *bool           `json:"inbound_secret_required"`
		OutboundEnabled         *bool           `json:"outbound_enabled"`
		OutboundSignEnabled     *bool           `json:"outbound_sign_enabled"`
		OutboundURL             *string         `json:"outbound_url"`
		OutboundFormat          *string         `json:"outbound_format"`
		OutboundMethod          *string         `json:"outbound_method"`
		OutboundPayloadTemplate *string         `json:"outbound_payload_template"`
		OutboundFieldMap        json.RawMessage `json:"outbound_field_map"`
		OutboundResponseMap     json.RawMessage `json:"outbound_response_map"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	wb, err := h.svc.Update(r.Context(), p.AccountID, wid, UpdateWebhookInput{
		Name:                    body.Name,
		Slug:                    body.Slug,
		IsActive:                body.IsActive,
		InboundEnabled:          body.InboundEnabled,
		InboundSecretRequired:   body.InboundSecretRequired,
		OutboundEnabled:         body.OutboundEnabled,
		OutboundSignEnabled:     body.OutboundSignEnabled,
		OutboundURL:             body.OutboundURL,
		OutboundFormat:          body.OutboundFormat,
		OutboundMethod:          body.OutboundMethod,
		OutboundPayloadTemplate: body.OutboundPayloadTemplate,
		OutboundFieldMap:        body.OutboundFieldMap,
		OutboundResponseMap:     body.OutboundResponseMap,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, wb)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if !h.guardUserEditable(w, r, p.AccountID, wid) {
		return
	}
	if err := h.svc.Delete(r.Context(), p.AccountID, wid); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) rotateSecret(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if !h.guardUserEditable(w, r, p.AccountID, wid) {
		return
	}
	secret, err := h.svc.RotateSecret(r.Context(), p.AccountID, wid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"secret": secret})
}

func (h *Handler) listEvents(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if ok, err := h.svc.OwnedBy(r.Context(), p.AccountID, wid); err != nil || !ok {
		httpx.Err(w, http.StatusNotFound, httpx.CodeNotFound, "webhook not found")
		return
	}
	items, err := h.svc.ListEvents(r.Context(), wid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []WebhookEvent{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) createEvent(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if !h.guardUserEditable(w, r, p.AccountID, wid) {
		return
	}
	var body CreateEventParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	e, err := h.svc.CreateEvent(r.Context(), wid, body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, e)
}

func (h *Handler) updateEvent(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if !h.guardUserEditable(w, r, p.AccountID, wid) {
		return
	}
	var body UpdateEventParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	e, err := h.svc.UpdateEvent(r.Context(), wid, idp(r, "eventId"), body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, e)
}

func (h *Handler) deleteEvent(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if !h.guardUserEditable(w, r, p.AccountID, wid) {
		return
	}
	if err := h.svc.DeleteEvent(r.Context(), wid, idp(r, "eventId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listFieldMap(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if ok, err := h.svc.OwnedBy(r.Context(), p.AccountID, wid); err != nil || !ok {
		httpx.Err(w, http.StatusNotFound, httpx.CodeNotFound, "webhook not found")
		return
	}
	items, err := h.svc.ListFieldMap(r.Context(), idp(r, "eventId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []FieldMapEntry{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) addFieldMap(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if !h.guardUserEditable(w, r, p.AccountID, wid) {
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
	e, err := h.svc.AddFieldMap(r.Context(), idp(r, "eventId"), body.SourceKey, body.TargetType, body.BuiltinField, body.CustomFieldID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, e)
}

func (h *Handler) deleteFieldMap(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.AssertUserEditableFieldMap(r.Context(), p.AccountID, idp(r, "mapId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.DeleteFieldMap(r.Context(), idp(r, "mapId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) samplePayload(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if ok, err := h.svc.OwnedBy(r.Context(), p.AccountID, wid); err != nil || !ok {
		httpx.Err(w, http.StatusNotFound, httpx.CodeNotFound, "webhook not found")
		return
	}
	sample, err := h.svc.LatestSamplePayload(r.Context(), wid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sample)
}

func (h *Handler) getDelivery(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	d, err := h.svc.GetDelivery(r.Context(), p.AccountID, wid, idp(r, "deliveryId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

func (h *Handler) replayDelivery(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	res, err := h.svc.ReplayDelivery(r.Context(), p.AccountID, wid, idp(r, "deliveryId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, res)
}

func (h *Handler) listAccountDeliveries(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	q := r.URL.Query()
	result, err := h.svc.ListAccountDeliveries(r.Context(), p.AccountID, ListAccountDeliveriesParams{
		Status:    q.Get("status"),
		WebhookID: int64(parseInt(q.Get("webhook_id"))),
		Page:      int(parseInt(q.Get("page"))),
		Limit:     int(parseInt(q.Get("limit"))),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) listDeliveries(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if ok, err := h.svc.OwnedBy(r.Context(), p.AccountID, wid); err != nil || !ok {
		httpx.Err(w, http.StatusNotFound, httpx.CodeNotFound, "webhook not found")
		return
	}
	q := r.URL.Query()
	result, err := h.svc.ListDeliveries(r.Context(), wid, int(parseInt(q.Get("page"))), int(parseInt(q.Get("limit"))))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) rotateOutboundSecret(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if !h.guardUserEditable(w, r, p.AccountID, wid) {
		return
	}
	secret, err := h.svc.RotateOutboundSecret(r.Context(), p.AccountID, wid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"secret": secret})
}

func (h *Handler) listTriggers(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if ok, err := h.svc.OwnedBy(r.Context(), p.AccountID, wid); err != nil || !ok {
		httpx.Err(w, http.StatusNotFound, httpx.CodeNotFound, "webhook not found")
		return
	}
	items, err := h.svc.ListTriggers(r.Context(), wid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) createTrigger(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if !h.guardUserEditable(w, r, p.AccountID, wid) {
		return
	}
	var body CreateTriggerInput
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	t, err := h.svc.CreateTrigger(r.Context(), wid, body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, t)
}

func (h *Handler) updateTrigger(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if !h.guardUserEditable(w, r, p.AccountID, wid) {
		return
	}
	var body UpdateTriggerInput
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	t, err := h.svc.UpdateTrigger(r.Context(), idp(r, "triggerId"), body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

func (h *Handler) deleteTrigger(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	wid := idp(r, "id")
	if !h.guardUserEditable(w, r, p.AccountID, wid) {
		return
	}
	if err := h.svc.DeleteTrigger(r.Context(), idp(r, "triggerId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func idp(r *http.Request, key string) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, key), 10, 64)
	return id
}

func parseInt(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
