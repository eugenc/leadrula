package pipelines

import (
	"context"
	"errors"
	"time"

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

func (s *Service) assertDisqStage(ctx context.Context, accountID, stageID int64) error {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM pipeline_stages ps
		   JOIN pipelines p ON p.id = ps.pipeline_id
		   WHERE ps.id = $1 AND p.account_id = $2 AND ps.stage_type = 'disqualification'
		 )`, stageID, accountID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.NotFound("disqualification stage not found")
	}
	return nil
}

func (s *Service) ListStageReasons(ctx context.Context, accountID, stageID int64) ([]DisqReason, error) {
	if err := s.assertDisqStage(ctx, accountID, stageID); err != nil {
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

func (s *Service) ListPipelineReasons(ctx context.Context, accountID, pipelineID int64) ([]DisqReason, error) {
	var owned bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pipelines WHERE id=$1 AND account_id=$2)`, pipelineID, accountID).Scan(&owned); err != nil {
		return nil, err
	}
	if !owned {
		return nil, httpx.NotFound("pipeline not found")
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

func (s *Service) CreateStageReason(ctx context.Context, accountID, stageID int64, label string) (*DisqReason, error) {
	if err := s.assertDisqStage(ctx, accountID, stageID); err != nil {
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

func (s *Service) UpdateStageReason(ctx context.Context, accountID, id int64, label *string, position *int, isActive *bool) (*DisqReason, error) {
	d := &DisqReason{}
	err := s.pool.QueryRow(ctx,
		`UPDATE disqualification_reasons dr SET
		   label = COALESCE($3, dr.label),
		   position = COALESCE($4, dr.position),
		   is_active = COALESCE($5, dr.is_active)
		 FROM pipeline_stages ps
		 JOIN pipelines p ON p.id = ps.pipeline_id
		 WHERE dr.id = $1 AND dr.stage_id = ps.id AND p.account_id = $2
		 RETURNING dr.id, dr.stage_id, dr.label, dr.position, dr.is_active, dr.created_at`,
		id, accountID, label, position, isActive).Scan(
		&d.ID, &d.StageID, &d.Label, &d.Position, &d.IsActive, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("reason not found")
	}
	return d, err
}

func (s *Service) DeleteStageReason(ctx context.Context, accountID, id int64) error {
	var inUse bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM leads WHERE disqualification_reason_id = $1)`, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return httpx.BusinessRule("cannot delete a reason in use; deactivate it instead")
	}
	ct, err := s.pool.Exec(ctx,
		`DELETE FROM disqualification_reasons dr
		 USING pipeline_stages ps, pipelines p
		 WHERE dr.id = $1 AND dr.stage_id = ps.id AND p.id = ps.pipeline_id AND p.account_id = $2`,
		id, accountID)
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
