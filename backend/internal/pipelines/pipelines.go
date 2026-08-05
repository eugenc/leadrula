// Package pipelines manages pipelines and their stages.
package pipelines

import (
	"context"
	"errors"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/collaboration"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	StageTypeStandard        = "standard"
	StageTypeAction          = "action"
	StageTypeDisqualification = "disqualification"
	StageTypeWon             = "won"
)

var ValidStageTypes = map[string]bool{
	StageTypeStandard:        true,
	StageTypeAction:          true,
	StageTypeDisqualification: true,
	StageTypeWon:             true,
}

func ValidateStageType(t string) error {
	if !ValidStageTypes[t] {
		return httpx.Validation("invalid stage_type")
	}
	return nil
}

type Pipeline struct {
	ID        int64     `json:"id"`
	PublicID  string    `json:"public_id"`
	AccountID int64     `json:"-"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

type Stage struct {
	ID         int64     `json:"id"`
	PublicID   string    `json:"public_id"`
	PipelineID int64     `json:"pipeline_id"`
	Name       string    `json:"name"`
	Position   int       `json:"position"`
	Color      string    `json:"color"`
	StageType  string    `json:"stage_type"`
	CreatedAt  time.Time `json:"created_at"`
}

type Service struct {
	pool   *pgxpool.Pool
	collab *collaboration.Repository
}

func NewService(pool *pgxpool.Pool, collab *collaboration.Repository) *Service {
	return &Service{pool: pool, collab: collab}
}

func (s *Service) List(ctx context.Context, p *auth.Principal) ([]Pipeline, error) {
	pubID, scoped := p.OversightPublisherID()
	if scoped {
		ids, err := s.collab.AllowedPipelineIDs(ctx, pubID, p.AccountID)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return []Pipeline{}, nil
		}
		rows, err := s.pool.Query(ctx,
			`SELECT id, public_id, account_id, name, position, created_at
			 FROM pipelines WHERE account_id = $1 AND id = ANY($2) ORDER BY position, id`,
			p.AccountID, ids)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanPipelines(rows)
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, public_id, account_id, name, position, created_at
		 FROM pipelines WHERE account_id = $1 ORDER BY position, id`, p.AccountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPipelines(rows)
}

func scanPipelines(rows pgx.Rows) ([]Pipeline, error) {
	var out []Pipeline
	for rows.Next() {
		var pl Pipeline
		if err := rows.Scan(&pl.ID, &pl.PublicID, &pl.AccountID, &pl.Name, &pl.Position, &pl.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, pl)
	}
	return out, rows.Err()
}

func (s *Service) ListForAccount(ctx context.Context, accountID int64) ([]Pipeline, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, public_id, account_id, name, position, created_at
		 FROM pipelines WHERE account_id = $1 ORDER BY position, id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPipelines(rows)
}

