package billing

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/webhook"
)

type Handler struct {
	svc           *Service
	webhookSecret string
}

func NewHandler(svc *Service, webhookSecret string) *Handler {
	return &Handler{svc: svc, webhookSecret: webhookSecret}
}

// RegisterPublisher mounts billing oversight: all-buyer ledger + dispute queue.
func (h *Handler) RegisterPublisher(r chi.Router) {
	r.Get("/billing/transactions", h.pubTransactions)
	r.Get("/billing/disputes", h.pubDisputes)
	r.With(auth.RequireRole("admin")).Post("/billing/disputes/{id}/accept", h.accept)
	r.With(auth.RequireRole("admin")).Post("/billing/disputes/{id}/reject", h.reject)
	r.With(auth.RequireRole("admin")).Post("/billing/manual-invoice", h.manualInvoice)
	r.With(auth.RequireRole("admin")).Post("/billing/stripe/connect", h.connectStripe)
	r.With(auth.RequireRole("admin")).Get("/billing/stripe/status", h.stripeStatus)
}

// RegisterBuyer mounts the buyer's own balance + ledger + disputes.
func (h *Handler) RegisterBuyer(r chi.Router) {
	r.Get("/billing/balance", h.balance)
	r.Post("/billing/balance/topup-intent", h.createTopupIntent)
	r.Get("/billing/transactions", h.buyerTransactions)
	r.Get("/billing/disputes", h.buyerDisputes)
	r.Post("/billing/disputes", h.openDispute)
	r.Post("/billing/stripe/setup-intent", h.createSetupIntent)
	r.Get("/billing/stripe/payment-methods", h.listPaymentMethods)
	r.Delete("/billing/stripe/payment-methods/{id}", h.detachPaymentMethod)
}

// StripeWebhook handles Stripe event delivery (no JWT — signature verified).
func (h *Handler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	if h.webhookSecret == "" {
		http.Error(w, "webhook not configured", http.StatusServiceUnavailable)
		return
	}
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), h.webhookSecret)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	switch event.Type {
	case "payment_intent.succeeded":
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			http.Error(w, "parse error", http.StatusBadRequest)
			return
		}
		if pi.Metadata["purpose"] != "balance_topup" {
			break
		}
		amountDollars := float64(pi.Amount) / 100.0
		chargeID := ""
		if pi.LatestCharge != nil {
			chargeID = pi.LatestCharge.ID
		} else if id, ok := event.Data.Object["latest_charge"].(string); ok {
			chargeID = id
		}
		if err := h.svc.ConfirmTopup(r.Context(),
			pi.Metadata["buyer_public_id"],
			amountDollars,
			pi.ID,
			chargeID,
		); err != nil {
			http.Error(w, "topup failed", http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) connectStripe(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		ReturnBaseURL string `json:"return_base_url"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	url, err := h.svc.ConnectStripe(r.Context(), p.AccountID, body.ReturnBaseURL)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"onboarding_url": url})
}

func (h *Handler) stripeStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	status, err := h.svc.RefreshStripeStatus(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": status})
}

func (h *Handler) createTopupIntent(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		AmountCents int64 `json:"amount_cents"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	clientSecret, err := h.svc.CreateTopupIntent(r.Context(), p.AccountID, body.AmountCents)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"client_secret": clientSecret})
}

func (h *Handler) createSetupIntent(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	clientSecret, err := h.svc.CreateSetupIntent(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"client_secret": clientSecret})
}

func (h *Handler) listPaymentMethods(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	methods, err := h.svc.ListPaymentMethods(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if methods == nil {
		httpx.JSON(w, http.StatusOK, []any{})
		return
	}
	httpx.JSON(w, http.StatusOK, methods)
}

func (h *Handler) detachPaymentMethod(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	pmID := chi.URLParam(r, "id")
	if err := h.svc.DetachPaymentMethod(r.Context(), p.AccountID, pmID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) pubTransactions(w http.ResponseWriter, r *http.Request) {
	buyerID, _ := strconv.ParseInt(r.URL.Query().Get("buyer_id"), 10, 64)
	items, err := h.svc.ListTransactions(r.Context(), buyerID, r.URL.Query().Get("type"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) pubDisputes(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListDisputes(r.Context(), 0, r.URL.Query().Get("status"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.AcceptDispute(r.Context(), idp(r), p.UserID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.RejectDispute(r.Context(), idp(r), p.UserID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) manualInvoice(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BuyerID     int64   `json:"buyer_id"`
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	t, err := h.svc.ManualInvoice(r.Context(), body.BuyerID, body.Amount, body.Description)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, t)
}

func (h *Handler) balance(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	bal, err := h.svc.GetBalance(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"balance": bal})
}

func (h *Handler) buyerTransactions(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListTransactions(r.Context(), p.AccountID, r.URL.Query().Get("type"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) buyerDisputes(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListDisputes(r.Context(), p.AccountID, r.URL.Query().Get("status"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) openDispute(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		TransactionID int64  `json:"transaction_id"`
		Reason        string `json:"reason"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	d, err := h.svc.OpenDispute(r.Context(), p.AccountID, body.TransactionID, body.Reason)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, d)
}

func idp(r *http.Request) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id
}
