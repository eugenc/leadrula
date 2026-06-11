package contracts

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type Participation struct {
	ID                      int64           `json:"id"`
	ContractID              int64           `json:"contract_id"`
	BuyerID                 int64           `json:"buyer_id"`
	BuyerName               string          `json:"buyer_name,omitempty"`
	Status                  string          `json:"status"`
	Delivery                string          `json:"delivery,omitempty"`
	BuyerPipelineID         *int64          `json:"buyer_pipeline_id,omitempty"`
	BuyerTargetStageID      *int64          `json:"buyer_target_stage_id,omitempty"`
	SourcePipelineID        *int64          `json:"source_pipeline_id,omitempty"`
	SourceStageID           *int64          `json:"source_stage_id,omitempty"`
	ReturnStageID           *int64          `json:"return_stage_id,omitempty"`
	IntegrationConnectionID *int64          `json:"integration_connection_id,omitempty"`
	OutboundWebhookID       *int64          `json:"outbound_webhook_id,omitempty"`
	CounterProposal         json.RawMessage `json:"counter_proposal,omitempty"`
	SupersededByContractID  *int64          `json:"superseded_by_contract_id,omitempty"`
	ContractName            string          `json:"contract_name,omitempty"`
	PublisherName           string          `json:"publisher_name,omitempty"`
	LeadType                string          `json:"lead_type,omitempty"`
	AllowedDeliveryModes    []string        `json:"allowed_delivery_modes,omitempty"`
	CreatedAt               time.Time       `json:"created_at"`
	UpdatedAt               time.Time       `json:"updated_at"`
}

const participationCols = `p.id, p.contract_id, p.buyer_id, p.status::text, COALESCE(p.delivery,''),
	p.buyer_pipeline_id, p.buyer_target_stage_id,
	p.source_pipeline_id, p.source_stage_id, p.return_stage_id,
	p.integration_connection_id, p.outbound_webhook_id,
	p.counter_proposal, p.superseded_by_contract_id, p.created_at, p.updated_at`

const participationReturningCols = `id, contract_id, buyer_id, status::text, COALESCE(delivery,''),
	buyer_pipeline_id, buyer_target_stage_id,
	source_pipeline_id, source_stage_id, return_stage_id,
	integration_connection_id, outbound_webhook_id,
	counter_proposal, superseded_by_contract_id, created_at, updated_at`

func scanParticipationFields(
	dest *Participation,
	delivery *string,
	counter *[]byte,
	row pgx.Row,
	extra ...any,
) error {
	scan := []any{
		&dest.ID, &dest.ContractID, &dest.BuyerID, &dest.Status, delivery,
		&dest.BuyerPipelineID, &dest.BuyerTargetStageID,
		&dest.SourcePipelineID, &dest.SourceStageID, &dest.ReturnStageID,
		&dest.IntegrationConnectionID, &dest.OutboundWebhookID,
		counter, &dest.SupersededByContractID, &dest.CreatedAt, &dest.UpdatedAt,
	}
	scan = append(scan, extra...)
	return row.Scan(scan...)
}

func scanParticipation(row pgx.Row) (*Participation, error) {
	p := &Participation{}
	var delivery string
	var counter []byte
	err := scanParticipationFields(p, &delivery, &counter, row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("participation not found")
		}
		return nil, err
	}
	p.Delivery = delivery
	if len(counter) > 0 {
		p.CounterProposal = counter
	}
	return p, nil
}

var allowedPublisherDelivery = map[string]bool{
	"leads": true, "leads_pipeline": true, "webhook": true,
}

var allowedDistributionStrategy = map[string]bool{
	"round_robin": true, "highest_price": true, "largest_spread": true,
}

func deliveryModesForLeadType(leadType string) []string {
	switch strings.TrimSpace(leadType) {
	case "Call":
		return nil
	default:
		return []string{"leads", "leads_pipeline", "webhook"}
	}
}

