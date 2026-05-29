// Package routing manages routing campaigns and per-campaign field mapping,
// plus the match engine used at intake.
package routing

import (
	"context"
	"errors"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Campaign struct {
	ID               int64     `json:"id"`
	PublisherID      int64     `json:"-"`
	CampaignName     string    `json:"campaign_name"`
	TargetPipelineID int64     `json:"target_pipeline_id"`
	TargetStageID    int64     `json:"target_stage_id"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
}

type FieldMapEntry struct {
	ID            int64     `json:"id"`
	CampaignID    int64     `json:"campaign_id"`
	SourceKey     string    `json:"source_key"`
	TargetType    string    `json:"target_type"`
	BuiltinField  *string   `json:"builtin_field"`
	CustomFieldID *int64    `json:"custom_field_id"`
	CreatedAt     time.Time `json:"created_at"`
}

// Match is the campaign target resolved at intake.
type Match struct {
	CampaignID       int64
	TargetPipelineID int64
	TargetStageID    int64
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// MatchCampaign finds an active campaign by name. nil means no match.
func MatchCampaign(ctx context.Context, q database.Querier, publisherID int64, campaignName string) (*Match, error) {
	if campaignName == "" {
		return nil, nil
	}
	m := &Match{}
	err := q.QueryRow(ctx,
		`SELECT id, target_pipeline_id, target_stage_id FROM routing_campaigns
		 WHERE publisher_id=$1 AND campaign_name=$2 AND is_active`,
		publisherID, campaignName).Scan(&m.CampaignID, &m.TargetPipelineID, &m.TargetStageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return m, nil
}

// FieldMap returns the field mapping rows for a campaign.
func FieldMap(ctx context.Context, q database.Querier, campaignID int64) ([]FieldMapEntry, error) {
	rows, err := q.Query(ctx,
		`SELECT id, campaign_id, source_key, target_type, builtin_field, custom_field_id, created_at
		 FROM routing_field_map WHERE campaign_id=$1`, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FieldMapEntry
	for rows.Next() {
		var e FieldMapEntry
		if err := rows.Scan(&e.ID, &e.CampaignID, &e.SourceKey, &e.TargetType, &e.BuiltinField, &e.CustomFieldID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Service) ListCampaigns(ctx context.Context, publisherID int64) ([]Campaign, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, publisher_id, campaign_name, target_pipeline_id, target_stage_id, is_active, created_at
		 FROM routing_campaigns WHERE publisher_id=$1 ORDER BY campaign_name`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Campaign
	for rows.Next() {
		var c Campaign
		if err := rows.Scan(&c.ID, &c.PublisherID, &c.CampaignName, &c.TargetPipelineID, &c.TargetStageID, &c.IsActive, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Service) CreateCampaign(ctx context.Context, publisherID int64, name string, pipelineID, stageID int64) (*Campaign, error) {
	c := &Campaign{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO routing_campaigns(publisher_id, campaign_name, target_pipeline_id, target_stage_id)
		 VALUES ($1,$2,$3,$4)
		 RETURNING id, publisher_id, campaign_name, target_pipeline_id, target_stage_id, is_active, created_at`,
		publisherID, name, pipelineID, stageID).Scan(
		&c.ID, &c.PublisherID, &c.CampaignName, &c.TargetPipelineID, &c.TargetStageID, &c.IsActive, &c.CreatedAt)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("a campaign with this name already exists")
		}
		return nil, err
	}
	return c, nil
}

func (s *Service) UpdateCampaign(ctx context.Context, publisherID, id int64, name *string, pipelineID, stageID *int64, isActive *bool) (*Campaign, error) {
	c := &Campaign{}
	err := s.pool.QueryRow(ctx,
		`UPDATE routing_campaigns SET
		   campaign_name = COALESCE($3, campaign_name),
		   target_pipeline_id = COALESCE($4, target_pipeline_id),
		   target_stage_id = COALESCE($5, target_stage_id),
		   is_active = COALESCE($6, is_active)
		 WHERE id=$1 AND publisher_id=$2
		 RETURNING id, publisher_id, campaign_name, target_pipeline_id, target_stage_id, is_active, created_at`,
		id, publisherID, name, pipelineID, stageID, isActive).Scan(
		&c.ID, &c.PublisherID, &c.CampaignName, &c.TargetPipelineID, &c.TargetStageID, &c.IsActive, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("campaign not found")
	}
	return c, err
}

func (s *Service) DeleteCampaign(ctx context.Context, publisherID, id int64) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM routing_campaigns WHERE id=$1 AND publisher_id=$2`, id, publisherID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("campaign not found")
	}
	return nil
}

func (s *Service) ListFieldMap(ctx context.Context, campaignID int64) ([]FieldMapEntry, error) {
	return FieldMap(ctx, s.pool, campaignID)
}

func (s *Service) AddFieldMap(ctx context.Context, campaignID int64, sourceKey, targetType string, builtinField *string, customFieldID *int64) (*FieldMapEntry, error) {
	if targetType != "builtin" && targetType != "custom" {
		return nil, httpx.Validation("target_type must be builtin or custom")
	}
	e := &FieldMapEntry{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO routing_field_map(campaign_id, source_key, target_type, builtin_field, custom_field_id)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, campaign_id, source_key, target_type, builtin_field, custom_field_id, created_at`,
		campaignID, sourceKey, targetType, builtinField, customFieldID).Scan(
		&e.ID, &e.CampaignID, &e.SourceKey, &e.TargetType, &e.BuiltinField, &e.CustomFieldID, &e.CreatedAt)
	if err != nil {
		if database.IsCheckViolation(err) {
			return nil, httpx.Validation("provide builtin_field for builtin target, or custom_field_id for custom target")
		}
		return nil, err
	}
	return e, nil
}

func (s *Service) DeleteFieldMap(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM routing_field_map WHERE id=$1`, id)
	return err
}

// CampaignOwnedBy verifies a campaign belongs to the publisher.
func (s *Service) CampaignOwnedBy(ctx context.Context, publisherID, campaignID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM routing_campaigns WHERE id=$1 AND publisher_id=$2)`, campaignID, publisherID).Scan(&ok)
	return ok, err
}
