package contracts

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/echayko/leadrula/backend/internal/customfields"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/routing"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type AvailableField struct {
	FieldType     string `json:"field_type"`
	BuiltinField  string `json:"builtin_field,omitempty"`
	CustomFieldID *int64 `json:"custom_field_id,omitempty"`
	Label         string `json:"label"`
	Key           string `json:"key"`
}

type ContractFieldMapOptions struct {
	AvailableFields []AvailableField           `json:"available_fields"`
	BuyerFields     []customfields.CustomField `json:"buyer_fields"`
}

type AddFieldMapParams struct {
	SrcType          string
	SrcBuiltin       string
	SrcCustomFieldID *int64
	DstType          string
	DstBuiltin       string
	DstCustomFieldID *int64
}

func (s *Service) ListFieldMapForBuyer(ctx context.Context, buyerID, contractID int64) ([]FieldMapEntry, error) {
	if _, err := s.GetForBuyerContract(ctx, buyerID, contractID); err != nil {
		return nil, err
	}
	partID, err := s.participationIDForBuyerContract(ctx, contractID, buyerID)
	if err != nil {
		return nil, err
	}
	return s.loadFieldMapEntries(ctx, contractID, partID)
}

func (s *Service) ListFieldMapForParticipationBuyer(ctx context.Context, buyerID, participationID int64) ([]FieldMapEntry, error) {
	part, err := s.GetParticipationForBuyer(ctx, buyerID, participationID)
	if err != nil {
		return nil, err
	}
	return s.loadFieldMapEntries(ctx, part.ContractID, &participationID)
}

func (s *Service) FieldMapOptionsForBuyer(ctx context.Context, buyerID, contractID int64) (*ContractFieldMapOptions, error) {
	c, err := s.GetForBuyerContract(ctx, buyerID, contractID)
	if err != nil {
		return nil, err
	}
	return s.fieldMapOptions(ctx, c.PublisherID, buyerID, contractID, nil)
}

func (s *Service) FieldMapOptionsForParticipationBuyer(ctx context.Context, buyerID, participationID int64) (*ContractFieldMapOptions, error) {
	part, err := s.GetParticipationForBuyer(ctx, buyerID, participationID)
	if err != nil {
		return nil, err
	}
	var pubID int64
	if err := s.pool.QueryRow(ctx, `SELECT publisher_id FROM contracts WHERE id = $1`, part.ContractID).Scan(&pubID); err != nil {
		return nil, err
	}
	return s.fieldMapOptions(ctx, pubID, buyerID, part.ContractID, nil)
}

func (s *Service) AddFieldMapForBuyer(ctx context.Context, buyerID, contractID int64, p AddFieldMapParams) (*FieldMapEntry, error) {
	if _, err := s.GetForBuyerContract(ctx, buyerID, contractID); err != nil {
		return nil, err
	}
	partID, err := s.participationIDForBuyerContract(ctx, contractID, buyerID)
	if err != nil {
		return nil, err
	}
	return s.addFieldMapEntry(ctx, contractID, partID, buyerID, p)
}

func (s *Service) AddFieldMapForParticipationBuyer(ctx context.Context, buyerID, participationID int64, p AddFieldMapParams) (*FieldMapEntry, error) {
	part, err := s.GetParticipationForBuyer(ctx, buyerID, participationID)
	if err != nil {
		return nil, err
	}
	return s.addFieldMapEntry(ctx, part.ContractID, &participationID, buyerID, p)
}

func (s *Service) DeleteFieldMapForBuyer(ctx context.Context, buyerID, contractID, mapID int64) error {
	if _, err := s.GetForBuyerContract(ctx, buyerID, contractID); err != nil {
		return err
	}
	partID, err := s.participationIDForBuyerContract(ctx, contractID, buyerID)
	if err != nil {
		return err
	}
	return s.deleteFieldMapEntry(ctx, contractID, partID, mapID)
}

func (s *Service) DeleteFieldMapForParticipationBuyer(ctx context.Context, buyerID, participationID, mapID int64) error {
	part, err := s.GetParticipationForBuyer(ctx, buyerID, participationID)
	if err != nil {
		return err
	}
	return s.deleteFieldMapEntry(ctx, part.ContractID, &participationID, mapID)
}