func validateAllowedDeliveryModes(leadType string, modes []string) error {
	allowed := deliveryModesForLeadType(leadType)
	if len(allowed) == 0 {
		return httpx.Validation("Call lead type is not supported in this version")
	}
	if len(modes) == 0 {
		return httpx.Validation("at least one delivery mode is required")
	}
	hasLeads := false
	for _, m := range modes {
		if m == "leads" {
			hasLeads = true
			break
		}
	}
	if !hasLeads {
		return httpx.Validation("lead inbox delivery mode is required")
	}
	allowedSet := map[string]bool{}
	for _, m := range allowed {
		allowedSet[m] = true
	}
	for _, m := range modes {
		if !allowedSet[m] {
			return httpx.Validation("invalid delivery mode for lead type: " + m)
		}
	}
	return nil
}

func (s *Service) ListParticipations(ctx context.Context, publisherID, contractID int64) ([]Participation, error) {
	if _, err := s.Get(ctx, publisherID, contractID); err != nil {
		return nil, err
	}
	return s.listParticipationsByContract(ctx, contractID)
}

func (s *Service) listParticipationsByContract(ctx context.Context, contractID int64) ([]Participation, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+participationCols+`, a.name
		 FROM contract_participations p
		 JOIN accounts a ON a.id = p.buyer_id
		 WHERE p.contract_id = $1
		 ORDER BY p.created_at`, contractID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Participation
	for rows.Next() {
		p := &Participation{}
		var delivery string
		var counter []byte
		var buyerName string
		if err := scanParticipationFields(p, &delivery, &counter, rows, &buyerName); err != nil {
			return nil, err
		}
		p.Delivery = delivery
		if len(counter) > 0 {
			p.CounterProposal = counter
		}
		p.BuyerName = buyerName
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (s *Service) ListParticipationsForBuyer(ctx context.Context, buyerID int64) ([]Participation, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+participationCols+`, c.name, pub.name, COALESCE(c.lead_type,''), c.allowed_delivery_modes
		 FROM contract_participations p
		 JOIN contracts c ON c.id = p.contract_id
		 JOIN accounts pub ON pub.id = c.publisher_id
		 WHERE p.buyer_id = $1 AND c.deleted_at IS NULL AND c.contract_type = 'sell'
		 ORDER BY p.updated_at DESC`, buyerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Participation
	for rows.Next() {
		p := &Participation{}
		var delivery string
		var counter []byte
		var contractName, publisherName, leadType string
		var allowed []string
		if err := scanParticipationFields(p, &delivery, &counter, rows,
			&contractName, &publisherName, &leadType, &allowed); err != nil {
			return nil, err
		}
		p.Delivery = delivery
		if len(counter) > 0 {
			p.CounterProposal = counter
		}
		p.ContractName = contractName
		p.PublisherName = publisherName
		p.LeadType = leadType
		p.AllowedDeliveryModes = allowed
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (s *Service) GetParticipation(ctx context.Context, participationID int64) (*Participation, error) {
	return scanParticipation(s.pool.QueryRow(ctx,
		`SELECT `+participationCols+` FROM contract_participations p WHERE p.id = $1`, participationID))
}

func (s *Service) GetParticipationForBuyer(ctx context.Context, buyerID, participationID int64) (*Participation, error) {
	p, err := scanParticipation(s.pool.QueryRow(ctx,
		`SELECT `+participationCols+` FROM contract_participations p
		 WHERE p.id = $1 AND p.buyer_id = $2`, participationID, buyerID))
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) GetParticipationForPublisher(ctx context.Context, publisherID, participationID int64) (*Participation, error) {
	p, err := scanParticipation(s.pool.QueryRow(ctx,
		`SELECT `+participationCols+`
		 FROM contract_participations p
		 JOIN contracts c ON c.id = p.contract_id
		 WHERE p.id = $1 AND c.publisher_id = $2 AND c.deleted_at IS NULL`,
		participationID, publisherID))
	if err != nil {
		return nil, err
	}
	return p, nil
}

type AddParticipationParams struct {
	BuyerID int64
}

func (s *Service) AddParticipation(ctx context.Context, publisherID, contractID int64, p AddParticipationParams) (*Participation, error) {
	c, err := s.Get(ctx, publisherID, contractID)
	if err != nil {
		return nil, err
	}
	if c.Status != "active" {
		return nil, httpx.Validation("contract must be active to add buyers")
	}
	if p.BuyerID == 0 {
		return nil, httpx.Validation("buyer_id is required")
	}
	if err := s.assertCounterpartyPartnership(ctx, publisherID, c.ContractType, p.BuyerID, "buyer"); err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	part, err := scanParticipation(tx.QueryRow(ctx,
		`INSERT INTO contract_participations(contract_id, buyer_id, status)
		 VALUES ($1,$2,'pending')
		 RETURNING `+participationReturningCols,
		contractID, p.BuyerID))
	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, httpx.Conflict("buyer already on this contract")
		}
		return nil, err
	}
	if err := s.copyContractTemplateToParticipation(ctx, tx, contractID, part.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.notifyParticipation(ctx, s.pool, p.BuyerID, "contract_participation_pending", map[string]any{
		"contract_id":      contractID,
		"participation_id": part.ID,
		"contract_name":    c.Name,
	})
	return part, nil
}

func (s *Service) copyContractTemplateToParticipation(ctx context.Context, tx pgx.Tx, contractID, participationID int64) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO contract_compensations(
		    contract_id, participation_id, kind, flat_amount, bid_min, bid_max, rev_percent, profit_percent,
		    cap_period, cap_total, cap_max_daily, trigger, trigger_stage_id,
		    source_pipeline_id, source_stage_id, counterparty_pipeline_id, counterparty_stage_id,
		    return_stage_id, delivery, position, payout_frequency, payout_weekday, payout_month_day)
		 SELECT contract_id, $2, kind, flat_amount, bid_min, bid_max, rev_percent, profit_percent,
		        cap_period, cap_total, cap_max_daily, trigger, trigger_stage_id,
		        source_pipeline_id, source_stage_id, counterparty_pipeline_id, counterparty_stage_id,
		        return_stage_id, delivery, position, payout_frequency, payout_weekday, payout_month_day
		 FROM contract_compensations
		 WHERE contract_id = $1 AND participation_id IS NULL`,
		contractID, participationID)
	return err
}

type AcceptParticipationParams struct {
	Delivery                string `json:"delivery"`
	BuyerPipelineID         int64  `json:"buyer_pipeline_id"`
	BuyerTargetStageID      int64  `json:"buyer_target_stage_id"`
	IntegrationConnectionID int64  `json:"integration_connection_id"`
	OutboundWebhookID       int64  `json:"outbound_webhook_id"`
}

func (s *Service) AcceptParticipation(ctx context.Context, buyerID, participationID int64, p AcceptParticipationParams) (*Participation, error) {
	part, err := s.GetParticipationForBuyer(ctx, buyerID, participationID)
	if err != nil {
		return nil, err
	}
	if part.Status != "pending" && part.Status != "counter_pending" {
		return nil, httpx.Validation("participation is not awaiting acceptance")
	}
	var allowed []string
	var leadType string
	if err := s.pool.QueryRow(ctx,
		`SELECT allowed_delivery_modes, COALESCE(lead_type,'') FROM contracts WHERE id = $1`,
		part.ContractID).Scan(&allowed, &leadType); err != nil {
		return nil, err
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
	buyerPipelineID := p.BuyerPipelineID
	buyerStageID := p.BuyerTargetStageID
	if delivery == "leads_pipeline" {
		if buyerPipelineID == 0 || buyerStageID == 0 {
			return nil, httpx.Validation("buyer_pipeline_id and buyer_target_stage_id are required for pipeline delivery")
		}
		if err := validateBuyerTargetStage(ctx, s.pool, buyerStageID, buyerPipelineID); err != nil {
			return nil, err
		}
	} else {
		buyerPipelineID = 0
		buyerStageID = 0
	}
	if err := s.ValidateParticipationFieldMapping(ctx, part.ContractID, participationID); err != nil {
		return nil, err
	}
	var webhookID *int64
	if delivery == "webhook" {
		if p.OutboundWebhookID == 0 {
			return nil, httpx.Validation("outbound_webhook_id is required for webhook delivery")
		}
		webhookID = &p.OutboundWebhookID
	}
	var connID *int64
	if p.IntegrationConnectionID != 0 {
		if err := validateBuyerCRMConnection(ctx, s.pool, buyerID, p.IntegrationConnectionID); err != nil {
			return nil, err
		}
		connID = &p.IntegrationConnectionID
	}
	updated, err := scanParticipation(s.pool.QueryRow(ctx,
		`UPDATE contract_participations SET
		   status = 'active', delivery = $2,
		   buyer_pipeline_id = NULLIF($3,0), buyer_target_stage_id = NULLIF($4,0),
		   outbound_webhook_id = $5, integration_connection_id = $6,
		   counter_proposal = NULL, updated_at = now(), buyer_responded_at = now()
		 WHERE id = $1
		 RETURNING `+participationReturningCols,
		participationID, delivery, buyerPipelineID, buyerStageID, webhookID, connID))
	if err != nil {
		return nil, err
	}
	var pubID int64
	var contractName string
	_ = s.pool.QueryRow(ctx,
		`SELECT publisher_id, name FROM contracts WHERE id = $1`, part.ContractID).Scan(&pubID, &contractName)
	s.notifyParticipation(ctx, s.pool, pubID, "contract_participation_accepted", map[string]any{
		"contract_id":      part.ContractID,
		"participation_id": participationID,
		"contract_name":    contractName,
		"buyer_id":         buyerID,
	})
	return updated, nil
}

func validateBuyerTargetStage(ctx context.Context, q database.Querier, stageID, pipelineID int64) error {
	var stageType string
	err := q.QueryRow(ctx,
		`SELECT stage_type FROM pipeline_stages WHERE id = $1 AND pipeline_id = $2`,
		stageID, pipelineID).Scan(&stageType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.Validation("invalid pipeline stage")
		}
		return err
	}
	if stageType != "standard" && stageType != "action" {
		return httpx.Validation("destination stage must be standard or action")
	}
	return nil
}

func validateBuyerCRMConnection(ctx context.Context, q database.Querier, buyerID, connID int64) error {
	var status, provider string
	err := q.QueryRow(ctx,
		`SELECT ic.status, ic.provider FROM integration_connections ic
		 WHERE ic.id = $1 AND ic.account_id = $2`, connID, buyerID).Scan(&status, &provider)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.Validation("integration connection not found")
		}
		return err
	}
	if status != "active" {
		return httpx.Validation("integration connection is not active")
	}
	crmProviders := map[string]bool{
		"pipedrive": true, "ghl": true, "hubspot": true, "zoho_crm": true, "salesforce": true, "sunbase": true,
	}
	if !crmProviders[provider] {
		return httpx.Validation("connection must be a CRM integration")
	}
	return nil
}

func (s *Service) DeclineParticipation(ctx context.Context, buyerID, participationID int64) (*Participation, error) {
	part, err := s.GetParticipationForBuyer(ctx, buyerID, participationID)
	if err != nil {
		return nil, err
	}
	if part.Status != "pending" && part.Status != "counter_pending" {
		return nil, httpx.Validation("participation is not awaiting response")
	}
	updated, err := scanParticipation(s.pool.QueryRow(ctx,
		`UPDATE contract_participations SET status = 'declined', updated_at = now(), buyer_responded_at = now()
		 WHERE id = $1 RETURNING `+participationReturningCols, participationID))
	if err != nil {
		return nil, err
	}
	var pubID int64
	_ = s.pool.QueryRow(ctx, `SELECT publisher_id FROM contracts WHERE id = $1`, part.ContractID).Scan(&pubID)
	s.notifyParticipation(ctx, s.pool, pubID, "contract_participation_declined", map[string]any{
		"contract_id":      part.ContractID,
		"participation_id": participationID,
	})
	return updated, nil
}

func (s *Service) DeclineParticipationByPublisher(ctx context.Context, publisherID, participationID int64) (*Participation, error) {
	part, err := s.GetParticipationForPublisher(ctx, publisherID, participationID)
	if err != nil {
		return nil, err
	}
	if part.Status != "counter_pending" {
		return nil, httpx.Validation("participation has no pending counter-offer")
	}
	return scanParticipation(s.pool.QueryRow(ctx,
		`UPDATE contract_participations SET status = 'pending', counter_proposal = NULL,
		   publisher_responded_at = now(), updated_at = now()
		 WHERE id = $1 RETURNING `+participationReturningCols, participationID))
}

func (s *Service) CounterParticipation(ctx context.Context, buyerID, participationID int64, proposal json.RawMessage) (*Participation, error) {
	part, err := s.GetParticipationForBuyer(ctx, buyerID, participationID)
	if err != nil {
		return nil, err
	}
	if part.Status != "pending" {
		return nil, httpx.Validation("only pending participations can be countered")
	}
	updated, err := scanParticipation(s.pool.QueryRow(ctx,
		`UPDATE contract_participations SET status = 'counter_pending', counter_proposal = $2,
		   updated_at = now(), buyer_responded_at = now()
		 WHERE id = $1 RETURNING `+participationReturningCols, participationID, proposal))
	if err != nil {
		return nil, err
	}
	var pubID int64
	_ = s.pool.QueryRow(ctx, `SELECT publisher_id FROM contracts WHERE id = $1`, part.ContractID).Scan(&pubID)
	s.notifyParticipation(ctx, s.pool, pubID, "contract_counter_pending", map[string]any{
		"contract_id":      part.ContractID,
		"participation_id": participationID,
	})
	return updated, nil
}

// PickParticipation selects an active participation for routing (excludes bid-only per_lead comps).
func PickParticipation(ctx context.Context, q database.Querier, contractID int64, leadCost float64) (int64, int64, error) {
	var strategy string
	var cursor int
	if err := q.QueryRow(ctx,
		`SELECT distribution_strategy, distribution_cursor FROM contracts WHERE id = $1 AND deleted_at IS NULL AND status = 'active'`,
		contractID).Scan(&strategy, &cursor); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, httpx.NotFound("contract not found")
		}
		return 0, 0, err
	}
	rows, err := q.Query(ctx,
		`SELECT p.id, p.buyer_id,
		        COALESCE(cc.flat_amount, cc.bid_max, 0)::float8 AS rate,
		        cc.id AS comp_id, cc.kind
		 FROM contract_participations p
		 JOIN LATERAL (
		   SELECT id, kind, flat_amount, bid_max FROM contract_compensations
		   WHERE participation_id = p.id AND trigger = 'per_lead'
		   ORDER BY position, id LIMIT 1
		 ) cc ON true
		 WHERE p.contract_id = $1 AND p.status = 'active' AND cc.kind <> 'bid'
		 ORDER BY p.id`, contractID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	type cand struct {
		id, buyerID, compID int64
		rate                float64
	}
	var cands []cand
	for rows.Next() {
		var c cand
		var kind string
		if err := rows.Scan(&c.id, &c.buyerID, &c.rate, &c.compID, &kind); err != nil {
			return 0, 0, err
		}
		cands = append(cands, c)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	if len(cands) == 0 {
		// Legacy fallback: contract with buyer_id
		var buyerID int64
		err := q.QueryRow(ctx,
			`SELECT buyer_id FROM contracts WHERE id = $1 AND buyer_id IS NOT NULL AND deleted_at IS NULL`,
			contractID).Scan(&buyerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, httpx.BusinessRule("no active buyer participation on contract")
		}
		if err != nil {
			return 0, 0, err
		}
		return 0, buyerID, nil
	}
	var pick cand
	switch strategy {
	case "highest_price":
		pick = cands[0]
		for _, c := range cands[1:] {
			if c.rate > pick.rate {
				pick = c
			}
		}
	case "largest_spread":
		pick = cands[0]
		bestSpread := pick.rate - leadCost
		for _, c := range cands[1:] {
			if sp := c.rate - leadCost; sp > bestSpread {
				pick = c
				bestSpread = sp
			}
		}
	default: // round_robin
		idx := cursor % len(cands)
		pick = cands[idx]
		_, _ = q.Exec(ctx, `UPDATE contracts SET distribution_cursor = $2 WHERE id = $1`, contractID, cursor+1)
	}
	return pick.id, pick.buyerID, nil
}
