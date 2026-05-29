package leads

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterBuyer mounts the lead/board routes for the buyer namespace.
func (h *Handler) RegisterBuyer(r chi.Router) {
	h.registerCommon(r)
}

// RegisterPublisher mounts the lead routes plus publisher-only extras.
func (h *Handler) RegisterPublisher(r chi.Router) {
	h.registerCommon(r)
	r.With(auth.RequireRole("admin")).Post("/leads", h.create)
	r.With(auth.RequireRole("admin")).Post("/leads/{id}/redistribute", h.redistribute)
}

func (h *Handler) registerCommon(r chi.Router) {
	r.Get("/leads", h.list)
	r.Get("/leads/{id}", h.get)
	r.Patch("/leads/{id}", h.update)
	r.Patch("/leads/{id}/stage", h.changeStage)
	r.Patch("/leads/{id}/action", h.setAction)
	r.Get("/leads/{id}/notes", h.listNotes)
	r.Post("/leads/{id}/notes", h.addNote)
	r.Get("/leads/{id}/stage-history", h.stageHistory)
	r.Post("/leads/{id}/followers", h.addFollower)
	r.Delete("/leads/{id}/followers/{userId}", h.removeFollower)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	q := r.URL.Query()
	f := ListFilters{
		Status:     q.Get("status"),
		Campaign:   q.Get("campaign"),
		PipelineID: parseInt(q.Get("pipeline_id")),
		StageID:    parseInt(q.Get("stage_id")),
		Assigned:   parseInt(q.Get("assigned")),
	}
	items, err := h.svc.repo.List(r.Context(), p, f)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	l, err := h.svc.repo.Get(r.Context(), p, id(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, l)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	leadID := id(r)
	var body struct {
		Fields         map[string]*string         `json:"fields"`
		AssignedUserID *int64                     `json:"assigned_user_id"`
		ClearAssignee  bool                       `json:"clear_assignee"`
		CustomValues   map[string]json.RawMessage `json:"custom_values"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if len(body.Fields) > 0 {
		if err := h.svc.repo.UpdateBuiltins(r.Context(), p.AccountID, leadID, body.Fields); err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	if body.ClearAssignee {
		if err := h.svc.repo.SetAssignee(r.Context(), p.AccountID, leadID, nil); err != nil {
			httpx.WriteError(w, err)
			return
		}
	} else if body.AssignedUserID != nil {
		if err := h.svc.repo.SetAssignee(r.Context(), p.AccountID, leadID, body.AssignedUserID); err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	for fidStr, val := range body.CustomValues {
		fid := parseInt(fidStr)
		if fid == 0 {
			continue
		}
		if err := h.svc.repo.UpsertCustomValue(r.Context(), h.svc.repo.pool, leadID, fid, val); err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	l, err := h.svc.repo.Get(r.Context(), p, leadID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, l)
}

func (h *Handler) changeStage(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		StageID      int64      `json:"stage_id"`
		ActionAt     *time.Time `json:"action_at"`
		DisqReasonID *int64     `json:"disqualification_reason_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	l, err := h.svc.ChangeStage(r.Context(), p, id(r), body.StageID, body.ActionAt, body.DisqReasonID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, l)
}

func (h *Handler) setAction(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		ActionAt *time.Time `json:"action_at"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if _, err := h.svc.repo.Get(r.Context(), p, id(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.repo.SetActionAt(r.Context(), h.svc.repo.pool, id(r), body.ActionAt); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listNotes(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if _, err := h.svc.repo.Get(r.Context(), p, id(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	notes, err := h.svc.repo.ListNotes(r.Context(), id(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, notes)
}

func (h *Handler) addNote(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if _, err := h.svc.repo.Get(r.Context(), p, id(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	n, err := h.svc.repo.AddNote(r.Context(), id(r), p.UserID, body.Body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, n)
}

func (h *Handler) stageHistory(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if _, err := h.svc.repo.Get(r.Context(), p, id(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	hist, err := h.svc.repo.StageHistory(r.Context(), id(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, hist)
}

func (h *Handler) addFollower(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		UserID int64 `json:"user_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	uid := body.UserID
	if uid == 0 {
		uid = p.UserID
	}
	if err := h.svc.repo.AddFollower(r.Context(), id(r), uid); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) removeFollower(w http.ResponseWriter, r *http.Request) {
	uid, _ := strconv.ParseInt(chi.URLParam(r, "userId"), 10, 64)
	if err := h.svc.repo.RemoveFollower(r.Context(), id(r), uid); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		FirstName  string `json:"first_name"`
		LastName   string `json:"last_name"`
		Phone      string `json:"phone"`
		Email      string `json:"email"`
		PipelineID int64  `json:"pipeline_id"`
		StageID    int64  `json:"stage_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	leadID, err := h.createLead(r.Context(), p.AccountID, body.FirstName, body.LastName, body.Phone, body.Email, body.PipelineID, body.StageID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	l, err := h.svc.repo.Get(r.Context(), p, leadID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, l)
}

func (h *Handler) createLead(ctx context.Context, accountID int64, first, last, phone, email string, pipelineID, stageID int64) (int64, error) {
	pool := h.svc.repo.pool
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	leadID, _, err := h.svc.repo.InsertLead(ctx, tx, accountID, accountID, "", nil)
	if err != nil {
		return 0, err
	}
	_ = h.svc.repo.SetBuiltinField(ctx, tx, leadID, "first_name", first)
	_ = h.svc.repo.SetBuiltinField(ctx, tx, leadID, "last_name", last)
	if phone != "" {
		_ = h.svc.repo.SetBuiltinField(ctx, tx, leadID, "phone", phone)
	}
	if email != "" {
		_ = h.svc.repo.SetBuiltinField(ctx, tx, leadID, "email", email)
	}
	if pipelineID != 0 && stageID != 0 {
		if _, err := tx.Exec(ctx, `UPDATE leads SET pipeline_id=$2, stage_id=$3 WHERE id=$1`, leadID, pipelineID, stageID); err != nil {
			return 0, err
		}
	}
	return leadID, tx.Commit(ctx)
}

func (h *Handler) redistribute(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		ContractID int64 `json:"contract_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	l, err := h.svc.Redistribute(r.Context(), p, id(r), body.ContractID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, l)
}

func id(r *http.Request) int64 {
	v, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return v
}

func parseInt(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
