package contracts

import (
	"net/http"
	"strconv"
	"strings"

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
		r.Patch("/return-rules/{ruleId}", h.updateRule)
		r.Delete("/return-rules/{ruleId}", h.deleteRule)
	})
}

// RegisterBuyer mounts the buyer's read-only contract + return-rule config.
func (h *Handler) RegisterBuyer(r chi.Router) {
	r.Get("/contracts", h.buyerList)
	r.Get("/contracts/{id}/publisher-stages", h.buyerPublisherStages)
	r.Get("/contracts/{id}/return-rules", h.buyerListRules)
	r.With(auth.RequireRole("admin")).Post("/contracts/{id}/return-rules", h.buyerAddRule)
	r.With(auth.RequireRole("admin")).Patch("/contracts/{id}/return-rules/{ruleId}", h.buyerUpdateRule)
	r.With(auth.RequireRole("admin")).Delete("/contracts/{id}/return-rules/{ruleId}", h.buyerDeleteRule)
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
		BuyerHandlerID   string  `json:"buyer_handler_id"`
		Name             string  `json:"name"`
		Description      string  `json:"description"`
		LeadType         string  `json:"lead_type"`
		SourcePipelineID int64   `json:"source_pipeline_id"`
		SourceStageID    int64   `json:"source_stage_id"`
		BuyerPipelineID  int64   `json:"buyer_pipeline_id"`
		ReturnStageID    int64   `json:"return_stage_id"`
		RatePerLead      float64 `json:"rate_per_lead"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	buyerID := body.BuyerID
	if buyerID == 0 && strings.TrimSpace(body.BuyerHandlerID) != "" {
		var err error
		buyerID, err = h.svc.LookupBuyerIDByHandler(r.Context(), strings.TrimSpace(strings.ToUpper(body.BuyerHandlerID)))
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	if buyerID == 0 {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "buyer_id or buyer_handler_id is required")
		return
	}
	c, err := h.svc.Create(r.Context(), p.AccountID, CreateParams{
		BuyerID:          buyerID,
		Name:             body.Name,
		Description:      body.Description,
		LeadType:         body.LeadType,
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
		Description *string  `json:"description"`
		LeadType    *string  `json:"lead_type"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	c, err := h.svc.Update(r.Context(), p.AccountID, idp(r, "id"), UpdateParams{
		Name:        body.Name,
		RatePerLead: body.RatePerLead,
		Status:      body.Status,
		Description: body.Description,
		LeadType:    body.LeadType,
	})
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
		BuyerStageID  int64 `json:"buyer_stage_id"`
		ReturnStageID int64 `json:"return_stage_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.BuyerStageID == 0 || body.ReturnStageID == 0 {
		httpx.WriteError(w, httpx.Validation("buyer_stage_id and return_stage_id are required"))
		return
	}
	rr, err := h.svc.AddReturnRule(r.Context(), idp(r, "id"), body.BuyerStageID, body.ReturnStageID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, rr)
}

func (h *Handler) updateRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BuyerStageID  int64 `json:"buyer_stage_id"`
		ReturnStageID int64 `json:"return_stage_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.BuyerStageID == 0 || body.ReturnStageID == 0 {
		httpx.WriteError(w, httpx.Validation("buyer_stage_id and return_stage_id are required"))
		return
	}
	rr, err := h.svc.UpdateReturnRule(r.Context(), idp(r, "ruleId"), body.BuyerStageID, body.ReturnStageID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rr)
}

func (h *Handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteReturnRule(r.Context(), idp(r, "ruleId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) buyerList(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListForBuyer(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) buyerListRules(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	cid := idp(r, "id")
	if _, err := h.svc.GetForBuyerContract(r.Context(), p.AccountID, cid); err != nil {
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
	cid := idp(r, "id")
	if _, err := h.svc.GetForBuyerContract(r.Context(), p.AccountID, cid); err != nil {
		httpx.WriteError(w, err)
		return
	}
	var body struct {
		BuyerStageID  int64 `json:"buyer_stage_id"`
		ReturnStageID int64 `json:"return_stage_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.BuyerStageID == 0 || body.ReturnStageID == 0 {
		httpx.WriteError(w, httpx.Validation("buyer_stage_id and return_stage_id are required"))
		return
	}
	rr, err := h.svc.AddReturnRule(r.Context(), cid, body.BuyerStageID, body.ReturnStageID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, rr)
}

func (h *Handler) buyerUpdateRule(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	cid := idp(r, "id")
	ruleID := idp(r, "ruleId")
	ruleContractID, err := h.svc.ReturnRuleBelongsToBuyer(r.Context(), p.AccountID, ruleID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if ruleContractID != cid {
		httpx.WriteError(w, httpx.NotFound("return rule not found"))
		return
	}
	var body struct {
		BuyerStageID  int64 `json:"buyer_stage_id"`
		ReturnStageID int64 `json:"return_stage_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.BuyerStageID == 0 || body.ReturnStageID == 0 {
		httpx.WriteError(w, httpx.Validation("buyer_stage_id and return_stage_id are required"))
		return
	}
	rr, err := h.svc.UpdateReturnRule(r.Context(), ruleID, body.BuyerStageID, body.ReturnStageID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rr)
}

func (h *Handler) buyerDeleteRule(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	cid := idp(r, "id")
	ruleID := idp(r, "ruleId")
	ruleContractID, err := h.svc.ReturnRuleBelongsToBuyer(r.Context(), p.AccountID, ruleID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if ruleContractID != cid {
		httpx.WriteError(w, httpx.NotFound("return rule not found"))
		return
	}
	if err := h.svc.DeleteReturnRule(r.Context(), ruleID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) buyerPublisherStages(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	cid := idp(r, "id")
	if _, err := h.svc.GetForBuyerContract(r.Context(), p.AccountID, cid); err != nil {
		httpx.WriteError(w, err)
		return
	}
	stages, err := h.svc.PublisherReturnStages(r.Context(), cid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, stages)
}

func idp(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	return id
}
