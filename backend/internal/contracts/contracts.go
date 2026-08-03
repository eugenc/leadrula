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
	PublisherID      int64     `json:"publisher_id,omitempty"`
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
	LeadCount                 int       `json:"lead_count"`
	Delivery                  string    `json:"delivery,omitempty"`
	BuyerTargetStageID        *int64    `json:"buyer_target_stage_id,omitempty"`
	IntegrationConnectionID   *int64    `json:"integration_connection_id,omitempty"`
	OutboundWebhookID         *int64    `json:"outbound_webhook_id,omitempty"`
	AllowedDeliveryModes      []string  `json:"allowed_delivery_modes,omitempty"`
	DistributionStrategy      string    `json:"distribution_strategy,omitempty"`
	ParentContractID       *int64    `json:"parent_contract_id,omitempty"`
	InviteToken            string    `json:"invite_token,omitempty"`
	AppointmentCalendarID  *int64    `json:"appointment_calendar_id,omitempty"`
	Participations         []Participation `json:"participations,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
}

type ReturnRule struct {
	ID              int64     `json:"id"`
	ContractID      int64     `json:"contract_id"`
	ParticipationID *int64    `json:"participation_id,omitempty"`
	BuyerStageID    int64     `json:"buyer_stage_id"`
	ReturnStageID   *int64    `json:"return_stage_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// ParticipationReturnRule is a buyer participation return route exposed to the publisher.
type ParticipationReturnRule struct {
	ReturnRule
	BuyerName      string `json:"buyer_name"`
	BuyerStageName string `json:"buyer_stage_name"`
}

