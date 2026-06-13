package billing

import (
	"context"
	"fmt"
	"log"

	"github.com/echayko/leadrula/backend/internal/accounts"
)

// ExecuteMarketplacePayoutTransfers sends cleared marketplace earnings to the publisher Connect account.
func (s *Service) ExecuteMarketplacePayoutTransfers(ctx context.Context, publisherID int64) error {
	if s.stripe == nil || !s.stripe.Enabled() {
		return nil
	}

	rows, err := s.pool.Query(ctx,
		`SELECT pc.id, pc.amount::float8, buyer.buyer_kind::text, pub.stripe_account_id, cc.kind
		 FROM compensation_payout_clears pc
		 JOIN contract_compensations cc ON cc.id = pc.compensation_id
		 JOIN contracts c ON c.id = cc.contract_id
		 JOIN accounts buyer ON buyer.id = c.buyer_id
		 JOIN accounts pub ON pub.id = c.publisher_id
		 WHERE c.publisher_id = $1
		   AND pc.stripe_transfer_status IN ('pending', 'failed')
		 ORDER BY pc.id`,
		publisherID)
	if err != nil {
		return err
	}
	defer rows.Close()

	connectReady := s.PublisherConnectReady(ctx, publisherID) == nil

	for rows.Next() {
		var clearID int64
		var amount float64
		var buyerKind, compKind string
		var stripeAccountID *string
		if err := rows.Scan(&clearID, &amount, &buyerKind, &stripeAccountID, &compKind); err != nil {
			return err
		}

		if compKind == "rev_share" || compKind == "profit_share" {
			if _, err := s.pool.Exec(ctx,
				`UPDATE compensation_payout_clears SET stripe_transfer_status = 'skipped' WHERE id = $1`,
				clearID); err != nil {
				return err
			}
			continue
		}

		if buyerKind == accounts.BuyerKindDirect {
			if _, err := s.pool.Exec(ctx,
				`UPDATE compensation_payout_clears SET stripe_transfer_status = 'skipped' WHERE id = $1`,
				clearID); err != nil {
				return err
			}
			continue
		}

		if buyerKind != accounts.BuyerKindMarketplace {
			continue
		}

		if !connectReady || stripeAccountID == nil || *stripeAccountID == "" {
			continue
		}

		amountCents := int64(amount * 100)
		netCents := s.stripe.NetTransferAmount(amountCents)
		if netCents < 1 {
			if _, err := s.pool.Exec(ctx,
				`UPDATE compensation_payout_clears SET stripe_transfer_status = 'skipped' WHERE id = $1`,
				clearID); err != nil {
				return err
			}
			continue
		}

		transferID, err := s.stripe.CreateTransfer(netCents, *stripeAccountID, fmt.Sprintf("clear-%d", clearID))
		if err != nil {
			log.Printf("billing: marketplace payout transfer clear %d: %v", clearID, err)
			if _, uerr := s.pool.Exec(ctx,
				`UPDATE compensation_payout_clears SET stripe_transfer_status = 'failed' WHERE id = $1`,
				clearID); uerr != nil {
				return uerr
			}
			continue
		}

		if _, err := s.pool.Exec(ctx,
			`UPDATE compensation_payout_clears
			 SET stripe_transfer_id = $2, stripe_transfer_status = 'sent'
			 WHERE id = $1`,
			clearID, transferID); err != nil {
			return err
		}
	}
	return rows.Err()
}