func (s *Service) Create(ctx context.Context, p *auth.Principal, name string) (*Pipeline, error) {
	pl := &Pipeline{}
	pubID, scoped := p.OversightPublisherID()
	var collabPub any
	if scoped {
		collabPub = pubID
	}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO pipelines(account_id, name, position, collaboration_publisher_id)
		 VALUES ($1, $2, COALESCE((SELECT MAX(position)+1 FROM pipelines WHERE account_id=$1), 0), $3)
		 RETURNING id, public_id, account_id, name, position, created_at`,
		p.AccountID, name, collabPub).Scan(&pl.ID, &pl.PublicID, &pl.AccountID, &pl.Name, &pl.Position, &pl.CreatedAt)
	return pl, err
}

func (s *Service) Update(ctx context.Context, p *auth.Principal, id int64, name *string, position *int) (*Pipeline, error) {
	if err := s.requirePipeline(ctx, p, id); err != nil {
		return nil, err
	}
	pl := &Pipeline{}
	err := s.pool.QueryRow(ctx,
		`UPDATE pipelines SET name = COALESCE($3, name), position = COALESCE($4, position)
		 WHERE id = $1 AND account_id = $2
		 RETURNING id, public_id, account_id, name, position, created_at`,
		id, p.AccountID, name, position).Scan(&pl.ID, &pl.PublicID, &pl.AccountID, &pl.Name, &pl.Position, &pl.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("pipeline not found")
	}
	return pl, err
}

func (s *Service) Delete(ctx context.Context, p *auth.Principal, id int64) error {
	if err := s.requirePipeline(ctx, p, id); err != nil {
		return err
	}
	if msg, err := s.pipelineDeleteBlocked(ctx, id); err != nil {
		return err
	} else if msg != "" {
		return httpx.BusinessRule(msg)
	}
	ct, err := s.pool.Exec(ctx, `DELETE FROM pipelines WHERE id = $1 AND account_id = $2`, id, p.AccountID)
	if err != nil {
		if database.IsForeignKeyViolation(err) {
			return httpx.BusinessRule("pipeline is referenced elsewhere and cannot be deleted")
		}
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("pipeline not found")
	}
	return nil
}

func (s *Service) pipelineDeleteBlocked(ctx context.Context, pipelineID int64) (string, error) {
	checks := []struct {
		query string
		msg   string
	}{
		{
			`SELECT EXISTS(
				SELECT 1 FROM leads
				WHERE pipeline_id = $1 AND deleted_at IS NULL)`,
			"cannot delete pipeline with leads assigned; move leads first",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM leads l
				JOIN pipeline_stages ps ON ps.id = l.stage_id
				WHERE ps.pipeline_id = $1 AND l.deleted_at IS NULL)`,
			"cannot delete pipeline with leads assigned; move leads first",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM contracts
				WHERE deleted_at IS NULL
				  AND (source_pipeline_id = $1 OR buyer_pipeline_id = $1))`,
			"cannot delete pipeline used by a contract",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM contracts c
				JOIN pipeline_stages ps ON ps.pipeline_id = $1
				WHERE c.deleted_at IS NULL
				  AND (c.source_stage_id = ps.id OR c.return_stage_id = ps.id))`,
			"cannot delete pipeline used by a contract",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM contract_return_rules crr
				JOIN pipeline_stages ps ON ps.pipeline_id = $1
				WHERE crr.buyer_stage_id = ps.id OR crr.return_stage_id = ps.id)`,
			"cannot delete pipeline used by a contract return rule",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM contract_participations cp
				WHERE cp.buyer_pipeline_id = $1 OR cp.source_pipeline_id = $1)`,
			"cannot delete pipeline used by a contract participation",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM contract_participations cp
				JOIN pipeline_stages ps ON ps.pipeline_id = $1
				WHERE cp.buyer_target_stage_id = ps.id
				   OR cp.source_stage_id = ps.id
				   OR cp.return_stage_id = ps.id)`,
			"cannot delete pipeline used by a contract participation",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM contract_compensations cc
				WHERE cc.source_pipeline_id = $1 OR cc.counterparty_pipeline_id = $1)`,
			"cannot delete pipeline used by contract compensation",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM contract_compensations cc
				JOIN pipeline_stages ps ON ps.pipeline_id = $1
				WHERE cc.trigger_stage_id = ps.id
				   OR cc.source_stage_id = ps.id
				   OR cc.counterparty_stage_id = ps.id
				   OR cc.return_stage_id = ps.id)`,
			"cannot delete pipeline used by contract compensation",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM routes
				WHERE origin_pipeline_id = $1 OR target_pipeline_id = $1)`,
			"cannot delete pipeline used by a route",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM routes r
				JOIN pipeline_stages ps ON ps.pipeline_id = $1
				WHERE r.origin_stage_id = ps.id OR r.target_stage_id = ps.id)`,
			"cannot delete pipeline used by a route",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM webhook_events
				WHERE target_pipeline_id = $1)`,
			"cannot delete pipeline used by a webhook",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM webhook_events we
				JOIN pipeline_stages ps ON ps.pipeline_id = $1
				WHERE we.target_stage_id = ps.id)`,
			"cannot delete pipeline used by a webhook",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM disputes
				WHERE placement_pipeline_id = $1)`,
			"cannot delete pipeline used by a dispute",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM disputes d
				JOIN pipeline_stages ps ON ps.pipeline_id = $1
				WHERE d.placement_stage_id = ps.id)`,
			"cannot delete pipeline used by a dispute",
		},
	}
	for _, c := range checks {
		var blocked bool
		if err := s.pool.QueryRow(ctx, c.query, pipelineID).Scan(&blocked); err != nil {
			return "", err
		}
		if blocked {
			return c.msg, nil
		}
	}
	return "", nil
}

// Stages

func (s *Service) ListStages(ctx context.Context, p *auth.Principal, pipelineID int64) ([]Stage, error) {
	if err := s.requirePipeline(ctx, p, pipelineID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT st.id, st.public_id, st.pipeline_id, st.name, st.position, st.color, st.stage_type, st.created_at
		 FROM pipeline_stages st JOIN pipelines p ON p.id = st.pipeline_id
		 WHERE st.pipeline_id = $1 AND p.account_id = $2
		 ORDER BY st.position, st.id`, pipelineID, p.AccountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStages(rows)
}

func (s *Service) ListStagesForAccount(ctx context.Context, accountID, pipelineID int64) ([]Stage, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT st.id, st.public_id, st.pipeline_id, st.name, st.position, st.color, st.stage_type, st.created_at
		 FROM pipeline_stages st JOIN pipelines p ON p.id = st.pipeline_id
		 WHERE st.pipeline_id = $1 AND p.account_id = $2
		 ORDER BY st.position, st.id`, pipelineID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStages(rows)
}

func (s *Service) CreateStage(ctx context.Context, p *auth.Principal, pipelineID int64, name, color, stageType string) (*Stage, error) {
	if err := s.requirePipeline(ctx, p, pipelineID); err != nil {
		return nil, err
	}
	if color == "" {
		color = "gray"
	}
	if err := validateColor(color); err != nil {
		return nil, err
	}
	if stageType == "" {
		stageType = StageTypeAction
	}
	if err := ValidateStageType(stageType); err != nil {
		return nil, err
	}
	st := &Stage{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO pipeline_stages(pipeline_id, name, position, color, stage_type)
		 VALUES ($1, $2, COALESCE((SELECT MAX(position)+1 FROM pipeline_stages WHERE pipeline_id=$1), 0), $3, $4)
		 RETURNING id, public_id, pipeline_id, name, position, color, stage_type, created_at`,
		pipelineID, name, color, stageType).Scan(
		&st.ID, &st.PublicID, &st.PipelineID, &st.Name, &st.Position, &st.Color, &st.StageType, &st.CreatedAt)
	return st, err
}