// PublisherReturnRule is a contract-level return route exposed to the publisher.
type PublisherReturnRule struct {
	ReturnRule
	BuyerStageName string `json:"buyer_stage_name"`
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

type routeSyncer interface {
	SyncContractDistributionRoute(ctx context.Context, publisherID, contractID int64) error
}

type Service struct {
	pool                     *pgxpool.Pool
	payoutTransfers          PayoutTransferExecutor
	compensationPayoutInvoicer CompensationPayoutInvoicer
	notif                    *notifications.Service
	accounts                 adminUserIDs
	routes                   routeSyncer
}

type adminUserIDs interface {
	AdminUserIDs(ctx context.Context, q database.Querier, accountID int64) ([]int64, error)
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (s *Service) SetPayoutExecutor(e PayoutTransferExecutor) { s.payoutTransfers = e }

func (s *Service) SetCompensationPayoutInvoicer(i CompensationPayoutInvoicer) {
	s.compensationPayoutInvoicer = i
}

func (s *Service) SetNotifier(n *notifications.Service, accounts adminUserIDs) {
	s.notif = n
	s.accounts = accounts
}

func (s *Service) SetRouteSyncer(r routeSyncer) { s.routes = r }

func (s *Service) syncContractRoute(ctx context.Context, publisherID, contractID int64) {
	if s.routes == nil {
		return
	}
	_ = s.routes.SyncContractDistributionRoute(ctx, publisherID, contractID)
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

const buyerContractCompLateral = `LEFT JOIN LATERAL (
	SELECT delivery, counterparty_stage_id
	FROM contract_compensations
	WHERE contract_id = c.id AND participation_id IS NULL
	ORDER BY position, id LIMIT 1
) cc ON true`

func scanBuyerContract(row pgx.Row, withPublisher bool) (*Contract, error) {
	c := &Contract{}
	var delivery string
	scan := []any{
		&c.ID, &c.PublicID, &c.HandlerID, &c.PublisherID, &c.BuyerID, &c.Name,
		&c.Description, &c.LeadType,
		&c.SourcePipelineID, &c.SourceStageID, &c.BuyerPipelineID, &c.ReturnStageID,
		&c.RatePerLead, &c.Status, &c.CapPeriod, &c.CapTotal, &c.CapMaxDaily,
		&c.CreatedAt, &c.ContractType, &c.MirrorContractID, &c.AllowedDeliveryModes,
		&delivery, &c.BuyerTargetStageID, &c.AppointmentCalendarID,
	}
	if withPublisher {
		scan = append(scan, &c.PublisherName, &c.LeadCount)
	}
	if err := row.Scan(scan...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("contract not found")
		}
		return nil, err
	}
	c.Delivery = delivery
	return c, nil
}

func (s *Service) ListForBuyer(ctx context.Context, buyerID int64) ([]Contract, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT c.id, c.public_id, c.handler_id, c.publisher_id, c.buyer_id, c.name,
		        COALESCE(c.description, ''), COALESCE(c.lead_type, ''),
		        c.source_pipeline_id, c.source_stage_id, c.buyer_pipeline_id, c.return_stage_id,
		        c.rate_per_lead::float8, c.status, c.cap_period, c.cap_total, c.cap_max_daily,
		        c.created_at, c.contract_type, c.mirror_contract_id, c.allowed_delivery_modes,
		        COALESCE(cc.delivery, ''), cc.counterparty_stage_id, c.appointment_calendar_id,
		        a.name, `+contractLeadCountSubquery+`
		 FROM contracts c
		 JOIN accounts a ON a.id = c.publisher_id
		 `+buyerContractCompLateral+`
		 WHERE c.buyer_id = $1 AND c.deleted_at IS NULL AND c.contract_type = 'sell'
		 ORDER BY c.created_at DESC`, buyerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Contract
	for rows.Next() {
		c, err := scanBuyerContract(rows, true)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (s *Service) GetForBuyerContract(ctx context.Context, buyerID, contractID int64) (*Contract, error) {
	return scanBuyerContract(s.pool.QueryRow(ctx,
		`SELECT c.id, c.public_id, c.handler_id, c.publisher_id, c.buyer_id, c.name,
		        COALESCE(c.description, ''), COALESCE(c.lead_type, ''),
		        c.source_pipeline_id, c.source_stage_id, c.buyer_pipeline_id, c.return_stage_id,
		        c.rate_per_lead::float8, c.status, c.cap_period, c.cap_total, c.cap_max_daily,
		        c.created_at, c.contract_type, c.mirror_contract_id, c.allowed_delivery_modes,
		        COALESCE(cc.delivery, ''), cc.counterparty_stage_id, c.appointment_calendar_id
		 FROM contracts c
		 `+buyerContractCompLateral+`
		 WHERE c.id = $1 AND c.buyer_id = $2 AND c.deleted_at IS NULL AND c.contract_type = 'sell'`,
		contractID, buyerID), false)
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
	c, err := s.Get(ctx, publisherID, contractID)
	if err != nil {
		return nil, err
	}
	openOffer := c.BuyerID == nil
	delivery := strings.TrimSpace(p.Delivery)
	if delivery == "" {
		delivery = "leads_pipeline"
	}
	if !allowedCompDelivery[delivery] {
		return nil, httpx.Validation("delivery must be leads or leads_pipeline")
	}
	sourcePipelineID := p.SourcePipelineID
	sourceStageID := p.SourceStageID
	returnStageID := p.ReturnStageID
	if delivery == "leads" {
		sourcePipelineID = 0
		sourceStageID = 0
		returnStageID = 0
	} else if openOffer {
		if sourcePipelineID == 0 || sourceStageID == 0 {
			return nil, httpx.Validation("source_pipeline_id and source_stage_id are required for pipeline delivery")
		}
		returnStageID = 0
	} else if sourceStageID == 0 {
		return nil, httpx.Validation("source_stage_id is required for pipeline delivery")
	}
	returnStageID = 0

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	c, err = scanContract(tx.QueryRow(ctx,
		`UPDATE contracts SET
		   source_pipeline_id = $3,
		   source_stage_id = $4,
		   return_stage_id = $5
		 WHERE id = $1 AND publisher_id = $2 AND deleted_at IS NULL
		 RETURNING `+contractCols,
		contractID, publisherID,
		nullableID(sourcePipelineID), nullableID(sourceStageID), nullableID(returnStageID)))
	if err != nil {
		return nil, err
	}

	var compSourcePipeline, compSourceStage, compReturnStage any
	if delivery == "leads" {
		compSourcePipeline = nil
		compSourceStage = nil
		compReturnStage = nil
	} else {
		compSourcePipeline = sourcePipelineID
		compSourceStage = sourceStageID
		compReturnStage = nullableID(returnStageID)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE contract_compensations SET
		   source_pipeline_id = $2,
		   source_stage_id = $3,
		   return_stage_id = $4,
		   delivery = $5
		 WHERE contract_id = $1`,
		contractID, compSourcePipeline, compSourceStage, compReturnStage, delivery); err != nil {
		return nil, err
	}

	buyerPipelineID := int64(0)
	if c.BuyerPipelineID != nil {
		buyerPipelineID = *c.BuyerPipelineID
	}
	if delivery == "leads_pipeline" && buyerPipelineID != 0 {
		if err := RebuildContractStageMaps(ctx, tx, contractID); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.Exec(ctx, `DELETE FROM contract_stage_maps WHERE contract_id = $1 AND participation_id IS NULL`, contractID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.syncContractRoute(ctx, publisherID, contractID)
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

// ── Return routes ─────────────────────────────────────────────────

func (s *Service) ListReturnRules(ctx context.Context, contractID int64) ([]ReturnRule, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, contract_id, participation_id, buyer_stage_id, return_stage_id, created_at
		 FROM contract_return_rules
		 WHERE contract_id = $1 AND participation_id IS NULL
		 ORDER BY id`, contractID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReturnRules(rows)
}

func (s *Service) ListReturnRulesForPublisher(ctx context.Context, publisherID, contractID int64) ([]PublisherReturnRule, error) {
	if _, err := s.Get(ctx, publisherID, contractID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT rr.id, rr.contract_id, rr.participation_id, rr.buyer_stage_id, rr.return_stage_id, rr.created_at,
		        COALESCE(bs.name, '')
		 FROM contract_return_rules rr
		 JOIN contracts c ON c.id = rr.contract_id
		 LEFT JOIN pipeline_stages bs ON bs.id = rr.buyer_stage_id
		 WHERE rr.contract_id = $1 AND rr.participation_id IS NULL AND c.publisher_id = $2
		 ORDER BY rr.id`, contractID, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PublisherReturnRule
	for rows.Next() {
		var rr PublisherReturnRule
		if err := rows.Scan(
			&rr.ID, &rr.ContractID, &rr.ParticipationID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt,
			&rr.BuyerStageName,
		); err != nil {
			return nil, err
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

func (s *Service) ListParticipationReturnRules(ctx context.Context, participationID int64) ([]ReturnRule, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, contract_id, participation_id, buyer_stage_id, return_stage_id, created_at
		 FROM contract_return_rules
		 WHERE participation_id = $1
		 ORDER BY id`, participationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReturnRules(rows)
}

func (s *Service) ListContractParticipationReturnRules(ctx context.Context, publisherID, contractID int64) ([]ParticipationReturnRule, error) {
	if _, err := s.Get(ctx, publisherID, contractID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT rr.id, rr.contract_id, rr.participation_id, rr.buyer_stage_id, rr.return_stage_id, rr.created_at,
		        COALESCE(a.name, ''), COALESCE(bs.name, '')
		 FROM contract_return_rules rr
		 JOIN contract_participations p ON p.id = rr.participation_id
		 JOIN contracts c ON c.id = rr.contract_id
		 LEFT JOIN accounts a ON a.id = p.buyer_id
		 LEFT JOIN pipeline_stages bs ON bs.id = rr.buyer_stage_id
		 WHERE rr.contract_id = $1 AND rr.participation_id IS NOT NULL AND c.publisher_id = $2
		 ORDER BY a.name, rr.id`, contractID, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ParticipationReturnRule
	for rows.Next() {
		var rr ParticipationReturnRule
		if err := rows.Scan(
			&rr.ID, &rr.ContractID, &rr.ParticipationID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt,
			&rr.BuyerName, &rr.BuyerStageName,
		); err != nil {
			return nil, err
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

func scanReturnRules(rows pgx.Rows) ([]ReturnRule, error) {
	var out []ReturnRule
	for rows.Next() {
		var rr ReturnRule
		if err := rows.Scan(&rr.ID, &rr.ContractID, &rr.ParticipationID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

func (s *Service) CountParticipationReturnRules(ctx context.Context, participationID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM contract_return_rules WHERE participation_id = $1`, participationID).Scan(&n)
	return n, err
}

func countContractReturnRules(ctx context.Context, q database.Querier, contractID int64) (int, error) {
	var n int
	err := q.QueryRow(ctx,
		`SELECT COUNT(*) FROM contract_return_rules WHERE contract_id = $1 AND participation_id IS NULL`,
		contractID).Scan(&n)
	return n, err
}

func (s *Service) CountContractReturnRules(ctx context.Context, contractID int64) (int, error) {
	return countContractReturnRules(ctx, s.pool, contractID)
}

func validateContractReturnRoutesRequired(ctx context.Context, q database.Querier, contractID int64, buyerPipelineID int64, pipelineDelivery bool) error {
	if !pipelineDelivery || buyerPipelineID == 0 {
		return nil
	}
	n, err := countContractReturnRules(ctx, q, contractID)
	if err != nil {
		return err
	}
	if n == 0 {
		return httpx.Validation("at least one return route is required for pipeline delivery")
	}
	return nil
}

func contractRequiresReturnRoutes(ctx context.Context, q database.Querier, contractID int64) (bool, error) {
	var buyerPipelineID *int64
	err := q.QueryRow(ctx,
		`SELECT buyer_pipeline_id FROM contracts WHERE id = $1 AND deleted_at IS NULL`,
		contractID).Scan(&buyerPipelineID)
	if err != nil {
		return false, err
	}
	if buyerPipelineID == nil || *buyerPipelineID == 0 {
		return false, nil
	}
	var delivery string
	err = q.QueryRow(ctx,
		`SELECT COALESCE(
		   (SELECT delivery FROM contract_compensations WHERE contract_id = $1 ORDER BY position, id LIMIT 1),
		   'leads_pipeline'
		 )`, contractID).Scan(&delivery)
	if err != nil {
		return false, err
	}
	return delivery == "leads_pipeline", nil
}

func (s *Service) validateReturnRuleStages(ctx context.Context, contractID, buyerStageID, returnStageID int64) error {
	var buyerPipelineID, sourcePipelineID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT buyer_pipeline_id, source_pipeline_id FROM contracts WHERE id = $1 AND deleted_at IS NULL`,
		contractID).Scan(&buyerPipelineID, &sourcePipelineID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("contract not found")
	}
	if err != nil {
		return err
	}
	if buyerPipelineID == nil || *buyerPipelineID == 0 {
		return httpx.Validation("contract buyer pipeline is not configured")
	}
	var ok bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pipeline_stages WHERE id = $1 AND pipeline_id = $2)`,
		buyerStageID, *buyerPipelineID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return httpx.Validation("buyer stage not in contract buyer pipeline")
	}
	if sourcePipelineID == nil || *sourcePipelineID == 0 {
		return httpx.Validation("contract publisher pipeline is not configured")
	}
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pipeline_stages WHERE id = $1 AND pipeline_id = $2)`,
		returnStageID, *sourcePipelineID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return httpx.Validation("return stage not in contract publisher pipeline")
	}
	return nil
}

func (s *Service) validateParticipationReturnRuleBuyerStage(ctx context.Context, participationID, buyerStageID, buyerPipelineOverride int64) (int64, error) {
	var contractID int64
	var storedBuyerPipelineID *int64
	var sourcePipelineID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT p.contract_id, p.buyer_pipeline_id, COALESCE(p.source_pipeline_id, c.source_pipeline_id)
		 FROM contract_participations p
		 JOIN contracts c ON c.id = p.contract_id
		 WHERE p.id = $1`, participationID).Scan(&contractID, &storedBuyerPipelineID, &sourcePipelineID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, httpx.NotFound("participation not found")
	}
	if err != nil {
		return 0, err
	}
	buyerPipelineID := int64(0)
	if storedBuyerPipelineID != nil {
		buyerPipelineID = *storedBuyerPipelineID
	}
	if buyerPipelineID == 0 && buyerPipelineOverride > 0 {
		buyerPipelineID = buyerPipelineOverride
	}
	if buyerPipelineID == 0 {
		return 0, httpx.Validation("buyer pipeline is not configured on participation")
	}
	if sourcePipelineID == nil || *sourcePipelineID == 0 {
		return 0, httpx.Validation("publisher pipeline is not configured on contract")
	}
	var ok bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pipeline_stages WHERE id = $1 AND pipeline_id = $2)`,
		buyerStageID, buyerPipelineID).Scan(&ok); err != nil {
		return 0, err
	}
	if !ok {
		return 0, httpx.Validation("buyer stage not in participation buyer pipeline")
	}
	return contractID, nil
}

func (s *Service) validateReturnRuleBuyerStage(ctx context.Context, contractID, buyerStageID, buyerPipelineOverride int64) error {
	var storedBuyerPipelineID, sourcePipelineID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT buyer_pipeline_id, source_pipeline_id FROM contracts WHERE id = $1 AND deleted_at IS NULL`,
		contractID).Scan(&storedBuyerPipelineID, &sourcePipelineID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("contract not found")
	}
	if err != nil {
		return err
	}
	buyerPipelineID := int64(0)
	if storedBuyerPipelineID != nil {
		buyerPipelineID = *storedBuyerPipelineID
	}
	if buyerPipelineID == 0 && buyerPipelineOverride > 0 {
		buyerPipelineID = buyerPipelineOverride
	}
	if buyerPipelineID == 0 {
		return httpx.Validation("buyer pipeline is not configured on contract")
	}
	if sourcePipelineID == nil || *sourcePipelineID == 0 {
		return httpx.Validation("publisher pipeline is not configured on contract")
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
	return nil
}

func (s *Service) AddReturnRule(ctx context.Context, contractID, buyerStageID, returnStageID int64) (*ReturnRule, error) {
	if err := s.validateReturnRuleStages(ctx, contractID, buyerStageID, returnStageID); err != nil {
		return nil, err
	}
	rr := &ReturnRule{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO contract_return_rules(contract_id, buyer_stage_id, return_stage_id) VALUES ($1,$2,$3)
		 ON CONFLICT (contract_id, buyer_stage_id) WHERE participation_id IS NULL
		 DO UPDATE SET return_stage_id = EXCLUDED.return_stage_id
		 RETURNING id, contract_id, participation_id, buyer_stage_id, return_stage_id, created_at`,
		contractID, buyerStageID, returnStageID).Scan(
		&rr.ID, &rr.ContractID, &rr.ParticipationID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("return route already exists for this buyer stage")
		}
		return nil, err
	}
	return rr, nil
}

func (s *Service) AddParticipationReturnRule(ctx context.Context, participationID, buyerStageID, buyerPipelineOverride int64) (*ReturnRule, error) {
	var status string
	if err := s.pool.QueryRow(ctx,
		`SELECT status::text FROM contract_participations WHERE id = $1`, participationID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("participation not found")
		}
		return nil, err
	}
	if !participationMutable(status) {
		return nil, httpx.Validation("participation cannot be edited")
	}
	contractID, err := s.validateParticipationReturnRuleBuyerStage(ctx, participationID, buyerStageID, buyerPipelineOverride)
	if err != nil {
		return nil, err
	}
	rr := &ReturnRule{}
	err = s.pool.QueryRow(ctx,
		`INSERT INTO contract_return_rules(contract_id, participation_id, buyer_stage_id, return_stage_id)
		 VALUES ($1,$2,$3,NULL)
		 ON CONFLICT (participation_id, buyer_stage_id) WHERE participation_id IS NOT NULL
		 DO UPDATE SET buyer_stage_id = EXCLUDED.buyer_stage_id
		 RETURNING id, contract_id, participation_id, buyer_stage_id, return_stage_id, created_at`,
		contractID, participationID, buyerStageID).Scan(
		&rr.ID, &rr.ContractID, &rr.ParticipationID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("return route already exists for this buyer stage")
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
		 RETURNING id, contract_id, participation_id, buyer_stage_id, return_stage_id, created_at`,
		ruleID, buyerStageID, returnStageID).Scan(
		&rr.ID, &rr.ContractID, &rr.ParticipationID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("return route already exists for this buyer stage")
		}
		return nil, err
	}
	return rr, nil
}

func (s *Service) validateParticipationReturnRuleDestination(ctx context.Context, publisherID, ruleID, returnStageID int64) (int64, int64, error) {
	var contractID int64
	var buyerStageID int64
	var participationID *int64
	var sourcePipelineID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT rr.contract_id, rr.buyer_stage_id, rr.participation_id, c.source_pipeline_id
		 FROM contract_return_rules rr
		 JOIN contracts c ON c.id = rr.contract_id
		 WHERE rr.id = $1 AND c.publisher_id = $2 AND c.deleted_at IS NULL`,
		ruleID, publisherID).Scan(&contractID, &buyerStageID, &participationID, &sourcePipelineID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, httpx.NotFound("return route not found")
	}
	if err != nil {
		return 0, 0, err
	}
	if participationID == nil {
		return 0, 0, httpx.NotFound("return route not found")
	}
	if sourcePipelineID == nil || *sourcePipelineID == 0 {
		return 0, 0, httpx.Validation("publisher pipeline is not configured on contract")
	}
	var ok bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pipeline_stages WHERE id = $1 AND pipeline_id = $2)`,
		returnStageID, *sourcePipelineID).Scan(&ok); err != nil {
		return 0, 0, err
	}
	if !ok {
		return 0, 0, httpx.Validation("return stage not in contract publisher pipeline")
	}
	return contractID, buyerStageID, nil
}

func (s *Service) UpdateParticipationReturnRuleDestination(ctx context.Context, publisherID, ruleID, returnStageID int64) (*ReturnRule, error) {
	contractID, buyerStageID, err := s.validateParticipationReturnRuleDestination(ctx, publisherID, ruleID, returnStageID)
	if err != nil {
		return nil, err
	}
	rr := &ReturnRule{}
	err = s.pool.QueryRow(ctx,
		`UPDATE contract_return_rules SET return_stage_id = $2
		 WHERE id = $1 AND participation_id IS NOT NULL
		 RETURNING id, contract_id, participation_id, buyer_stage_id, return_stage_id, created_at`,
		ruleID, returnStageID).Scan(
		&rr.ID, &rr.ContractID, &rr.ParticipationID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("return route not found")
	}
	if err != nil {
		return nil, err
	}
	if rr.ContractID != contractID || rr.BuyerStageID != buyerStageID {
		return nil, httpx.NotFound("return route not found")
	}
	return rr, nil
}

func (s *Service) validateContractReturnRuleDestination(ctx context.Context, publisherID, ruleID, returnStageID int64) (int64, int64, error) {
	var contractID int64
	var buyerStageID int64
	var participationID *int64
	var sourcePipelineID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT rr.contract_id, rr.buyer_stage_id, rr.participation_id, c.source_pipeline_id
		 FROM contract_return_rules rr
		 JOIN contracts c ON c.id = rr.contract_id
		 WHERE rr.id = $1 AND c.publisher_id = $2 AND c.deleted_at IS NULL`,
		ruleID, publisherID).Scan(&contractID, &buyerStageID, &participationID, &sourcePipelineID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, httpx.NotFound("return route not found")
	}
	if err != nil {
		return 0, 0, err
	}
	if participationID != nil {
		return 0, 0, httpx.NotFound("return route not found")
	}
	if sourcePipelineID == nil || *sourcePipelineID == 0 {
		return 0, 0, httpx.Validation("publisher pipeline is not configured on contract")
	}
	var ok bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pipeline_stages WHERE id = $1 AND pipeline_id = $2)`,
		returnStageID, *sourcePipelineID).Scan(&ok); err != nil {
		return 0, 0, err
	}
	if !ok {
		return 0, 0, httpx.Validation("return stage not in contract publisher pipeline")
	}
	return contractID, buyerStageID, nil
}

func (s *Service) UpdateContractReturnRuleDestination(ctx context.Context, publisherID, ruleID, returnStageID int64) (*ReturnRule, error) {
	contractID, buyerStageID, err := s.validateContractReturnRuleDestination(ctx, publisherID, ruleID, returnStageID)
	if err != nil {
		return nil, err
	}
	rr := &ReturnRule{}
	err = s.pool.QueryRow(ctx,
		`UPDATE contract_return_rules SET return_stage_id = $2
		 WHERE id = $1 AND participation_id IS NULL
		 RETURNING id, contract_id, participation_id, buyer_stage_id, return_stage_id, created_at`,
		ruleID, returnStageID).Scan(
		&rr.ID, &rr.ContractID, &rr.ParticipationID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("return route not found")
	}
	if err != nil {
		return nil, err
	}
	if rr.ContractID != contractID || rr.BuyerStageID != buyerStageID {
		return nil, httpx.NotFound("return route not found")
	}
	return rr, nil
}

func (s *Service) UpdateParticipationReturnRule(ctx context.Context, participationID, ruleID, buyerStageID, buyerPipelineOverride int64) (*ReturnRule, error) {
	var status string
	if err := s.pool.QueryRow(ctx,
		`SELECT status::text FROM contract_participations WHERE id = $1`, participationID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("participation not found")
		}
		return nil, err
	}
	if !participationMutable(status) {
		return nil, httpx.Validation("participation cannot be edited")
	}
	var ruleParticipationID int64
	err := s.pool.QueryRow(ctx,
		`SELECT participation_id FROM contract_return_rules WHERE id = $1`, ruleID).Scan(&ruleParticipationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("return route not found")
	}
	if err != nil {
		return nil, err
	}
	if ruleParticipationID != participationID {
		return nil, httpx.NotFound("return route not found")
	}
	contractID, err := s.validateParticipationReturnRuleBuyerStage(ctx, participationID, buyerStageID, buyerPipelineOverride)
	if err != nil {
		return nil, err
	}
	rr := &ReturnRule{}
	err = s.pool.QueryRow(ctx,
		`UPDATE contract_return_rules SET buyer_stage_id = $2, contract_id = $3
		 WHERE id = $1
		 RETURNING id, contract_id, participation_id, buyer_stage_id, return_stage_id, created_at`,
		ruleID, buyerStageID, contractID).Scan(
		&rr.ID, &rr.ContractID, &rr.ParticipationID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("return route already exists for this buyer stage")
		}
		return nil, err
	}
	return rr, nil
}

func (s *Service) DeleteReturnRule(ctx context.Context, ruleID int64) error {
	var contractID int64
	var participationID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT contract_id, participation_id FROM contract_return_rules WHERE id = $1`, ruleID).Scan(&contractID, &participationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("return rule not found")
	}
	if err != nil {
		return err
	}
	if participationID == nil {
		required, err := contractRequiresReturnRoutes(ctx, s.pool, contractID)
		if err != nil {
			return err
		}
		if required {
			n, err := countContractReturnRules(ctx, s.pool, contractID)
			if err != nil {
				return err
			}
			if n <= 1 {
				return httpx.Validation("cannot remove the last return route while pipeline delivery is configured")
			}
		}
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM contract_return_rules WHERE id = $1`, ruleID)
	return err
}

func (s *Service) AddBuyerContractReturnRule(ctx context.Context, buyerID, contractID, buyerStageID, buyerPipelineOverride int64) (*ReturnRule, error) {
	if _, err := s.GetForBuyerContract(ctx, buyerID, contractID); err != nil {
		return nil, err
	}
	if err := s.validateReturnRuleBuyerStage(ctx, contractID, buyerStageID, buyerPipelineOverride); err != nil {
		return nil, err
	}
	rr := &ReturnRule{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO contract_return_rules(contract_id, buyer_stage_id, return_stage_id) VALUES ($1,$2,NULL)
		 ON CONFLICT (contract_id, buyer_stage_id) WHERE participation_id IS NULL
		 DO UPDATE SET buyer_stage_id = EXCLUDED.buyer_stage_id
		 RETURNING id, contract_id, participation_id, buyer_stage_id, return_stage_id, created_at`,
		contractID, buyerStageID).Scan(
		&rr.ID, &rr.ContractID, &rr.ParticipationID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("return route already exists for this buyer stage")
		}
		return nil, err
	}
	return rr, nil
}

func (s *Service) UpdateBuyerContractReturnRule(ctx context.Context, buyerID, contractID, ruleID, buyerStageID, buyerPipelineOverride int64) (*ReturnRule, error) {
	if _, err := s.GetForBuyerContract(ctx, buyerID, contractID); err != nil {
		return nil, err
	}
	var ruleContractID int64
	var participationID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT contract_id, participation_id FROM contract_return_rules WHERE id = $1`, ruleID).Scan(&ruleContractID, &participationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("return route not found")
	}
	if err != nil {
		return nil, err
	}
	if ruleContractID != contractID || participationID != nil {
		return nil, httpx.NotFound("return route not found")
	}
	if err := s.validateReturnRuleBuyerStage(ctx, contractID, buyerStageID, buyerPipelineOverride); err != nil {
		return nil, err
	}
	rr := &ReturnRule{}
	err = s.pool.QueryRow(ctx,
		`UPDATE contract_return_rules SET buyer_stage_id = $2
		 WHERE id = $1 AND participation_id IS NULL
		 RETURNING id, contract_id, participation_id, buyer_stage_id, return_stage_id, created_at`,
		ruleID, buyerStageID).Scan(
		&rr.ID, &rr.ContractID, &rr.ParticipationID, &rr.BuyerStageID, &rr.ReturnStageID, &rr.CreatedAt)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("return route already exists for this buyer stage")
		}
		return nil, err
	}
	return rr, nil
}

func (s *Service) DeleteBuyerContractReturnRule(ctx context.Context, buyerID, contractID, ruleID int64) error {
	if _, err := s.GetForBuyerContract(ctx, buyerID, contractID); err != nil {
		return err
	}
	var ruleContractID int64
	var participationID *int64
	err := s.pool.QueryRow(ctx,
		`SELECT contract_id, participation_id FROM contract_return_rules WHERE id = $1`, ruleID).Scan(&ruleContractID, &participationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("return route not found")
	}
	if err != nil {
		return err
	}
	if ruleContractID != contractID || participationID != nil {
		return httpx.NotFound("return route not found")
	}
	return s.DeleteReturnRule(ctx, ruleID)
}

func (s *Service) validateBuyerContractDelivery(ctx context.Context, buyerID, contractID int64, p AcceptParticipationParams, requireReturnRoutes bool) (*validatedParticipationDelivery, error) {
	c, err := s.GetForBuyerContract(ctx, buyerID, contractID)
	if err != nil {
		return nil, err
	}
	if c.Status != "active" {
		return nil, httpx.BusinessRule("publisher contract is not active")
	}
	var allowed []string
	if len(c.AllowedDeliveryModes) > 0 {
		allowed = c.AllowedDeliveryModes
	} else {
		allowed = []string{"leads", "leads_pipeline"}
	}
	delivery := strings.TrimSpace(p.Delivery)
	if delivery == "" {
		return nil, httpx.Validation("delivery is required")
	}
	ok := false
	for _, m := range allowed {
		if m == delivery {
			ok = true
			break
		}
	}
	if !ok {
		return nil, httpx.Validation("delivery mode not allowed on this contract")
	}
	if !allowedPublisherDelivery[delivery] {
		return nil, httpx.Validation("invalid delivery mode")
	}
	if delivery == "webhook" {
		return nil, httpx.Validation("webhook delivery is not available on direct contracts")
	}
	if p.IntegrationConnectionID != 0 {
		return nil, httpx.Validation("crm integration is not available on direct contracts")
	}
	buyerPipelineID := p.BuyerPipelineID
	buyerStageID := p.BuyerTargetStageID
	if delivery == "leads_pipeline" {
		if buyerPipelineID == 0 {
			if c.BuyerPipelineID != nil {
				buyerPipelineID = *c.BuyerPipelineID
			}
		}
		if buyerPipelineID == 0 || buyerStageID == 0 {
			return nil, httpx.Validation("buyer_pipeline_id and buyer_target_stage_id are required for pipeline delivery")
		}
		if err := validateBuyerTargetStage(ctx, s.pool, buyerStageID, buyerPipelineID); err != nil {
			return nil, err
		}
		if requireReturnRoutes {
			n, err := countContractReturnRules(ctx, s.pool, contractID)
			if err != nil {
				return nil, err
			}
			if n == 0 {
				return nil, httpx.Validation("at least one return route is required for pipeline delivery")
			}
		}
	} else {
		buyerPipelineID = 0
		buyerStageID = 0
	}
	return &validatedParticipationDelivery{
		delivery:        delivery,
		buyerPipelineID: buyerPipelineID,
		buyerStageID:    buyerStageID,
	}, nil
}

func (s *Service) UpdateBuyerContractDelivery(ctx context.Context, buyerID, contractID int64, p AcceptParticipationParams) (*Contract, error) {
	validated, err := s.validateBuyerContractDelivery(ctx, buyerID, contractID, p, false)
	if err != nil {
		return nil, err
	}
	if validated.delivery == "leads_pipeline" {
		if err := applyBuyerStageTriggersToWon(ctx, s.pool, contractID, 0, validated.buyerPipelineID); err != nil {
			return nil, err
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`UPDATE contracts SET buyer_pipeline_id = NULLIF($2,0) WHERE id = $1`,
		contractID, validated.buyerPipelineID); err != nil {
		return nil, err
	}
	res, err := tx.Exec(ctx,
		`UPDATE contract_compensations SET
		   delivery = $2,
		   counterparty_pipeline_id = NULLIF($3,0),
		   counterparty_stage_id = NULLIF($4,0)
		 WHERE contract_id = $1 AND participation_id IS NULL
		   AND id = (
		     SELECT id FROM contract_compensations
		     WHERE contract_id = $1 AND participation_id IS NULL
		     ORDER BY position, id LIMIT 1
		   )`,
		contractID, validated.delivery, validated.buyerPipelineID, validated.buyerStageID)
	if err != nil {
		return nil, err
	}
	if res.RowsAffected() == 0 {
		return nil, httpx.Validation("contract compensation is not configured")
	}
	if validated.delivery == "leads_pipeline" {
		if err := RebuildContractStageMaps(ctx, tx, contractID, RebuildStageMapParams{
			BuyerTargetStageID: validated.buyerStageID,
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetForBuyerContract(ctx, buyerID, contractID)
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

// ReturnRuleBelongsToBuyer checks whether a return route belongs to one of the buyer's contracts or participations.
func (s *Service) ReturnRuleBelongsToBuyer(ctx context.Context, buyerID, ruleID int64) (int64, error) {
	var contractID int64
	err := s.pool.QueryRow(ctx,
		`SELECT rr.contract_id FROM contract_return_rules rr
		 LEFT JOIN contracts c ON c.id = rr.contract_id AND c.buyer_id = $2 AND c.deleted_at IS NULL
		 LEFT JOIN contract_participations p ON p.id = rr.participation_id AND p.buyer_id = $2
		 WHERE rr.id = $1 AND (c.id IS NOT NULL OR p.id IS NOT NULL)`,
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

// FindReturnRule checks whether entering newStageID triggers a return for the lead's contract.
func FindReturnRule(ctx context.Context, q database.Querier, contractID, buyerAccountID, newStageID int64) (*ReturnInfo, error) {
	var sourcePipelineID *int64
	var returnStageID, publisherID int64
	err := q.QueryRow(ctx,
		`SELECT COALESCE(p.source_pipeline_id, c.source_pipeline_id),
		        rr.return_stage_id,
		        c.publisher_id
		 FROM contract_return_rules rr
		 JOIN contracts c ON c.id = rr.contract_id
		 LEFT JOIN contract_participations p ON p.id = rr.participation_id
		 WHERE rr.contract_id = $1 AND rr.buyer_stage_id = $3
		   AND rr.return_stage_id IS NOT NULL
		   AND (rr.participation_id IS NULL OR p.buyer_id = $2)
		 ORDER BY CASE WHEN rr.participation_id IS NULL THEN 1 ELSE 0 END, rr.id
		 LIMIT 1`,
		contractID, buyerAccountID, newStageID).Scan(&sourcePipelineID, &returnStageID, &publisherID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if sourcePipelineID == nil || *sourcePipelineID == 0 {
		return nil, httpx.BusinessRule("publisher pipeline is not configured on contract")
	}
	if returnStageID == 0 {
		return nil, httpx.BusinessRule("return destination is misconfigured for this stage")
	}
	return &ReturnInfo{
		SourcePipelineID: *sourcePipelineID,
		ReturnStageID:    returnStageID,
		PublisherID:      publisherID,
	}, nil
}

// ValidateReturnDestination checks that returnStageID belongs to sourcePipelineID.
func ValidateReturnDestination(ctx context.Context, q database.Querier, sourcePipelineID, returnStageID int64) error {
	var ok bool
	if err := q.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pipeline_stages WHERE id = $1 AND pipeline_id = $2)`,
		returnStageID, sourcePipelineID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return httpx.BusinessRule("return destination is misconfigured for this stage")
	}
	return nil
}
