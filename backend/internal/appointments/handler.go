package appointments

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterBuyer(r chi.Router) {
	r.Get("/availability", h.getAvailability)
	r.With(auth.RequireRole("admin")).Put("/availability", h.putAvailability)
	r.Get("/appointment-slots", h.listBuyerSlots)
	r.With(auth.RequireRole("admin")).Post("/appointment-slots", h.createBuyerSlot)
	r.With(auth.RequireRole("admin")).Patch("/appointment-slots/{id}", h.patchBuyerSlot)
	r.With(auth.RequireRole("admin")).Post("/appointment-slots/copy", h.copyBuyerSlots)
	r.Get("/appointments", h.listBuyerAppointments)
}

func (h *Handler) RegisterPublisher(r chi.Router) {
	r.Get("/appointments/contracts", h.listPublisherContracts)
	r.Get("/appointments/slots", h.listFreeSlots)
	r.Get("/appointments/calendar-markers", h.listCalendarMarkers)
	r.Get("/appointments/booked", h.listPublisherBookings)
	r.With(auth.RequireRole("admin", "user")).Post("/appointments/book", h.book)
	r.Get("/contracts/{id}/appointment-slots", h.listContractSlots)
	r.With(auth.RequireRole("admin")).Put("/contracts/{id}/appointment-slots", h.putContractSlots)
}

func (h *Handler) getAvailability(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	a, err := h.svc.GetBuyerAvailability(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

func (h *Handler) putAvailability(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Schedule  json.RawMessage `json:"schedule"`
		Timezone  string          `json:"timezone"`
		BufferMin int             `json:"buffer_min"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	a, err := h.svc.PutBuyerAvailability(r.Context(), p.AccountID, PutAvailabilityParams{
		Schedule: body.Schedule, Timezone: body.Timezone, BufferMin: body.BufferMin,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

func (h *Handler) listBuyerSlots(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	slots, err := h.svc.ListBuyerSlots(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": slots})
}

func (h *Handler) createBuyerSlot(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body CreateSlotParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	sl, err := h.svc.CreateBuyerSlot(r.Context(), p.AccountID, body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, sl)
}

func (h *Handler) patchBuyerSlot(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id := pathInt(r, "id")
	var body PatchSlotParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	sl, err := h.svc.PatchBuyerSlot(r.Context(), p.AccountID, int64(id), body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sl)
}

func (h *Handler) copyBuyerSlots(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		FromWeekday int   `json:"from_weekday"`
		ToWeekdays  []int `json:"to_weekdays"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	slots, err := h.svc.CopyBuyerSlots(r.Context(), p.AccountID, body.FromWeekday, body.ToWeekdays)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": slots})
}

func (h *Handler) listBuyerAppointments(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListBuyerBookings(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listPublisherContracts(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListPublisherContracts(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listFreeSlots(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	contractID, _ := strconv.ParseInt(r.URL.Query().Get("contract_id"), 10, 64)
	date := r.URL.Query().Get("date")
	if contractID == 0 {
		httpx.WriteError(w, httpx.Validation("contract_id required"))
		return
	}
	items, err := h.svc.ListFreeSlots(r.Context(), p.AccountID, contractID, date)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listCalendarMarkers(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	contractID, _ := strconv.ParseInt(r.URL.Query().Get("contract_id"), 10, 64)
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if contractID == 0 {
		httpx.WriteError(w, httpx.Validation("contract_id required"))
		return
	}
	items, err := h.svc.ListCalendarMarkers(r.Context(), p.AccountID, contractID, from, to)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listPublisherBookings(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListPublisherBookings(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listContractSlots(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id := int64(pathInt(r, "id"))
	items, err := h.svc.ListContractSlots(r.Context(), p.AccountID, id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) putContractSlots(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id := int64(pathInt(r, "id"))
	var body PutContractSlotsParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	items, err := h.svc.PutContractSlots(r.Context(), p.AccountID, id, body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) book(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		ContractID          int64  `json:"contract_id"`
		BuyerSlotID         int64  `json:"buyer_slot_id"`
		SlotStart           string `json:"slot_start"`
		DeliveryMode        string `json:"delivery_mode"`
		LeadID              int64  `json:"lead_id"`
		FirstName           string `json:"first_name"`
		LastName            string `json:"last_name"`
		Phone               string `json:"phone"`
		Email               string `json:"email"`
		Source              string `json:"source"`
		PublisherPipelineID int64  `json:"publisher_pipeline_id"`
		PublisherStageID    int64  `json:"publisher_stage_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	slotStart, err := time.Parse(time.RFC3339, body.SlotStart)
	if err != nil {
		httpx.WriteError(w, httpx.Validation("invalid slot_start"))
		return
	}
	row, err := h.svc.Book(r.Context(), p, BookParams{
		ContractID: body.ContractID, BuyerSlotID: body.BuyerSlotID, SlotStart: slotStart,
		DeliveryMode: body.DeliveryMode, LeadID: body.LeadID,
		FirstName: body.FirstName, LastName: body.LastName, Phone: body.Phone, Email: body.Email, Source: body.Source,
		PublisherPipelineID: body.PublisherPipelineID, PublisherStageID: body.PublisherStageID,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, row)
}

func pathInt(r *http.Request, key string) int {
	v, _ := strconv.Atoi(chi.URLParam(r, key))
	return v
}
