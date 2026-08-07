package appointments

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/echayko/leadrula/backend/internal/apikeys"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

// RegisterPublicRoutes mounts API-key-authenticated appointment endpoints under /api/v1.
func (h *Handler) RegisterPublicRoutes(r chi.Router, apikeysSvc *apikeys.Service) {
	r.With(apikeysSvc.RequireAppointmentsRead).Get("/api/v1/appointments/contracts", h.publicListContracts)
	r.With(apikeysSvc.RequireAppointmentsRead).Get("/api/v1/appointments/slots", h.publicListSlots)
	r.With(apikeysSvc.RequireAppointmentsRead).Get("/api/v1/appointments/calendar-markers", h.publicListCalendarMarkers)
	r.With(apikeysSvc.RequireAppointmentsWrite).Post("/api/v1/appointments/book", h.publicBook)
	r.With(apikeysSvc.RequireAppointmentsRead).Get("/api/v1/appointments/booked", h.publicListBooked)
	r.With(apikeysSvc.RequireAppointmentsRead).Get("/api/v1/booking-calendars", h.publicListBookingCalendars)
	r.With(apikeysSvc.RequireAppointmentsRead).Get("/api/v1/booking-calendars/{id}", h.publicGetBookingCalendar)
	r.With(apikeysSvc.RequireAppointmentsRead).Get("/api/v1/booking-calendars/{id}/slots", h.publicListBookingCalendarSlots)
}

