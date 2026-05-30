// Package contracts manages publisher↔buyer contracts and return rules.
package contracts

import (
	"context"
	"errors"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Contract struct {
	ID               int64     `json:"id"`
	PublicID         string    `json:"public_id"`
	PublisherID      int64     `json:"-"`
	BuyerID          int64     `json:"buyer_id"`
	BuyerName        string    `json:"buyer_name,omitempty"`
	Name             string    `json:"name"`
	SourcePipelineID int64     `json:"source_pipeline_id"`
	SourceStageID    int64     `json:"source_stage_id"`
	BuyerPipelineID  int64     `json:"buyer_pipeline_id"`
	ReturnStageID    int64     `json:"return_stage_id"`
	RatePerLead      float64   `json:"rate_per_lead"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

type ReturnRule struct {
	ID             int64     `json:"id"`
	ContractID     int64     `json:"contract_id"`
	BuyerStageID   int64     `json:"buyer_stage_id"`
	ReturnStageID  int64     `json:"return_stage_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// PublisherStage is a pipeline stage exposed to buyers for return-rule To Stage picks.
type PublisherStage struct {
	ID                     int64  `json:"id"`
	PublicID               string `json:"public_id"`
	PipelineID             int64  `json:"pipeline_id"`
	Name                   string `json:"name"`
	Position               int    `json:"position"`
	Color                  string `json:"color"`
	PromptActionDatetime   bool   `json:"prompt_action_datetime"`
	PromptDisqualification bool   `json:"prompt_disqualification"`
}

// Target is the minimal contract info the intake flow needs.
type Target struct {
	ID              int64
	BuyerID         int64
	BuyerPipelineID int64
	RatePerLead     float64
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (s *Service) Pool() *pgxpool.Pool { return s.pool }

const contractCols = `id, public_id, publisher_id, buyer_id, name,
	source_pipeline_id, source_stage_id, buyer_pipeline_id, return_stage_id,
	rate_per_lead::float8, status, created_at`

func scanContract(row pgx.Row) (*Contract, error) {
	c := &Contract{}
	err := row.Scan(&c.ID, &c.PublicID, &c.PublisherID, &c.BuyerID, &c.Name,
		&c.SourcePipelineID, &c.SourceStageID, &c.BuyerPipelineID, &c.ReturnStageID,
		&c.RatePerLead, &c.Status, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("contract not found")
		}
		return nil, err
	}
	return c, nil
}

func (s *Service) List(ctx context.Context, publisherID int64) ([]Contract, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT c.id, c.public_id, c.publisher_id, c.buyer_id, c.name,
		        c.source_pipeline_id, c.source_stage_id, c.buyer_pipeline_id, c.return_stage_id,
		        c.rate_per_lead::float8, c.status, c.created_at, a.name
		 FROM contracts c JOIN accounts a ON a.id = c.buyer_id
		 WHERE c.publisher_id = $1 AND c.deleted_at IS NULL
		 ORDER BY c.created_at DESC`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Contract
	for rows.Next() {
		var c Contract
		if err := rows.Scan(&c.ID, &c.PublicID, &c.PublisherID, &c.BuyerID, &c.Name,
			&c.SourcePipelineID, &c.SourceStageID, &c.BuyerPipelineID, &c.ReturnStageID,
			&c.RatePerLead, &c.Status, &c.CreatedAt, &c.BuyerName); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Service) Get(ctx context.Context, publisherID, id int64) (*Contract, error) {
	return scanContract(s.pool.QueryRow(ctx,
		`SELECT `+contractCols+` FROM contracts WHERE id = $1 AND publisher_id = $2 AND deleted_at IS NULL`,
		id, publisherID))
}

// GetForBuyer returns the buyer's single contract.
func (s *Service) GetForBuyer(ctx context.Context, buyerID int64) (*Contract, error) {
	return scanContract(s.pool.QueryRow(ctx,
		`SELECT `+contractCols+` FROM contracts WHERE buyer_id = $1 AND deleted_at IS NULL`, buyerID))
}

type CreateParams struct {
	BuyerID          int64
	Name             string
	SourcePipelineID int64
	SourceStageID    int64
	BuyerPipelineID  int64
	ReturnStageID    int64
	RatePerLead      float64
}

func (s *Service) Create(ctx context.Context, publisherID int64, p CreateParams) (*Contract, error) {
	c, err := scanContract(s.pool.QueryRow(ctx,
		`INSERT INTO contracts(publisher_id, buyer_id, name, source_pipeline_id, source_stage_id,
		    buyer_pipeline_id, return_stage_id, rate_per_lead)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING `+contractCols,
		publisherID, p.BuyerID, p.Name, p.SourcePipelineID, p.SourceStageID,
		p.BuyerPipelineID, p.ReturnStageID, p.RatePerLead))
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("a contract already exists for this buyer")
		}
		return nil, err
	}
	return c, nil
}

func (s *Service) Update(ctx context.Context, publisherID, id int64, name *string, rate *float64, status *string) (*Contract, error) {
	return scanContract(s.pool.QueryRow(ctx,
		`UPDATE contracts SET
		   name = COALESCE($3, name),
		   rate_per_lead = COALESCE($4, rate_per_lead),
		   status = COALESCE($5, status)
		 WHERE id = $1 AND publisher_id = $2 AND deleted_at IS NULL
		 RETURNING `+contractCols,
		id, publisherID, name, rate, status))
}

func (s *Service) Delete(ctx context.Context, publisherID, id int64) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE contracts SET deleted_at = now() WHERE id = $1 AND publisher_id = $2 AND deleted_at IS NULL`,
		id, publisherID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("contract not found")
	}
	return nil
}

// ── Return rules ──────────────────────────────────────────────────

func (s *Service) ListReturnRules(ctx context.Context, contractID int64) ([]ReturnRule, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, contract_id, buyer_stage_id, return_stage_id, created_at FROM contract_return_rules
		 WHERE contract_id = $1 ORDER BY id`, contractID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReturnRule
	for rows.Next() {
		var rr ReturnRule
		if err := rows.Scan(&rr.ID, &rr.ContractID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

func (s *Service) validateReturnRuleStages(ctx context.Context, contractID, buyerStageID, returnStageID int64) error {
	var buyerPipelineID, sourcePipelineID int64
	err := s.pool.QueryRow(ctx,
		`SELECT buyer_pipeline_id, source_pipeline_id FROM contracts WHERE id = $1 AND deleted_at IS NULL`,
		contractID).Scan(&buyerPipelineID, &sourcePipelineID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("contract not found")
	}
	if err != nil {
		return err
	}
	var ok bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pipeline_stages WHERE id = $1 AND pipeline_id = $2)`,
		buyerStageID, buyerPipelineID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return httpx.Validation("buyer stage not in contract buyer pipeline")
	}
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pipeline_stages WHERE id = $1 AND pipeline_id = $2)`,
		returnStageID, sourcePipelineID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return httpx.Validation("return stage not in contract publisher pipeline")
	}
	return nil
}

func (s *Service) AddReturnRule(ctx context.Context, contractID, buyerStageID, returnStageID int64) (*ReturnRule, error) {
	if err := s.validateReturnRuleStages(ctx, contractID, buyerStageID, returnStageID); err != nil {
		return nil, err
	}
	rr := &ReturnRule{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO contract_return_rules(contract_id, buyer_stage_id, return_stage_id) VALUES ($1,$2,$3)
		 ON CONFLICT (contract_id, buyer_stage_id) DO UPDATE SET return_stage_id = EXCLUDED.return_stage_id
		 RETURNING id, contract_id, buyer_stage_id, return_stage_id, created_at`,
		contractID, buyerStageID, returnStageID).Scan(
		&rr.ID, &rr.ContractID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("return rule already exists for this buyer stage")
		}
		return nil, err
	}
	return rr, nil
}

