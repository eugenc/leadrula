// Package calendar exposes calendar views over leads.action_at.
package calendar

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/collaboration"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	LeadID     int64     `json:"lead_id"`
	Title      string    `json:"title"`
	StageID    *int64    `json:"stage_id"`
	PipelineID *int64    `json:"pipeline_id"`
	UserID     *int64    `json:"user_id"`
	ActionAt   time.Time `json:"action_at"`
	Overdue    bool      `json:"overdue"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// List returns calendar events for an account. If userID != 0, filters to that
// user's assigned leads.
func (s *Service) List(ctx context.Context, p *auth.Principal, userID int64, from, to *time.Time) ([]Event, error) {
	where := `l.owner_account_id = $1 AND l.action_at IS NOT NULL AND l.deleted_at IS NULL`
	args := []any{p.AccountID}
	if userID != 0 {
		args = append(args, userID)
		where += fmt.Sprintf(` AND l.assigned_user_id = $%d`, len(args))
	}
	if from != nil {
		args = append(args, from)
		where += fmt.Sprintf(` AND l.action_at >= $%d`, len(args))
	}
	if to != nil {
		args = append(args, to)
		where += fmt.Sprintf(` AND l.action_at <= $%d`, len(args))
	}
	where, args = collaboration.AppendLeadScope(p, where, args)

	rows, err := s.pool.Query(ctx,
		`SELECT l.id, (l.first_name || ' ' || l.last_name), l.stage_id, l.pipeline_id, l.assigned_user_id, l.action_at
		 FROM leads l
		 WHERE `+where+`
		 ORDER BY l.action_at`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.LeadID, &e.Title, &e.StageID, &e.PipelineID, &e.UserID, &e.ActionAt); err != nil {
			return nil, err
		}
		e.Overdue = e.ActionAt.Before(now)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListForAccount returns calendar events without collaboration scoping (publisher oversight).
func (s *Service) ListForAccount(ctx context.Context, accountID, userID int64, from, to *time.Time) ([]Event, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, title, stage_id, pipeline_id, user_id, action_at
		 FROM v_calendar
		 WHERE account_id = $1
		   AND ($2 = 0 OR user_id = $2)
		   AND ($3::timestamptz IS NULL OR action_at >= $3)
		   AND ($4::timestamptz IS NULL OR action_at <= $4)
		 ORDER BY action_at`,
		accountID, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.LeadID, &e.Title, &e.StageID, &e.PipelineID, &e.UserID, &e.ActionAt); err != nil {
			return nil, err
		}
		e.Overdue = e.ActionAt.Before(now)
		out = append(out, e)
	}
	return out, rows.Err()
}

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/calendar/global", h.global)
	r.Get("/calendar/me", h.me)
}

func (h *Handler) global(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	from, to := parseRange(r)
	items, err := h.svc.List(r.Context(), p, 0, from, to)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	from, to := parseRange(r)
	items, err := h.svc.List(r.Context(), p, p.UserID, from, to)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func parseRange(r *http.Request) (*time.Time, *time.Time) {
	var from, to *time.Time
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = &t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = &t
		}
	}
	return from, to
}