func (h *Handler) publicListContracts(w http.ResponseWriter, r *http.Request) {
	acct := auth.APIKeyAccountFromContext(r.Context())
	if acct == nil {
		httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	switch acct.AccountType {
	case "publisher":
		items, err := h.svc.ListPublisherContracts(r.Context(), acct.AccountID)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
	case "buyer":
		items, err := h.svc.ListBuyerAppointmentContracts(r.Context(), acct.AccountID)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		if items == nil {
			items = []BuyerAppointmentContract{}
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
	default:
		httpx.Err(w, http.StatusForbidden, httpx.CodeForbidden, "publisher or buyer API key required")
	}
}

func (h *Handler) publicListSlots(w http.ResponseWriter, r *http.Request) {
	acct := auth.APIKeyAccountFromContext(r.Context())
	if acct == nil {
		httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	contractID, _ := strconv.ParseInt(r.URL.Query().Get("contract_id"), 10, 64)
	calendarID, _ := strconv.ParseInt(r.URL.Query().Get("calendar_id"), 10, 64)
	date := r.URL.Query().Get("date")
	if contractID == 0 && calendarID == 0 {
		httpx.WriteError(w, httpx.Validation("contract_id or calendar_id required"))
		return
	}
	if contractID != 0 && calendarID != 0 {
		httpx.WriteError(w, httpx.Validation("provide contract_id or calendar_id, not both"))
		return
	}
	var (
		items []FreeSlot
		err   error
	)
	switch acct.AccountType {
	case "publisher":
		if calendarID != 0 {
			items, err = h.svc.ListFreeSlotsByPublisherCalendar(r.Context(), acct.AccountID, calendarID, date)
		} else {
			items, err = h.svc.ListFreeSlots(r.Context(), acct.AccountID, contractID, date, bookingTargetActive)
		}
	case "buyer":
		if calendarID != 0 {
			items, err = h.svc.ListFreeSlotsByBuyerCalendar(r.Context(), acct.AccountID, calendarID, date)
		} else {
			items, err = h.svc.ListFreeSlotsForBuyer(r.Context(), acct.AccountID, contractID, date, bookingTargetActive)
		}
	default:
		httpx.Err(w, http.StatusForbidden, httpx.CodeForbidden, "publisher or buyer API key required")
		return
	}
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []FreeSlot{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) publicListCalendarMarkers(w http.ResponseWriter, r *http.Request) {
	acct := auth.APIKeyAccountFromContext(r.Context())
	if acct == nil {
		httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	contractID, _ := strconv.ParseInt(r.URL.Query().Get("contract_id"), 10, 64)
	calendarID, _ := strconv.ParseInt(r.URL.Query().Get("calendar_id"), 10, 64)
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if contractID == 0 && calendarID == 0 {
		httpx.WriteError(w, httpx.Validation("contract_id or calendar_id required"))
		return
	}
	if contractID != 0 && calendarID != 0 {
		httpx.WriteError(w, httpx.Validation("provide contract_id or calendar_id, not both"))
		return
	}
	var (
		items []CalendarDayMarker
		err   error
	)
	switch acct.AccountType {
	case "publisher":
		if calendarID != 0 {
			items, err = h.svc.ListPublisherCalendarMarkers(r.Context(), acct.AccountID, calendarID, from, to)
		} else {
			items, err = h.svc.ListCalendarMarkers(r.Context(), acct.AccountID, contractID, from, to, bookingTargetActive)
		}
	case "buyer":
		if calendarID != 0 {
			items, err = h.svc.ListBuyerCalendarMarkers(r.Context(), acct.AccountID, calendarID, from, to)
		} else {
			items, err = h.svc.ListCalendarMarkersForBuyer(r.Context(), acct.AccountID, contractID, from, to, bookingTargetActive)
		}
	default:
		httpx.Err(w, http.StatusForbidden, httpx.CodeForbidden, "publisher or buyer API key required")
		return
	}
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []CalendarDayMarker{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) publicBook(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.principalFromAPIKey(r.Context(), auth.APIKeyAccountFromContext(r.Context()))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var body struct {
		ContractID          int64  `json:"contract_id"`
		CalendarID          int64  `json:"calendar_id"`
		BuyerSlotID         int64  `json:"buyer_slot_id"`
		PublisherSlotID     int64  `json:"publisher_slot_id"`
		SlotStart           string `json:"slot_start"`
		DurationMin         int    `json:"duration_min"`
		CustomTime          bool   `json:"custom_time"`
		DeliveryMode        string `json:"delivery_mode"`
		LeadID              int64  `json:"lead_id"`
		FirstName           string `json:"first_name"`
		LastName            string `json:"last_name"`
		Phone               string `json:"phone"`
		Email               string `json:"email"`
		Source              string `json:"source"`
		PublisherPipelineID int64  `json:"publisher_pipeline_id"`
		PublisherStageID    int64  `json:"publisher_stage_id"`
		ExternalEventID     string `json:"external_event_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	slotStart, err := time.Parse(time.RFC3339, body.SlotStart)
	if err != nil {
		httpx.WriteError(w, httpx.Validation("invalid slot_start"))
		return
	}
	params := BookParams{
		ContractID: body.ContractID, CalendarID: body.CalendarID, BuyerSlotID: body.BuyerSlotID, PublisherSlotID: body.PublisherSlotID,
		SlotStart: slotStart, DurationMin: body.DurationMin, CustomTime: body.CustomTime,
		DeliveryMode: body.DeliveryMode, LeadID: body.LeadID,
		FirstName: body.FirstName, LastName: body.LastName, Phone: body.Phone, Email: body.Email, Source: body.Source,
		PublisherPipelineID: body.PublisherPipelineID, PublisherStageID: body.PublisherStageID,
		ExternalEventID: body.ExternalEventID, ExternalProvider: "voiceuni",
		BookingTarget: bookingTargetActive,
	}
	var row *BookingRow
	if p.AccountType == "buyer" {
		row, err = h.svc.BookAsBuyer(r.Context(), p, params)
	} else {
		row, err = h.svc.Book(r.Context(), p, params)
	}
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, row)
}

func (h *Handler) publicListBooked(w http.ResponseWriter, r *http.Request) {
	acct := auth.APIKeyAccountFromContext(r.Context())
	if acct == nil {
		httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	switch acct.AccountType {
	case "publisher":
		items, err := h.svc.ListPublisherBookingsFiltered(r.Context(), acct.AccountID, parsePublicBookedParams(r))
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
	case "buyer":
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		limit, _ := strconv.Atoi(q.Get("limit"))
		contractID, _ := strconv.ParseInt(q.Get("contract_id"), 10, 64)
		publisherID, _ := strconv.ParseInt(q.Get("publisher_id"), 10, 64)
		from, to, dateErr := parseBookedDateRange(q.Get("from"), q.Get("to"))
		if dateErr != nil {
			httpx.WriteError(w, httpx.Validation("invalid from or to date"))
			return
		}
		preset := q.Get("appointment_preset")
		if from != nil || to != nil {
			preset = "all"
		}
		result, err := h.svc.ListBuyerBookings(r.Context(), BuyerListParams{
			BuyerID:           acct.AccountID,
			Page:              page,
			Limit:             limit,
			Sort:              q.Get("sort"),
			SortDir:           q.Get("sort_dir"),
			Q:                 q.Get("q"),
			ContractID:        contractID,
			PublisherID:       publisherID,
			AppointmentPreset: preset,
			From:              from,
			To:                to,
		})
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, result)
	default:
		httpx.Err(w, http.StatusForbidden, httpx.CodeForbidden, "publisher or buyer API key required")
	}
}

func (h *Handler) publicListBookingCalendars(w http.ResponseWriter, r *http.Request) {
	acct := auth.APIKeyAccountFromContext(r.Context())
	if acct == nil {
		httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	switch acct.AccountType {
	case "publisher":
		items, err := h.svc.ListPublisherBookingCalendars(r.Context(), acct.AccountID)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		for i := range items {
			_ = h.svc.enrichBookingCalendarContracts(r.Context(), &items[i], acct.AccountID, calendarSourcePublisher)
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
	case "buyer":
		items, err := h.svc.ListBookingCalendars(r.Context(), acct.AccountID)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		for i := range items {
			_ = h.svc.enrichBookingCalendarContracts(r.Context(), &items[i], acct.AccountID, calendarSourceBuyer)
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
	default:
		httpx.Err(w, http.StatusForbidden, httpx.CodeForbidden, "publisher or buyer API key required")
	}
}

func (h *Handler) publicGetBookingCalendar(w http.ResponseWriter, r *http.Request) {
	acct := auth.APIKeyAccountFromContext(r.Context())
	if acct == nil {
		httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	calendarID := int64(pathInt(r, "id"))
	switch acct.AccountType {
	case "publisher":
		cal, err := h.svc.GetPublisherBookingCalendar(r.Context(), acct.AccountID, calendarID)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		_ = h.svc.enrichBookingCalendarContracts(r.Context(), cal, acct.AccountID, calendarSourcePublisher)
		httpx.JSON(w, http.StatusOK, cal)
	case "buyer":
		cal, err := h.svc.GetBookingCalendar(r.Context(), acct.AccountID, calendarID)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		_ = h.svc.enrichBookingCalendarContracts(r.Context(), cal, acct.AccountID, calendarSourceBuyer)
		httpx.JSON(w, http.StatusOK, cal)
	default:
		httpx.Err(w, http.StatusForbidden, httpx.CodeForbidden, "publisher or buyer API key required")
	}
}

func (h *Handler) publicListBookingCalendarSlots(w http.ResponseWriter, r *http.Request) {
	acct := auth.APIKeyAccountFromContext(r.Context())
	if acct == nil {
		httpx.Err(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	calendarID := int64(pathInt(r, "id"))
	switch acct.AccountType {
	case "publisher":
		items, err := h.svc.ListPublisherCalendarSlots(r.Context(), acct.AccountID, calendarID)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
	case "buyer":
		items, err := h.svc.ListCalendarSlots(r.Context(), acct.AccountID, calendarID)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
	default:
		httpx.Err(w, http.StatusForbidden, httpx.CodeForbidden, "publisher or buyer API key required")
	}
}

func (s *Service) principalFromAPIKey(ctx context.Context, acct *auth.APIKeyAccount) (*auth.Principal, error) {
	if acct == nil {
		return nil, httpx.NewError(httpx.CodeUnauthorized, "unauthorized")
	}
	if acct.AccountType != "publisher" && acct.AccountType != "buyer" {
		return nil, httpx.NewError(httpx.CodeForbidden, "publisher or buyer API key required")
	}
	if s.accounts == nil {
		return nil, httpx.NewError(httpx.CodeInternal, "accounts service unavailable")
	}
	adminIDs, err := s.accounts.AdminUserIDs(ctx, s.pool, acct.AccountID)
	if err != nil {
		return nil, err
	}
	if len(adminIDs) == 0 {
		return nil, httpx.Validation("account has no admin users")
	}
	return &auth.Principal{
		AccountID:   acct.AccountID,
		AccountType: acct.AccountType,
		Role:        "admin",
		UserID:      adminIDs[0],
	}, nil
}

func parsePublicBookedParams(r *http.Request) BookedListParams {
	q := r.URL.Query()
	from, to, _ := parseBookedDateRange(q.Get("from"), q.Get("to"))
	contractID, _ := strconv.ParseInt(q.Get("contract_id"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	return BookedListParams{
		From:       from,
		To:         to,
		ContractID: contractID,
		Limit:      limit,
	}
}
