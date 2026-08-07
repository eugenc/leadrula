package appointments

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/apikeys"
	"github.com/echayko/leadrula/backend/internal/auth"
)

func TestAPIKeyAppointmentScopes(t *testing.T) {
	t.Parallel()
	readKey := &auth.APIKeyAccount{Scopes: []string{"appointments:read"}}
	writeKey := &auth.APIKeyAccount{Scopes: []string{"appointments:write"}}
	leadsKey := &auth.APIKeyAccount{Scopes: []string{"leads:read", "leads:write"}}

	if !readKey.CanReadAppointments() {
		t.Fatal("appointments:read should allow read")
	}
	if readKey.CanWriteAppointments() {
		t.Fatal("appointments:read should not allow write")
	}
	if !writeKey.CanReadAppointments() {
		t.Fatal("appointments:write should allow read")
	}
	if !writeKey.CanWriteAppointments() {
		t.Fatal("appointments:write should allow write")
	}
	if leadsKey.CanReadAppointments() {
		t.Fatal("leads scopes should not grant appointments read")
	}
	if leadsKey.CanWriteAppointments() {
		t.Fatal("leads scopes should not grant appointments write")
	}
}

func TestRequireAppointmentsReadBlocksMissingScope(t *testing.T) {
	svc := &apikeys.Service{}
	ok := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/appointments/contracts", nil)
	req = req.WithContext(auth.WithAPIKeyAccount(req.Context(), &auth.APIKeyAccount{
		AccountID: 1, AccountType: "publisher", Scopes: []string{"leads:read"},
	}))
	svc.RequireAppointmentsRead(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(ok, req)
	if ok.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", ok.Code)
	}
}

func TestRequireAppointmentsWriteBlocksReadOnlyScope(t *testing.T) {
	svc := &apikeys.Service{}
	ok := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/appointments/book", nil)
	req = req.WithContext(auth.WithAPIKeyAccount(req.Context(), &auth.APIKeyAccount{
		AccountID: 1, AccountType: "publisher", Scopes: []string{"appointments:read"},
	}))
	svc.RequireAppointmentsWrite(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})).ServeHTTP(ok, req)
	if ok.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", ok.Code)
	}
}

func TestPublicListContractsRejectsPlatformKey(t *testing.T) {
	h := NewHandler(NewService(nil, nil, nil, nil))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/appointments/contracts", nil)
	req = req.WithContext(auth.WithAPIKeyAccount(req.Context(), &auth.APIKeyAccount{
		AccountID: 1, AccountType: "platform", Scopes: []string{"appointments:read"},
	}))
	h.publicListContracts(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestPrincipalFromAPIKeyRequiresAdminUser(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ctx := context.Background()

	var accountID int64
	err := pool.QueryRow(ctx, `SELECT id FROM accounts WHERE type = 'publisher' LIMIT 1`).Scan(&accountID)
	if err != nil || accountID == 0 {
		t.Skip("no publisher account")
	}

	svc := NewService(pool, nil, accounts.NewRepository(pool), nil)
	p, err := svc.principalFromAPIKey(ctx, &auth.APIKeyAccount{
		AccountID: accountID, AccountType: "publisher", Scopes: []string{"appointments:write"},
	})
	if err != nil {
		if err.Error() == "validation_error: account has no admin users" {
			t.Skip("publisher account has no admin users")
		}
		t.Fatalf("principalFromAPIKey: %v", err)
	}
	if p.UserID == 0 {
		t.Fatal("expected UserID on principal")
	}
}
