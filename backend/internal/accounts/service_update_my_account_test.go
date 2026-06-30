package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/permissions"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

func strPtr(s string) *string { return &s }

func adminPrincipal(accountType string, accountID int64, accountPublicID string) *auth.Principal {
	return &auth.Principal{
		Role:            "admin",
		AccountType:     accountType,
		AccountID:       accountID,
		AccountPublicID: accountPublicID,
		Perms:           permissions.FullAccess(accountType),
	}
}

func TestUpdateMyAccount_validation(t *testing.T) {
	svc := NewService(nil, nil, nil, nil)
	ctx := context.Background()

	tests := []struct {
		name   string
		p      *auth.Principal
		params UpdateMyAccountParams
		wantCode string
	}{
		{
			name:     "non-admin",
			p:        &auth.Principal{Role: "user", AccountType: "buyer"},
			params:   UpdateMyAccountParams{Timezone: strPtr("America/New_York")},
			wantCode: httpx.CodeForbidden,
		},
		{
			name:     "platform account",
			p:        adminPrincipal("platform", 0, ""),
			params:   UpdateMyAccountParams{Timezone: strPtr("America/New_York")},
			wantCode: httpx.CodeValidation,
		},
		{
			name:     "invalid timezone",
			p:        adminPrincipal("buyer", 1, ""),
			params:   UpdateMyAccountParams{Timezone: strPtr("Invalid/Zone")},
			wantCode: httpx.CodeValidation,
		},
		{
			name:     "empty timezone",
			p:        adminPrincipal("publisher", 0, "pub-id"),
			params:   UpdateMyAccountParams{Timezone: strPtr("  ")},
			wantCode: httpx.CodeValidation,
		},
		{
			name:     "empty patch",
			p:        adminPrincipal("buyer", 1, ""),
			params:   UpdateMyAccountParams{},
			wantCode: httpx.CodeValidation,
		},
		{
			name:     "invalid contact email",
			p:        adminPrincipal("buyer", 1, ""),
			params:   UpdateMyAccountParams{ContactEmail: strPtr("not-an-email")},
			wantCode: httpx.CodeValidation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateMyAccount(ctx, tc.p, tc.params)
			if err == nil {
				t.Fatal("expected error")
			}
			var appErr *httpx.AppError
			if !errors.As(err, &appErr) {
				t.Fatalf("expected AppError, got %v", err)
			}
			if appErr.Code != tc.wantCode {
				t.Fatalf("code %q, want %q", appErr.Code, tc.wantCode)
			}
		})
	}
}

func TestPatchMyAccount_handler(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	svc := NewService(NewRepository(pool), nil, nil, nil)
	h := NewHandler(svc)

	email := fmt.Sprintf("patch-me-account-%d@example.com", time.Now().UnixNano())
	createBody := fmt.Sprintf(
		`{"name":"TZ Test Pub","admin_first_name":"A","admin_last_name":"B","admin_email":%q,"timezone":"America/Toronto"}`,
		email,
	)
	createReq := httptest.NewRequest(http.MethodPost, "/platform/publishers", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	h.createPublisher(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", createRec.Code, createRec.Body.String())
	}

	var createResp struct {
		Data struct {
			PublicID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	var accountID int64
	var accountPublicID string
	err = pool.QueryRow(ctx,
		`SELECT id, public_id FROM accounts WHERE public_id = $1`,
		createResp.Data.PublicID,
	).Scan(&accountID, &accountPublicID)
	if err != nil {
		t.Fatalf("load account: %v", err)
	}

	patchReq := httptest.NewRequest(
		http.MethodPatch,
		"/auth/me/account",
		strings.NewReader(`{"timezone":"America/Los_Angeles","website":"https://example.com","phone":"555-0100"}`),
	)
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq = patchReq.WithContext(auth.WithPrincipal(patchReq.Context(), adminPrincipal("publisher", accountID, accountPublicID)))
	patchRec := httptest.NewRecorder()
	h.patchMyAccount(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch status %d: %s", patchRec.Code, patchRec.Body.String())
	}

	var patchResp struct {
		Data struct {
			Account struct {
				Timezone string `json:"timezone"`
				Website  string `json:"website"`
				Phone    string `json:"phone"`
			} `json:"account"`
		} `json:"data"`
	}
	if err := json.Unmarshal(patchRec.Body.Bytes(), &patchResp); err != nil {
		t.Fatalf("decode patch: %v body=%s", err, patchRec.Body.String())
	}
	if patchResp.Data.Account.Timezone != "America/Los_Angeles" {
		t.Fatalf("timezone %q, want America/Los_Angeles", patchResp.Data.Account.Timezone)
	}
	if patchResp.Data.Account.Website != "https://example.com" {
		t.Fatalf("website %q, want https://example.com", patchResp.Data.Account.Website)
	}
	if patchResp.Data.Account.Phone != "555-0100" {
		t.Fatalf("phone %q, want 555-0100", patchResp.Data.Account.Phone)
	}
}

func TestPatchMyAccount_handlerNonAdmin(t *testing.T) {
	h := NewHandler(NewService(nil, nil, nil, nil))

	r := chi.NewRouter()
	r.With(auth.RequireRole("admin")).Patch("/auth/me/account", h.patchMyAccount)

	req := httptest.NewRequest(http.MethodPatch, "/auth/me/account", strings.NewReader(`{"timezone":"America/New_York"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.WithPrincipal(req.Context(), &auth.Principal{
		Role:        "user",
		AccountType: "buyer",
	}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403: %s", rec.Code, rec.Body.String())
	}
}
