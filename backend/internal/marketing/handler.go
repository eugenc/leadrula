package marketing

import (
	"log"
	"net/http"
	"net/mail"
	"strings"

	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type QuoteRequest struct {
	FullName      string `json:"full_name"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	Role          string `json:"role"`
	MonthlyVolume string `json:"monthly_volume"`
	Verticals     string `json:"verticals"`
	Message       string `json:"message"`
	Website       string `json:"website"`
}

type Handler struct {
	email   *notifications.EmailSender
	quoteTo string
	mailgun bool
}

func NewHandler(email *notifications.EmailSender, quoteTo string, mailgunConfigured bool) *Handler {
	return &Handler{email: email, quoteTo: quoteTo, mailgun: mailgunConfigured}
}

func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/public/quote", h.submitQuote)
}

func (h *Handler) submitQuote(w http.ResponseWriter, r *http.Request) {
	if h.mailgun && h.quoteTo == "" {
		httpx.Err(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable, "quote requests are temporarily unavailable")
		return
	}

	var body QuoteRequest
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}

	sub, err := normalizeQuote(body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	if strings.TrimSpace(body.Website) != "" {
		httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	log.Printf("marketing quote request from %s ip=%s", sub.Email, r.RemoteAddr)

	if err := h.email.SendQuoteRequest(h.quoteTo, sub); err != nil {
		log.Printf("marketing quote sales email failed: %v", err)
		httpx.Err(w, http.StatusInternalServerError, httpx.CodeInternal, "failed to send quote request")
		return
	}
	if err := h.email.SendQuoteConfirmation(sub.Email, sub.FullName); err != nil {
		log.Printf("marketing quote confirmation email failed: %v", err)
	}

	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func normalizeQuote(body QuoteRequest) (notifications.QuoteSubmission, error) {
	sub := notifications.QuoteSubmission{
		FullName:      trimField(body.FullName, 120),
		Email:         strings.ToLower(trimField(body.Email, 254)),
		Phone:         trimField(body.Phone, 40),
		Role:          trimField(body.Role, 40),
		MonthlyVolume: trimField(body.MonthlyVolume, 40),
		Verticals:     trimField(body.Verticals, 200),
		Message:       trimField(body.Message, 4000),
	}

	if sub.FullName == "" {
		return sub, httpx.Validation("full name is required")
	}
	if sub.Email == "" {
		return sub, httpx.Validation("email is required")
	}
	if _, err := mail.ParseAddress(sub.Email); err != nil {
		return sub, httpx.Validation("email is invalid")
	}
	if sub.Role == "" {
		sub.Role = "Publisher"
	}
	if sub.MonthlyVolume == "" {
		sub.MonthlyVolume = "Under 2,500"
	}
	return sub, nil
}

func trimField(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max]
	}
	return s
}
