package contracts

import (
	"encoding/json"
	"net/http"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) listParticipations(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListParticipations(r.Context(), p.AccountID, idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []Participation{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) addParticipation(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		BuyerID        int64  `json:"buyer_id"`
		BuyerHandlerID string `json:"buyer_handler_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	buyerID := body.BuyerID
	if buyerID == 0 && body.BuyerHandlerID != "" {
		var err error
		buyerID, err = h.svc.LookupBuyerIDByHandler(r.Context(), body.BuyerHandlerID)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	part, err := h.svc.AddParticipation(r.Context(), p.AccountID, idp(r, "id"), AddParticipationParams{BuyerID: buyerID})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, part)
}

func (h *Handler) createInvite(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	info, err := h.svc.EnsureInviteToken(r.Context(), p.AccountID, idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, info)
}

func (h *Handler) acceptCounter(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	c, err := h.svc.AcceptCounter(r.Context(), p.AccountID, idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) rejectCounter(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	part, err := h.svc.GetParticipationForPublisher(r.Context(), p.AccountID, idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if part.Status != "counter_pending" {
		httpx.WriteError(w, httpx.Validation("participation has no pending counter-offer"))
		return
	}
	updated, err := h.svc.DeclineParticipationByPublisher(r.Context(), p.AccountID, part.ID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func (h *Handler) reinviteParticipation(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	part, err := h.svc.ReinviteParticipation(r.Context(), p.AccountID, idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, part)
}

func (h *Handler) buyerListParticipations(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListParticipationsForBuyer(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []Participation{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) buyerGetParticipation(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	part, err := h.svc.GetParticipationForBuyer(r.Context(), p.AccountID, idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, part)
}

func (h *Handler) buyerAcceptParticipation(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body AcceptParticipationParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	part, err := h.svc.AcceptParticipation(r.Context(), p.AccountID, idp(r, "id"), body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, part)
}

func (h *Handler) buyerDeclineParticipation(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	part, err := h.svc.DeclineParticipation(r.Context(), p.AccountID, idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, part)
}

func (h *Handler) buyerCounterParticipation(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body json.RawMessage
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	part, err := h.svc.CounterParticipation(r.Context(), p.AccountID, idp(r, "id"), body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, part)
}

func (h *Handler) buyerAttachInvite(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	token := chi.URLParam(r, "token")
	part, err := h.svc.AttachByInvite(r.Context(), p.AccountID, token)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, part)
}

func (h *Handler) buyerUpdateParticipationDelivery(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body AcceptParticipationParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	part, err := h.svc.UpdateParticipationDelivery(r.Context(), p.AccountID, idp(r, "id"), body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, part)
}

func (h *Handler) buyerUpdateParticipationStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Status string `json:"status"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	part, err := h.svc.UpdateParticipationStatus(r.Context(), p.AccountID, idp(r, "id"), body.Status)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, part)
}

func (h *Handler) updateOffer(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		AllowedDeliveryModes *[]string `json:"allowed_delivery_modes"`
		DistributionStrategy *string   `json:"distribution_strategy"`
		SourcePipelineID     *int64    `json:"source_pipeline_id"`
		SourceStageID        *int64    `json:"source_stage_id"`
		ReturnStageID        *int64    `json:"return_stage_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	c, err := h.svc.UpdateOffer(r.Context(), p.AccountID, idp(r, "id"), OfferUpdateParams{
		AllowedDeliveryModes: body.AllowedDeliveryModes,
		DistributionStrategy: body.DistributionStrategy,
		SourcePipelineID:     body.SourcePipelineID,
		SourceStageID:        body.SourceStageID,
		ReturnStageID:        body.ReturnStageID,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) buyerListParticipationReturnRoutes(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	partID := idp(r, "id")
	if _, err := h.svc.GetParticipationForBuyer(r.Context(), p.AccountID, partID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	rules, err := h.svc.ListParticipationReturnRules(r.Context(), partID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rules)
}

func (h *Handler) buyerListParticipationPublisherStages(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	partID := idp(r, "id")
	part, err := h.svc.GetParticipationForBuyer(r.Context(), p.AccountID, partID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	stages, err := h.svc.PublisherReturnStages(r.Context(), part.ContractID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, stages)
}

func (h *Handler) buyerAddParticipationReturnRoute(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	partID := idp(r, "id")
	if _, err := h.svc.GetParticipationForBuyer(r.Context(), p.AccountID, partID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	var body struct {
		BuyerStageID     int64 `json:"buyer_stage_id"`
		BuyerPipelineID  int64 `json:"buyer_pipeline_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.BuyerStageID == 0 {
		httpx.WriteError(w, httpx.Validation("buyer_stage_id is required"))
		return
	}
	rr, err := h.svc.AddParticipationReturnRule(r.Context(), partID, body.BuyerStageID, body.BuyerPipelineID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, rr)
}

func (h *Handler) buyerUpdateParticipationReturnRoute(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	partID := idp(r, "id")
	ruleID := idp(r, "ruleId")
	if _, err := h.svc.GetParticipationForBuyer(r.Context(), p.AccountID, partID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	var body struct {
		BuyerStageID    int64 `json:"buyer_stage_id"`
		BuyerPipelineID int64 `json:"buyer_pipeline_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.BuyerStageID == 0 {
		httpx.WriteError(w, httpx.Validation("buyer_stage_id is required"))
		return
	}
	rr, err := h.svc.UpdateParticipationReturnRule(r.Context(), partID, ruleID, body.BuyerStageID, body.BuyerPipelineID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rr)
}

func (h *Handler) buyerDeleteParticipationReturnRoute(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	partID := idp(r, "id")
	ruleID := idp(r, "ruleId")
	part, err := h.svc.GetParticipationForBuyer(r.Context(), p.AccountID, partID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if !participationMutable(part.Status) {
		httpx.WriteError(w, httpx.Validation("participation cannot be edited"))
		return
	}
	var ruleParticipationID int64
	err = h.svc.Pool().QueryRow(r.Context(),
		`SELECT participation_id FROM contract_return_rules WHERE id = $1`, ruleID).Scan(&ruleParticipationID)
	if err != nil || ruleParticipationID != partID {
		httpx.WriteError(w, httpx.NotFound("return route not found"))
		return
	}
	if err := h.svc.DeleteReturnRule(r.Context(), ruleID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
