package contracts

import (
	"context"
	"errors"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

// Target is the minimal contract info the intake/routing flow needs.
type Target struct {
	ID              int64
	ParticipationID int64
	BuyerID         int64
	BuyerPipelineID int64
	BuyerStageID    int64
	RatePerLead     float64
	CompensationID  int64
	Delivery        string
	IntegrationID   int64
	WebhookID       int64
}

const perLeadCompSQL = `
SELECT cc.id, c.buyer_id, COALESCE(cc.counterparty_pipeline_id, c.buyer_pipeline_id),
       COALESCE(cc.flat_amount, cc.bid_max, 0)::float8
FROM contract_compensations cc
JOIN contracts c ON c.id = cc.contract_id
WHERE cc.contract_id = $1 AND cc.trigger = 'per_lead'
  AND cc.kind IN ('flat_rate', 'bid')
  AND c.deleted_at IS NULL AND c.status = 'active'
ORDER BY cc.position, cc.id
LIMIT 1`

func loadPerLeadTarget(ctx context.Context, q database.Querier, contractID int64) (*Target, error) {
	t := &Target{}
	err := q.QueryRow(ctx, perLeadCompSQL, contractID).Scan(
		&t.CompensationID, &t.BuyerID, &t.BuyerPipelineID, &t.RatePerLead)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return loadLegacyTarget(ctx, q, contractID)
		}
		return nil, err
	}
	t.ID = contractID
	return t, nil
}

func loadLegacyTarget(ctx context.Context, q database.Querier, contractID int64) (*Target, error) {
	t := &Target{}
	err := q.QueryRow(ctx,
		`SELECT id, buyer_id, buyer_pipeline_id, rate_per_lead::float8
		 FROM contracts WHERE id = $1 AND deleted_at IS NULL AND status = 'active'`, contractID).Scan(
		&t.ID, &t.BuyerID, &t.BuyerPipelineID, &t.RatePerLead)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("contract not found")
		}
		return nil, err
	}
	return t, nil
}

func loadPerLeadTargetByComp(ctx context.Context, q database.Querier, compID int64) (*Target, error) {
	t := &Target{}
	err := q.QueryRow(ctx,
		`SELECT cc.contract_id, c.buyer_id, COALESCE(cc.counterparty_pipeline_id, c.buyer_pipeline_id),
		        COALESCE(cc.flat_amount, cc.bid_max, 0)::float8, cc.id
		 FROM contract_compensations cc
		 JOIN contracts c ON c.id = cc.contract_id
		 WHERE cc.id = $1 AND cc.trigger = 'per_lead' AND c.deleted_at IS NULL AND c.status = 'active'`,
		compID).Scan(&t.ID, &t.BuyerID, &t.BuyerPipelineID, &t.RatePerLead, &t.CompensationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("compensation not found")
		}
		return nil, err
	}
	return t, nil
}

// GetTarget loads routing target; uses route compensation when set.
func GetTarget(ctx context.Context, q database.Querier, contractID int64, routeCompensationID *int64) (*Target, error) {
	if routeCompensationID != nil && *routeCompensationID != 0 {
		return loadPerLeadTargetByComp(ctx, q, *routeCompensationID)
	}
	return loadPerLeadTarget(ctx, q, contractID)
}

// GetTargetForRoute picks a participation and loads its routing target.
func GetTargetForRoute(ctx context.Context, q database.Querier, contractID int64, routeCompensationID *int64, leadCost float64) (*Target, error) {
	if routeCompensationID != nil && *routeCompensationID != 0 {
		return loadPerLeadTargetByComp(ctx, q, *routeCompensationID)
	}
	partID, _, err := PickParticipation(ctx, q, contractID, leadCost)
	if err != nil {
		return nil, err
	}
	if partID == 0 {
		return loadPerLeadTarget(ctx, q, contractID)
	}
	return loadParticipationTarget(ctx, q, partID)
}

// GetTargetByParticipation loads the routing target for a specific participation.
// Used by call routing where the winning buyer is already known.
func GetTargetByParticipation(ctx context.Context, q database.Querier, participationID int64) (*Target, error) {
	return loadParticipationTarget(ctx, q, participationID)
}

