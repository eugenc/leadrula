package appointments

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type BuyerListParams struct {
	BuyerID           int64
	CalendarID        int64 // 0 = all calendars
	Page              int
	Limit             int
	Sort              string
	SortDir           string
	Q                 string
	ContractID        int64
	PublisherID       int64
	AppointmentPreset string
	From              *time.Time
	To                *time.Time
}

type BuyerListResult struct {
	Items []BookingRow `json:"items"`
	Total int          `json:"total"`
}

const (
	buyerListSortBooked      = "booked_at"
	buyerListSortAppointment = "appointment_at"
)

func (s *Service) ListBuyerBookings(ctx context.Context, p BuyerListParams) (*BuyerListResult, error) {
	if p.Limit <= 0 {
		p.Limit = 25
	}
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.AppointmentPreset == "" {
		p.AppointmentPreset = "this_week"
	}
	tz, err := s.getAccountTimezone(ctx, p.BuyerID)
	if err != nil {
		return nil, err
	}
	return s.listBuyerAppointments(ctx, p, tz)
}

func (s *Service) ListBuyerBookingsForCalendar(ctx context.Context, buyerID, calendarID int64) ([]BookingRow, error) {
	if _, err := s.loadCalendar(ctx, buyerID, calendarID); err != nil {
		return nil, err
	}
	res, err := s.ListBuyerBookings(ctx, BuyerListParams{
		BuyerID:           buyerID,
		CalendarID:        calendarID,
		Page:              1,
		Limit:             500,
		Sort:              buyerListSortBooked,
		SortDir:           "desc",
		AppointmentPreset: "all",
	})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

func (s *Service) listBuyerAppointments(ctx context.Context, p BuyerListParams, tz string) (*BuyerListResult, error) {
	from, to, excludeNullAppt := appointmentPresetBounds(p.AppointmentPreset, tz, time.Now())
	if p.From != nil || p.To != nil {
		from = p.From
		to = p.To
		excludeNullAppt = true
	}
	baseSQL, args := buyerAppointmentsBaseSQL(p)

	where := ""
	if p.CalendarID > 0 {
		args = append(args, p.CalendarID)
		where += fmt.Sprintf(" AND calendar_id = $%d", len(args))
	}
	if p.ContractID > 0 {
		args = append(args, p.ContractID)
		where += fmt.Sprintf(" AND contract_id = $%d", len(args))
	}
	if p.PublisherID > 0 {
		args = append(args, p.PublisherID)
		where += fmt.Sprintf(" AND publisher_id = $%d", len(args))
	}
	if q := strings.TrimSpace(p.Q); q != "" {
		args = append(args, "%"+q+"%")
		n := len(args)
		where += fmt.Sprintf(` AND (
			lead_name ILIKE $%d OR COALESCE(phone,'') ILIKE $%d OR COALESCE(email,'') ILIKE $%d
		)`, n, n, n)
	}
	if excludeNullAppt {
		where += " AND appointment_at IS NOT NULL"
		if from != nil {
			args = append(args, *from)
			where += fmt.Sprintf(" AND appointment_at >= $%d", len(args))
		}
		if to != nil {
			args = append(args, *to)
			where += fmt.Sprintf(" AND appointment_at < $%d", len(args))
		}
	}

	sortCol := buyerListSortBooked
	if p.Sort == buyerListSortAppointment {
		sortCol = buyerListSortAppointment
	}
	sortDir := "DESC"
	if strings.EqualFold(p.SortDir, "asc") {
		sortDir = "ASC"
	}
	nulls := "NULLS LAST"
	if sortCol == buyerListSortBooked {
		nulls = ""
	}

	countSQL := fmt.Sprintf("SELECT COUNT(*)::int FROM (%s) sub WHERE 1=1%s", baseSQL, where)
	var total int
	if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (p.Page - 1) * p.Limit
	args = append(args, p.Limit, offset)
	limitArg := len(args) - 1
	offsetArg := len(args)
	listSQL := fmt.Sprintf(`
		SELECT id, contract_id, contract_name, lead_id, lead_name, phone, email,
		       booked_at, appointment_at, duration_min, delivery_mode,
		       buyer_name, publisher_name, calendar_name, lead_status, is_route
		FROM (%s) sub
		WHERE 1=1%s
		ORDER BY %s %s %s, id DESC
		LIMIT $%d OFFSET $%d`,
		baseSQL, where, sortCol, sortDir, nulls, limitArg, offsetArg)

	rows, err := s.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []BookingRow
	var routeLeadIDs []int64
	for rows.Next() {
		var r BookingRow
		var phone, email *string
		var isRoute bool
		if err := rows.Scan(&r.ID, &r.ContractID, &r.ContractName, &r.LeadID, &r.LeadName,
			&phone, &email, &r.BookedAt, &r.AppointmentAt, &r.DurationMin, &r.DeliveryMode,
			&r.BuyerName, &r.PublisherName, &r.CalendarName, &r.LeadStatus, &isRoute); err != nil {
			return nil, err
		}
		if phone != nil {
			r.Phone = *phone
		}
		if email != nil {
			r.Email = *email
		}
		r.DeliveryStatus = "delivered"
		r.IsRoute = isRoute
		if isRoute && r.AppointmentAt == nil {
			routeLeadIDs = append(routeLeadIDs, r.LeadID)
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(routeLeadIDs) > 0 {
		if err := s.enrichRouteAppointmentTimes(ctx, items, routeLeadIDs); err != nil {
			return nil, err
		}
	}

	return &BuyerListResult{Items: items, Total: total}, nil
}

func buyerAppointmentsBaseSQL(p BuyerListParams) (string, []any) {
	args := []any{p.BuyerID}
	buyerArg := len(args)

	bookingSQL := fmt.Sprintf(`
		SELECT b.id AS id,
		       COALESCE(b.contract_id, 0) AS contract_id,
		       COALESCE(c.name, '') AS contract_name,
		       b.lead_id,
		       TRIM(COALESCE(l.first_name, '') || ' ' || COALESCE(l.last_name, '')) AS lead_name,
		       l.phone,
		       l.email,
		       b.created_at AS booked_at,
		       b.slot_start AS appointment_at,
		       b.duration_min,
		       b.delivery_mode,
		       COALESCE(buyer.name, '') AS buyer_name,
		       COALESCE(pub.name, '') AS publisher_name,
		       COALESCE(bbc.name, pbc_direct.name, bbc_slot.name, pbc_slot.name, '') AS calendar_name,
		       COALESCE(l.status::text, '') AS lead_status,
		       false AS is_route,
		       COALESCE(c.publisher_id, 0) AS publisher_id,
		       COALESCE(b.buyer_calendar_id, bsl.calendar_id, psl.calendar_id) AS calendar_id
		FROM lead_appointment_bookings b
		LEFT JOIN contracts c ON c.id = b.contract_id
		JOIN leads l ON l.id = b.lead_id
		LEFT JOIN buyer_appointment_slots bsl ON bsl.id = b.buyer_slot_id
		LEFT JOIN publisher_appointment_slots psl ON psl.id = b.publisher_slot_id
		LEFT JOIN buyer_booking_calendars bbc ON bbc.id = b.buyer_calendar_id
		LEFT JOIN publisher_booking_calendars pbc_direct ON pbc_direct.id = b.publisher_calendar_id
		LEFT JOIN buyer_booking_calendars bbc_slot ON bbc_slot.id = bsl.calendar_id
		LEFT JOIN publisher_booking_calendars pbc_slot ON pbc_slot.id = psl.calendar_id
		LEFT JOIN accounts buyer ON buyer.id = COALESCE(c.buyer_id, bbc.account_id)
		LEFT JOIN accounts pub ON pub.id = c.publisher_id
		WHERE l.deleted_at IS NULL
		  AND l.owner_account_id = $%d
		  AND l.status IN ('distributed', 'closed', 'review')`, buyerArg)

	routeSQL := fmt.Sprintf(`
		SELECT (-l.id) AS id,
		       l.contract_id,
		       COALESCE(c.name, '') AS contract_name,
		       l.id AS lead_id,
		       TRIM(COALESCE(l.first_name, '') || ' ' || COALESCE(l.last_name, '')) AS lead_name,
		       l.phone,
		       l.email,
		       COALESCE(re.created_at, l.updated_at) AS booked_at,
		       l.action_at AS appointment_at,
		       0 AS duration_min,
		       'contract' AS delivery_mode,
		       COALESCE(buyer.name, '') AS buyer_name,
		       COALESCE(pub.name, '') AS publisher_name,
		       COALESCE(route_cal.name, '') AS calendar_name,
		       COALESCE(l.status::text, '') AS lead_status,
		       true AS is_route,
		       c.publisher_id,
		       c.appointment_calendar_id AS calendar_id
		FROM leads l
		JOIN contracts c ON c.id = l.contract_id AND c.lead_type = 'Appointment'
		JOIN accounts buyer ON buyer.id = c.buyer_id
		JOIN accounts pub ON pub.id = c.publisher_id
		LEFT JOIN buyer_booking_calendars route_cal ON route_cal.id = c.appointment_calendar_id
		LEFT JOIN LATERAL (
			SELECT re.created_at
			FROM route_executions re
			WHERE re.lead_id = l.id
			  AND re.target_account_id = $%d
			  AND re.status = 'success'
			ORDER BY re.created_at DESC
			LIMIT 1
		) re ON true
		WHERE l.deleted_at IS NULL
		  AND l.owner_account_id = $%d
		  AND l.contract_id IS NOT NULL
		  AND l.status IN ('distributed', 'closed', 'review')
		  AND NOT EXISTS (
			SELECT 1 FROM lead_appointment_bookings b WHERE b.lead_id = l.id
		  )`, buyerArg, buyerArg)

	return bookingSQL + " UNION ALL " + routeSQL, args
}

func appointmentPresetBounds(preset, tz string, now time.Time) (from, to *time.Time, excludeNull bool) {
	if preset == "" || preset == "all" {
		return nil, nil, false
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	now = now.In(loc)
	excludeNull = true

	switch preset {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		end := start.AddDate(0, 0, 1)
		return &start, &end, excludeNull
	case "this_week":
		daysSinceMon := (int(now.Weekday()) + 6) % 7
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -daysSinceMon)
		end := start.AddDate(0, 0, 7)
		return &start, &end, excludeNull
	case "this_month":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		end := start.AddDate(0, 1, 0)
		return &start, &end, excludeNull
	default:
		return nil, nil, false
	}
}
