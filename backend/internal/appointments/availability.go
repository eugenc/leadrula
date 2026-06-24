package appointments

import (
	"context"
	"encoding/json"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func (s *Service) GetBuyerAvailability(ctx context.Context, buyerID int64) (*Availability, error) {
	return s.loadAvailability(ctx, buyerID)
}

type PutAvailabilityParams struct {
	Schedule  json.RawMessage
	Timezone  string
	BufferMin int
}

func (s *Service) PutBuyerAvailability(ctx context.Context, buyerID int64, p PutAvailabilityParams) (*Availability, error) {
	if p.BufferMin < 0 || p.BufferMin > maxBufferMin {
		return nil, httpx.Validation("buffer_min must be between 0 and 60")
	}
	tz := p.Timezone
	if tz == "" {
		var err error
		tz, err = s.getAccountTimezone(ctx, buyerID)
		if err != nil {
			return nil, err
		}
	}
	if tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return nil, httpx.Validation("invalid timezone")
		}
	}
	sched := p.Schedule
	if len(sched) == 0 {
		sched = json.RawMessage(`{}`)
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO buyer_availability(account_id, schedule, timezone, buffer_min, updated_at)
		 VALUES ($1, $2, $3, $4, now())
		 ON CONFLICT (account_id) DO UPDATE SET
		   schedule = EXCLUDED.schedule,
		   timezone = EXCLUDED.timezone,
		   buffer_min = EXCLUDED.buffer_min,
		   updated_at = now()`,
		buyerID, sched, tz, p.BufferMin)
	if err != nil {
		return nil, err
	}
	return s.loadAvailability(ctx, buyerID)
}

func (s *Service) ListBuyerSlots(ctx context.Context, buyerID int64) ([]BuyerSlot, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, account_id, weekday, start_time::text, duration_min, capacity, disabled_at
		 FROM buyer_appointment_slots WHERE account_id=$1 ORDER BY weekday, start_time`, buyerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BuyerSlot
	for rows.Next() {
		var sl BuyerSlot
		if err := rows.Scan(&sl.ID, &sl.AccountID, &sl.Weekday, &sl.StartTime, &sl.DurationMin, &sl.Capacity, &sl.DisabledAt); err != nil {
			return nil, err
		}
		out = append(out, sl)
	}
	return out, rows.Err()
}

type CreateSlotParams struct {
	Weekday     int
	StartTime   string
	DurationMin int
	Capacity    int
}

