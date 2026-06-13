package leads

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
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
	r.With(auth.RequireRole("admin")).Post("/leads/{id}/redistribute", h.redistribute)
}

func (h *Handler) registerCommon(r chi.Router) {
	r.Get("/leads/views", h.listViews)
	r.Post("/leads/views", h.createView)
	r.Patch("/leads/views/{viewId}", h.updateView)
	r.Delete("/leads/views/{viewId}", h.deleteView)
	r.Get("/leads", h.list)
	r.Get("/leads/tags", h.listTags)
	r.With(auth.RequireRole("admin", "user")).Post("/leads", h.create)
	r.With(auth.RequireRole("admin", "user")).Post("/leads/import", h.importLeads)
	r.With(auth.RequireRole("admin")).Post("/leads/bulk", h.bulk)
	r.With(auth.RequireRole("admin")).Delete("/leads/{id}", h.delete)
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
	src := q.Get("source")
	if src == "" {
		src = q.Get("campaign")
	}
	f := ListFilters{
		Status:     q.Get("status"),
		Source:     src,
		PipelineID: parseInt(q.Get("pipeline_id")),
		StageID:    parseInt(q.Get("stage_id")),
		Assigned:   parseInt(q.Get("assigned")),
		Tag:        q.Get("tag"),
		Search:     strings.TrimSpace(q.Get("q")),
	}
	tz := h.svc.AccountTimezone(r.Context(), p.AccountID)
	f.FilterTZ = tz

	actionOn := q.Get("action_on")
	if actionOn != "" {
		if actionOn == "today" {
			actionOn = todayInTZ(tz)
		}
		f.ActionOn = actionOn
		f.ActionTZ = tz
	}
	if q.Get("action_overdue") == "1" {
		f.ActionOverdue = true
	}

	if filtersRaw := q.Get("filters"); filtersRaw != "" {
		conditions, err := ParseFiltersJSON(filtersRaw)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		f.Conditions = conditions
	}

	opts := ListOptions{
		ListFilters: f,
		Page:        int(parseInt(q.Get("page"))),
		Limit:       int(parseInt(q.Get("limit"))),
		Sort:        q.Get("sort"),
		SortDir:     q.Get("sort_dir"),
		All:         q.Get("all") == "1",
	}

	if viewID := q.Get("view_id"); viewID != "" {
		conditions, view, err := h.svc.ResolveViewFilters(r.Context(), p, viewID)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		f.Conditions = conditions
		opts.ListFilters = f
		if opts.Sort == "" && view != nil && view.Sort != "" {
			opts.Sort = view.Sort
		}
		if opts.SortDir == "" && view != nil && view.SortDir != "" {
			opts.SortDir = view.SortDir
		}
	}

	result, err := h.svc.repo.List(r.Context(), p, opts)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) listViews(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	views, err := h.svc.ListViews(r.Context(), p, r.URL.Query().Get("placement"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, views)
}

func (h *Handler) createView(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body CreateViewInput
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	v, err := h.svc.CreateView(r.Context(), p, body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, v)
}

func (h *Handler) updateView(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body UpdateViewInput
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	v, err := h.svc.UpdateView(r.Context(), p, chi.URLParam(r, "viewId"), body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

func (h *Handler) deleteView(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteView(r.Context(), p, chi.URLParam(r, "viewId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listTags(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	tags, err := h.svc.repo.ListTagSuggestions(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, tags)
}

func (h *Handler) bulk(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Action     string  `json:"action"`
		IDs        []int64 `json:"ids"`
		UserID     int64   `json:"user_id"`
		ContractID int64   `json:"contract_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	result, auditChanges, err := h.svc.Bulk(r.Context(), p, BulkParams{
		Action:     BulkAction(body.Action),
		LeadIDs:    body.IDs,
		UserID:     body.UserID,
		ContractID: body.ContractID,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	auth.ApplyImpersonationChanges(r, auditChanges)
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	n, err := h.svc.repo.Delete(r.Context(), p, []int64{id(r)})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if n == 0 {
		httpx.WriteError(w, httpx.NotFound("lead not found"))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	l, err := h.svc.repo.GetByRef(r.Context(), p, chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, l)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	leadID := id(r)
	before, err := h.svc.repo.Get(r.Context(), p, leadID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var body struct {
		Fields         map[string]*string         `json:"fields"`
		AssignedUserID *int64                     `json:"assigned_user_id"`
		ClearAssignee  bool                       `json:"clear_assignee"`
		CustomValues   map[string]json.RawMessage `json:"custom_values"`
		Tags           *[]string                  `json:"tags"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	for k := range body.Fields {
		if IsMoneyBuiltin(k) {
			httpx.WriteError(w, httpx.Validation("cost and revenue cannot be edited manually"))
			return
		}
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
	var customFieldIDs []int64
	for fidStr := range body.CustomValues {
		if fid := parseInt(fidStr); fid != 0 {
			customFieldIDs = append(customFieldIDs, fid)
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
	if body.Tags != nil {
		if err := h.svc.repo.SetTags(r.Context(), p.AccountID, leadID, *body.Tags); err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	l, err := h.svc.repo.Get(r.Context(), p, leadID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if p.Impersonator != nil {
		fieldNames, err := h.svc.repo.CustomFieldNames(r.Context(), p.AccountID, customFieldIDs)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		in := leadUpdateInput{
			Fields:         body.Fields,
			AssignedUserID: body.AssignedUserID,
			ClearAssignee:  body.ClearAssignee,
			CustomValues:   body.CustomValues,
			Tags:           body.Tags,
		}
		auth.ApplyImpersonationChanges(r, diffLeadUpdate(before, l, in, fieldNames))
	}
	// Fire outbound webhook trigger for lead update.
	h.svc.fireOutbound(r.Context(), p.AccountID, "lead.update", l, PipelineContext{
		PipelineID: l.PipelineID,
		StageID:    l.StageID,
	})
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
	l, auditChanges, err := h.svc.ChangeStage(r.Context(), p, id(r), body.StageID, body.ActionAt, body.DisqReasonID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	auth.ApplyImpersonationChanges(r, auditChanges)
	httpx.JSON(w, http.StatusOK, l)
}

func (h *Handler) setAction(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	leadID := id(r)
	before, err := h.svc.repo.Get(r.Context(), p, leadID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var body struct {
		ActionAt *time.Time `json:"action_at"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.repo.SetActionAt(r.Context(), h.svc.repo.pool, leadID, body.ActionAt); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if p.Impersonator != nil {
		auth.ApplyImpersonationChanges(r, actionAtChange(before.ActionAt, body.ActionAt))
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
	var body CreateLeadInput
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	l, err := h.svc.CreateLead(r.Context(), p, body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, l)
}

func (h *Handler) importLeads(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body ImportLeadsInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Printf("leads/import decode failed: %v", err)
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "invalid JSON body: "+err.Error())
		return
	}
	result, err := h.svc.ImportLeads(r.Context(), p, body)
	if err != nil {
		log.Printf("leads/import failed: rows=%d dest=%q pipeline=%d stage=%d: %v",
			len(body.Rows), body.Destination, body.PipelineID, body.StageID, err)
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
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

func todayInTZ(tz string) string {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return time.Now().In(loc).Format("2006-01-02")
}
