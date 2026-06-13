package pipelines

import (
	"context"
	"errors"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type DisqReason struct {
	ID        int64     `json:"id"`
	StageID   int64     `json:"stage_id"`
	StageName string    `json:"stage_name,omitempty"`
	Label     string    `json:"label"`
	Position  int       `json:"position"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

func (s *Service) assertDisqStage(ctx context.Context, p *auth.Principal, stageID int64) error {
	if err := s.requireStage(ctx, p, stageID); err != nil {
		return err
	}
	var disq bool
	err := s.pool.QueryRow(ctx,
		`SELECT stage_type = 'disqualification' FROM pipeline_stages WHERE id = $1`, stageID).Scan(&disq)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.NotFound("disqualification stage not found")
		}
		return err
	}
	if !disq {
		return httpx.NotFound("disqualification stage not found")
	}
	return nil
}

func (s *Service) ListStageReasons(ctx context.Context, p *auth.Principal, stageID int64) ([]DisqReason, error) {
	if err := s.assertDisqStage(ctx, p, stageID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, stage_id, label, position, is_active, created_at
		 FROM disqualification_reasons WHERE stage_id = $1 ORDER BY position, id`, stageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDisqReasons(rows)
}

func (s *Service) ListPipelineReasons(ctx context.Context, p *auth.Principal, pipelineID int64) ([]DisqReason, error) {
	if err := s.requirePipeline(ctx, p, pipelineID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT dr.id, dr.stage_id, ps.name, dr.label, dr.position, dr.is_active, dr.created_at
		 FROM disqualification_reasons dr
		 JOIN pipeline_stages ps ON ps.id = dr.stage_id
		 WHERE ps.pipeline_id = $1 AND ps.stage_type = 'disqualification'
		 ORDER BY ps.position, dr.position, dr.id`, pipelineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DisqReason
	for rows.Next() {
		var d DisqReason
		if err := rows.Scan(&d.ID, &d.StageID, &d.StageName, &d.Label, &d.Position, &d.IsActive, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Service) CreateStageReason(ctx context.Context, p *auth.Principal, stageID int64, label string) (*DisqReason, error) {
	if err := s.assertDisqStage(ctx, p, stageID); err != nil {
		return nil, err
	}
	d := &DisqReason{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO disqualification_reasons(stage_id, label, position)
		 VALUES ($1, $2, COALESCE((SELECT MAX(position)+1 FROM disqualification_reasons WHERE stage_id=$1), 0))
		 RETURNING id, stage_id, label, position, is_active, created_at`,
		stageID, label).Scan(&d.ID, &d.StageID, &d.Label, &d.Position, &d.IsActive, &d.CreatedAt)
	return d, err
}

func (s *Service) UpdateStageReason(ctx context.Context, p *auth.Principal, id int64, label *string, position *int, isActive *bool) (*DisqReason, error) {
	var stageID int64
	err := s.pool.QueryRow(ctx, `SELECT stage_id FROM disqualification_reasons WHERE id = $1`, id).Scan(&stageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("reason not found")
		}
		return nil, err
	}
	if err := s.assertDisqStage(ctx, p, stageID); err != nil {
		return nil, err
	}
	d := &DisqReason{}
	err = s.pool.QueryRow(ctx,
		`UPDATE disqualification_reasons dr SET
		   label = COALESCE($2, dr.label),
		   position = COALESCE($3, dr.position),
		   is_active = COALESCE($4, dr.is_active)
		 WHERE dr.id = $1
		 RETURNING dr.id, dr.stage_id, dr.label, dr.position, dr.is_active, dr.created_at`,
		id, label, position, isActive).Scan(
		&d.ID, &d.StageID, &d.Label, &d.Position, &d.IsActive, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("reason not found")
	}
	return d, err
}

func (s *Service) DeleteStageReason(ctx context.Context, p *auth.Principal, id int64) error {
	var stageID int64
	err := s.pool.QueryRow(ctx, `SELECT stage_id FROM disqualification_reasons WHERE id = $1`, id).Scan(&stageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.NotFound("reason not found")
		}
		return err
	}
	if err := s.assertDisqStage(ctx, p, stageID); err != nil {
		return err
	}
	var inUse bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM leads WHERE disqualification_reason_id = $1)`, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return httpx.BusinessRule("cannot delete a reason in use; deactivate it instead")
	}
	ct, err := s.pool.Exec(ctx, `DELETE FROM disqualification_reasons WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("reason not found")
	}
	return nil
}

func (s *Service) ReasonBelongsToStage(ctx context.Context, stageID, reasonID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM disqualification_reasons WHERE id=$1 AND stage_id=$2)`, reasonID, stageID).Scan(&ok)
	return ok, err
}

func (s *Service) ReasonBelongsToLeadPipeline(ctx context.Context, leadID, reasonID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM disqualification_reasons dr
		   JOIN pipeline_stages ps ON ps.id = dr.stage_id
		   JOIN leads l ON l.pipeline_id = ps.pipeline_id
		   WHERE dr.id = $1 AND l.id = $2
		 )`, reasonID, leadID).Scan(&ok)
	return ok, err
}

func (s *Service) reasonBelongsToRulePipeline(ctx context.Context, ruleStageID, reasonID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM disqualification_reasons dr
		   JOIN pipeline_stages ps ON ps.id = dr.stage_id
		   JOIN pipeline_stages rule_ps ON rule_ps.pipeline_id = ps.pipeline_id
		   WHERE dr.id = $1 AND rule_ps.id = $2
		 )`, reasonID, ruleStageID).Scan(&ok)
	return ok, err
}

func (s *Service) HasActiveStageReasons(ctx context.Context, stageID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM disqualification_reasons WHERE stage_id=$1 AND is_active=true)`, stageID).Scan(&ok)
	return ok, err
}

func scanDisqReasons(rows pgx.Rows) ([]DisqReason, error) {
	var out []DisqReason
	for rows.Next() {
		var d DisqReason
		if err := rows.Scan(&d.ID, &d.StageID, &d.Label, &d.Position, &d.IsActive, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