func (s *Service) CreateBuyerSlot(ctx context.Context, buyerID int64, p CreateSlotParams) (*BuyerSlot, error) {
	if p.DurationMin < minDurationMin || p.DurationMin > maxDurationMin {
		return nil, httpx.Validation("duration_min must be between 15 and 240")
	}
	if p.Capacity < minCapacity || p.Capacity > maxCapacity {
		return nil, httpx.Validation("capacity must be between 1 and 20")
	}
	if p.Weekday < 0 || p.Weekday > 6 {
		return nil, httpx.Validation("weekday must be 0-6")
	}
	avail, err := s.loadAvailability(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	sched := WeeklySchedule{}
	if len(avail.Schedule) > 0 {
		_ = json.Unmarshal(avail.Schedule, &sched)
	}
	loc := loadLocation(avail.Timezone)
	ref := timeNowWeekdayRef(p.Weekday, loc)
	slotStart, err := combineDateAndTime(ref, p.StartTime, loc)
	if err != nil {
		return nil, httpx.Validation(err.Error())
	}
	if !slotInsideWorkingHours(sched, time.Weekday(p.Weekday), slotStart, p.DurationMin) {
		return nil, httpx.Validation("slot must fall within working hours")
	}
	if err := s.validateSlotOverlap(ctx, buyerID, p.Weekday, slotStart, p.DurationMin, avail.BufferMin, 0); err != nil {
		return nil, err
	}
	var id int64
	err = s.pool.QueryRow(ctx,
		`INSERT INTO buyer_appointment_slots(account_id, weekday, start_time, duration_min, capacity)
		 VALUES ($1,$2,$3::time,$4,$5)
		 RETURNING id`, buyerID, p.Weekday, p.StartTime, p.DurationMin, p.Capacity).Scan(&id)
	if err != nil {
		return nil, err
	}
	if err := s.syncNewSlotToContracts(ctx, buyerID, id); err != nil {
		return nil, err
	}
	slots, err := s.ListBuyerSlots(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	for _, sl := range slots {
		if sl.ID == id {
			return &sl, nil
		}
	}
	return nil, httpx.NotFound("slot not found")
}

type PatchSlotParams struct {
	StartTime   *string
	DurationMin *int
	Capacity    *int
	Disabled    *bool
}

func (s *Service) PatchBuyerSlot(ctx context.Context, buyerID, slotID int64, p PatchSlotParams) (*BuyerSlot, error) {
	slots, err := s.ListBuyerSlots(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	var cur *BuyerSlot
	for i := range slots {
		if slots[i].ID == slotID {
			cur = &slots[i]
			break
		}
	}
	if cur == nil {
		return nil, httpx.NotFound("slot not found")
	}
	dur := cur.DurationMin
	if p.DurationMin != nil {
		dur = *p.DurationMin
	}
	cap := cur.Capacity
	if p.Capacity != nil {
		cap = *p.Capacity
	}
	start := cur.StartTime
	if p.StartTime != nil {
		start = *p.StartTime
	}
	if dur < minDurationMin || dur > maxDurationMin {
		return nil, httpx.Validation("duration_min must be between 15 and 240")
	}
	if cap < minCapacity || cap > maxCapacity {
		return nil, httpx.Validation("capacity must be between 1 and 20")
	}
	avail, err := s.loadAvailability(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	if p.StartTime != nil || p.DurationMin != nil {
		sched := WeeklySchedule{}
		_ = json.Unmarshal(avail.Schedule, &sched)
		loc := loadLocation(avail.Timezone)
		ref := timeNowWeekdayRef(cur.Weekday, loc)
		slotStart, err := combineDateAndTime(ref, start, loc)
		if err != nil {
			return nil, httpx.Validation(err.Error())
		}
		if !slotInsideWorkingHours(sched, time.Weekday(cur.Weekday), slotStart, dur) {
			return nil, httpx.Validation("slot must fall within working hours")
		}
		if err := s.validateSlotOverlap(ctx, buyerID, cur.Weekday, slotStart, dur, avail.BufferMin, slotID); err != nil {
			return nil, err
		}
	}
	if p.Disabled != nil {
		if *p.Disabled {
			_, err = s.pool.Exec(ctx,
				`UPDATE buyer_appointment_slots SET
				   start_time = COALESCE($3::time, start_time),
				   duration_min = $4, capacity = $5, disabled_at = now(), updated_at = now()
				 WHERE id=$1 AND account_id=$2`,
				slotID, buyerID, nullStrPtr(p.StartTime), dur, cap)
		} else {
			_, err = s.pool.Exec(ctx,
				`UPDATE buyer_appointment_slots SET
				   start_time = COALESCE($3::time, start_time),
				   duration_min = $4, capacity = $5, disabled_at = NULL, updated_at = now()
				 WHERE id=$1 AND account_id=$2`,
				slotID, buyerID, nullStrPtr(p.StartTime), dur, cap)
		}
	} else {
		_, err = s.pool.Exec(ctx,
			`UPDATE buyer_appointment_slots SET
			   start_time = COALESCE($3::time, start_time),
			   duration_min = $4, capacity = $5, updated_at = now()
			 WHERE id=$1 AND account_id=$2`,
			slotID, buyerID, nullStrPtr(p.StartTime), dur, cap)
	}
	if err != nil {
		return nil, err
	}
	slots, err = s.ListBuyerSlots(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	for _, sl := range slots {
		if sl.ID == slotID {
			return &sl, nil
		}
	}
	return nil, httpx.NotFound("slot not found")
}

func nullStrPtr(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}

func (s *Service) validateSlotOverlap(ctx context.Context, buyerID int64, weekday int, start time.Time, duration, buffer int, excludeID int64) error {
	slots, err := s.ListBuyerSlots(ctx, buyerID)
	if err != nil {
		return err
	}
	candidate := timeInterval{start: start, duration: duration, buffer: buffer}
	var existing []timeInterval
	avail, _ := s.loadAvailability(ctx, buyerID)
	loc := loadLocation(avail.Timezone)
	for _, sl := range slots {
		if sl.ID == excludeID || sl.DisabledAt != nil {
			continue
		}
		if sl.Weekday != weekday {
			continue
		}
		st, err := combineDateAndTime(timeNowWeekdayRef(weekday, loc), sl.StartTime, loc)
		if err != nil {
			continue
		}
		existing = append(existing, timeInterval{start: st, duration: sl.DurationMin, buffer: buffer})
	}
	if err := validateNoOverlap(existing, candidate); err != nil {
		return httpx.Validation(err.Error())
	}
	return nil
}

func timeNowWeekdayRef(weekday int, loc *time.Location) time.Time {
	now := time.Now().In(loc)
	diff := int(time.Weekday(weekday)) - int(now.Weekday())
	return now.AddDate(0, 0, diff)
}

func (s *Service) syncNewSlotToContracts(ctx context.Context, buyerID, slotID int64) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO contract_appointment_slots(contract_id, buyer_slot_id, enabled)
		 SELECT c.id, $2, true
		 FROM contracts c
		 WHERE c.buyer_id = $1 AND c.lead_type = 'Appointment' AND c.status = 'active'
		   AND c.deleted_at IS NULL
		 ON CONFLICT DO NOTHING`, buyerID, slotID)
	return err
}

func (s *Service) CopyBuyerSlots(ctx context.Context, buyerID int64, fromWeekday int, toWeekdays []int) ([]BuyerSlot, error) {
	src, err := s.ListBuyerSlots(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	for _, d := range toWeekdays {
		if d < 0 || d > 6 {
			return nil, httpx.Validation("invalid weekday")
		}
		if d == fromWeekday {
			continue
		}
		for _, sl := range src {
			if sl.Weekday != fromWeekday || sl.DisabledAt != nil {
				continue
			}
			_, err := s.CreateBuyerSlot(ctx, buyerID, CreateSlotParams{
				Weekday:     d,
				StartTime:   sl.StartTime,
				DurationMin: sl.DurationMin,
				Capacity:    sl.Capacity,
			})
			if err != nil {
				if database.IsUniqueViolation(err) {
					continue
				}
				return nil, err
			}
		}
	}
	return s.ListBuyerSlots(ctx, buyerID)
}