func (s *Service) UpdateStage(ctx context.Context, p *auth.Principal, stageID int64, name, color, stageType *string) (*Stage, error) {
	if err := s.requireStage(ctx, p, stageID); err != nil {
		return nil, err
	}
	if color != nil {
		if err := validateColor(*color); err != nil {
			return nil, err
		}
	}
	if stageType != nil {
		if err := ValidateStageType(*stageType); err != nil {
			return nil, err
		}
	}
	st := &Stage{}
	err := s.pool.QueryRow(ctx,
		`UPDATE pipeline_stages st SET
		   name = COALESCE($3, st.name),
		   color = COALESCE($4, st.color),
		   stage_type = COALESCE($5, st.stage_type)
		 FROM pipelines p
		 WHERE st.id = $1 AND p.id = st.pipeline_id AND p.account_id = $2
		 RETURNING st.id, st.public_id, st.pipeline_id, st.name, st.position, st.color, st.stage_type, st.created_at`,
		stageID, p.AccountID, name, color, stageType).Scan(
		&st.ID, &st.PublicID, &st.PipelineID, &st.Name, &st.Position, &st.Color, &st.StageType, &st.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("stage not found")
	}
	return st, err
}

func (s *Service) DeleteStage(ctx context.Context, p *auth.Principal, stageID int64) error {
	if err := s.requireStage(ctx, p, stageID); err != nil {
		return err
	}
	if msg, err := s.stageDeleteBlocked(ctx, stageID); err != nil {
		return err
	} else if msg != "" {
		return httpx.BusinessRule(msg)
	}
	ct, err := s.pool.Exec(ctx,
		`DELETE FROM pipeline_stages st USING pipelines p
		 WHERE st.id = $1 AND p.id = st.pipeline_id AND p.account_id = $2`,
		stageID, p.AccountID)
	if err != nil {
		if database.IsForeignKeyViolation(err) {
			return httpx.BusinessRule("stage is referenced elsewhere and cannot be deleted")
		}
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("stage not found")
	}
	return nil
}

func (s *Service) stageDeleteBlocked(ctx context.Context, stageID int64) (string, error) {
	checks := []struct {
		query string
		msg   string
	}{
		{
			`SELECT EXISTS(
				SELECT 1 FROM leads
				WHERE stage_id = $1 AND deleted_at IS NULL)`,
			"cannot delete stage with leads assigned; move leads first",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM contracts
				WHERE deleted_at IS NULL
				  AND (source_stage_id = $1 OR return_stage_id = $1))`,
			"cannot delete stage used by a contract",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM contract_return_rules
				WHERE buyer_stage_id = $1 OR return_stage_id = $1)`,
			"cannot delete stage used by a contract return rule",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM contract_participations
				WHERE buyer_target_stage_id = $1
				   OR source_stage_id = $1
				   OR return_stage_id = $1)`,
			"cannot delete stage used by a contract participation",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM contract_compensations
				WHERE trigger_stage_id = $1
				   OR source_stage_id = $1
				   OR counterparty_stage_id = $1
				   OR return_stage_id = $1)`,
			"cannot delete stage used by contract compensation",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM routes
				WHERE origin_stage_id = $1 OR target_stage_id = $1)`,
			"cannot delete stage used by a route",
		},
		{
			`SELECT EXISTS(
				SELECT 1 FROM webhook_events
				WHERE target_stage_id = $1)`,
			"cannot delete stage used by a webhook",
		},
	}
	for _, c := range checks {
		var blocked bool
		if err := s.pool.QueryRow(ctx, c.query, stageID).Scan(&blocked); err != nil {
			return "", err
		}
		if blocked {
			return c.msg, nil
		}
	}
	return "", nil
}

// Reorder sets stage positions to match the given order. Uses a temporary
// offset to avoid colliding with the (pipeline_id, position) unique index.
func (s *Service) Reorder(ctx context.Context, p *auth.Principal, pipelineID int64, orderedStageIDs []int64) error {
	if err := s.requirePipeline(ctx, p, pipelineID); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE pipeline_stages SET position = position + 100000 WHERE pipeline_id = $1`, pipelineID); err != nil {
		return err
	}
	for i, id := range orderedStageIDs {
		if _, err := tx.Exec(ctx,
			`UPDATE pipeline_stages SET position = $3 WHERE id = $1 AND pipeline_id = $2`,
			id, pipelineID, i); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func scanStages(rows pgx.Rows) ([]Stage, error) {
	var out []Stage
	for rows.Next() {
		var st Stage
		if err := rows.Scan(&st.ID, &st.PublicID, &st.PipelineID, &st.Name, &st.Position, &st.Color,
			&st.StageType, &st.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}
