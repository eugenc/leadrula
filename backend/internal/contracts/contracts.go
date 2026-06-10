// Package contracts manages publisher↔buyer contracts and return rules.
package contracts

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Contract struct {
	ID               int64     `json:"id"`
	PublicID         string    `json:"public_id"`
	HandlerID        string    `json:"handler_id"`
	PublisherID      int64     `json:"-"`
	BuyerID          *int64    `json:"buyer_id,omitempty"`
	BuyerName        string    `json:"buyer_name,omitempty"`
	BuyerAccountType string    `json:"buyer_account_type,omitempty"`
	PublisherName    string    `json:"publisher_name,omitempty"`
	Name             string    `json:"name"`
	Description      string    `json:"description,omitempty"`
	LeadType         string    `json:"lead_type,omitempty"`
	SourcePipelineID *int64    `json:"source_pipeline_id,omitempty"`
	SourceStageID    *int64    `json:"source_stage_id,omitempty"`
	BuyerPipelineID  *int64    `json:"buyer_pipeline_id,omitempty"`
	ReturnStageID    *int64    `json:"return_stage_id,omitempty"`
	RatePerLead      float64   `json:"rate_per_lead"`
	Status           string    `json:"status"`
	CapPeriod        string    `json:"cap_period"`
	CapTotal         *int      `json:"cap_total,omitempty"`
	CapMaxDaily      *int      `json:"cap_max_daily,omitempty"`
	ContractType     string    `json:"contract_type"`
	MirrorContractID *int64    `json:"mirror_contract_id,omitempty"`
	LeadCount              int       `json:"lead_count"`
	AllowedDeliveryModes   []string  `json:"allowed_delivery_modes,omitempty"`
	DistributionStrategy   string    `json:"distribution_strategy,omitempty"`
	ParentContractID       *int64    `json:"parent_contract_id,omitempty"`
	InviteToken            string    `json:"invite_token,omitempty"`
	Participations         []Participation `json:"participations,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
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
	ID         int64  `json:"id"`
	PublicID   string `json:"public_id"`
	PipelineID int64  `json:"pipeline_id"`
	Name       string `json:"name"`
	Position   int    `json:"position"`
	Color      string `json:"color"`
	StageType  string `json:"stage_type"`
}

type Service struct {
	pool            *pgxpool.Pool
	payoutTransfers PayoutTransferExecutor
	notif           *notifications.Service
	accounts        adminUserIDs
}

type adminUserIDs interface {
	AdminUserIDs(ctx context.Context, q database.Querier, accountID int64) ([]int64, error)
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (s *Service) SetPayoutExecutor(e PayoutTransferExecutor) { s.payoutTransfers = e }

func (s *Service) SetNotifier(n *notifications.Service, accounts adminUserIDs) {
	s.notif = n
	s.accounts = accounts
}

func (s *Service) Pool() *pgxpool.Pool { return s.pool }

const contractCols = `id, public_id, handler_id, publisher_id, buyer_id, name,
	COALESCE(description, '') AS description, COALESCE(lead_type, '') AS lead_type,
	source_pipeline_id, source_stage_id, buyer_pipeline_id, return_stage_id,
	rate_per_lead::float8, status, cap_period, cap_total, cap_max_daily, created_at,
	contract_type, mirror_contract_id`

const contractLeadCountSubquery = `(SELECT COUNT(DISTINCT t.lead_id) FROM transactions t
 WHERE t.contract_id = c.id AND t.type = 'debit' AND t.lead_id IS NOT NULL
   AND (t.description LIKE 'lead routed:%'
        OR t.description = 'lead routed from intake queue'
        OR t.description = 'lead re-distributed'))`

func scanContract(row pgx.Row) (*Contract, error) {
	c := &Contract{}
	err := row.Scan(&c.ID, &c.PublicID, &c.HandlerID, &c.PublisherID, &c.BuyerID, &c.Name,
		&c.Description, &c.LeadType,
		&c.SourcePipelineID, &c.SourceStageID, &c.BuyerPipelineID, &c.ReturnStageID,
		&c.RatePerLead, &c.Status, &c.CapPeriod, &c.CapTotal, &c.CapMaxDaily, &c.CreatedAt,
		&c.ContractType, &c.MirrorContractID)
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
		`SELECT c.id, c.public_id, c.handler_id, c.publisher_id, c.buyer_id, c.name,
		        COALESCE(c.description, ''), COALESCE(c.lead_type, ''),
		        c.source_pipeline_id, c.source_stage_id, c.buyer_pipeline_id, c.return_stage_id,
		        c.rate_per_lead::float8, c.status, c.cap_period, c.cap_total, c.cap_max_daily,
		        c.created_at, c.contract_type, c.mirror_contract_id,
		        COALESCE(a.name, ''), COALESCE(a.type::text, ''), `+contractLeadCountSubquery+`
		 FROM contracts c LEFT JOIN accounts a ON a.id = c.buyer_id
		 WHERE c.publisher_id = $1 AND c.deleted_at IS NULL
		 ORDER BY c.created_at DESC`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Contract
	for rows.Next() {
		var c Contract
		if err := rows.Scan(&c.ID, &c.PublicID, &c.HandlerID, &c.PublisherID, &c.BuyerID, &c.Name,
			&c.Description, &c.LeadType,
			&c.SourcePipelineID, &c.SourceStageID, &c.BuyerPipelineID, &c.ReturnStageID,
			&c.RatePerLead, &c.Status, &c.CapPeriod, &c.CapTotal, &c.CapMaxDaily,
			&c.CreatedAt, &c.ContractType, &c.MirrorContractID, &c.BuyerName, &c.BuyerAccountType, &c.LeadCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Service) Get(ctx context.Context, publisherID, id int64) (*Contract, error) {
	c, err := scanContract(s.pool.QueryRow(ctx,
		`SELECT `+contractCols+` FROM contracts WHERE id = $1 AND publisher_id = $2 AND deleted_at IS NULL`,
		id, publisherID))
	if err != nil {
		return nil, err
	}
	if err := s.enrichOffer(ctx, c); err != nil {
		return nil, err
	}
	parts, err := s.listParticipationsByContract(ctx, id)
	if err != nil {
		return nil, err
	}
	c.Participations = parts
	return c, nil
}

func (s *Service) ListForBuyer(ctx context.Context, buyerID int64) ([]Contract, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT c.id, c.public_id, c.handler_id, c.publisher_id, c.buyer_id, c.name,
		        COALESCE(c.description, ''), COALESCE(c.lead_type, ''),
		        c.source_pipeline_id, c.source_stage_id, c.buyer_pipeline_id, c.return_stage_id,
		        c.rate_per_lead::float8, c.status, c.cap_period, c.cap_total, c.cap_max_daily,
		        c.created_at, c.contract_type, c.mirror_contract_id, a.name, `+contractLeadCountSubquery+`
		 FROM contracts c JOIN accounts a ON a.id = c.publisher_id
		 WHERE c.buyer_id = $1 AND c.deleted_at IS NULL AND c.contract_type = 'sell'
		 ORDER BY c.created_at DESC`, buyerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Contract
	for rows.Next() {
		var c Contract
		if err := rows.Scan(&c.ID, &c.PublicID, &c.HandlerID, &c.PublisherID, &c.BuyerID, &c.Name,
			&c.Description, &c.LeadType,
			&c.SourcePipelineID, &c.SourceStageID, &c.BuyerPipelineID, &c.ReturnStageID,
			&c.RatePerLead, &c.Status, &c.CapPeriod, &c.CapTotal, &c.CapMaxDaily,
			&c.CreatedAt, &c.ContractType, &c.MirrorContractID, &c.PublisherName, &c.LeadCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Service) GetForBuyerContract(ctx context.Context, buyerID, contractID int64) (*Contract, error) {
	return scanContract(s.pool.QueryRow(ctx,
		`SELECT `+contractCols+` FROM contracts WHERE id = $1 AND buyer_id = $2 AND deleted_at IS NULL AND contract_type = 'sell'`,
		contractID, buyerID))
}

type CreateParams struct {
	BuyerID          int64
	ContractType     string
	Name             string
	Description      string
	LeadType         string
	CapPeriod        string
	CapTotal         *int
	CapMaxDaily      *int
	SourcePipelineID int64
	SourceStageID    int64
	BuyerPipelineID  int64
	ReturnStageID    int64
	RatePerLead      float64
	Delivery         string
	Compensations          []CompensationParams
	LeadCriteria           *LeadCriteria
	AllowedDeliveryModes   []string
	DistributionStrategy   string
}

func (s *Service) Create(ctx context.Context, publisherID int64, p CreateParams) (*Contract, error) {
	if p.ContractType == "" {
		p.ContractType = "sell"
	}
	return s.createWithMirror(ctx, publisherID, p)
}

func (s *Service) LookupBuyerIDByHandler(ctx context.Context, handlerID string) (int64, error) {
	return s.LookupAccountIDByHandler(ctx, handlerID, "buyer")
}

func (s *Service) CounterpartyAccountType(ctx context.Context, accountID int64) (string, error) {
	return counterpartyType(ctx, s.pool, accountID)
}

func (s *Service) LookupAccountIDByHandler(ctx context.Context, handlerID string, wantType string) (int64, error) {
	var id int64
	var typ string
	err := s.pool.QueryRow(ctx, `SELECT id, type FROM accounts WHERE handler_id = $1`, handlerID).Scan(&id, &typ)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, httpx.NotFound("account not found")
	}
	if err != nil {
		return 0, err
	}
	if wantType != "" && typ != wantType {
		return 0, httpx.NotFound("account not found")
	}
	return id, nil
}

type UpdateParams struct {
	Name          *string
	RatePerLead   *float64
	Status        *string
	Description   *string
	LeadType      *string
	ContractType  *string
	CapPeriod     *string
	CapTotal      *int
	CapMaxDaily   *int
	PatchCap      bool
}

type DeliveryUpdateParams struct {
	Delivery         string
	SourcePipelineID int64
	SourceStageID    int64
	BuyerPipelineID  int64
	ReturnStageID    int64
}

func (s *Service) UpdateDelivery(ctx context.Context, publisherID, contractID int64, p DeliveryUpdateParams) (*Contract, error) {
	if _, err := s.Get(ctx, publisherID, contractID); err != nil {
		return nil, err
	}
	delivery := strings.TrimSpace(p.Delivery)
	if delivery == "" {
		delivery = "leads_pipeline"
	}
	if !allowedCompDelivery[delivery] {
		return nil, httpx.Validation("delivery must be leads or leads_pipeline")
	}
	sourcePipelineID := p.SourcePipelineID
	sourceStageID := p.SourceStageID
	buyerPipelineID := p.BuyerPipelineID
	returnStageID := p.ReturnStageID
	if delivery == "leads" {
		sourcePipelineID = 0
		sourceStageID = 0
		buyerPipelineID = 0
		returnStageID = 0
	} else if sourceStageID == 0 || buyerPipelineID == 0 || returnStageID == 0 {
		return nil, httpx.Validation("source_stage_id, buyer_pipeline_id, and return_stage_id are required for pipeline delivery")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	c, err := scanContract(tx.QueryRow(ctx,
		`UPDATE contracts SET
		   source_pipeline_id = $3,
		   source_stage_id = $4,
		   buyer_pipeline_id = $5,
		   return_stage_id = $6
		 WHERE id = $1 AND publisher_id = $2 AND deleted_at IS NULL
		 RETURNING `+contractCols,
		contractID, publisherID,
		nullableID(sourcePipelineID), nullableID(sourceStageID),
		nullableID(buyerPipelineID), nullableID(returnStageID)))
	if err != nil {
		return nil, err
	}

	var compSourcePipeline, compSourceStage, compBuyerPipeline, compReturnStage any
	if delivery == "leads" {
		compSourcePipeline = nil
		compSourceStage = nil
		compBuyerPipeline = nil
		compReturnStage = nil
	} else {
		compSourcePipeline = sourcePipelineID
		compSourceStage = sourceStageID
		compBuyerPipeline = buyerPipelineID
		compReturnStage = returnStageID
	}
	if _, err := tx.Exec(ctx,
		`UPDATE contract_compensations SET
		   source_pipeline_id = $2,
		   source_stage_id = $3,
		   counterparty_pipeline_id = $4,
		   return_stage_id = $5,
		   delivery = $6
		 WHERE contract_id = $1`,
		contractID, compSourcePipeline, compSourceStage, compBuyerPipeline, compReturnStage, delivery); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) Update(ctx context.Context, publisherID, id int64, p UpdateParams) (*Contract, error) {
	if p.PatchCap {
		return scanContract(s.pool.QueryRow(ctx,
			`UPDATE contracts SET
			   name = COALESCE($3, name),
			   rate_per_lead = COALESCE($4, rate_per_lead),
			   status = COALESCE($5, status),
			   description = COALESCE($6, description),
			   lead_type = COALESCE($7, lead_type),
			   cap_period = COALESCE($8, cap_period),
			   cap_total = $9,
			   cap_max_daily = $10
			 WHERE id = $1 AND publisher_id = $2 AND deleted_at IS NULL
			 RETURNING `+contractCols,
			id, publisherID, p.Name, p.RatePerLead, p.Status, p.Description, p.LeadType,
			p.CapPeriod, p.CapTotal, p.CapMaxDaily))
	}
	return scanContract(s.pool.QueryRow(ctx,
		`UPDATE contracts SET
		   name = COALESCE($3, name),
		   rate_per_lead = COALESCE($4, rate_per_lead),
		   status = COALESCE($5, status),
		   description = COALESCE($6, description),
		   lead_type = COALESCE($7, lead_type)
		 WHERE id = $1 AND publisher_id = $2 AND deleted_at IS NULL
		 RETURNING `+contractCols,
		id, publisherID, p.Name, p.RatePerLead, p.Status, p.Description, p.LeadType))
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

func (s *Service) PublisherReturnStages(ctx context.Context, contractID int64) ([]PublisherStage, error) {
	var sourcePipelineID int64
	err := s.pool.QueryRow(ctx,
		`SELECT source_pipeline_id FROM contracts WHERE id = $1 AND deleted_at IS NULL`, contractID).Scan(&sourcePipelineID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("contract not found")
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, public_id, pipeline_id, name, position, color, stage_type
		 FROM pipeline_stages WHERE pipeline_id = $1 ORDER BY position, id`, sourcePipelineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PublisherStage
	for rows.Next() {
		var st PublisherStage
		if err := rows.Scan(&st.ID, &st.PublicID, &st.PipelineID, &st.Name, &st.Position, &st.Color, &st.StageType); err != nil {
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

// ReturnRuleBelongsToBuyer checks whether a return rule belongs to one of the buyer's contracts.
func (s *Service) ReturnRuleBelongsToBuyer(ctx context.Context, buyerID, ruleID int64) (int64, error) {
	var contractID int64
	err := s.pool.QueryRow(ctx,
		`SELECT rr.contract_id FROM contract_return_rules rr
		 JOIN contracts c ON c.id = rr.contract_id
		 WHERE rr.id = $1 AND c.buyer_id = $2 AND c.deleted_at IS NULL`,
		ruleID, buyerID).Scan(&contractID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, httpx.NotFound("return rule not found")
	}
	return contractID, err
}

// ── Helpers used inside intake / leads transactions (accept Querier) ──

// FindByBuyerPipeline resolves the contract whose buyer pipeline matches.
func FindByBuyerPipeline(ctx context.Context, q database.Querier, buyerPipelineID int64) (*Target, error) {
	var contractID int64
	err := q.QueryRow(ctx,
		`SELECT id FROM contracts WHERE buyer_pipeline_id = $1 AND status = 'active' AND deleted_at IS NULL
		 LIMIT 1`, buyerPipelineID).Scan(&contractID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return GetTargetByContract(ctx, q, contractID)
}

// FindByBuyer resolves the buyer's active contract target.
func FindByBuyer(ctx context.Context, q database.Querier, buyerID int64) (*Target, error) {
	var contractID int64
	err := q.QueryRow(ctx,
		`SELECT id FROM contracts WHERE buyer_id = $1 AND status = 'active' AND deleted_at IS NULL
		 LIMIT 1`, buyerID).Scan(&contractID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return GetTargetByContract(ctx, q, contractID)
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
