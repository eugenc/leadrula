package marketing

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/go-chi/chi/v5"
)

func TestSubmitQuote_validation(t *testing.T) {
	h := NewHandler(notifications.NewEmailSender("", "", "dev@test", "", "http://localhost:5173"), "", false)
	r := chi.NewRouter()
	h.RegisterPublicRoutes(r)

	body := map[string]string{"email": "bad"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/public/quote", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSubmitQuote_honeypot(t *testing.T) {
	h := NewHandler(notifications.NewEmailSender("", "", "dev@test", "", "http://localhost:5173"), "", false)
	r := chi.NewRouter()
	h.RegisterPublicRoutes(r)

	body := map[string]string{
		"full_name": "Jane",
		"email":     "jane@example.com",
		"website":   "http://spam.test",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/public/quote", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestSubmitQuote_ok(t *testing.T) {
	h := NewHandler(notifications.NewEmailSender("", "", "dev@test", "", "http://localhost:5173"), "sales@leadrula.com", false)
	r := chi.NewRouter()
	h.RegisterPublicRoutes(r)

	body := map[string]string{
		"full_name":      "Jane Doe",
		"email":          "jane@company.com",
		"role":           "Publisher",
		"monthly_volume": "Under 2,500",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/public/quote", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
}
