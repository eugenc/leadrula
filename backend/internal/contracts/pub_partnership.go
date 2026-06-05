package contracts

import (
	"context"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

// ValidateCounterpartyAccountType ensures buy uses publisher only; sell uses buyer or publisher.
func ValidateCounterpartyAccountType(contractType, accountType string) error {
	switch contractType {
	case "buy":
		if accountType != "publisher" {
			return httpx.Validation("buy contracts require a publisher counterparty")
		}
		return nil
	case "sell":
		if accountType == "buyer" || accountType == "publisher" {
			return nil
		}
		return httpx.Validation("sell contracts require a buyer or publisher counterparty")
	default:
		return httpx.Validation("contract_type must be buy or sell")
	}
}

func hasPublisherPartnership(ctx context.Context, q database.Querier, pubA, pubB int64) (bool, error) {
	a, b := pubA, pubB
	if a > b {
		a, b = b, a
	}
	var ok bool
	err := q.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM publisher_partnerships
		   WHERE publisher_a_id = $1 AND publisher_b_id = $2 AND status = 'active'
		 )`, a, b).Scan(&ok)
	return ok, err
}

func (s *Service) assertCounterpartyPartnership(ctx context.Context, ownerID int64, contractType string, counterpartyID int64, counterpartyType string) error {
	if err := ValidateCounterpartyAccountType(contractType, counterpartyType); err != nil {
		return err
	}
	if contractType == "sell" && counterpartyType == "buyer" {
		var ok bool
		err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM partnerships WHERE publisher_id = $1 AND buyer_id = $2 AND status = 'active')`,
			ownerID, counterpartyID).Scan(&ok)
		if err != nil {
			return err
		}
		if !ok {
			return httpx.Validation("no active partnership with this buyer")
		}
		return nil
	}
	if contractType == "buy" && counterpartyType == "publisher" {
		ok, err := hasPublisherPartnership(ctx, s.pool, ownerID, counterpartyID)
		if err != nil {
			return err
		}
		if !ok {
			return httpx.Validation("no active publisher partnership with this counterparty")
		}
		return nil
	}
	if contractType == "sell" && counterpartyType == "publisher" {
		ok, err := hasPublisherPartnership(ctx, s.pool, ownerID, counterpartyID)
		if err != nil {
			return err
		}
		if !ok {
			return httpx.Validation("no active publisher partnership with this counterparty")
		}
		return nil
	}
	return httpx.Validation("invalid contract type and counterparty combination")
}
