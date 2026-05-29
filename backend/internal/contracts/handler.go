package contracts

import (
	"net/http"
	"strconv"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterPublisher mounts the publisher contract management routes (admin).
func (h *Handler) RegisterPublisher(r chi.Router) {
	r.Get("/contracts", h.list)
	r.Get("/contracts/{id}", h.get)
	r.Get("/contracts/{id}/return-rules", h.listRules)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole("admin"))
		r.Post("/contracts", h.create)
		r.Patch("/contracts/{id}", h.update)
		r.Delete("/contracts/{id}", h.delete)
		r.Post("/contracts/{id}/return-rules", h.addRule)
		r.Delete("/return-rules/{ruleId}", h.deleteRule)
	})
}

// RegisterBuyer mounts the buyer's read-only contract + return-rule config.
func (h *Handler) RegisterBuyer(r chi.Router) {
	r.Get("/contract", h.buyerContract)
	r.Get("/contract/return-rules", h.buyerListRules)
	r.With(auth.RequireRole("admin")).Post("/contract/return-rules", h.buyerAddRule)
	r.With(auth.RequireRole("admin")).Delete("/contract/return-rules/{ruleId}", h.deleteRule)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.List(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	c, err := h.svc.Get(r.Context(), p.AccountID, idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		BuyerID          int64   `json:"buyer_id"`
		Name             string  `json:"name"`
		SourcePipelineID int64   `json:"source_pipeline_id"`
		SourceStageID    int64   `json:"source_stage_id"`
		BuyerPipelineID  int64   `json:"buyer_pipeline_id"`
		ReturnStageID    int64   `json:"return_stage_id"`
		RatePerLead      float64 `json:"rate_per_lead"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	c, err := h.svc.Create(r.Context(), p.AccountID, CreateParams{
		BuyerID:          body.BuyerID,
		Name:             body.Name,
		SourcePipelineID: body.SourcePipelineID,
		SourceStageID:    body.SourceStageID,
		BuyerPipelineID:  body.BuyerPipelineID,
		ReturnStageID:    body.ReturnStageID,
		RatePerLead:      body.RatePerLead,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name        *string  `json:"name"`
		RatePerLead *float64 `json:"rate_per_lead"`
		Status      *string  `json:"status"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	c, err := h.svc.Update(r.Context(), p.AccountID, idp(r, "id"), body.Name, body.RatePerLead, body.Status)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.Delete(r.Context(), p.AccountID, idp(r, "id")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listRules(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if _, err := h.svc.Get(r.Context(), p.AccountID, idp(r, "id")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	rules, err := h.svc.ListReturnRules(r.Context(), idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rules)
}

func (h *Handler) addRule(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if _, err := h.svc.Get(r.Context(), p.AccountID, idp(r, "id")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	var body struct {
		BuyerStageID int64 `json:"buyer_stage_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	rr, err := h.svc.AddReturnRule(r.Context(), idp(r, "id"), body.BuyerStageID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, rr)
}

func (h *Handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteReturnRule(r.Context(), idp(r, "ruleId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) buyerContract(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	c, err := h.svc.GetForBuyer(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) buyerListRules(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	cid, err := h.svc.ContractIDForBuyer(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	rules, err := h.svc.ListReturnRules(r.Context(), cid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rules)
}

func (h *Handler) buyerAddRule(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	cid, err := h.svc.ContractIDForBuyer(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var body struct {
		BuyerStageID int64 `json:"buyer_stage_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	rr, err := h.svc.AddReturnRule(r.Context(), cid, body.BuyerStageID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, rr)
}

func idp(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	return id
}
