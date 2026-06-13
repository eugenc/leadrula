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
	r.Get("/contracts/{id}/compensations", h.listCompensations)
	r.Get("/contracts/{id}/lead-criteria", h.getLeadCriteria)
	r.Get("/contracts/{id}/participations", h.listParticipations)
	r.Get("/payouts/summary", h.payoutSummary)
	r.Get("/payouts/by-compensation", h.payoutByCompensation)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole("admin"))
		r.Post("/contracts", h.create)
		r.Patch("/contracts/{id}", h.update)
		r.Delete("/contracts/{id}", h.delete)
		r.Post("/contracts/{id}/return-rules", h.addRule)
		r.Post("/contracts/{id}/compensations", h.addCompensation)
		r.Patch("/contracts/{id}/compensations/{compId}", h.updateCompensation)
		r.Delete("/contracts/{id}/compensations/{compId}", h.deleteCompensation)
		r.Post("/contracts/{id}/leads/{leadId}/accrue", h.accrueManual)
		r.Patch("/contracts/{id}/lead-criteria", h.saveLeadCriteria)
		r.Patch("/contracts/{id}/offer", h.updateOffer)
		r.Post("/contracts/{id}/participations", h.addParticipation)
		r.Post("/contracts/{id}/invites", h.createInvite)
		r.Post("/participations/{id}/accept-counter", h.acceptCounter)
		r.Post("/participations/{id}/reject-counter", h.rejectCounter)
		r.Post("/participations/{id}/reinvite", h.reinviteParticipation)
		r.Patch("/return-rules/{ruleId}", h.updateRule)
		r.Delete("/return-rules/{ruleId}", h.deleteRule)
	})
}

