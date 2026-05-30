package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims are embedded in the access token.
type Claims struct {
	Subject          string `json:"sub"`   // user public_id
	AccountID        string `json:"acct"`  // account public_id (buyer when impersonating)
	AccountType      string `json:"atype"` // publisher | buyer
	Role             string `json:"role"`  // admin | user | follower
	Impersonating    bool   `json:"imp,omitempty"`
	ImpersonatorAcct string `json:"imp_acct,omitempty"`
	CollabVersion    int64  `json:"collab_ver,omitempty"`
	jwt.RegisteredClaims
}

// RefreshClaims are embedded in the refresh token.
type RefreshClaims struct {
	Subject string `json:"sub"`
	jwt.RegisteredClaims
}

// TokenManager issues and verifies access/refresh JWTs.
type TokenManager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

func NewTokenManager(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

// Identity describes the authenticated principal used to mint tokens.
type Identity struct {
	UserPublicID    string
	AccountPublicID string
	AccountType     string
	Role            string
	Impersonating   bool
	ImpersonatorAcct string
	CollabVersion   int64
}

func (tm *TokenManager) IssueAccess(id Identity) (string, error) {
	now := time.Now()
	claims := Claims{
		Subject:          id.UserPublicID,
		AccountID:        id.AccountPublicID,
		AccountType:      id.AccountType,
		Role:             id.Role,
		Impersonating:    id.Impersonating,
		ImpersonatorAcct: id.ImpersonatorAcct,
		CollabVersion:    id.CollabVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.accessTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(tm.accessSecret)
}

func (tm *TokenManager) IssueRefresh(userPublicID string) (string, error) {
	now := time.Now()
	claims := RefreshClaims{
		Subject: userPublicID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.refreshTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(tm.refreshSecret)
}

func (tm *TokenManager) ParseAccess(token string) (*Claims, error) {
	claims := &Claims{}
	t, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return tm.accessSecret, nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New("invalid access token")
	}
	return claims, nil
}

func (tm *TokenManager) ParseRefresh(token string) (*RefreshClaims, error) {
	claims := &RefreshClaims{}
	t, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return tm.refreshSecret, nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New("invalid refresh token")
	}
	return claims, nil
}
