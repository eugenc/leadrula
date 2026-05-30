// Package oversight provides the publisher admin's read-only view of buyers.
package oversight

import (
	"context"
	"net/http"
	"strconv"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/billing"
	"github.com/echayko/leadrula/backend/internal/calendar"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/pipelines"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	accounts    *accounts.Repository
	accountsSvc *accounts.Service
	leads       *leads.Repository
	pipelines   *pipelines.Service
	billing     *billing.Service
	calendar    *calendar.Service
	collab      CollabGranter
}

type CollabGranter interface {
	GrantOnCreate(ctx context.Context, publisherID, buyerID, requestedBy int64) error
}

func NewHandler(acc *accounts.Repository, accSvc *accounts.Service, leadRepo *leads.Repository, pl *pipelines.Service, bl *billing.Service, cal *calendar.Service, collab CollabGranter) *Handler {
	return &Handler{accounts: acc, accountsSvc: accSvc, leads: leadRepo, pipelines: pl, billing: bl, calendar: cal, collab: collab}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/buyers", h.listBuyers)
	r.With(auth.RequireRole("admin")).Post("/buyers", h.createBuyer)
	r.With(auth.RequireRole("admin")).Patch("/buyers/{id}", h.updateBuyer)
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

func (h *Handler) createBuyer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name               string  `json:"name"`
		Website            string  `json:"website"`
		Timezone           string  `json:"timezone"`
		AdminFirstName     string  `json:"admin_first_name"`
		AdminLastName      string  `json:"admin_last_name"`
		AdminEmail         string  `json:"admin_email"`
		StartingBalance    float64 `json:"starting_balance"`
		CollaborateEnabled bool    `json:"collaborate_enabled"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	buyer, err := h.accountsSvc.CreateBuyer(r.Context(), accounts.CreateBuyerParams{
		Name:            body.Name,
		Website:         body.Website,
		Timezone:        body.Timezone,
		AdminEmail:      body.AdminEmail,
		AdminFirstName:  body.AdminFirstName,
		AdminLastName:   body.AdminLastName,
		StartingBalance: body.StartingBalance,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if body.CollaborateEnabled && h.collab != nil {
		p := auth.FromContext(r.Context())
		if err := h.collab.GrantOnCreate(r.Context(), p.AccountID, buyer.ID, p.UserID); err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	httpx.JSON(w, http.StatusCreated, buyer)
}

func (h *Handler) getBuyer(w http.ResponseWriter, r *http.Request) {
	a, err := h.accounts.GetAccount(r.Context(), id(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if a.Type != "buyer" {
		httpx.WriteError(w, httpx.NotFound("buyer not found"))
		return
	}
	httpx.JSON(w, http.StatusOK, h.buyerDetail(r.Context(), a))
}

func (h *Handler) updateBuyer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     *string `json:"name"`
		Website  *string `json:"website"`
		Timezone *string `json:"timezone"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	a, err := h.accountsSvc.UpdateBuyer(r.Context(), id(r), accounts.UpdateBuyerParams{
		Name: body.Name, Website: body.Website, Timezone: body.Timezone,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, h.buyerDetail(r.Context(), a))
}

func (h *Handler) buyerDetail(ctx context.Context, a *accounts.Account) map[string]any {
	bal, _ := h.billing.GetBalance(ctx, a.ID)
	out := map[string]any{
		"id": a.ID, "public_id": a.PublicID, "name": a.Name, "type": a.Type,
		"website": a.Website, "timezone": a.Timezone, "balance": bal,
		"admin_name": "", "admin_email": "",
	}
	admin, err := h.accounts.PrimaryAdminContact(ctx, a.ID)
	if err == nil && admin != nil {
		out["admin_name"] = admin.FullName
		out["admin_email"] = admin.Email
	}
	return out
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
