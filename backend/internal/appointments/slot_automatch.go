package appointments

import (
	"context"
	"time"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

// ResolveSlotFromStart finds the contract slot template matching slotStart (minute precision).
func (s *Service) ResolveSlotFromStart(ctx context.Context, contractID int64, slotStart time.Time) (BookParams, error) {
	active, err := s.resolveBookingCalendar(ctx, contractID, false, bookingTargetActive)
	if err != nil {
		return BookParams{}, err
	}
	var loc *time.Location
	switch active.Source {
	case calendarSourceBuyer:
		cal, err := s.loadCalendarByID(ctx, active.CalendarID)
		if err != nil {
			return BookParams{}, err
		}
		loc = loadLocation(cal.Timezone)
	case calendarSourcePublisher:
		cal, err := s.loadPublisherCalendarByID(ctx, active.CalendarID)
		if err != nil {
			return BookParams{}, err
		}
		loc = loadLocation(cal.Timezone)
	default:
		return BookParams{}, httpx.Validation("appointment calendar is not configured")
	}
	slotStart = slotStart.In(loc)
	if !bookingWindowOK(slotStart, time.Now()) {
		return BookParams{}, httpx.Validation("slot is outside booking window")
	}

	switch active.Source {
	case calendarSourceBuyer:
		contractSlots, err := s.listActiveContractBuyerSlots(ctx, contractID)
		if err != nil {
			return BookParams{}, err
		}
		for i := range contractSlots {
			cs := &contractSlots[i]
			if !cs.Enabled || cs.Disabled {
				continue
			}
			expected, err := combineDateAndTime(slotStart, cs.StartTime, loc)
			if err != nil {
				continue
			}
			if !expected.Truncate(time.Minute).Equal(slotStart.Truncate(time.Minute)) {
				continue
			}
			return BookParams{
				ContractID:    contractID,
				BuyerSlotID:   cs.BuyerSlotID,
				SlotStart:     slotStart,
				BookingTarget: bookingTargetActive,
			}, nil
		}
	case calendarSourcePublisher:
		contractSlots, err := s.listActiveContractPublisherSlots(ctx, contractID)
		if err != nil {
			return BookParams{}, err
		}
		for i := range contractSlots {
			cs := &contractSlots[i]
			if !cs.Enabled || cs.Disabled {
				continue
			}
			expected, err := combineDateAndTime(slotStart, cs.StartTime, loc)
			if err != nil {
				continue
			}
			if !expected.Truncate(time.Minute).Equal(slotStart.Truncate(time.Minute)) {
				continue
			}
			return BookParams{
				ContractID:      contractID,
				PublisherSlotID: cs.PublisherSlotID,
				SlotStart:       slotStart,
				BookingTarget:   bookingTargetActive,
			}, nil
		}
	}
	return BookParams{}, httpx.Validation("slot_start does not match any available slot")
}

// ResolveSlotFromPublisherCalendar finds the publisher calendar slot template matching slotStart.
func (s *Service) ResolveSlotFromPublisherCalendar(ctx context.Context, publisherID, calendarID int64, slotStart time.Time) (BookParams, error) {
	cal, err := s.loadPublisherCalendar(ctx, publisherID, calendarID)
	if err != nil {
		return BookParams{}, err
	}
	ok, err := s.publisherCalendarConfigured(ctx, calendarID)
	if err != nil {
		return BookParams{}, err
	}
	if !ok {
		return BookParams{}, httpx.Validation("appointment calendar is not configured")
	}
	loc := loadLocation(cal.Timezone)
	slotStart = slotStart.In(loc)
	if !bookingWindowOK(slotStart, time.Now()) {
		return BookParams{}, httpx.Validation("slot is outside booking window")
	}

	slots, err := s.listPublisherCalendarSlotsDirect(ctx, publisherID, calendarID)
	if err != nil {
		return BookParams{}, err
	}
	for i := range slots {
		cs := &slots[i]
		if !cs.Enabled || cs.Disabled {
			continue
		}
		expected, err := combineDateAndTime(slotStart, cs.StartTime, loc)
		if err != nil {
			continue
		}
		if !expected.Truncate(time.Minute).Equal(slotStart.Truncate(time.Minute)) {
			continue
		}
		return BookParams{
			CalendarID:      calendarID,
			PublisherSlotID: cs.PublisherSlotID,
			SlotStart:       slotStart,
		}, nil
	}
	return BookParams{}, httpx.Validation("slot_start does not match any available slot")
}
