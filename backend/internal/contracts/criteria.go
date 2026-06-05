package contracts

import (
	"context"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type RequiredField struct {
	ID            int64  `json:"id,omitempty"`
	FieldType     string `json:"field_type"`
	BuiltinField  string `json:"builtin_field,omitempty"`
	CustomFieldID *int64 `json:"custom_field_id,omitempty"`
}

type FieldMapEntry struct {
	ID               int64  `json:"id,omitempty"`
	SrcType          string `json:"src_type"`
	SrcBuiltin       string `json:"src_builtin,omitempty"`
	SrcCustomFieldID *int64 `json:"src_custom_field_id,omitempty"`
	DstType          string `json:"dst_type"`
	DstBuiltin       string `json:"dst_builtin,omitempty"`
	DstCustomFieldID *int64 `json:"dst_custom_field_id,omitempty"`
}

type FilterRule struct {
	ID            int64  `json:"id,omitempty"`
	FieldType     string `json:"field_type"`
	BuiltinField  string `json:"builtin_field,omitempty"`
	CustomFieldID *int64 `json:"custom_field_id,omitempty"`
	Operator      string `json:"operator"`
	Value         string `json:"value"`
}

type QualityRule struct {
	ID           int64  `json:"id,omitempty"`
	BuyerStageID int64  `json:"buyer_stage_id"`
	OnFail       string `json:"on_fail"`
}

type LeadCriteria struct {
	RequiredFields []RequiredField `json:"required_fields"`
	FieldMap       []FieldMapEntry `json:"field_map"`
	FilterRules    []FilterRule    `json:"filter_rules"`
	QualityRules   []QualityRule   `json:"quality_rules"`
}

func (s *Service) GetLeadCriteria(ctx context.Context, publisherID, contractID int64) (*LeadCriteria, error) {
	if _, err := s.Get(ctx, publisherID, contractID); err != nil {
		return nil, err
	}
	return s.loadLeadCriteria(ctx, contractID)
}

func (s *Service) loadLeadCriteria(ctx context.Context, contractID int64) (*LeadCriteria, error) {
	out := &LeadCriteria{
		RequiredFields: []RequiredField{},
		FieldMap:       []FieldMapEntry{},
		FilterRules:    []FilterRule{},
		QualityRules:   []QualityRule{},
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, field_type, COALESCE(builtin_field,''), custom_field_id
		 FROM contract_required_fields WHERE contract_id = $1 ORDER BY id`, contractID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r RequiredField
		var builtin string
		if err := rows.Scan(&r.ID, &r.FieldType, &builtin, &r.CustomFieldID); err != nil {
			rows.Close()
			return nil, err
		}
		r.BuiltinField = builtin
		out.RequiredFields = append(out.RequiredFields, r)
	}
	rows.Close()

	rows, err = s.pool.Query(ctx,
		`SELECT id, src_type, src_builtin, src_custom_field_id, dst_type, dst_builtin, dst_custom_field_id
		 FROM contract_field_map WHERE contract_id = $1 ORDER BY id`, contractID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var e FieldMapEntry
		if err := rows.Scan(&e.ID, &e.SrcType, &e.SrcBuiltin, &e.SrcCustomFieldID, &e.DstType, &e.DstBuiltin, &e.DstCustomFieldID); err != nil {
			rows.Close()
			return nil, err
		}
		out.FieldMap = append(out.FieldMap, e)
	}
	rows.Close()

	rows, err = s.pool.Query(ctx,
		`SELECT id, field_type, COALESCE(builtin_field,''), custom_field_id, operator, value
		 FROM contract_filter_rules WHERE contract_id = $1 ORDER BY id`, contractID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r FilterRule
		var builtin string
		if err := rows.Scan(&r.ID, &r.FieldType, &builtin, &r.CustomFieldID, &r.Operator, &r.Value); err != nil {
			rows.Close()
			return nil, err
		}
		r.BuiltinField = builtin
		out.FilterRules = append(out.FilterRules, r)
	}
	rows.Close()

	rows, err = s.pool.Query(ctx,
		`SELECT id, buyer_stage_id, on_fail FROM contract_quality_rules WHERE contract_id = $1 ORDER BY id`, contractID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r QualityRule
		if err := rows.Scan(&r.ID, &r.BuyerStageID, &r.OnFail); err != nil {
			rows.Close()
			return nil, err
		}
		out.QualityRules = append(out.QualityRules, r)
	}
	return out, rows.Err()
}

func (s *Service) SaveLeadCriteria(ctx context.Context, publisherID, contractID int64, c LeadCriteria) error {
	if _, err := s.Get(ctx, publisherID, contractID); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, table := range []string{
		"contract_required_fields",
		"contract_field_map",
		"contract_filter_rules",
		"contract_quality_rules",
	} {
		if _, err := tx.Exec(ctx, `DELETE FROM `+table+` WHERE contract_id = $1`, contractID); err != nil {
			return err
		}
	}

	for _, r := range c.RequiredFields {
		if _, err := tx.Exec(ctx,
			`INSERT INTO contract_required_fields(contract_id, field_type, builtin_field, custom_field_id)
			 VALUES ($1,$2,NULLIF($3,''),$4)`,
			contractID, r.FieldType, r.BuiltinField, r.CustomFieldID); err != nil {
			return err
		}
	}
	for _, e := range c.FieldMap {
		if _, err := tx.Exec(ctx,
			`INSERT INTO contract_field_map(contract_id, src_type, src_builtin, src_custom_field_id, dst_type, dst_builtin, dst_custom_field_id)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			contractID, e.SrcType, e.SrcBuiltin, e.SrcCustomFieldID, e.DstType, e.DstBuiltin, e.DstCustomFieldID); err != nil {
			return err
		}
	}
	for _, r := range c.FilterRules {
		if _, err := tx.Exec(ctx,
			`INSERT INTO contract_filter_rules(contract_id, field_type, builtin_field, custom_field_id, operator, value)
			 VALUES ($1,$2,NULLIF($3,''),$4,$5,$6)`,
			contractID, r.FieldType, r.BuiltinField, r.CustomFieldID, r.Operator, r.Value); err != nil {
			return err
		}
	}
	for _, r := range c.QualityRules {
		if r.BuyerStageID == 0 {
			return httpx.Validation("buyer_stage_id is required for quality rules")
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO contract_quality_rules(contract_id, buyer_stage_id, on_fail) VALUES ($1,$2,$3)`,
			contractID, r.BuyerStageID, r.OnFail); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func saveLeadCriteriaTx(ctx context.Context, tx pgx.Tx, contractID int64, c *LeadCriteria) error {
	if c == nil {
		return nil
	}
	for _, r := range c.RequiredFields {
		if _, err := tx.Exec(ctx,
			`INSERT INTO contract_required_fields(contract_id, field_type, builtin_field, custom_field_id)
			 VALUES ($1,$2,NULLIF($3,''),$4)`,
			contractID, r.FieldType, r.BuiltinField, r.CustomFieldID); err != nil {
			return err
		}
	}
	for _, e := range c.FieldMap {
		if _, err := tx.Exec(ctx,
			`INSERT INTO contract_field_map(contract_id, src_type, src_builtin, src_custom_field_id, dst_type, dst_builtin, dst_custom_field_id)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			contractID, e.SrcType, e.SrcBuiltin, e.SrcCustomFieldID, e.DstType, e.DstBuiltin, e.DstCustomFieldID); err != nil {
			return err
		}
	}
	for _, r := range c.FilterRules {
		if _, err := tx.Exec(ctx,
			`INSERT INTO contract_filter_rules(contract_id, field_type, builtin_field, custom_field_id, operator, value)
			 VALUES ($1,$2,NULLIF($3,''),$4,$5,$6)`,
			contractID, r.FieldType, r.BuiltinField, r.CustomFieldID, r.Operator, r.Value); err != nil {
			return err
		}
	}
	for _, r := range c.QualityRules {
		if _, err := tx.Exec(ctx,
			`INSERT INTO contract_quality_rules(contract_id, buyer_stage_id, on_fail) VALUES ($1,$2,$3)`,
			contractID, r.BuyerStageID, r.OnFail); err != nil {
			return err
		}
	}
	return nil
}
