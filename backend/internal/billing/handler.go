package billing

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
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
	h.registerDisputeWorkflow(r)
	// Publisher opens a return dispute from a returned lead (admin only).
	r.With(auth.RequireRole("admin")).Post("/leads/{id}/dispute", h.openReturnDispute)
	r.With(auth.RequireRole("admin")).Post("/billing/invoices", h.createInvoice)
	r.With(auth.RequireRole("admin")).Get("/billing/invoices", h.pubInvoices)
	r.With(auth.RequireRole("admin")).Post("/billing/invoices/{id}/mark-paid", h.markInvoicePaid)
	r.With(auth.RequireRole("admin")).Post("/billing/invoices/{id}/void", h.voidInvoice)
	r.With(auth.RequireRole("admin")).Post("/billing/stripe/connect", h.connectStripe)
	r.With(auth.RequireRole("admin")).Get("/billing/stripe/status", h.stripeStatus)
	r.With(auth.RequireRole("admin")).Post("/billing/stripe/keys", h.saveStripeKeys)
	r.With(auth.RequireRole("admin")).Get("/billing/stripe/keys/status", h.stripeKeysStatus)
}

// RegisterPublic mounts unauthenticated billing callbacks.
func (h *Handler) RegisterPublic(r chi.Router) {
	r.Get("/publisher/billing/stripe/oauth/callback", h.stripeOAuthCallback)
}

// RegisterBuyer mounts the buyer's own balance + ledger + disputes.
func (h *Handler) RegisterBuyer(r chi.Router) {
	r.Get("/billing/balance", h.balance)
	r.Get("/billing/stripe/config", h.buyerStripeConfig)
	r.Post("/billing/balance/topup-intent", h.createTopupIntent)
	r.Post("/billing/balance/confirm-topup", h.confirmTopup)
	r.Get("/billing/transactions", h.buyerTransactions)
	r.Get("/billing/disputes", h.buyerDisputes)
	r.Post("/billing/disputes", h.openDispute)
	h.registerDisputeWorkflow(r)
	r.Get("/billing/invoices", h.buyerInvoices)
	r.Post("/billing/invoices/{id}/pay-intent", h.createInvoicePayIntent)
	r.Post("/billing/invoices/{id}/confirm-payment", h.confirmInvoicePayment)
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
		chargeID := ""
		if pi.LatestCharge != nil {
			chargeID = pi.LatestCharge.ID
		} else if id, ok := event.Data.Object["latest_charge"].(string); ok {
			chargeID = id
		}
		amountDollars := float64(pi.Amount) / 100.0
		switch pi.Metadata["purpose"] {
		case "balance_topup":
			if err := h.svc.ConfirmTopup(r.Context(),
				pi.Metadata["buyer_public_id"],
				amountDollars,
				pi.ID,
				chargeID,
			); err != nil {
				http.Error(w, "topup failed", http.StatusInternalServerError)
				return
			}
		case "invoice_payment":
			if err := h.svc.ConfirmInvoicePayment(r.Context(),
				pi.Metadata["invoice_public_id"],
				amountDollars,
				pi.ID,
				chargeID,
			); err != nil {
				http.Error(w, "invoice payment failed", http.StatusInternalServerError)
				return
			}
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
	oauthURL, err := h.svc.ConnectStripe(r.Context(), p.AccountID, body.ReturnBaseURL)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"oauth_url": oauthURL})
}

func (h *Handler) stripeOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}
	returnBaseURL, err := h.svc.CompleteStripeOAuth(r.Context(), code, state)
	if err != nil {
		http.Error(w, "oauth failed", http.StatusBadRequest)
		return
	}
	target := returnBaseURL + "/p/integrations?stripe=complete"
	http.Redirect(w, r, target, http.StatusFound)
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