func RequireFieldMappingComplete(ctx context.Context, q database.Querier, contractID, buyerID, participationID int64) error {
	partID := participationID
	if partID == 0 {
		id, err := participationIDForBuyerContractQuerier(ctx, q, contractID, buyerID)
		if err != nil {
			return err
		}
		if id != nil {
			partID = *id
		}
	}
	required, err := loadTemplateRequiredFields(ctx, q, contractID)
	if err != nil {
		return err
	}
	if len(required) == 0 {
		return nil
	}
	mapped, err := loadFieldMapEntriesQuerier(ctx, q, contractID, nullableParticipationID(partID))
	if err != nil {
		return err
	}
	if missing := missingRequiredFieldMaps(required, mapped); len(missing) > 0 {
		return httpx.BusinessRule("field mapping incomplete: " + strings.Join(missing, ", "))
	}
	return nil
}

func ContractFieldMapForRoute(ctx context.Context, q database.Querier, contractID, participationID int64) ([]routing.RouteFieldMapEntry, error) {
	partID := nullableParticipationID(participationID)
	if participationID == 0 {
		var buyerID int64
		if err := q.QueryRow(ctx, `SELECT buyer_id FROM contracts WHERE id = $1`, contractID).Scan(&buyerID); err == nil && buyerID != 0 {
			id, err := participationIDForBuyerContractQuerier(ctx, q, contractID, buyerID)
			if err != nil {
				return nil, err
			}
			partID = id
		}
	}
	entries, err := loadFieldMapEntriesQuerier(ctx, q, contractID, partID)
	if err != nil {
		return nil, err
	}
	out := make([]routing.RouteFieldMapEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, contractFieldMapToRoute(e))
	}
	return out, nil
}

func (s *Service) ValidateParticipationFieldMapping(ctx context.Context, contractID, participationID int64) error {
	required, err := loadTemplateRequiredFields(ctx, s.pool, contractID)
	if err != nil {
		return err
	}
	if len(required) == 0 {
		return nil
	}
	mapped, err := s.loadFieldMapEntries(ctx, contractID, &participationID)
	if err != nil {
		return err
	}
	if missing := missingRequiredFieldMaps(required, mapped); len(missing) > 0 {
		return httpx.Validation("field mapping incomplete: " + strings.Join(missing, ", "))
	}
	return nil
}

func contractFieldMapToRoute(e FieldMapEntry) routing.RouteFieldMapEntry {
	row := routing.RouteFieldMapEntry{
		SrcType:          e.SrcType,
		DstType:          e.DstType,
		SrcCustomFieldID: e.SrcCustomFieldID,
		DstCustomFieldID: e.DstCustomFieldID,
	}
	if e.SrcBuiltin != "" {
		b := e.SrcBuiltin
		row.SrcBuiltin = &b
	}
	if e.DstBuiltin != "" {
		b := e.DstBuiltin
		row.DstBuiltin = &b
	}
	return row
}

