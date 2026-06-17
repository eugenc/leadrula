package pipelines

import (
	"context"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

func (s *Service) requirePipeline(ctx context.Context, p *auth.Principal, pipelineID int64) error {
	var owned bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pipelines WHERE id=$1 AND account_id=$2)`,
		pipelineID, p.AccountID).Scan(&owned); err != nil {
		return err
	}
	if !owned {
		return httpx.NotFound("pipeline not found")
	}
	pubID, scoped := p.OversightPublisherID()
	if !scoped {
		return nil
	}
	allowed, err := s.collab.AllowedPipelineIDs(ctx, pubID, p.AccountID)
	if err != nil {
		return err
	}
	for _, id := range allowed {
		if id == pipelineID {
			return nil
		}
	}
	return httpx.NotFound("pipeline not found")
}

func (s *Service) requireStage(ctx context.Context, p *auth.Principal, stageID int64) error {
	var pipelineID int64
	err := s.pool.QueryRow(ctx, `SELECT pipeline_id FROM pipeline_stages WHERE id=$1`, stageID).Scan(&pipelineID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return httpx.NotFound("stage not found")
		}
		return err
	}
	return s.requirePipeline(ctx, p, pipelineID)
}

func (s *Service) CheckStageAccess(ctx context.Context, p *auth.Principal, stageID int64) error {
	return s.requireStage(ctx, p, stageID)
}