func (h *Handler) saveStripeKeys(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		SecretKey      string `json:"secret_key"`
		PublishableKey string `json:"publishable_key"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.SavePublisherStripeKeys(r.Context(), p.AccountID, body.SecretKey, body.PublishableKey); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) stripeKeysStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	st, err := h.svc.PublisherStripeKeysStatus(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, st)
}

func (h *Handler) buyerStripeConfig(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	cfg, err := h.svc.BuyerStripeConfig(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cfg)
}

func (h *Handler) createTopupIntent(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		AmountCents int64 `json:"amount_cents"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	result, err := h.svc.CreateTopupIntent(r.Context(), p.AccountID, body.AmountCents)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) confirmTopup(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		PaymentIntentID string `json:"payment_intent_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.PaymentIntentID == "" {
		httpx.WriteError(w, httpx.Validation("payment_intent_id is required"))
		return
	}
	if err := h.svc.ConfirmDirectTopup(r.Context(), p.AccountID, body.PaymentIntentID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) createSetupIntent(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	result, err := h.svc.CreateSetupIntent(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
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
	p := auth.FromContext(r.Context())
	filterBuyerID, _ := strconv.ParseInt(r.URL.Query().Get("buyer_id"), 10, 64)
	items, err := h.svc.ListPublisherTransactions(r.Context(), p.AccountID, filterBuyerID, r.URL.Query().Get("type"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) pubDisputes(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListPublisherDisputes(r.Context(), p.AccountID, r.URL.Query().Get("status"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

// registerDisputeWorkflow mounts the negotiated dispute endpoints shared by the
// buyer and publisher namespaces. Any active user on either account may act.
func (h *Handler) registerDisputeWorkflow(r chi.Router) {
	r.Post("/billing/disputes/{id}/accept", h.accept)
	r.Post("/billing/disputes/{id}/reject", h.reject)
	r.Post("/billing/disputes/{id}/placement", h.placement)
	r.Get("/billing/disputes/{id}/messages", h.listMessages)
	r.Post("/billing/disputes/{id}/messages", h.postMessage)
	r.Get("/billing/disputes/attachments/{id}", h.downloadAttachment)
}

const maxAttachmentBytes = 10 << 20 // 10MB per file

var allowedAttachmentTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
	"application/pdf": true, "text/plain": true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true,
	"application/vnd.ms-excel":                                                true,
}

// parseUploads reads multipart "files" entries, enforcing per-file size and type.
func parseUploads(r *http.Request) ([]UploadFile, error) {
	if r.MultipartForm == nil {
		if err := r.ParseMultipartForm(maxAttachmentBytes); err != nil {
			return nil, httpx.Validation("invalid upload")
		}
	}
	if r.MultipartForm == nil {
		return nil, nil
	}
	headers := r.MultipartForm.File["files"]
	var out []UploadFile
	for _, fh := range headers {
		if fh.Size > maxAttachmentBytes {
			return nil, httpx.Validation("each attachment must be 10MB or smaller")
		}
		ct := fh.Header.Get("Content-Type")
		if !allowedAttachmentTypes[ct] {
			return nil, httpx.Validation("unsupported attachment type: " + ct)
		}
		f, err := fh.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(f, maxAttachmentBytes))
		f.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, UploadFile{Filename: fh.Filename, ContentType: ct, Size: fh.Size, Data: data})
	}
	return out, nil
}

func (h *Handler) openReturnDispute(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Reason       string `json:"reason"`
		DeadlineDays int    `json:"deadline_days"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	d, err := h.svc.OpenReturnDispute(r.Context(), p.AccountID, p.UserID, idp(r), body.Reason, body.DeadlineDays)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, d)
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		PipelineID int64 `json:"pipeline_id"`
		StageID    int64 `json:"stage_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := h.svc.ResolveAccept(r.Context(), p.AccountID, p.UserID, idp(r), body.PipelineID, body.StageID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	files, err := parseUploads(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.ResolveReject(r.Context(), p.AccountID, p.UserID, idp(r), r.FormValue("body"), files); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) placement(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		PipelineID int64 `json:"pipeline_id"`
		StageID    int64 `json:"stage_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.SubmitPlacement(r.Context(), p.AccountID, p.UserID, idp(r), body.PipelineID, body.StageID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	msgs, err := h.svc.ListDisputeMessages(r.Context(), p.AccountID, idp(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if msgs == nil {
		msgs = []DisputeMessage{}
	}
	httpx.JSON(w, http.StatusOK, msgs)
}

func (h *Handler) postMessage(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	files, err := parseUploads(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	msg, err := h.svc.PostMessage(r.Context(), p.AccountID, p.UserID, idp(r), r.FormValue("body"), files)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, msg)
}

func (h *Handler) downloadAttachment(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	reader, contentType, filename, err := h.svc.DisputeAttachment(r.Context(), p.AccountID, idp(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	defer reader.Close()
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Content-Disposition", "inline; filename=\""+filename+"\"")
	_, _ = io.Copy(w, reader)
}

func (h *Handler) createInvoice(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		BuyerID     int64   `json:"buyer_id"`
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	inv, err := h.svc.CreatePrepayInvoice(r.Context(), p.AccountID, body.BuyerID, body.Amount, body.Description)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, inv)
}

func (h *Handler) pubInvoices(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListPublisherInvoices(r.Context(), p.AccountID, r.URL.Query().Get("status"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []Invoice{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) markInvoicePaid(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		PaymentMethod string `json:"payment_method"`
		Note          string `json:"note"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	inv, err := h.svc.MarkInvoicePaid(r.Context(), p.AccountID, idp(r), p.UserID, body.PaymentMethod, body.Note)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, inv)
}

func (h *Handler) voidInvoice(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	inv, err := h.svc.VoidInvoice(r.Context(), p.AccountID, idp(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, inv)
}

func (h *Handler) buyerInvoices(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListBuyerInvoices(r.Context(), p.AccountID, r.URL.Query().Get("status"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []Invoice{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) createInvoicePayIntent(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	result, err := h.svc.CreateInvoicePaymentIntent(r.Context(), p.AccountID, idp(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) confirmInvoicePayment(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		PaymentIntentID string `json:"payment_intent_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.PaymentIntentID == "" {
		httpx.WriteError(w, httpx.Validation("payment_intent_id is required"))
		return
	}
	if err := h.svc.ConfirmDirectInvoicePayment(r.Context(), p.AccountID, idp(r), body.PaymentIntentID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
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
		DeadlineDays  int    `json:"deadline_days"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	d, err := h.svc.OpenDispute(r.Context(), p.AccountID, p.UserID, body.TransactionID, body.Reason, body.DeadlineDays)
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

// unused import guard for url in redirects — kept for future query escaping
var _ = url.Values{}
