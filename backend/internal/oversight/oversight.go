// Package oversight provides the publisher admin's read-only view of buyers.
package oversight

import (
	"net/http"
	"strconv"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/billing"
	"github.com/echayko/leadrula/backend/internal/calendar"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/pipelines"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	accounts  *accounts.Repository
	leads     *leads.Repository
	pipelines *pipelines.Service
	billing   *billing.Service
	calendar  *calendar.Service
}

func NewHandler(acc *accounts.Repository, leadRepo *leads.Repository, pl *pipelines.Service, bl *billing.Service, cal *calendar.Service) *Handler {
	return &Handler{accounts: acc, leads: leadRepo, pipelines: pl, billing: bl, calendar: cal}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/buyers", h.listBuyers)
	r.Get("/buyers/{id}", h.getBuyer)
	r.Get("/buyers/{id}/leads", h.buyerLeads)
	r.Get("/buyers/{id}/pipelines", h.buyerPipelines)
	r.Get("/buyers/{id}/pipelines/{pid}/stages", h.buyerStages)
	r.Get("/buyers/{id}/calendar", h.buyerCalendar)
	r.Get("/buyers/{id}/billing", h.buyerBilling)
}

func (h *Handler) listBuyers(w http.ResponseWriter, r *http.Request) {
	items, err := h.accounts.ListBuyers(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) getBuyer(w http.ResponseWriter, r *http.Request) {
	a, err := h.accounts.GetAccount(r.Context(), id(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	bal, _ := h.billing.GetBalance(r.Context(), a.ID)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id": a.ID, "public_id": a.PublicID, "name": a.Name, "type": a.Type,
		"timezone": a.Timezone, "balance": bal,
	})
}

func (h *Handler) buyerLeads(w http.ResponseWriter, r *http.Request) {
	items, err := h.leads.ListByAccount(r.Context(), id(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) buyerPipelines(w http.ResponseWriter, r *http.Request) {
	items, err := h.pipelines.List(r.Context(), id(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) buyerStages(w http.ResponseWriter, r *http.Request) {
	buyerID := id(r)
	pid, _ := strconv.ParseInt(chi.URLParam(r, "pid"), 10, 64)
	items, err := h.pipelines.ListStages(r.Context(), buyerID, pid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) buyerCalendar(w http.ResponseWriter, r *http.Request) {
	items, err := h.calendar.List(r.Context(), id(r), 0, nil, nil)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) buyerBilling(w http.ResponseWriter, r *http.Request) {
	bal, err := h.billing.GetBalance(r.Context(), id(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	txns, err := h.billing.ListTransactions(r.Context(), id(r), "")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"balance": bal, "transactions": txns})
}

func id(r *http.Request) int64 {
	v, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return v
}
