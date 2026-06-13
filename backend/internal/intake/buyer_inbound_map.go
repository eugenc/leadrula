package intake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/routing"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func (s *Service) buyerInboundFieldMaps(ctx context.Context, q database.Querier, buyerID int64, sourceSlug string) ([]routing.SourceFieldMapEntry, error) {
	rows, err := q.Query(ctx,
		`SELECT source_key FROM buyer_inbound_field_map
		 WHERE buyer_id = $1 AND source_slug = $2`,
		buyerID, sourceSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var maps []routing.SourceFieldMapEntry
	for rows.Next() {
		var e routing.SourceFieldMapEntry
		if err := rows.Scan(&e.SourceKey); err != nil {
			return nil, err
		}
		maps = append(maps, e)
	}
	return maps, rows.Err()
}

func (s *Service) enrichQueueItemForBuyer(ctx context.Context, buyerID int64, it *QueueItem) error {
	var raw map[string]any
	if len(it.RawPayload) > 0 {
		if err := json.Unmarshal(it.RawPayload, &raw); err != nil {
			return err
		}
	}
	slug := ""
	if it.Source != nil {
		slug = *it.Source
	}
	maps, err := s.buyerInboundFieldMaps(ctx, s.pool, buyerID, slug)
	if err != nil {
		return err
	}
	it.UnmappedKeys = computeUnmappedKeys(raw, maps)
	if it.UnmappedKeys == nil {
		it.UnmappedKeys = []string{}
	}
	return nil
}

type buyerRoutingLead struct {
	QueueItem
	OwnerAccountID int64
	PublisherID    int64
	QueueID        *int64
}

func (s *Service) loadBuyerRoutingLead(ctx context.Context, buyerID, itemID int64) (*buyerRoutingLead, error) {
	var row buyerRoutingLead
	var leadStatus string
	var queueID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(q.id, l.id), l.id, l.first_name, l.last_name, l.phone,
		        COALESCE(q.source, l.source), COALESCE(q.raw_payload, l.raw_payload),
		        l.status, l.owner_account_id, l.publisher_id, q.id
		 FROM leads l
		 LEFT JOIN lead_intake_queue q ON q.lead_id = l.id
		 WHERE (l.id = $1 OR q.id = $1)`,
		itemID).
		Scan(
			&row.ID, &row.LeadID, &row.FirstName, &row.LastName, &row.Phone,
			&row.Source, &row.RawPayload, &leadStatus, &row.OwnerAccountID, &row.PublisherID, &queueID,
		)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("routing log item not found")
	}
	if err != nil {
		return nil, err
	}

	var routed bool
	err = s.pool.QueryRow(ctx,
		`SELECT EXISTS (
		  SELECT 1 FROM transactions t
		  WHERE t.buyer_id = $1 AND t.lead_id = $2
		    AND t.contract_id IS NOT NULL AND t.type = 'debit'
		    AND t.lead_id IS NOT NULL
		    AND (
		      t.description LIKE 'lead routed:%'
		      OR t.description = 'lead routed from intake queue'
		      OR t.description = 'lead re-distributed'
		    )
		)`,
		buyerID, row.LeadID).Scan(&routed)
	if err != nil {
		return nil, err
	}
	if !routed || row.OwnerAccountID != buyerID {
		return nil, httpx.NotFound("routing log item not found")
	}

	row.Status = buyerLogStatus(buyerID, leadStatus, row.OwnerAccountID)
	row.QueueID = queueID
	if queueID != nil {
		row.ID = *queueID
	} else {
		row.ID = row.LeadID
	}
	return &row, nil
}

func (s *Service) scanBuyerRoutingItem(ctx context.Context, buyerID int64, itemID int64) (*QueueItem, error) {
	row, err := s.loadBuyerRoutingLead(ctx, buyerID, itemID)
	if err != nil {
		return nil, err
	}
	it := row.QueueItem
	if err := s.enrichQueueItemForBuyer(ctx, buyerID, &it); err != nil {
		return nil, err
	}
	return &it, nil
}

func validateBuyerCustomField(ctx context.Context, q database.Querier, buyerID, customFieldID int64) error {
	var ok bool
	err := q.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM custom_fields WHERE id = $1 AND account_id = $2)`,
		customFieldID, buyerID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.Validation("custom field not found")
	}
	return nil
}

// MapInboundFieldForBuyer saves a buyer payload mapping and applies the value to the lead.
func (s *Service) MapInboundFieldForBuyer(ctx context.Context, buyerID, itemID int64, sourceKey, targetType string, builtinField *string, customFieldID *int64) (*QueueItem, error) {
	if sourceKey == "" {
		return nil, httpx.Validation("source_key is required")
	}
	if targetType != "builtin" && targetType != "custom" && targetType != "ignore" {
		return nil, httpx.Validation("target_type must be builtin, custom, or ignore")
	}
	if targetType == "builtin" && (builtinField == nil || *builtinField == "") {
		return nil, httpx.Validation("builtin_field is required for builtin target")
	}
	if targetType == "custom" && (customFieldID == nil || *customFieldID == 0) {
		return nil, httpx.Validation("custom_field_id is required for custom target")
	}

	row, err := s.loadBuyerRoutingLead(ctx, buyerID, itemID)
	if err != nil {
		return nil, err
	}

	var raw map[string]any
	if err := json.Unmarshal(row.RawPayload, &raw); err != nil {
		return nil, err
	}
	flat := flattenPayload(raw)
	v, ok := flat[sourceKey]
	if !ok {
		return nil, httpx.Validation("source_key not found in payload")
	}

	sourceSlug := ""
	if row.Source != nil {
		sourceSlug = *row.Source
	}

	if targetType == "custom" {
		if err := validateBuyerCustomField(ctx, s.pool, buyerID, *customFieldID); err != nil {
			return nil, err
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`DELETE FROM buyer_inbound_field_map WHERE buyer_id = $1 AND source_slug = $2 AND source_key = $3`,
		buyerID, sourceSlug, sourceKey); err != nil {
		return nil, err
	}

	var insertBuiltin *string
	var insertCustom *int64
	if targetType != "ignore" {
		insertBuiltin = builtinField
		insertCustom = customFieldID
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO buyer_inbound_field_map(buyer_id, source_slug, source_key, target_type, builtin_field, custom_field_id)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		buyerID, sourceSlug, sourceKey, targetType, insertBuiltin, insertCustom); err != nil {
		return nil, fmt.Errorf("insert buyer inbound field map: %w", err)
	}

	if targetType == "builtin" && builtinField != nil {
		if err := leads.ApplyMappedField(ctx, tx, s.leads, buyerID, row.LeadID, *builtinField, v); err != nil {
			return nil, err
		}
	} else if targetType == "custom" {
		valJSON, _ := json.Marshal(v)
		if err := s.leads.UpsertCustomValue(ctx, tx, row.LeadID, *customFieldID, valJSON); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.scanBuyerRoutingItem(ctx, buyerID, itemID)
}
