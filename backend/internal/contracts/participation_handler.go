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

func (h *Handler) updateOffer(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		AllowedDeliveryModes   *[]string `json:"allowed_delivery_modes"`
		DistributionStrategy   *string   `json:"distribution_strategy"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	c, err := h.svc.UpdateOffer(r.Context(), p.AccountID, idp(r, "id"), OfferUpdateParams{
		AllowedDeliveryModes: body.AllowedDeliveryModes,
		DistributionStrategy: body.DistributionStrategy,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}
