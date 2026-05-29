package billing

import (
	"net/http"
	"strconv"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterPublisher mounts billing oversight: all-buyer ledger + dispute queue.
func (h *Handler) RegisterPublisher(r chi.Router) {
	r.Get("/billing/transactions", h.pubTransactions)
	r.Get("/billing/disputes", h.pubDisputes)
	r.With(auth.RequireRole("admin")).Post("/billing/disputes/{id}/accept", h.accept)
	r.With(auth.RequireRole("admin")).Post("/billing/disputes/{id}/reject", h.reject)
	r.With(auth.RequireRole("admin")).Post("/billing/manual-invoice", h.manualInvoice)
}

// RegisterBuyer mounts the buyer's own balance + ledger + disputes.
func (h *Handler) RegisterBuyer(r chi.Router) {
	r.Get("/billing/balance", h.balance)
	r.Post("/billing/balance/topup", h.topup)
	r.Get("/billing/transactions", h.buyerTransactions)
	r.Get("/billing/disputes", h.buyerDisputes)
	r.Post("/billing/disputes", h.openDispute)
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

func (h *Handler) topup(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Amount float64 `json:"amount"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	t, err := h.svc.Topup(r.Context(), p.AccountID, body.Amount, "balance top-up")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, t)
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
