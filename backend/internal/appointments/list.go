package appointments

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type BookedListParams struct {
	From       *time.Time
	To         *time.Time
	ContractID int64
	Page       int
	Limit      int
}

func deliveryStatus(mode string, leadStatus string, ownerIsBuyer bool) string {
	if mode == "publisher_pipeline" && !ownerIsBuyer {
		return "pending delivery"
	}
	if leadStatus == "distributed" || leadStatus == "closed" {
		return "delivered"
	}
	return leadStatus
}

func (s *Service) ListPublisherBookings(ctx context.Context, publisherID int64) ([]BookingRow, error) {
	return s.ListPublisherBookingsFiltered(ctx, publisherID, BookedListParams{})
}

func (s *Service) ListPublisherBookingsFiltered(ctx context.Context, publisherID int64, p BookedListParams) ([]BookingRow, error) {
	where := `(pub.id = $1 OR pbc.account_id = $1)`
	args := []any{publisherID}
	where, args = appendBookedFilters(where, args, p)
	limit := p.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	return s.listBookingsQuery(ctx, where, args, "b.slot_start DESC", limit)
}

func appendBookedFilters(where string, args []any, p BookedListParams) (string, []any) {
	if p.ContractID > 0 {
		args = append(args, p.ContractID)
		where += fmt.Sprintf(" AND b.contract_id = $%d", len(args))
	}
	if p.From != nil {
		args = append(args, *p.From)
		where += fmt.Sprintf(" AND b.slot_start >= $%d", len(args))
	}
	if p.To != nil {
		args = append(args, *p.To)
		where += fmt.Sprintf(" AND b.slot_start < $%d", len(args))
	}
	return where, args
}

func (s *Service) listBookingsQuery(ctx context.Context, where string, args []any, order string, limit int) ([]BookingRow, error) {
	n := len(args)
	query := fmt.Sprintf(`
		SELECT b.id, COALESCE(b.contract_id, 0), COALESCE(c.name,''), b.lead_id,
		       TRIM(COALESCE(l.first_name,'') || ' ' || COALESCE(l.last_name,'')),
		       COALESCE(l.phone,''), COALESCE(l.email,''),
		       b.created_at, b.slot_start, b.duration_min, b.delivery_mode,
		       COALESCE(buyer.name,''), COALESCE(pub.name,''), COALESCE(bbc.name, pbc.name, ''),
		       COALESCE(l.status::text,''),
		       COALESCE(b.external_event_id,'')
		FROM lead_appointment_bookings b
		LEFT JOIN contracts c ON c.id = b.contract_id
		JOIN leads l ON l.id = b.lead_id
		LEFT JOIN buyer_appointment_slots bsl ON bsl.id = b.buyer_slot_id
		LEFT JOIN publisher_appointment_slots psl ON psl.id = b.publisher_slot_id
		LEFT JOIN publisher_booking_calendars pbc ON pbc.id = b.publisher_calendar_id
		LEFT JOIN buyer_booking_calendars bbc ON bbc.id = b.buyer_calendar_id
		LEFT JOIN accounts buyer ON buyer.id = COALESCE(c.buyer_id, bbc.account_id)
		LEFT JOIN accounts pub ON pub.id = COALESCE(c.publisher_id, pbc.account_id)
		WHERE l.deleted_at IS NULL
		  AND (b.custom_time = true OR b.buyer_slot_id IS NOT NULL OR b.publisher_slot_id IS NOT NULL)
		  AND %s
		ORDER BY %s`, where, order)
	if limit > 0 {
		n++
		query += fmt.Sprintf(" LIMIT $%d", n)
		args = append(args, limit)
	}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BookingRow
	for rows.Next() {
		var r BookingRow
		var leadStatus string
		var slotStart time.Time
		if err := rows.Scan(&r.ID, &r.ContractID, &r.ContractName, &r.LeadID, &r.LeadName,
			&r.Phone, &r.Email, &r.BookedAt, &slotStart, &r.DurationMin, &r.DeliveryMode,
			&r.BuyerName, &r.PublisherName, &r.CalendarName, &leadStatus, &r.ExternalEventID); err != nil {
			return nil, err
		}
		if !slotStart.IsZero() {
			t := slotStart
			r.AppointmentAt = &t
		}
		r.LeadStatus = leadStatus
		ownerIsBuyer := strings.TrimSpace(r.BuyerName) != "" && leadStatus == "distributed"
		r.DeliveryStatus = deliveryStatus(r.DeliveryMode, leadStatus, ownerIsBuyer)
		out = append(out, r)
	}
	return out, rows.Err()
}

func parseBookedDateRange(fromStr, toStr string) (from, to *time.Time, err error) {
	parseOne := func(s string, endOfDay bool) (*time.Time, error) {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil
		}
		if t, e := time.Parse(time.RFC3339, s); e == nil {
			return &t, nil
		}
		if t, e := time.Parse("2006-01-02", s); e == nil {
			if endOfDay {
				t = t.Add(24 * time.Hour)
			}
			return &t, nil
		}
		return nil, fmt.Errorf("invalid date")
	}
	from, err = parseOne(fromStr, false)
	if err != nil {
		return nil, nil, err
	}
	to, err = parseOne(toStr, true)
	if err != nil {
		return nil, nil, err
	}
	return from, to, nil
}
