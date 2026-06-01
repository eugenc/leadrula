package collaboration

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
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
	r.Route("/collaboration", func(r chi.Router) {
		r.With(auth.RequireRole("admin")).Get("/summaries", h.listSummaries)
		r.With(auth.RequireRole("admin")).Get("/buyers/{buyerId}", h.publisherStatus)
		r.With(auth.RequireRole("admin")).Post("/buyers/{buyerId}/request", h.publisherRequest)
		r.With(auth.RequireRole("admin")).Post("/buyers/{buyerId}/accept", h.publisherAccept)
		r.With(auth.RequireRole("admin")).Post("/buyers/{buyerId}/reject", h.publisherReject)
		r.With(auth.RequireRole("admin")).Post("/accept", h.publisherAcceptByPublicID)
		r.With(auth.RequireRole("admin")).Post("/reject", h.publisherRejectByPublicID)
		r.With(auth.RequireRole("admin")).Get("/logs/actors", h.publisherLogActors)
		r.With(auth.RequireRole("admin")).Get("/logs", h.publisherLogs)
	})
}

func (h *Handler) RegisterBuyerRoutes(r chi.Router) {
	r.Get("/publisher", h.buyerPublisher)
	r.With(auth.RequireRole("admin")).Get("/logs/actors", h.buyerLogActors)
	r.With(auth.RequireRole("admin")).Get("/logs", h.buyerLogs)
	r.Route("/collaboration", func(r chi.Router) {
		r.With(auth.RequireRole("admin")).Get("/", h.buyerStatus)
		r.With(auth.RequireRole("admin")).Post("/invite", h.buyerInvite)
		r.With(auth.RequireRole("admin")).Post("/accept", h.buyerAccept)
		r.With(auth.RequireRole("admin")).Post("/reject", h.buyerReject)
		r.With(auth.RequireRole("admin")).Delete("/", h.buyerRevoke)
		r.Route("/publishers/{publisherId}", func(r chi.Router) {
			r.With(auth.RequireRole("admin")).Get("/", h.buyerPublisherCollabStatus)
			r.With(auth.RequireRole("admin")).Post("/invite", h.buyerPublisherInvite)
			r.With(auth.RequireRole("admin")).Post("/accept", h.buyerPublisherAccept)
			r.With(auth.RequireRole("admin")).Post("/reject", h.buyerPublisherReject)
			r.With(auth.RequireRole("admin")).Delete("/", h.buyerPublisherRevoke)
		})
	})
}

func (h *Handler) listSummaries(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListSummaries(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) publisherStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	buyerID, err := buyerIDParam(r)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, err.Error())
		return
	}
	res, err := h.svc.StatusForPublisher(r.Context(), p.AccountID, buyerID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) publisherRequest(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	buyerID, err := buyerIDParam(r)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, err.Error())
		return
	}
	res, err := h.svc.RequestFromPublisher(r.Context(), p, buyerID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) publisherAccept(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	buyerID, err := buyerIDParam(r)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, err.Error())
		return
	}
	res, err := h.svc.AcceptForPublisher(r.Context(), p, buyerID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) publisherReject(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	buyerID, err := buyerIDParam(r)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, err.Error())
		return
	}
	res, err := h.svc.RejectForPublisher(r.Context(), p, buyerID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) publisherAcceptByPublicID(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		BuyerID string `json:"buyer_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	res, err := h.svc.AcceptByBuyerPublicID(r.Context(), p, body.BuyerID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) publisherRejectByPublicID(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		BuyerID string `json:"buyer_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	res, err := h.svc.RejectByBuyerPublicID(r.Context(), p, body.BuyerID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) buyerStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	res, err := h.svc.StatusForBuyer(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) buyerPublisher(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	res, err := h.svc.PublisherForBuyer(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) publisherLogs(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	params, ok := parseAuditListParams(w, r)
	if !ok {
		return
	}
	res, err := h.svc.AuditLogListForPublisher(r.Context(), p.AccountID, params)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) publisherLogActors(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.AuditActorsForPublisher(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func parseAuditListParams(w http.ResponseWriter, r *http.Request) (AuditListParams, bool) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	params := AuditListParams{Page: page, Limit: limit}

	if fromRaw := q.Get("from"); fromRaw != "" {
		t, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "invalid from datetime")
			return params, false
		}
		params.From = &t
	}
	if toRaw := q.Get("to"); toRaw != "" {
		t, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "invalid to datetime")
			return params, false
		}
		params.To = &t
	}
	if actorRaw := q.Get("actor_user_id"); actorRaw != "" {
		actorID, err := strconv.ParseInt(actorRaw, 10, 64)
		if err != nil || actorID <= 0 {
			httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "invalid actor_user_id")
			return params, false
		}
		params.ActorUserID = &actorID
	}
	if buyerRaw := q.Get("buyer_id"); buyerRaw != "" {
		buyerID, err := strconv.ParseInt(buyerRaw, 10, 64)
		if err != nil || buyerID <= 0 {
			httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "invalid buyer_id")
			return params, false
		}
		params.FilterBuyerID = &buyerID
	}
	return params, true
}

func (h *Handler) buyerLogs(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	params, ok := parseAuditListParams(w, r)
	if !ok {
		return
	}
	res, err := h.svc.AuditLogListForBuyer(r.Context(), p.AccountID, params)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) buyerLogActors(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.AuditActorsForBuyer(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) buyerInvite(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Email string `json:"email"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	res, err := h.svc.InvitePublisherUser(r.Context(), p, body.Email)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) buyerAccept(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	res, err := h.svc.AcceptForBuyer(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) buyerReject(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	res, err := h.svc.RejectForBuyer(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) buyerRevoke(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	res, err := h.svc.Revoke(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) buyerPublisherCollabStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	pubID, err := publisherIDParam(r)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, err.Error())
		return
	}
	res, err := h.svc.StatusForPublisher(r.Context(), pubID, p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) buyerPublisherInvite(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	pubID, err := publisherIDParam(r)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, err.Error())
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	res, err := h.svc.InvitePublisherUserForPublisher(r.Context(), p, pubID, body.Email)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) buyerPublisherAccept(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	pubID, err := publisherIDParam(r)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, err.Error())
		return
	}
	res, err := h.svc.AcceptForBuyerPublisher(r.Context(), p, pubID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) buyerPublisherReject(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	pubID, err := publisherIDParam(r)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, err.Error())
		return
	}
	res, err := h.svc.RejectForBuyerPublisher(r.Context(), p, pubID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) buyerPublisherRevoke(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	pubID, err := publisherIDParam(r)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, err.Error())
		return
	}
	res, err := h.svc.RevokeForBuyerPublisher(r.Context(), p, pubID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) RegisterAuthRoutes(r chi.Router) {
	r.Post("/auth/impersonate", h.impersonate)
	r.Post("/auth/impersonate/end", h.endImpersonate)
}

func (h *Handler) impersonate(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		BuyerID string `json:"buyer_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	res, err := h.svc.StartImpersonation(r.Context(), p, body.BuyerID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) endImpersonate(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.EndImpersonation(r.Context(), p); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func buyerIDParam(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "buyerId"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errInvalidBuyerID
	}
	return id, nil
}

var errInvalidBuyerID = errors.New("invalid buyer id")
var errInvalidPublisherID = errors.New("invalid publisher id")

func publisherIDParam(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "publisherId"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errInvalidPublisherID
	}
	return id, nil
}
