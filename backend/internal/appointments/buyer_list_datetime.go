package appointments

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/jackc/pgx/v5"
)

type contractFieldMapRow struct {
	SrcType          string
	SrcBuiltin       string
	SrcCustomFieldID *int64
	DstType          string
	DstBuiltin       string
	DstCustomFieldID *int64
}

func (s *Service) resolveAppointmentFromFieldMaps(ctx context.Context, contractID, buyerID int64, lead *leads.Lead) (*time.Time, error) {
	partID, err := s.participationIDForContract(ctx, contractID, buyerID)
	if err != nil {
		return nil, err
	}

	if partID != nil {
		maps, err := loadContractFieldMaps(ctx, s.pool, contractID, partID)
		if err != nil {
			return nil, err
		}
		if t := resolveDatetimeFromMaps(ctx, s.pool, lead, maps); t != nil {
			return t, nil
		}
	}

	maps, err := loadContractFieldMaps(ctx, s.pool, contractID, nil)
	if err != nil {
		return nil, err
	}
	return resolveDatetimeFromMaps(ctx, s.pool, lead, maps), nil
}

func (s *Service) participationIDForContract(ctx context.Context, contractID, buyerID int64) (*int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM contract_participations
		 WHERE contract_id = $1 AND buyer_id = $2
		 ORDER BY id LIMIT 1`,
		contractID, buyerID).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

func loadContractFieldMaps(ctx context.Context, q database.Querier, contractID int64, participationID *int64) ([]contractFieldMapRow, error) {
	var rows pgx.Rows
	var err error
	if participationID == nil {
		rows, err = q.Query(ctx,
			`SELECT src_type, COALESCE(src_builtin,''), src_custom_field_id,
			        dst_type, COALESCE(dst_builtin,''), dst_custom_field_id
			 FROM contract_field_map
			 WHERE contract_id = $1 AND participation_id IS NULL
			 ORDER BY id`, contractID)
	} else {
		rows, err = q.Query(ctx,
			`SELECT src_type, COALESCE(src_builtin,''), src_custom_field_id,
			        dst_type, COALESCE(dst_builtin,''), dst_custom_field_id
			 FROM contract_field_map
			 WHERE contract_id = $1 AND participation_id = $2
			 ORDER BY id`, contractID, *participationID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []contractFieldMapRow
	for rows.Next() {
		var r contractFieldMapRow
		if err := rows.Scan(&r.SrcType, &r.SrcBuiltin, &r.SrcCustomFieldID,
			&r.DstType, &r.DstBuiltin, &r.DstCustomFieldID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func resolveDatetimeFromMaps(ctx context.Context, q database.Querier, lead *leads.Lead, maps []contractFieldMapRow) *time.Time {
	for _, m := range maps {
		if !isActionAtDstMap(m) {
			continue
		}
		if t := readSourceDatetime(lead, m); t != nil {
			return t
		}
	}
	for _, m := range maps {
		if !isDatetimeDstMap(ctx, q, m) {
			continue
		}
		if t := readSourceDatetime(lead, m); t != nil {
			return t
		}
	}
	return nil
}

func isActionAtDstMap(m contractFieldMapRow) bool {
	return m.DstType == "builtin" && m.DstBuiltin == "action_at"
}

func isDatetimeDstMap(ctx context.Context, q database.Querier, m contractFieldMapRow) bool {
	if isActionAtDstMap(m) {
		return false
	}
	if m.DstType != "custom" || m.DstCustomFieldID == nil {
		return false
	}
	var ftype string
	if err := q.QueryRow(ctx, `SELECT type FROM custom_fields WHERE id = $1`, *m.DstCustomFieldID).Scan(&ftype); err != nil {
		return false
	}
	return ftype == "date" || ftype == "datetime"
}

func readSourceDatetime(lead *leads.Lead, m contractFieldMapRow) *time.Time {
	raw, ok := readMapSourceValue(lead, m)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil
	}
	t, err := leads.ParseActionAt(raw)
	if err != nil || t == nil {
		return nil
	}
	return t
}

func readMapSourceValue(lead *leads.Lead, m contractFieldMapRow) (string, bool) {
	if m.SrcType == "builtin" && m.SrcBuiltin != "" {
		v := leadBuiltinString(lead, m.SrcBuiltin)
		return v, v != ""
	}
	if m.SrcType == "custom" && m.SrcCustomFieldID != nil && lead.CustomValues != nil {
		raw, ok := lead.CustomValues[fmt.Sprintf("%d", *m.SrcCustomFieldID)]
		if !ok || len(raw) == 0 {
			return "", false
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return strings.TrimSpace(s), s != ""
		}
		return strings.TrimSpace(string(raw)), true
	}
	return "", false
}

func leadBuiltinString(lead *leads.Lead, field string) string {
	switch field {
	case "first_name":
		return lead.FirstName
	case "last_name":
		return lead.LastName
	case "phone":
		if lead.Phone != nil {
			return *lead.Phone
		}
	case "email":
		if lead.Email != nil {
			return *lead.Email
		}
	case "action_at":
		if lead.ActionAt != nil {
			return lead.ActionAt.Format(time.RFC3339)
		}
	}
	return ""
}

func (s *Service) enrichRouteAppointmentTimes(ctx context.Context, items []BookingRow, leadIDs []int64) error {
	leadsByID := map[int64]*leads.Lead{}
	for _, id := range leadIDs {
		lead, err := s.leads.GetByID(ctx, s.pool, id)
		if err != nil {
			continue
		}
		if err := leads.LoadCustomValues(ctx, s.pool, lead); err != nil {
			continue
		}
		leadsByID[id] = lead
	}

	for i := range items {
		if !items[i].IsRoute || items[i].AppointmentAt != nil {
			continue
		}
		lead := leadsByID[items[i].LeadID]
		if lead == nil {
			continue
		}
		t, err := s.resolveAppointmentFromFieldMaps(ctx, items[i].ContractID, lead.OwnerAccountID, lead)
		if err != nil || t == nil {
			continue
		}
		items[i].AppointmentAt = t
	}
	return nil
}