func loadParticipationTarget(ctx context.Context, q database.Querier, participationID int64) (*Target, error) {
	t := &Target{}
	var delivery string
	err := q.QueryRow(ctx,
		`SELECT p.contract_id, p.id, p.buyer_id,
		        COALESCE(p.buyer_pipeline_id, cc.counterparty_pipeline_id, 0),
		        COALESCE(p.buyer_target_stage_id, 0),
		        COALESCE(cc.flat_amount, cc.bid_max, 0)::float8, cc.id,
		        COALESCE(p.delivery, cc.delivery, 'leads'),
		        COALESCE(p.integration_connection_id, 0),
		        COALESCE(p.outbound_webhook_id, 0)
		 FROM contract_participations p
		 JOIN LATERAL (
		   SELECT id, flat_amount, bid_max, counterparty_pipeline_id, delivery
		   FROM contract_compensations
		   WHERE participation_id = p.id AND trigger = 'per_lead'
		   ORDER BY position, id LIMIT 1
		 ) cc ON true
		 WHERE p.id = $1 AND p.status = 'active'`,
		participationID).Scan(
		&t.ID, &t.ParticipationID, &t.BuyerID, &t.BuyerPipelineID, &t.BuyerStageID,
		&t.RatePerLead, &t.CompensationID, &delivery, &t.IntegrationID, &t.WebhookID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("participation not found")
		}
		return nil, err
	}
	t.Delivery = delivery
	return t, nil
}

// GetTargetByContract is backward-compatible entry without route compensation.
func GetTargetByContract(ctx context.Context, q database.Querier, contractID int64) (*Target, error) {
	return GetTarget(ctx, q, contractID, nil)
}

// RequireActiveContract rejects delivery when the contract is missing or not active.
func RequireActiveContract(ctx context.Context, q database.Querier, contractID int64) error {
	var status string
	err := q.QueryRow(ctx,
		`SELECT status FROM contracts WHERE id = $1 AND deleted_at IS NULL`, contractID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.NotFound("contract not found")
		}
		return err
	}
	if status != "active" {
		return httpx.BusinessRule("contract is not active")
	}
	return nil
}

// GetTargetForPreassignedBuyer loads the routing target for a pre-assigned buyer on a contract.
func GetTargetForPreassignedBuyer(ctx context.Context, q database.Querier, contractID, buyerID int64) (*Target, error) {
	var participationID int64
	err := q.QueryRow(ctx,
		`SELECT id FROM contract_participations
		 WHERE contract_id = $1 AND buyer_id = $2 AND status = 'active'
		 ORDER BY id LIMIT 1`,
		contractID, buyerID).Scan(&participationID)
	if err == nil {
		return loadParticipationTarget(ctx, q, participationID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var contractBuyerID int64
	err = q.QueryRow(ctx,
		`SELECT buyer_id FROM contracts
		 WHERE id = $1 AND deleted_at IS NULL AND status = 'active' AND buyer_id IS NOT NULL`,
		contractID).Scan(&contractBuyerID)
	if errors.Is(err, pgx.ErrNoRows) || contractBuyerID != buyerID {
		return nil, httpx.BusinessRule("pre-assigned buyer has no active participation on this contract")
	}
	if err != nil {
		return nil, err
	}
	return loadPerLeadTarget(ctx, q, contractID)
}

// FindActiveContractByBuyerPipeline returns the active contract for a buyer on a publisher pipeline.
func FindActiveContractByBuyerPipeline(ctx context.Context, q database.Querier, publisherID, buyerID, sourcePipelineID int64) (int64, error) {
	var contractID int64
	err := q.QueryRow(ctx,
		`SELECT id FROM contracts
		 WHERE publisher_id=$1 AND buyer_id=$2 AND source_pipeline_id=$3
		   AND status='active' AND deleted_at IS NULL
		 ORDER BY id DESC LIMIT 1`,
		publisherID, buyerID, sourcePipelineID).Scan(&contractID)
	if err == nil {
		return contractID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	err = q.QueryRow(ctx,
		`SELECT c.id FROM contracts c
		 JOIN contract_participations p ON p.contract_id = c.id
		 WHERE c.publisher_id = $1 AND c.source_pipeline_id = $3
		   AND p.buyer_id = $2 AND p.status = 'active'
		   AND c.status = 'active' AND c.deleted_at IS NULL
		 ORDER BY c.id DESC LIMIT 1`,
		publisherID, buyerID, sourcePipelineID).Scan(&contractID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, httpx.BusinessRule("no active contract for buyer on this pipeline")
		}
		return 0, err
	}
	return contractID, nil
}
