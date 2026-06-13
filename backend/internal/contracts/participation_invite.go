package contracts

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func randomInviteToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type InviteInfo struct {
	Token     string `json:"token"`
	HandlerID string `json:"handler_id"`
	URL       string `json:"url,omitempty"`
}

func (s *Service) EnsureInviteToken(ctx context.Context, publisherID, contractID int64) (*InviteInfo, error) {
	c, err := s.Get(ctx, publisherID, contractID)
	if err != nil {
		return nil, err
	}
	if c.Status != "active" {
		return nil, httpx.Validation("only active contracts can have invite links")
	}
	token := c.InviteToken
	if token == "" {
		for range 5 {
			t, err := randomInviteToken()
			if err != nil {
				return nil, err
			}
			_, err = s.pool.Exec(ctx,
				`UPDATE contracts SET invite_token = $3 WHERE id = $1 AND publisher_id = $2 AND invite_token IS NULL`,
				contractID, publisherID, t)
			if err == nil {
				token = t
				break
			}
			if !database.IsUniqueViolation(err) {
				return nil, err
			}
		}
		if token == "" {
			_ = s.pool.QueryRow(ctx, `SELECT invite_token FROM contracts WHERE id = $1`, contractID).Scan(&token)
		}
	}
	return &InviteInfo{Token: token, HandlerID: c.HandlerID}, nil
}

func (s *Service) AttachByInvite(ctx context.Context, buyerID int64, token string) (*Participation, error) {
	if token == "" {
		return nil, httpx.Validation("invite token is required")
	}
	var contractID int64
	var publisherID int64
	var status string
	err := s.pool.QueryRow(ctx,
		`SELECT id, publisher_id, status FROM contracts
		 WHERE invite_token = $1 AND deleted_at IS NULL AND contract_type = 'sell'`, token).
		Scan(&contractID, &publisherID, &status)
	if err != nil {
		return nil, httpx.NotFound("invite not found")
	}
	if status != "active" {
		return nil, httpx.Validation("contract is not accepting buyers")
	}
	var existingID int64
	var existingStatus string
	err = s.pool.QueryRow(ctx,
		`SELECT id, status::text FROM contract_participations
		 WHERE contract_id = $1 AND buyer_id = $2`, contractID, buyerID).
		Scan(&existingID, &existingStatus)
	if err == nil && (existingStatus == "withdrawn" || existingStatus == "declined") {
		return s.ReinviteParticipation(ctx, publisherID, existingID)
	}
	return s.AddParticipation(ctx, publisherID, contractID, AddParticipationParams{BuyerID: buyerID})
}
