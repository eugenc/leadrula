package appointments

import (
	"context"
	"encoding/json"
	"time"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

func parseWeeklySchedule(raw json.RawMessage) WeeklySchedule {
	var sched WeeklySchedule
	_ = json.Unmarshal(raw, &sched)
	return sched
}

type DayBookedSlot struct {
	SlotStart   time.Time `json:"slot_start"`
	DurationMin int       `json:"duration_min"`
}

type DayHourSlot struct {
	SlotStart   time.Time `json:"slot_start"`
	DurationMin int       `json:"duration_min"`
}

func dayBounds(date time.Time, loc *time.Location) (start, end time.Time) {
	d := date.In(loc)
	start = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	end = start.AddDate(0, 0, 1)
	return start, end
}

func (s *Service) listBookedOnBuyerCalendar(ctx context.Context, calendarID int64, date time.Time, loc *time.Location) ([]DayBookedSlot, error) {
	start, end := dayBounds(date, loc)
	rows, err := s.pool.Query(ctx,
		`SELECT b.slot_start, b.duration_min
		 FROM lead_appointment_bookings b
		 WHERE b.buyer_calendar_id = $1
		   AND b.slot_start >= $2 AND b.slot_start < $3
		 ORDER BY b.slot_start`,
		calendarID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDayBookedSlots(rows)
}

func (s *Service) listBookedOnPublisherCalendar(ctx context.Context, calendarID int64, date time.Time, loc *time.Location) ([]DayBookedSlot, error) {
	start, end := dayBounds(date, loc)
	rows, err := s.pool.Query(ctx,
		`SELECT b.slot_start, b.duration_min
		 FROM lead_appointment_bookings b
		 WHERE b.publisher_calendar_id = $1
		   AND b.slot_start >= $2 AND b.slot_start < $3
		 ORDER BY b.slot_start`,
		calendarID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDayBookedSlots(rows)
}

func (s *Service) listBookedOnContract(ctx context.Context, contractID int64, date time.Time, loc *time.Location) ([]DayBookedSlot, error) {
	start, end := dayBounds(date, loc)
	rows, err := s.pool.Query(ctx,
		`SELECT b.slot_start, b.duration_min
		 FROM lead_appointment_bookings b
		 WHERE b.contract_id = $1
		   AND b.slot_start >= $2 AND b.slot_start < $3
		 ORDER BY b.slot_start`,
		contractID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDayBookedSlots(rows)
}

func scanDayBookedSlots(rows pgx.Rows) ([]DayBookedSlot, error) {
	var out []DayBookedSlot
	for rows.Next() {
		var r DayBookedSlot
		if err := rows.Scan(&r.SlotStart, &r.DurationMin); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) listHoursFromBuyerSlots(slots []ContractSlot, date time.Time, loc *time.Location) []DayHourSlot {
	weekday := int(date.Weekday())
	var out []DayHourSlot
	for _, cs := range slots {
		if !cs.Enabled || cs.Disabled || cs.Weekday != weekday {
			continue
		}
		slot := BuyerSlot{DurationMin: cs.DurationMin, Capacity: cs.Capacity}
		dur := effectiveDuration(slot, &cs)
		slotStart, err := combineDateAndTime(date, cs.StartTime, loc)
		if err != nil {
			continue
		}
		out = append(out, DayHourSlot{SlotStart: slotStart, DurationMin: dur})
	}
	return out
}

func (s *Service) listHoursFromPublisherSlots(slots []ContractPublisherSlot, date time.Time, loc *time.Location) []DayHourSlot {
	weekday := int(date.Weekday())
	var out []DayHourSlot
	for _, cs := range slots {
		if !cs.Enabled || cs.Disabled || cs.Weekday != weekday {
			continue
		}
		slot := PublisherSlot{DurationMin: cs.DurationMin, Capacity: cs.Capacity}
		dur := effectivePublisherDuration(slot, &cs)
		slotStart, err := combineDateAndTime(date, cs.StartTime, loc)
		if err != nil {
			continue
		}
		out = append(out, DayHourSlot{SlotStart: slotStart, DurationMin: dur})
	}
	return out
}

func (s *Service) dayInfoBuyerCalendar(ctx context.Context, buyerID, calendarID int64, dateStr string) (booked []DayBookedSlot, hours []DayHourSlot, workingHours *DayWorkingHours, err error) {
	cal, err := s.loadCalendar(ctx, buyerID, calendarID)
	if err != nil {
		return nil, nil, nil, err
	}
	loc := loadLocation(cal.Timezone)
	date, err := parseDateParam(dateStr, loc)
	if err != nil {
		return nil, nil, nil, err
	}
	slots, err := s.listBuyerCalendarSlotsDirect(ctx, buyerID, calendarID)
	if err != nil {
		return nil, nil, nil, err
	}
	booked, err = s.listBookedOnBuyerCalendar(ctx, calendarID, date, loc)
	if err != nil {
		return nil, nil, nil, err
	}
	wh := workingHoursForDate(parseWeeklySchedule(cal.Schedule), date)
	return booked, s.listHoursFromBuyerSlots(slots, date, loc), wh, nil
}

func (s *Service) dayInfoPublisherCalendar(ctx context.Context, publisherID, calendarID int64, dateStr string) (booked []DayBookedSlot, hours []DayHourSlot, workingHours *DayWorkingHours, err error) {
	cal, err := s.loadPublisherCalendar(ctx, publisherID, calendarID)
	if err != nil {
		return nil, nil, nil, err
	}
	loc := loadLocation(cal.Timezone)
	date, err := parseDateParam(dateStr, loc)
	if err != nil {
		return nil, nil, nil, err
	}
	slots, err := s.listPublisherCalendarSlotsDirect(ctx, publisherID, calendarID)
	if err != nil {
		return nil, nil, nil, err
	}
	booked, err = s.listBookedOnPublisherCalendar(ctx, calendarID, date, loc)
	if err != nil {
		return nil, nil, nil, err
	}
	wh := workingHoursForDate(parseWeeklySchedule(cal.Schedule), date)
	return booked, s.listHoursFromPublisherSlots(slots, date, loc), wh, nil
}

func (s *Service) dayInfoContract(ctx context.Context, accountID, contractID int64, dateStr string, asBuyer bool, target string) (booked []DayBookedSlot, hours []DayHourSlot, workingHours *DayWorkingHours, err error) {
	if asBuyer {
		if err := s.contractOwnedByBuyer(ctx, accountID, contractID); err != nil {
			return nil, nil, nil, err
		}
	} else if _, err = s.contractBuyerID(ctx, accountID, contractID); err != nil {
		return nil, nil, nil, err
	}
	active, err := s.resolveBookingCalendar(ctx, contractID, asBuyer, target)
	if err != nil {
		return nil, nil, nil, err
	}
	switch active.Source {
	case calendarSourceBuyer:
		cal, err := s.loadCalendarByID(ctx, active.CalendarID)
		if err != nil {
			return nil, nil, nil, err
		}
		loc := loadLocation(cal.Timezone)
		date, err := parseDateParam(dateStr, loc)
		if err != nil {
			return nil, nil, nil, err
		}
		var contractSlots []ContractSlot
		switch target {
		case bookingTargetActive:
			contractSlots, err = s.listActiveContractBuyerSlots(ctx, contractID)
		case bookingTargetCross:
			if asBuyer {
				return nil, nil, nil, httpx.Validation("buyer cross-booking uses publisher calendar")
			}
			contractSlots, err = s.ListContractSlots(ctx, accountID, contractID)
		default:
			contractSlots, err = s.listOwnBuyerCalendarSlots(ctx, accountID, contractID, asBuyer)
		}
		if err != nil {
			return nil, nil, nil, err
		}
		booked, err = s.listBookedOnContract(ctx, contractID, date, loc)
		if err != nil {
			return nil, nil, nil, err
		}
		wh := workingHoursForDate(parseWeeklySchedule(cal.Schedule), date)
		return booked, s.listHoursFromBuyerSlots(contractSlots, date, loc), wh, nil
	case calendarSourcePublisher:
		cal, err := s.loadPublisherCalendarByID(ctx, active.CalendarID)
		if err != nil {
			return nil, nil, nil, err
		}
		loc := loadLocation(cal.Timezone)
		date, err := parseDateParam(dateStr, loc)
		if err != nil {
			return nil, nil, nil, err
		}
		var contractSlots []ContractPublisherSlot
		switch target {
		case bookingTargetActive:
			contractSlots, err = s.listActiveContractPublisherSlots(ctx, contractID)
		case bookingTargetCross:
			if asBuyer {
				contractSlots, err = s.ListContractPublisherSlotsForBuyer(ctx, accountID, contractID)
			} else {
				return nil, nil, nil, httpx.Validation("publisher cross-booking uses buyer calendar")
			}
		default:
			contractSlots, err = s.listOwnPublisherCalendarSlots(ctx, accountID, contractID, asBuyer)
		}
		if err != nil {
			return nil, nil, nil, err
		}
		booked, err = s.listBookedOnContract(ctx, contractID, date, loc)
		if err != nil {
			return nil, nil, nil, err
		}
		wh := workingHoursForDate(parseWeeklySchedule(cal.Schedule), date)
		return booked, s.listHoursFromPublisherSlots(contractSlots, date, loc), wh, nil
	default:
		return nil, nil, nil, httpx.Validation("appointment calendar is not configured")
	}
}

func emptyDayBooked() []DayBookedSlot { return []DayBookedSlot{} }
func emptyDayHours() []DayHourSlot    { return []DayHourSlot{} }