func (s *Service) fieldMapOptions(ctx context.Context, publisherID, buyerID, contractID int64, participationID *int64) (*ContractFieldMapOptions, error) {
	required, err := loadTemplateRequiredFields(ctx, s.pool, contractID)
	if err != nil {
		return nil, err
	}
	cf := customfields.NewService(s.pool)
	pubFields, err := cf.ListFields(ctx, publisherID)
	if err != nil {
		return nil, err
	}
	buyerFields, err := cf.ListFields(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	pubByID := map[int64]string{}
	for _, f := range pubFields {
		pubByID[f.ID] = f.Name
	}
	available := make([]AvailableField, 0, len(required))
	for _, r := range required {
		af := AvailableField{
			FieldType:     r.FieldType,
			BuiltinField:  r.BuiltinField,
			CustomFieldID: r.CustomFieldID,
		}
		if r.FieldType == "custom" && r.CustomFieldID != nil {
			af.Label = pubByID[*r.CustomFieldID]
			if af.Label == "" {
				af.Label = fmt.Sprintf("Custom field #%d", *r.CustomFieldID)
			}
			af.Key = fmt.Sprintf("cf:%d", *r.CustomFieldID)
		} else {
			af.Label = strings.ReplaceAll(r.BuiltinField, "_", " ")
			af.Key = r.BuiltinField
		}
		available = append(available, af)
	}
	if buyerFields == nil {
		buyerFields = []customfields.CustomField{}
	}
	return &ContractFieldMapOptions{
		AvailableFields: available,
		BuyerFields:     buyerFields,
	}, nil
}

func loadTemplateRequiredFields(ctx context.Context, q database.Querier, contractID int64) ([]RequiredField, error) {
	rows, err := q.Query(ctx,
		`SELECT id, field_type, COALESCE(builtin_field,''), custom_field_id
		 FROM contract_required_fields
		 WHERE contract_id = $1 AND participation_id IS NULL
		 ORDER BY id`, contractID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RequiredField
	for rows.Next() {
		var r RequiredField
		var builtin string
		if err := rows.Scan(&r.ID, &r.FieldType, &builtin, &r.CustomFieldID); err != nil {
			return nil, err
		}
		r.BuiltinField = builtin
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) loadFieldMapEntries(ctx context.Context, contractID int64, participationID *int64) ([]FieldMapEntry, error) {
	return loadFieldMapEntriesQuerier(ctx, s.pool, contractID, participationID)
}

func loadFieldMapEntriesQuerier(ctx context.Context, q database.Querier, contractID int64, participationID *int64) ([]FieldMapEntry, error) {
	var rows pgx.Rows
	var err error
	if participationID == nil {
		rows, err = q.Query(ctx,
			`SELECT id, src_type, src_builtin, src_custom_field_id, dst_type, dst_builtin, dst_custom_field_id
			 FROM contract_field_map
			 WHERE contract_id = $1 AND participation_id IS NULL
			 ORDER BY id`, contractID)
	} else {
		rows, err = q.Query(ctx,
			`SELECT id, src_type, src_builtin, src_custom_field_id, dst_type, dst_builtin, dst_custom_field_id
			 FROM contract_field_map
			 WHERE contract_id = $1 AND participation_id = $2
			 ORDER BY id`, contractID, *participationID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FieldMapEntry
	for rows.Next() {
		var e FieldMapEntry
		if err := rows.Scan(&e.ID, &e.SrcType, &e.SrcBuiltin, &e.SrcCustomFieldID, &e.DstType, &e.DstBuiltin, &e.DstCustomFieldID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Service) addFieldMapEntry(ctx context.Context, contractID int64, participationID *int64, buyerID int64, p AddFieldMapParams) (*FieldMapEntry, error) {
	if err := validateFieldMapParams(p); err != nil {
		return nil, err
	}
	required, err := loadTemplateRequiredFields(ctx, s.pool, contractID)
	if err != nil {
		return nil, err
	}
	if !requiredFieldMatches(required, p.SrcType, p.SrcBuiltin, p.SrcCustomFieldID) {
		return nil, httpx.Validation("source field is not an available contract field")
	}
	if err := validateBuyerDstField(ctx, s.pool, buyerID, p.DstType, p.DstBuiltin, p.DstCustomFieldID); err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if participationID == nil {
		_, err = tx.Exec(ctx,
			`DELETE FROM contract_field_map
			 WHERE contract_id = $1 AND participation_id IS NULL
			   AND src_type = $2 AND COALESCE(src_builtin,'') = $3
			   AND COALESCE(src_custom_field_id,0) = COALESCE($4,0)`,
			contractID, p.SrcType, p.SrcBuiltin, p.SrcCustomFieldID)
	} else {
		_, err = tx.Exec(ctx,
			`DELETE FROM contract_field_map
			 WHERE contract_id = $1 AND participation_id = $2
			   AND src_type = $3 AND COALESCE(src_builtin,'') = $4
			   AND COALESCE(src_custom_field_id,0) = COALESCE($5,0)`,
			contractID, *participationID, p.SrcType, p.SrcBuiltin, p.SrcCustomFieldID)
	}
	if err != nil {
		return nil, err
	}

	var e FieldMapEntry
	var row pgx.Row
	if participationID == nil {
		row = tx.QueryRow(ctx,
			`INSERT INTO contract_field_map(
			    contract_id, src_type, src_builtin, src_custom_field_id, dst_type, dst_builtin, dst_custom_field_id)
			 VALUES ($1,$2,NULLIF($3,''),$4,$5,NULLIF($6,''),$7)
			 RETURNING id, src_type, src_builtin, src_custom_field_id, dst_type, dst_builtin, dst_custom_field_id`,
			contractID, p.SrcType, p.SrcBuiltin, p.SrcCustomFieldID, p.DstType, p.DstBuiltin, p.DstCustomFieldID)
	} else {
		row = tx.QueryRow(ctx,
			`INSERT INTO contract_field_map(
			    contract_id, participation_id, src_type, src_builtin, src_custom_field_id, dst_type, dst_builtin, dst_custom_field_id)
			 VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,NULLIF($7,''),$8)
			 RETURNING id, src_type, src_builtin, src_custom_field_id, dst_type, dst_builtin, dst_custom_field_id`,
			contractID, *participationID, p.SrcType, p.SrcBuiltin, p.SrcCustomFieldID, p.DstType, p.DstBuiltin, p.DstCustomFieldID)
	}
	if err := row.Scan(&e.ID, &e.SrcType, &e.SrcBuiltin, &e.SrcCustomFieldID, &e.DstType, &e.DstBuiltin, &e.DstCustomFieldID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *Service) deleteFieldMapEntry(ctx context.Context, contractID int64, participationID *int64, mapID int64) error {
	var ct pgconn.CommandTag
	var err error
	if participationID == nil {
		ct, err = s.pool.Exec(ctx,
			`DELETE FROM contract_field_map WHERE id = $1 AND contract_id = $2 AND participation_id IS NULL`,
			mapID, contractID)
	} else {
		ct, err = s.pool.Exec(ctx,
			`DELETE FROM contract_field_map WHERE id = $1 AND contract_id = $2 AND participation_id = $3`,
			mapID, contractID, *participationID)
	}
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("field mapping not found")
	}
	return nil
}

func validateFieldMapParams(p AddFieldMapParams) error {
	if p.SrcType != "builtin" && p.SrcType != "custom" {
		return httpx.Validation("src_type must be builtin or custom")
	}
	if p.DstType != "builtin" && p.DstType != "custom" {
		return httpx.Validation("dst_type must be builtin or custom")
	}
	if p.SrcType == "builtin" && strings.TrimSpace(p.SrcBuiltin) == "" {
		return httpx.Validation("src_builtin is required for builtin source")
	}
	if p.SrcType == "custom" && (p.SrcCustomFieldID == nil || *p.SrcCustomFieldID == 0) {
		return httpx.Validation("src_custom_field_id is required for custom source")
	}
	if p.DstType == "builtin" && strings.TrimSpace(p.DstBuiltin) == "" {
		return httpx.Validation("dst_builtin is required for builtin destination")
	}
	if p.DstType == "custom" && (p.DstCustomFieldID == nil || *p.DstCustomFieldID == 0) {
		return httpx.Validation("dst_custom_field_id is required for custom destination")
	}
	return nil
}

func validateBuyerDstField(ctx context.Context, q database.Querier, buyerID int64, dstType, dstBuiltin string, dstCustomID *int64) error {
	if dstType == "custom" {
		var ok bool
		err := q.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM custom_fields WHERE id = $1 AND account_id = $2)`,
			*dstCustomID, buyerID).Scan(&ok)
		if err != nil {
			return err
		}
		if !ok {
			return httpx.Validation("destination custom field does not belong to buyer")
		}
	}
	return nil
}

func requiredFieldMatches(required []RequiredField, srcType, srcBuiltin string, srcCustomID *int64) bool {
	for _, r := range required {
		if r.FieldType != srcType {
			continue
		}
		if srcType == "builtin" && r.BuiltinField == srcBuiltin {
			return true
		}
		if srcType == "custom" && r.CustomFieldID != nil && srcCustomID != nil && *r.CustomFieldID == *srcCustomID {
			return true
		}
	}
	return false
}

func missingRequiredFieldMaps(required []RequiredField, mapped []FieldMapEntry) []string {
	var missing []string
	for _, r := range required {
		found := false
		for _, m := range mapped {
			if r.FieldType == m.SrcType {
				if r.FieldType == "builtin" && r.BuiltinField == m.SrcBuiltin {
					found = true
					break
				}
				if r.FieldType == "custom" && r.CustomFieldID != nil && m.SrcCustomFieldID != nil && *r.CustomFieldID == *m.SrcCustomFieldID {
					found = true
					break
				}
			}
		}
		if !found {
			if r.FieldType == "custom" && r.CustomFieldID != nil {
				missing = append(missing, fmt.Sprintf("custom:%d", *r.CustomFieldID))
			} else {
				missing = append(missing, r.BuiltinField)
			}
		}
	}
	return missing
}

func (s *Service) participationIDForBuyerContract(ctx context.Context, contractID, buyerID int64) (*int64, error) {
	return participationIDForBuyerContractQuerier(ctx, s.pool, contractID, buyerID)
}

func participationIDForBuyerContractQuerier(ctx context.Context, q database.Querier, contractID, buyerID int64) (*int64, error) {
	var id int64
	err := q.QueryRow(ctx,
		`SELECT id FROM contract_participations
		 WHERE contract_id = $1 AND buyer_id = $2 AND status = 'active'
		 ORDER BY id LIMIT 1`, contractID, buyerID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func nullableParticipationID(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}
