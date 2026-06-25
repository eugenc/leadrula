package appointments

import (
	"context"
	"fmt"
	"strings"
)

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
	return s.listBookingsQuery(ctx,
		`c.publisher_id = $1`, []any{publisherID}, "b.created_at DESC", 0)
}

func (s *Service) ListBuyerBookings(ctx context.Context, buyerID int64) ([]BookingRow, error) {
	return s.listBookingsQuery(ctx,
		`l.owner_account_id = $1 AND l.status IN ('distributed', 'closed')`, []any{buyerID}, "b.created_at DESC", 0)
}

func (s *Service) listBookingsQuery(ctx context.Context, where string, args []any, order string, limit int) ([]BookingRow, error) {
	n := len(args)
	query := fmt.Sprintf(`
		SELECT b.id, b.contract_id, COALESCE(c.name,''), b.lead_id,
		       TRIM(COALESCE(l.first_name,'') || ' ' || COALESCE(l.last_name,'')),
		       COALESCE(l.phone,''), COALESCE(l.email,''),
		       b.slot_start, b.duration_min, b.delivery_mode,
		       COALESCE(buyer.name,''), COALESCE(pub.name,''), COALESCE(l.status,''),
		       b.created_at
		FROM lead_appointment_bookings b
		JOIN contracts c ON c.id = b.contract_id
		JOIN leads l ON l.id = b.lead_id
		JOIN buyer_appointment_slots sl ON sl.id = b.buyer_slot_id
		JOIN accounts buyer ON buyer.id = c.buyer_id
		JOIN accounts pub ON pub.id = c.publisher_id
		WHERE l.deleted_at IS NULL AND %s
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
		if err := rows.Scan(&r.ID, &r.ContractID, &r.ContractName, &r.LeadID, &r.LeadName,
			&r.Phone, &r.Email, &r.SlotStart, &r.DurationMin, &r.DeliveryMode,
			&r.BuyerName, &r.PublisherName, &leadStatus, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.LeadStatus = leadStatus
		ownerIsBuyer := strings.TrimSpace(r.BuyerName) != "" && leadStatus == "distributed"
		r.DeliveryStatus = deliveryStatus(r.DeliveryMode, leadStatus, ownerIsBuyer)
		out = append(out, r)
	}
	return out, rows.Err()
}
