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
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

func TestUpdateMyAccount_validation(t *testing.T) {
	svc := NewService(nil, nil, nil, nil)
	ctx := context.Background()

	tests := []struct {
		name     string
		p        *auth.Principal
		timezone string
		wantCode string
	}{
		{
			name:     "non-admin",
			p:        &auth.Principal{Role: "user", AccountType: "buyer"},
			timezone: "America/New_York",
			wantCode: httpx.CodeForbidden,
		},
		{
			name:     "platform account",
			p:        &auth.Principal{Role: "admin", AccountType: "platform"},
			timezone: "America/New_York",
			wantCode: httpx.CodeValidation,
		},
		{
			name:     "invalid timezone",
			p:        &auth.Principal{Role: "admin", AccountType: "buyer", AccountID: 1},
			timezone: "Invalid/Zone",
			wantCode: httpx.CodeValidation,
		},
		{
			name:     "empty timezone",
			p:        &auth.Principal{Role: "admin", AccountType: "publisher", AccountPublicID: "pub-id"},
			timezone: "  ",
			wantCode: httpx.CodeValidation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateMyAccount(ctx, tc.p, tc.timezone)
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
		strings.NewReader(`{"timezone":"America/Los_Angeles"}`),
	)
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq = patchReq.WithContext(auth.WithPrincipal(patchReq.Context(), &auth.Principal{
		AccountID:       accountID,
		AccountPublicID: accountPublicID,
		AccountType:     "publisher",
		Role:            "admin",
	}))
	patchRec := httptest.NewRecorder()
	h.patchMyAccount(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch status %d: %s", patchRec.Code, patchRec.Body.String())
	}

	var patchResp struct {
		Data struct {
			Account struct {
				Timezone string `json:"timezone"`
			} `json:"account"`
		} `json:"data"`
	}
	if err := json.Unmarshal(patchRec.Body.Bytes(), &patchResp); err != nil {
		t.Fatalf("decode patch: %v body=%s", err, patchRec.Body.String())
	}
	if patchResp.Data.Account.Timezone != "America/Los_Angeles" {
		t.Fatalf("timezone %q, want America/Los_Angeles", patchResp.Data.Account.Timezone)
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