// RegisterBuyer mounts the buyer's read-only contract + return-rule config.
func (h *Handler) RegisterBuyer(r chi.Router) {
	r.Get("/contracts", h.buyerList)
	r.Get("/participations", h.buyerListParticipations)
	r.Get("/participations/{id}", h.buyerGetParticipation)
	r.With(auth.RequireRole("admin")).Post("/participations/{id}/accept", h.buyerAcceptParticipation)
	r.With(auth.RequireRole("admin")).Post("/participations/{id}/decline", h.buyerDeclineParticipation)
	r.With(auth.RequireRole("admin")).Post("/participations/{id}/counter", h.buyerCounterParticipation)
	r.With(auth.RequireRole("admin")).Patch("/participations/{id}", h.buyerUpdateParticipationDelivery)
	r.With(auth.RequireRole("admin")).Patch("/participations/{id}/status", h.buyerUpdateParticipationStatus)
	r.With(auth.RequireRole("admin")).Post("/contract-invites/{token}/attach", h.buyerAttachInvite)
	r.Get("/contracts/{id}/publisher-stages", h.buyerPublisherStages)
	r.Get("/contracts/{id}/return-rules", h.buyerListRules)
	r.Get("/contracts/{id}/compensations", h.buyerListCompensations)
	r.Get("/contracts/{id}/field-map", h.buyerListContractFieldMap)
	r.Get("/contracts/{id}/field-map/options", h.buyerContractFieldMapOptions)
	r.Get("/participations/{id}/compensations", h.buyerListParticipationCompensations)
	r.Get("/participations/{id}/field-map", h.buyerListParticipationFieldMap)
	r.Get("/participations/{id}/field-map/options", h.buyerParticipationFieldMapOptions)
	r.With(auth.RequireRole("admin")).Post("/contracts/{id}/return-rules", h.buyerAddRule)
	r.With(auth.RequireRole("admin")).Patch("/contracts/{id}/return-rules/{ruleId}", h.buyerUpdateRule)
	r.With(auth.RequireRole("admin")).Delete("/contracts/{id}/return-rules/{ruleId}", h.buyerDeleteRule)
	r.With(auth.RequireRole("admin")).Patch("/contracts/{id}/compensations/{compId}", h.buyerPatchContractCompensation)
	r.With(auth.RequireRole("admin")).Patch("/participations/{id}/compensations/{compId}", h.buyerPatchParticipationCompensation)
	r.With(auth.RequireRole("admin")).Post("/contracts/{id}/field-map", h.buyerAddContractFieldMap)
	r.With(auth.RequireRole("admin")).Delete("/contracts/{id}/field-map/{mapId}", h.buyerDeleteContractFieldMap)
	r.With(auth.RequireRole("admin")).Post("/participations/{id}/field-map", h.buyerAddParticipationFieldMap)
	r.With(auth.RequireRole("admin")).Delete("/participations/{id}/field-map/{mapId}", h.buyerDeleteParticipationFieldMap)
	r.Get("/participations/{id}/return-routes", h.buyerListParticipationReturnRoutes)
	r.Get("/participations/{id}/publisher-stages", h.buyerListParticipationPublisherStages)
	r.With(auth.RequireRole("admin")).Post("/participations/{id}/return-routes", h.buyerAddParticipationReturnRoute)
	r.With(auth.RequireRole("admin")).Patch("/participations/{id}/return-routes/{ruleId}", h.buyerUpdateParticipationReturnRoute)
	r.With(auth.RequireRole("admin")).Delete("/participations/{id}/return-routes/{ruleId}", h.buyerDeleteParticipationReturnRoute)
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

type contractCreateBody struct {
	Status                string               `json:"status"`
	BuyerID               int64                `json:"buyer_id"`
	BuyerHandlerID        string               `json:"buyer_handler_id"`
	CounterpartyHandlerID string               `json:"counterparty_handler_id"`
	ContractType          string               `json:"contract_type"`
	Name                  string               `json:"name"`
	Description           string               `json:"description"`
	LeadType              string               `json:"lead_type"`
	CapPeriod             string               `json:"cap_period"`
	CapTotal              *int                 `json:"cap_total"`
	CapMaxDaily           *int                 `json:"cap_max_daily"`
	SourcePipelineID      int64                `json:"source_pipeline_id"`
	SourceStageID         int64                `json:"source_stage_id"`
	BuyerPipelineID       int64                `json:"buyer_pipeline_id"`
	ReturnStageID         int64                `json:"return_stage_id"`
	RatePerLead           float64              `json:"rate_per_lead"`
	Delivery              string               `json:"delivery"`
	Compensations         []CompensationParams `json:"compensations"`
	LeadCriteria         *LeadCriteria        `json:"lead_criteria"`
	AllowedDeliveryModes []string             `json:"allowed_delivery_modes"`
	DistributionStrategy string               `json:"distribution_strategy"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body contractCreateBody
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	contractType := strings.TrimSpace(body.ContractType)
	if contractType == "" {
		contractType = "sell"
	}
	if err := validateContractType("publisher", contractType); err != nil {
		httpx.WriteError(w, err)
		return
	}

	status := strings.TrimSpace(body.Status)
	if status == "" {
		status = "active"
	}
	if status == "draft" {
		c, err := h.svc.CreateDraft(r.Context(), p.AccountID, createParamsFromCreateBody(body, contractType))
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.JSON(w, http.StatusCreated, c)
		return
	}

	buyerID := body.BuyerID
	handlerID := strings.TrimSpace(strings.ToUpper(body.BuyerHandlerID))
	if handlerID == "" {
		handlerID = strings.TrimSpace(strings.ToUpper(body.CounterpartyHandlerID))
	}
	lookupType := ""
	if contractType == "buy" {
		lookupType = "publisher"
	}
	if buyerID == 0 && handlerID != "" {
		var err error
		buyerID, err = h.svc.LookupAccountIDByHandler(r.Context(), handlerID, lookupType)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	if buyerID == 0 && contractType == "sell" && len(body.AllowedDeliveryModes) > 0 {
		if err := validateLeadType(body.LeadType, true); err != nil {
			httpx.WriteError(w, err)
			return
		}
		capPeriod := body.CapPeriod
		if capPeriod == "" {
			capPeriod = "one_time"
		}
		if err := validateCapLimits(capPeriod, body.CapTotal, body.CapMaxDaily); err != nil {
			httpx.WriteError(w, err)
			return
		}
		comps := body.Compensations
		for i := range comps {
			if err := validateCompensationParams(comps[i]); err != nil {
				httpx.WriteError(w, err)
				return
			}
		}
		params := createParamsFromCreateBody(body, contractType)
		params.Compensations = comps
		params.CapPeriod = capPeriod
		c, err := h.svc.CreateActiveOffer(r.Context(), p.AccountID, params)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.JSON(w, http.StatusCreated, c)
		return
	}
	if buyerID == 0 {
		msg := "counterparty is required (or set allowed_delivery_modes for an open sell offer)"
		if contractType == "buy" {
			msg = "publisher counterparty is required"
		}
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, msg)
		return
	}
	ct, err := h.svc.CounterpartyAccountType(r.Context(), buyerID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := ValidateCounterpartyAccountType(contractType, ct); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := validateLeadType(body.LeadType, true); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := validateLeadCriteriaForActivation(body.LeadCriteria); err != nil {
		httpx.WriteError(w, err)
		return
	}
	capPeriod := body.CapPeriod
	if capPeriod == "" {
		capPeriod = "one_time"
	}
	if err := validateCapLimits(capPeriod, body.CapTotal, body.CapMaxDaily); err != nil {
		httpx.WriteError(w, err)
		return
	}
	comps := body.Compensations
	if len(comps) > 0 {
		for i := range comps {
			if err := validateCompensationParams(comps[i]); err != nil {
				httpx.WriteError(w, err)
				return
			}
		}
	}
	params := createParamsFromCreateBody(body, contractType)
	params.BuyerID = buyerID
	params.CapPeriod = capPeriod
	params.Compensations = comps
	c, err := h.svc.Create(r.Context(), p.AccountID, params)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body contractPatchBody
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	contractID := idp(r, "id")
	existing, err := h.svc.Get(r.Context(), p.AccountID, contractID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if body.Status != nil && *body.Status == "draft" && existing.Status != "draft" {
		httpx.WriteError(w, httpx.Validation("cannot revert an active contract to draft"))
		return
	}
	if body.Status != nil && *body.Status == "active" && existing.Status == "draft" {
		params := patchToCreateParams(body, existing)
		var c *Contract
		var err error
		if params.ContractType == "sell" && params.BuyerID == 0 {
			c, err = h.svc.ActivateOfferDraft(r.Context(), p.AccountID, contractID, params)
		} else {
			c, err = h.svc.ActivateDraft(r.Context(), p.AccountID, contractID, params)
		}
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, c)
		return
	}
	if existing.Status == "active" && (body.AllowedDeliveryModes != nil || body.DistributionStrategy != nil) {
		c, err := h.svc.UpdateOffer(r.Context(), p.AccountID, contractID, OfferUpdateParams{
			AllowedDeliveryModes: func() *[]string {
				if body.AllowedDeliveryModes != nil {
					return &body.AllowedDeliveryModes
				}
				return nil
			}(),
			DistributionStrategy: body.DistributionStrategy,
		})
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, c)
		return
	}
	if existing.Status == "draft" && body.isDraftSave() {
		c, err := h.svc.UpdateDraft(r.Context(), p.AccountID, contractID, patchToCreateParams(body, existing))
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, c)
		return
	}
	if body.Status != nil && existing.Status != "draft" {
		if err := validatePublisherContractStatus(*body.Status); err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	if body.LeadType != nil {
		if err := validateLeadType(*body.LeadType, true); err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	patchCap := body.CapPeriod != nil
	if patchCap {
		period := *body.CapPeriod
		if period == "" {
			period = "one_time"
		}
		if err := validateCapLimits(period, body.CapTotal, body.CapMaxDaily); err != nil {
			httpx.WriteError(w, err)
			return
		}
		body.CapPeriod = &period
	}
	if body.Delivery != nil {
		delivery := *body.Delivery
		sourcePipelineID := int64(0)
		sourceStageID := int64(0)
		buyerPipelineID := int64(0)
		returnStageID := int64(0)
		if body.SourcePipelineID != nil {
			sourcePipelineID = *body.SourcePipelineID
		}
		if body.SourceStageID != nil {
			sourceStageID = *body.SourceStageID
		}
		if body.BuyerPipelineID != nil {
			buyerPipelineID = *body.BuyerPipelineID
		}
		if body.ReturnStageID != nil {
			returnStageID = *body.ReturnStageID
		}
		c, err := h.svc.UpdateDelivery(r.Context(), p.AccountID, contractID, DeliveryUpdateParams{
			Delivery:         delivery,
			SourcePipelineID: sourcePipelineID,
			SourceStageID:    sourceStageID,
			BuyerPipelineID:  buyerPipelineID,
			ReturnStageID:    returnStageID,
		})
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, c)
		return
	}
	c, err := h.svc.Update(r.Context(), p.AccountID, contractID, UpdateParams{
		Name:          body.Name,
		RatePerLead:   body.RatePerLead,
		Status:        body.Status,
		Description:   body.Description,
		LeadType:      body.LeadType,
		CapPeriod:     body.CapPeriod,
		CapTotal:      body.CapTotal,
		CapMaxDaily:   body.CapMaxDaily,
		PatchCap:      patchCap,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) getLeadCriteria(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	c, err := h.svc.GetLeadCriteria(r.Context(), p.AccountID, idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) saveLeadCriteria(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var c LeadCriteria
	if !httpx.DecodeJSON(w, r, &c) {
		return
	}
	if err := h.svc.SaveLeadCriteria(r.Context(), p.AccountID, idp(r, "id"), c); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listCompensations(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListCompensations(r.Context(), p.AccountID, idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []Compensation{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) buyerListCompensations(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	cid := idp(r, "id")
	items, err := h.svc.ListCompensationsForBuyer(r.Context(), p.AccountID, cid)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []Compensation{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) addCompensation(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	cid := idp(r, "id")
	params, ok := decodeCompensationBody(w, r)
	if !ok {
		return
	}
	c, err := h.svc.AddCompensation(r.Context(), p.AccountID, cid, params)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *Handler) updateCompensation(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	cid := idp(r, "id")
	compID := idp(r, "compId")
	params, ok := decodeCompensationBody(w, r)
	if !ok {
		return
	}
	c, err := h.svc.UpdateCompensation(r.Context(), p.AccountID, cid, compID, params)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) deleteCompensation(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteCompensation(r.Context(), p.AccountID, idp(r, "id"), idp(r, "compId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) accrueManual(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		CompensationID int64 `json:"compensation_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.CompensationID == 0 {
		httpx.WriteError(w, httpx.Validation("compensation_id is required"))
		return
	}
	if err := h.svc.AccrueManual(r.Context(), p.AccountID, idp(r, "id"), body.CompensationID, idp(r, "leadId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func decodeCompensationBody(w http.ResponseWriter, r *http.Request) (CompensationParams, bool) {
	var body struct {
		Kind                   string   `json:"kind"`
		FlatAmount             *float64 `json:"flat_amount"`
		BidMin                 *float64 `json:"bid_min"`
		BidMax                 *float64 `json:"bid_max"`
		RevPercent             *float64 `json:"rev_percent"`
		ProfitPercent          *float64 `json:"profit_percent"`
		CapPeriod              string   `json:"cap_period"`
		CapTotal               *int     `json:"cap_total"`
		CapMaxDaily            *int     `json:"cap_max_daily"`
		Trigger                string   `json:"trigger"`
		TriggerStageID         *int64   `json:"trigger_stage_id"`
		SourcePipelineID       *int64   `json:"source_pipeline_id"`
		SourceStageID          *int64   `json:"source_stage_id"`
		CounterpartyPipelineID *int64   `json:"counterparty_pipeline_id"`
		CounterpartyStageID    *int64   `json:"counterparty_stage_id"`
		ReturnStageID          *int64   `json:"return_stage_id"`
		Delivery               string   `json:"delivery"`
		Position               int      `json:"position"`
		PayoutFrequency        *string  `json:"payout_frequency"`
		PayoutWeekday          *int     `json:"payout_weekday"`
		PayoutMonthDay         *int     `json:"payout_month_day"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return CompensationParams{}, false
	}
	period := body.CapPeriod
	if period == "" {
		period = "one_time"
	}
	if err := validateCapLimits(period, body.CapTotal, body.CapMaxDaily); err != nil {
		httpx.WriteError(w, err)
		return CompensationParams{}, false
	}
	return CompensationParams{
		Kind:                   body.Kind,
		FlatAmount:             body.FlatAmount,
		BidMin:                 body.BidMin,
		BidMax:                 body.BidMax,
		RevPercent:             body.RevPercent,
		ProfitPercent:          body.ProfitPercent,
		CapPeriod:              period,
		CapTotal:               body.CapTotal,
		CapMaxDaily:            body.CapMaxDaily,
		Trigger:                body.Trigger,
		TriggerStageID:         body.TriggerStageID,
		SourcePipelineID:       body.SourcePipelineID,
		SourceStageID:          body.SourceStageID,
		CounterpartyPipelineID: body.CounterpartyPipelineID,
		CounterpartyStageID:    body.CounterpartyStageID,
		ReturnStageID:          body.ReturnStageID,
		Delivery:               body.Delivery,
		Position:               body.Position,
		PayoutFrequency:        body.PayoutFrequency,
		PayoutWeekday:          body.PayoutWeekday,
		PayoutMonthDay:         body.PayoutMonthDay,
	}, true
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
	httpx.WriteError(w, httpx.Forbidden("return routes on this contract are configured by the publisher"))
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
	httpx.WriteError(w, httpx.Forbidden("return routes on this contract are configured by the publisher"))
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
	httpx.WriteError(w, httpx.Forbidden("return routes on this contract are configured by the publisher"))
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

var allowedLeadTypes = map[string]bool{
	"Data": true, "Appointment": true, "Call": true,
}

var allowedPublisherContractStatuses = map[string]bool{
	"active": true, "paused": true, "terminated": true,
}

func validatePublisherContractStatus(status string) error {
	status = strings.TrimSpace(status)
	if !allowedPublisherContractStatuses[status] {
		return httpx.Validation("status must be active, paused, or terminated")
	}
	return nil
}

func validateLeadType(s string, required bool) error {
	s = strings.TrimSpace(s)
	if s == "" {
		if required {
			return httpx.Validation("lead_type is required")
		}
		return nil
	}
	if !allowedLeadTypes[s] {
		return httpx.Validation("lead_type must be Data, Appointment, or Call")
	}
	return nil
}

var allowedCapPeriods = map[string]bool{
	"one_time": true, "weekly": true, "monthly": true,
}

func validateContractType(ownerType, contractType string) error {
	contractType = strings.TrimSpace(contractType)
	if contractType != "buy" && contractType != "sell" {
		return httpx.Validation("contract_type must be buy or sell")
	}
	if ownerType == "buyer" && contractType != "buy" {
		return httpx.Validation("buyer accounts may only create buy contracts")
	}
	return nil
}

func validateCapLimits(period string, capTotal, capMaxDaily *int) error {
	period = strings.TrimSpace(period)
	if !allowedCapPeriods[period] {
		return httpx.Validation("cap_period must be one_time, weekly, or monthly")
	}
	if capTotal != nil && *capTotal <= 0 {
		return httpx.Validation("cap_total must be greater than 0")
	}
	if capMaxDaily != nil && *capMaxDaily <= 0 {
		return httpx.Validation("cap_max_daily must be greater than 0")
	}
	if capMaxDaily != nil && period != "weekly" && period != "monthly" {
		return httpx.Validation("cap_max_daily is only allowed for weekly or monthly cap periods")
	}
	return nil
}

func (h *Handler) payoutSummary(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	sum, err := h.svc.PayoutSummary(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sum)
}

func (h *Handler) payoutByCompensation(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rows, err := h.svc.PayoutByCompensation(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if rows == nil {
		rows = []CompensationPayoutRow{}
	}
	httpx.JSON(w, http.StatusOK, rows)
}

type contractPatchBody struct {
	Name               *string              `json:"name"`
	RatePerLead        *float64             `json:"rate_per_lead"`
	Status             *string              `json:"status"`
	Description        *string              `json:"description"`
	LeadType           *string              `json:"lead_type"`
	ContractType       *string              `json:"contract_type"`
	CapPeriod          *string              `json:"cap_period"`
	CapTotal           *int                 `json:"cap_total"`
	CapMaxDaily        *int                 `json:"cap_max_daily"`
	BuyerID            *int64               `json:"buyer_id"`
	Delivery           *string              `json:"delivery"`
	SourcePipelineID   *int64               `json:"source_pipeline_id"`
	SourceStageID      *int64               `json:"source_stage_id"`
	BuyerPipelineID    *int64               `json:"buyer_pipeline_id"`
	ReturnStageID      *int64               `json:"return_stage_id"`
	Compensations      []CompensationParams `json:"compensations"`
	LeadCriteria          *LeadCriteria        `json:"lead_criteria"`
	AllowedDeliveryModes  []string             `json:"allowed_delivery_modes"`
	DistributionStrategy  *string              `json:"distribution_strategy"`
}

func (b contractPatchBody) isDraftSave() bool {
	return b.Name != nil || b.BuyerID != nil || b.ContractType != nil || b.LeadType != nil ||
		b.Description != nil || b.Delivery != nil || b.Compensations != nil || b.LeadCriteria != nil ||
		b.SourcePipelineID != nil || b.SourceStageID != nil || b.BuyerPipelineID != nil || b.ReturnStageID != nil ||
		b.RatePerLead != nil || b.CapPeriod != nil || b.CapTotal != nil || b.CapMaxDaily != nil ||
		len(b.AllowedDeliveryModes) > 0 || b.DistributionStrategy != nil
}

func createParamsFromCreateBody(body contractCreateBody, contractType string) CreateParams {
	delivery := strings.TrimSpace(body.Delivery)
	if delivery == "" && len(body.Compensations) > 0 {
		delivery = body.Compensations[0].Delivery
	}
	capPeriod := body.CapPeriod
	if capPeriod == "" {
		capPeriod = "one_time"
	}
	return CreateParams{
		BuyerID:          body.BuyerID,
		ContractType:     contractType,
		Name:             body.Name,
		Description:      body.Description,
		LeadType:         body.LeadType,
		CapPeriod:        capPeriod,
		CapTotal:         body.CapTotal,
		CapMaxDaily:      body.CapMaxDaily,
		SourcePipelineID: body.SourcePipelineID,
		SourceStageID:    body.SourceStageID,
		BuyerPipelineID:  body.BuyerPipelineID,
		ReturnStageID:    body.ReturnStageID,
		RatePerLead:      body.RatePerLead,
		Delivery:         delivery,
		Compensations:          body.Compensations,
		LeadCriteria:           body.LeadCriteria,
		AllowedDeliveryModes:   body.AllowedDeliveryModes,
		DistributionStrategy:   body.DistributionStrategy,
	}
}

func patchToCreateParams(body contractPatchBody, existing *Contract) CreateParams {
	name := existing.Name
	if body.Name != nil {
		name = *body.Name
	}
	desc := existing.Description
	if body.Description != nil {
		desc = *body.Description
	}
	leadType := existing.LeadType
	if body.LeadType != nil {
		leadType = *body.LeadType
	}
	contractType := existing.ContractType
	if body.ContractType != nil {
		contractType = *body.ContractType
	}
	buyerID := derefInt64(existing.BuyerID)
	if body.BuyerID != nil {
		buyerID = *body.BuyerID
	}
	capPeriod := existing.CapPeriod
	if body.CapPeriod != nil {
		capPeriod = *body.CapPeriod
	}
	capTotal := existing.CapTotal
	if body.CapTotal != nil {
		capTotal = body.CapTotal
	}
	capMaxDaily := existing.CapMaxDaily
	if body.CapMaxDaily != nil {
		capMaxDaily = body.CapMaxDaily
	}
	rate := existing.RatePerLead
	if body.RatePerLead != nil {
		rate = *body.RatePerLead
	}
	sourcePipeline := derefInt64(existing.SourcePipelineID)
	if body.SourcePipelineID != nil {
		sourcePipeline = *body.SourcePipelineID
	}
	sourceStage := derefInt64(existing.SourceStageID)
	if body.SourceStageID != nil {
		sourceStage = *body.SourceStageID
	}
	buyerPipeline := derefInt64(existing.BuyerPipelineID)
	if body.BuyerPipelineID != nil {
		buyerPipeline = *body.BuyerPipelineID
	}
	returnStage := derefInt64(existing.ReturnStageID)
	if body.ReturnStageID != nil {
		returnStage = *body.ReturnStageID
	}
	delivery := ""
	if body.Delivery != nil {
		delivery = *body.Delivery
	}
	comps := body.Compensations
	if comps == nil {
		comps = []CompensationParams{}
	}
	criteria := body.LeadCriteria
	modes := existing.AllowedDeliveryModes
	if body.AllowedDeliveryModes != nil {
		modes = body.AllowedDeliveryModes
	}
	strategy := existing.DistributionStrategy
	if body.DistributionStrategy != nil {
		strategy = *body.DistributionStrategy
	}
	return CreateParams{
		BuyerID:                buyerID,
		ContractType:           contractType,
		Name:                   name,
		Description:            desc,
		LeadType:               leadType,
		CapPeriod:              capPeriod,
		CapTotal:               capTotal,
		CapMaxDaily:      capMaxDaily,
		SourcePipelineID: sourcePipeline,
		SourceStageID:    sourceStage,
		BuyerPipelineID:  buyerPipeline,
		ReturnStageID:    returnStage,
		RatePerLead:      rate,
		Delivery:         delivery,
		Compensations:          comps,
		LeadCriteria:           criteria,
		AllowedDeliveryModes:   modes,
		DistributionStrategy:   strategy,
	}
}