func (s *Service) UpdateReturnRule(ctx context.Context, ruleID, buyerStageID, returnStageID int64) (*ReturnRule, error) {
	var contractID int64
	err := s.pool.QueryRow(ctx,
		`SELECT contract_id FROM contract_return_rules WHERE id = $1`, ruleID).Scan(&contractID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("return rule not found")
	}
	if err != nil {
		return nil, err
	}
	if err := s.validateReturnRuleStages(ctx, contractID, buyerStageID, returnStageID); err != nil {
		return nil, err
	}
	rr := &ReturnRule{}
	err = s.pool.QueryRow(ctx,
		`UPDATE contract_return_rules SET buyer_stage_id = $2, return_stage_id = $3
		 WHERE id = $1
		 RETURNING id, contract_id, buyer_stage_id, return_stage_id, created_at`,
		ruleID, buyerStageID, returnStageID).Scan(
		&rr.ID, &rr.ContractID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("return rule already exists for this buyer stage")
		}
		return nil, err
	}
	return rr, nil
}

func (s *Service) DeleteReturnRule(ctx context.Context, ruleID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM contract_return_rules WHERE id = $1`, ruleID)
	return err
}

func (s *Service) PublisherReturnStages(ctx context.Context, buyerID int64) ([]PublisherStage, error) {
	c, err := s.GetForBuyer(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, public_id, pipeline_id, name, position, color,
		        prompt_action_datetime, prompt_disqualification
		 FROM pipeline_stages WHERE pipeline_id = $1 ORDER BY position, id`, c.SourcePipelineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PublisherStage
	for rows.Next() {
		var st PublisherStage
		if err := rows.Scan(&st.ID, &st.PublicID, &st.PipelineID, &st.Name, &st.Position, &st.Color,
			&st.PromptActionDatetime, &st.PromptDisqualification); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// ReturnRuleContractID returns the contract id for a return rule (buyer access checks).
func (s *Service) ReturnRuleContractID(ctx context.Context, ruleID int64) (int64, error) {
	var contractID int64
	err := s.pool.QueryRow(ctx,
		`SELECT contract_id FROM contract_return_rules WHERE id = $1`, ruleID).Scan(&contractID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, httpx.NotFound("return rule not found")
	}
	return contractID, err
}

// ContractIDForBuyer returns the buyer's contract id (for return-rule scoping).
func (s *Service) ContractIDForBuyer(ctx context.Context, buyerID int64) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM contracts WHERE buyer_id = $1 AND deleted_at IS NULL`, buyerID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, httpx.NotFound("no contract for buyer")
	}
	return id, err
}

// ── Helpers used inside intake / leads transactions (accept Querier) ──

// FindByBuyerPipeline resolves the contract whose buyer pipeline matches.
func FindByBuyerPipeline(ctx context.Context, q database.Querier, buyerPipelineID int64) (*Target, error) {
	t := &Target{}
	err := q.QueryRow(ctx,
		`SELECT id, buyer_id, buyer_pipeline_id, rate_per_lead::float8
		 FROM contracts WHERE buyer_pipeline_id = $1 AND status = 'active' AND deleted_at IS NULL
		 LIMIT 1`, buyerPipelineID).Scan(&t.ID, &t.BuyerID, &t.BuyerPipelineID, &t.RatePerLead)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

// FindByBuyer resolves the buyer's active contract target.
func FindByBuyer(ctx context.Context, q database.Querier, buyerID int64) (*Target, error) {
	t := &Target{}
	err := q.QueryRow(ctx,
		`SELECT id, buyer_id, buyer_pipeline_id, rate_per_lead::float8
		 FROM contracts WHERE buyer_id = $1 AND status = 'active' AND deleted_at IS NULL
		 LIMIT 1`, buyerID).Scan(&t.ID, &t.BuyerID, &t.BuyerPipelineID, &t.RatePerLead)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

// GetTarget loads a contract target by id.
func GetTarget(ctx context.Context, q database.Querier, contractID int64) (*Target, error) {
	t := &Target{}
	err := q.QueryRow(ctx,
		`SELECT id, buyer_id, buyer_pipeline_id, rate_per_lead::float8
		 FROM contracts WHERE id = $1 AND deleted_at IS NULL`, contractID).Scan(
		&t.ID, &t.BuyerID, &t.BuyerPipelineID, &t.RatePerLead)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("contract not found")
		}
		return nil, err
	}
	return t, nil
}

// ReturnInfo holds where a returned lead lands back on the publisher side.
type ReturnInfo struct {
	SourcePipelineID int64
	ReturnStageID    int64
	PublisherID      int64
}

// FindReturnRule checks whether entering newStageID triggers a return for the
// lead's contract; returns the publisher landing info if so, else nil.
func FindReturnRule(ctx context.Context, q database.Querier, contractID, newStageID int64) (*ReturnInfo, error) {
	ri := &ReturnInfo{}
	err := q.QueryRow(ctx,
		`SELECT c.source_pipeline_id, rr.return_stage_id, c.publisher_id
		 FROM contract_return_rules rr
		 JOIN contracts c ON c.id = rr.contract_id
		 WHERE rr.contract_id = $1 AND rr.buyer_stage_id = $2`,
		contractID, newStageID).Scan(&ri.SourcePipelineID, &ri.ReturnStageID, &ri.PublisherID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ri, nil
}
