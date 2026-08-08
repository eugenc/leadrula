package appointments

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/permissions"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterBuyer(r chi.Router) {
	r.Get("/booking-calendars", h.listBookingCalendars)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Post("/booking-calendars", h.createBookingCalendar)
	r.Get("/booking-calendars/{calendarId}", h.getBookingCalendar)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Put("/booking-calendars/{calendarId}", h.putBookingCalendar)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Delete("/booking-calendars/{calendarId}", h.deleteBookingCalendar)
	r.Get("/booking-calendars/{calendarId}/slots", h.listCalendarSlots)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Post("/booking-calendars/{calendarId}/slots", h.createCalendarSlot)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Patch("/booking-calendars/{calendarId}/slots/{slotId}", h.patchCalendarSlot)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Post("/booking-calendars/{calendarId}/slots/copy", h.copyCalendarSlots)
	r.Get("/booking-calendars/{calendarId}/markers", h.listBuyerCalendarMarkers)
	r.Get("/booking-calendars/{calendarId}/appointments", h.listBuyerCalendarAppointments)

	r.Get("/appointments", h.listBuyerAppointments)
	r.Get("/appointments/contracts", h.listBuyerAppointmentContracts)
	r.Get("/appointments/slots", h.listBuyerFreeSlots)
	r.Get("/appointments/calendar-markers", h.listBuyerAppointmentCalendarMarkers)
	r.With(auth.RequireRole("admin", "user")).Post("/appointments/book", h.bookAsBuyer)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Patch("/contracts/{id}/appointment-calendar", h.setContractAppointmentCalendar)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Patch("/contracts/{id}/appointment-calendar-source", h.setContractAppointmentCalendarSource)
	r.Get("/contracts/{id}/publisher-appointment-slots", h.listContractPublisherSlotsForBuyer)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Put("/contracts/{id}/publisher-appointment-slots", h.putContractPublisherSlotsForBuyer)
}

func (h *Handler) RegisterPublisher(r chi.Router) {
	r.Get("/booking-calendars", h.listPublisherBookingCalendars)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Post("/booking-calendars", h.createPublisherBookingCalendar)
	r.Get("/booking-calendars/{calendarId}", h.getPublisherBookingCalendar)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Put("/booking-calendars/{calendarId}", h.putPublisherBookingCalendar)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Delete("/booking-calendars/{calendarId}", h.deletePublisherBookingCalendar)
	r.Get("/booking-calendars/{calendarId}/slots", h.listPublisherCalendarSlots)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Post("/booking-calendars/{calendarId}/slots", h.createPublisherCalendarSlot)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Patch("/booking-calendars/{calendarId}/slots/{slotId}", h.patchPublisherCalendarSlot)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Post("/booking-calendars/{calendarId}/slots/copy", h.copyPublisherCalendarSlots)
	r.Get("/booking-calendars/{calendarId}/markers", h.listPublisherCalendarMarkers)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Patch("/contracts/{id}/appointment-calendar", h.setContractPublisherAppointmentCalendar)

	r.Get("/appointments/contracts", h.listPublisherContracts)
	r.Get("/appointments/slots", h.listFreeSlots)
	r.Get("/appointments/calendar-markers", h.listCalendarMarkers)
	r.Get("/appointments/booked", h.listPublisherBookings)
	r.With(auth.RequireRole("admin", "user")).Post("/appointments/book", h.book)
	r.Get("/contracts/{id}/appointment-slots", h.listContractSlots)
	r.With(auth.RequirePermission(permissions.ActionAppointmentsManage)).Put("/contracts/{id}/appointment-slots", h.putContractSlots)
}

