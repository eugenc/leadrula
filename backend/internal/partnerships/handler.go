package partnerships

import (
	"net/http"
	"strconv"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/permissions"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterPublisherRoutes(r chi.Router) {
	r.Route("/partnerships", func(r chi.Router) {
		r.With(auth.RequirePermission(permissions.ActionContractsPartners)).Get("/", h.listPublisher)
		r.With(auth.RequirePermission(permissions.ActionContractsPartners)).Post("/request", h.requestPublisher)
		r.With(auth.RequirePermission(permissions.ActionContractsPartners)).Post("/{id}/accept", h.acceptPublisher)
		r.With(auth.RequirePermission(permissions.ActionContractsPartners)).Post("/{id}/reject", h.rejectPublisher)
		r.With(auth.RequirePermission(permissions.ActionContractsPartners)).Get("/publishers", h.listPartnerPublishers)
		r.With(auth.RequirePermission(permissions.ActionContractsPartners)).Post("/publishers/link", h.linkPublisher)
	})
}

func (h *Handler) RegisterBuyerRoutes(r chi.Router) {
	r.Route("/partnerships", func(r chi.Router) {
		r.With(auth.RequirePermission(permissions.ActionContractsPartners)).Get("/", h.listBuyer)
		r.With(auth.RequirePermission(permissions.ActionContractsPartners)).Post("/request", h.requestBuyer)
		r.With(auth.RequirePermission(permissions.ActionContractsPartners)).Post("/{id}/accept", h.acceptBuyer)
		r.With(auth.RequirePermission(permissions.ActionContractsPartners)).Post("/{id}/reject", h.rejectBuyer)
	})
}

func (h *Handler) listPublisher(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListForPublisher(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []ListItem{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) listBuyer(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListForBuyer(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []ListItem{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) requestPublisher(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		BuyerHandlerID string `json:"buyer_handler_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	item, err := h.svc.RequestFromPublisher(r.Context(), p, body.BuyerHandlerID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) requestBuyer(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		PublisherHandlerID string `json:"publisher_handler_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	item, err := h.svc.RequestFromBuyer(r.Context(), p, body.PublisherHandlerID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) acceptPublisher(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	item, err := h.svc.AcceptForPublisher(r.Context(), p, idParam(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) rejectPublisher(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.RejectForPublisher(r.Context(), p, idParam(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

func (h *Handler) acceptBuyer(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	item, err := h.svc.AcceptForBuyer(r.Context(), p, idParam(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) rejectBuyer(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.RejectForBuyer(r.Context(), p, idParam(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

func (h *Handler) listPartnerPublishers(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := ListPartnerPublishers(r.Context(), h.svc.repo.Pool(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []PartnerPublisher{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) linkPublisher(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		PublisherHandlerID string `json:"publisher_handler_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.RequestPublisherPartnership(r.Context(), p, body.PublisherHandlerID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func idParam(r *http.Request) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id
}