func (h *Handler) listBookingCalendars(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListBookingCalendars(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createBookingCalendar(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name     string `json:"name"`
		Timezone string `json:"timezone"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	cal, err := h.svc.CreateBookingCalendar(r.Context(), p.AccountID, CreateCalendarParams{
		Name: body.Name, Timezone: body.Timezone,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, cal)
}

func (h *Handler) getBookingCalendar(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	cal, err := h.svc.GetBookingCalendar(r.Context(), p.AccountID, int64(pathInt(r, "calendarId")))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cal)
}

func (h *Handler) putBookingCalendar(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name      string          `json:"name"`
		Schedule  json.RawMessage `json:"schedule"`
		Timezone  string          `json:"timezone"`
		BufferMin int             `json:"buffer_min"`
		Location  string          `json:"location"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	cal, err := h.svc.PutBookingCalendar(r.Context(), p.AccountID, int64(pathInt(r, "calendarId")), PutCalendarParams{
		Name: body.Name, Schedule: body.Schedule, Timezone: body.Timezone, BufferMin: body.BufferMin, Location: body.Location,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cal)
}

func (h *Handler) deleteBookingCalendar(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteBookingCalendar(r.Context(), p.AccountID, int64(pathInt(r, "calendarId"))); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listCalendarSlots(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	slots, err := h.svc.ListCalendarSlots(r.Context(), p.AccountID, int64(pathInt(r, "calendarId")))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": slots})
}

func (h *Handler) createCalendarSlot(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body CreateSlotParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	sl, err := h.svc.CreateCalendarSlot(r.Context(), p.AccountID, int64(pathInt(r, "calendarId")), body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, sl)
}

func (h *Handler) patchCalendarSlot(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body PatchSlotParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	sl, err := h.svc.PatchCalendarSlot(r.Context(), p.AccountID, int64(pathInt(r, "calendarId")), int64(pathInt(r, "slotId")), body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sl)
}

func (h *Handler) copyCalendarSlots(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		FromWeekday int   `json:"from_weekday"`
		ToWeekdays  []int `json:"to_weekdays"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	slots, err := h.svc.CopyCalendarSlots(r.Context(), p.AccountID, int64(pathInt(r, "calendarId")), body.FromWeekday, body.ToWeekdays)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": slots})
}

func (h *Handler) listBuyerCalendarMarkers(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	items, err := h.svc.ListBuyerCalendarMarkers(r.Context(), p.AccountID, int64(pathInt(r, "calendarId")), from, to)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listBuyerCalendarAppointments(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListBuyerBookingsForCalendar(r.Context(), p.AccountID, int64(pathInt(r, "calendarId")))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) setContractAppointmentCalendar(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		AppointmentCalendarID int64 `json:"appointment_calendar_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.AppointmentCalendarID == 0 {
		httpx.WriteError(w, httpx.Validation("appointment_calendar_id is required"))
		return
	}
	if err := h.svc.SetContractAppointmentCalendar(r.Context(), p.AccountID, int64(pathInt(r, "id")), body.AppointmentCalendarID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setContractAppointmentCalendarSource(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Source                string `json:"source"`
		AppointmentCalendarID int64  `json:"appointment_calendar_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.Source == "" {
		httpx.WriteError(w, httpx.Validation("source is required"))
		return
	}
	if err := h.svc.SetContractAppointmentCalendarSource(r.Context(), p.AccountID, int64(pathInt(r, "id")), SetContractCalendarSourceParams{
		Source: body.Source, AppointmentCalendarID: body.AppointmentCalendarID,
	}); err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listContractPublisherSlotsForBuyer(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListContractPublisherSlotsForBuyer(r.Context(), p.AccountID, int64(pathInt(r, "id")))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) putContractPublisherSlotsForBuyer(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body PutContractPublisherSlotsParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	items, err := h.svc.PutContractPublisherSlotsForBuyer(r.Context(), p.AccountID, int64(pathInt(r, "id")), body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listPublisherBookingCalendars(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListPublisherBookingCalendars(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createPublisherBookingCalendar(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name     string `json:"name"`
		Timezone string `json:"timezone"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	cal, err := h.svc.CreatePublisherBookingCalendar(r.Context(), p.AccountID, CreateCalendarParams{
		Name: body.Name, Timezone: body.Timezone,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, cal)
}

func (h *Handler) getPublisherBookingCalendar(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	cal, err := h.svc.GetPublisherBookingCalendar(r.Context(), p.AccountID, int64(pathInt(r, "calendarId")))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cal)
}

func (h *Handler) putPublisherBookingCalendar(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Name      string          `json:"name"`
		Schedule  json.RawMessage `json:"schedule"`
		Timezone  string          `json:"timezone"`
		BufferMin int             `json:"buffer_min"`
		Location  string          `json:"location"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	cal, err := h.svc.PutPublisherBookingCalendar(r.Context(), p.AccountID, int64(pathInt(r, "calendarId")), PutCalendarParams{
		Name: body.Name, Schedule: body.Schedule, Timezone: body.Timezone, BufferMin: body.BufferMin, Location: body.Location,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cal)
}

func (h *Handler) deletePublisherBookingCalendar(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeletePublisherBookingCalendar(r.Context(), p.AccountID, int64(pathInt(r, "calendarId"))); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listPublisherCalendarSlots(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	slots, err := h.svc.ListPublisherCalendarSlots(r.Context(), p.AccountID, int64(pathInt(r, "calendarId")))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": slots})
}

func (h *Handler) createPublisherCalendarSlot(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body CreateSlotParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	sl, err := h.svc.CreatePublisherCalendarSlot(r.Context(), p.AccountID, int64(pathInt(r, "calendarId")), body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, sl)
}

func (h *Handler) patchPublisherCalendarSlot(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body PatchSlotParams
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	sl, err := h.svc.PatchPublisherCalendarSlot(r.Context(), p.AccountID, int64(pathInt(r, "calendarId")), int64(pathInt(r, "slotId")), body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sl)
}

func (h *Handler) copyPublisherCalendarSlots(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		FromWeekday int   `json:"from_weekday"`
		ToWeekdays  []int `json:"to_weekdays"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	slots, err := h.svc.CopyPublisherCalendarSlots(r.Context(), p.AccountID, int64(pathInt(r, "calendarId")), body.FromWeekday, body.ToWeekdays)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": slots})
}

func (h *Handler) listPublisherCalendarMarkers(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	items, err := h.svc.ListPublisherCalendarMarkers(r.Context(), p.AccountID, int64(pathInt(r, "calendarId")), from, to)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) setContractPublisherAppointmentCalendar(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		PublisherAppointmentCalendarID int64 `json:"publisher_appointment_calendar_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.PublisherAppointmentCalendarID == 0 {
		httpx.WriteError(w, httpx.Validation("publisher_appointment_calendar_id is required"))
		return
	}
	if err := h.svc.SetContractPublisherAppointmentCalendar(r.Context(), p.AccountID, int64(pathInt(r, "id")), body.PublisherAppointmentCalendarID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listBuyerAppointments(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	contractID, _ := strconv.ParseInt(q.Get("contract_id"), 10, 64)
	publisherID, _ := strconv.ParseInt(q.Get("publisher_id"), 10, 64)

	result, err := h.svc.ListBuyerBookings(r.Context(), BuyerListParams{
		BuyerID:           p.AccountID,
		Page:              page,
		Limit:             limit,
		Sort:              q.Get("sort"),
		SortDir:           q.Get("sort_dir"),
		Q:                 q.Get("q"),
		ContractID:        contractID,
		PublisherID:       publisherID,
		AppointmentPreset: q.Get("appointment_preset"),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) listBuyerAppointmentContracts(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListBuyerAppointmentContracts(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []BuyerAppointmentContract{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listBuyerFreeSlots(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
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
	var booked []DayBookedSlot
	var hours []DayHourSlot
	var workingHours *DayWorkingHours
	if calendarID != 0 {
		items, err = h.svc.ListFreeSlotsByBuyerCalendar(r.Context(), p.AccountID, calendarID, date)
		if err == nil {
			booked, hours, workingHours, err = h.svc.dayInfoBuyerCalendar(r.Context(), p.AccountID, calendarID, date)
		}
	} else {
		target := parseBookingTarget(r.URL.Query().Get("booking_target"))
		items, err = h.svc.ListFreeSlotsForBuyer(r.Context(), p.AccountID, contractID, date, target)
		if err == nil {
			booked, hours, workingHours, err = h.svc.dayInfoContract(r.Context(), p.AccountID, contractID, date, true, target)
		}
	}
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []FreeSlot{}
	}
	if booked == nil {
		booked = emptyDayBooked()
	}
	if hours == nil {
		hours = emptyDayHours()
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "booked": booked, "hours": hours, "working_hours": workingHours})
}

func (h *Handler) listBuyerAppointmentCalendarMarkers(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
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
	if calendarID != 0 {
		items, err = h.svc.ListBuyerCalendarMarkers(r.Context(), p.AccountID, calendarID, from, to)
	} else {
		target := parseBookingTarget(r.URL.Query().Get("booking_target"))
		items, err = h.svc.ListCalendarMarkersForBuyer(r.Context(), p.AccountID, contractID, from, to, target)
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

func (h *Handler) bookAsBuyer(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		ContractID      int64  `json:"contract_id"`
		CalendarID      int64  `json:"calendar_id"`
		BuyerSlotID     int64  `json:"buyer_slot_id"`
		PublisherSlotID int64  `json:"publisher_slot_id"`
		SlotStart       string `json:"slot_start"`
		DurationMin     int    `json:"duration_min"`
		CustomTime      bool   `json:"custom_time"`
		DeliveryMode    string `json:"delivery_mode"`
		LeadID          int64  `json:"lead_id"`
		FirstName       string `json:"first_name"`
		LastName        string `json:"last_name"`
		Phone           string `json:"phone"`
		Email           string `json:"email"`
		Source          string `json:"source"`
		BookingTarget   string `json:"booking_target"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	slotStart, err := time.Parse(time.RFC3339, body.SlotStart)
	if err != nil {
		httpx.WriteError(w, httpx.Validation("invalid slot_start"))
		return
	}
	row, err := h.svc.BookAsBuyer(r.Context(), p, BookParams{
		ContractID: body.ContractID, CalendarID: body.CalendarID, BuyerSlotID: body.BuyerSlotID, PublisherSlotID: body.PublisherSlotID, SlotStart: slotStart,
		DurationMin: body.DurationMin, CustomTime: body.CustomTime,
		DeliveryMode: body.DeliveryMode, LeadID: body.LeadID,
		FirstName: body.FirstName, LastName: body.LastName, Phone: body.Phone, Email: body.Email, Source: body.Source,
		BookingTarget: parseBookingTarget(body.BookingTarget),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, row)
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
	var booked []DayBookedSlot
	var hours []DayHourSlot
	var workingHours *DayWorkingHours
	if calendarID != 0 {
		items, err = h.svc.ListFreeSlotsByPublisherCalendar(r.Context(), p.AccountID, calendarID, date)
		if err == nil {
			booked, hours, workingHours, err = h.svc.dayInfoPublisherCalendar(r.Context(), p.AccountID, calendarID, date)
		}
	} else {
		target := parseBookingTarget(r.URL.Query().Get("booking_target"))
		items, err = h.svc.ListFreeSlots(r.Context(), p.AccountID, contractID, date, target)
		if err == nil {
			booked, hours, workingHours, err = h.svc.dayInfoContract(r.Context(), p.AccountID, contractID, date, false, target)
		}
	}
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []FreeSlot{}
	}
	if booked == nil {
		booked = emptyDayBooked()
	}
	if hours == nil {
		hours = emptyDayHours()
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "booked": booked, "hours": hours, "working_hours": workingHours})
}

func (h *Handler) listCalendarMarkers(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
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
	if calendarID != 0 {
		items, err = h.svc.ListPublisherCalendarMarkers(r.Context(), p.AccountID, calendarID, from, to)
	} else {
		target := parseBookingTarget(r.URL.Query().Get("booking_target"))
		items, err = h.svc.ListCalendarMarkers(r.Context(), p.AccountID, contractID, from, to, target)
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
		BookingTarget       string `json:"booking_target"`
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
		ContractID: body.ContractID, CalendarID: body.CalendarID, BuyerSlotID: body.BuyerSlotID, PublisherSlotID: body.PublisherSlotID, SlotStart: slotStart,
		DurationMin: body.DurationMin, CustomTime: body.CustomTime,
		DeliveryMode: body.DeliveryMode, LeadID: body.LeadID,
		FirstName: body.FirstName, LastName: body.LastName, Phone: body.Phone, Email: body.Email, Source: body.Source,
		PublisherPipelineID: body.PublisherPipelineID, PublisherStageID: body.PublisherStageID,
		BookingTarget: parseBookingTarget(body.BookingTarget),
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
