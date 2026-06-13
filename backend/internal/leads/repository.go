package leads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/collaboration"
	"github.com/echayko/leadrula/backend/internal/customfields"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

var builtinFields = map[string]bool{
	"first_name": true, "last_name": true, "phone": true, "email": true,
	"address": true, "city": true, "state": true, "zip": true, "source": true,
	"external_id": true,
}

const leadCols = `id, public_id, owner_account_id, publisher_id, contract_id,
	first_name, last_name, phone, email, address, city, state, zip, source, external_id,
	cost, revenue,
	pipeline_id, stage_id, position, assigned_user_id, action_at, status,
	disqualification_reason_id, created_at, updated_at, tags`

const leadNotDeleted = `deleted_at IS NULL`

func scanLead(row pgx.Row) (*Lead, error) {
	l := &Lead{}
	err := row.Scan(&l.ID, &l.PublicID, &l.OwnerAccountID, &l.PublisherID, &l.ContractID,
		&l.FirstName, &l.LastName, &l.Phone, &l.Email, &l.Address, &l.City, &l.State, &l.Zip, &l.Source, &l.ExternalID,
		&l.Cost, &l.Revenue,
		&l.PipelineID, &l.StageID, &l.Position, &l.AssignedUserID, &l.ActionAt, &l.Status,
		&l.DisqReasonID, &l.CreatedAt, &l.UpdatedAt, &l.Tags)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("lead not found")
		}
		return nil, err
	}
	if l.Tags == nil {
		l.Tags = []string{}
	}
	return l, nil
}

// InsertLead creates a lead in review status owned by the publisher.
func (r *Repository) InsertLead(ctx context.Context, q database.Querier, ownerAccountID, publisherID int64, source string, rawPayload []byte) (int64, string, error) {
	var id int64
	var publicID string
	var src interface{}
	if source != "" {
		src = source
	}
	if len(rawPayload) == 0 {
		rawPayload = []byte("{}")
	}
	err := q.QueryRow(ctx,
		`INSERT INTO leads(owner_account_id, publisher_id, source, raw_payload, status)
		 VALUES ($1,$2,$3,$4,'review') RETURNING id, public_id`,
		ownerAccountID, publisherID, src, rawPayload).Scan(&id, &publicID)
	return id, publicID, err
}

// SetBuiltinField writes a whitelisted built-in column.
func (r *Repository) SetBuiltinField(ctx context.Context, q database.Querier, leadID int64, field, value string) error {
	if !builtinFields[field] {
		return fmt.Errorf("unknown builtin field %q", field)
	}
	sql := fmt.Sprintf(`UPDATE leads SET %s = $2 WHERE id = $1`, field)
	_, err := q.Exec(ctx, sql, leadID, value)
	return err
}

func (r *Repository) UpsertCustomValue(ctx context.Context, q database.Querier, leadID, customFieldID int64, valueJSON []byte) error {
	var ftype string
	var format *string
	err := q.QueryRow(ctx, `SELECT type, format FROM custom_fields WHERE id = $1`, customFieldID).Scan(&ftype, &format)
	if err == nil && (ftype == "date" || ftype == "datetime") {
		field := customfields.CustomField{Type: ftype, Format: format}
		if normalized, normErr := customfields.NormalizeValue(field, valueJSON); normErr == nil {
			valueJSON = normalized
		}
	}
	_, err = q.Exec(ctx,
		`INSERT INTO lead_custom_values(lead_id, custom_field_id, value) VALUES ($1,$2,$3)
		 ON CONFLICT (lead_id, custom_field_id) DO UPDATE SET value = EXCLUDED.value`,
		leadID, customFieldID, valueJSON)
	return err
}

// PlaceInPipeline assigns owner + pipeline/stage + contract.
func (r *Repository) PlaceInPipeline(ctx context.Context, q database.Querier, leadID, ownerAccountID, pipelineID, stageID int64, contractID *int64) error {
	_, err := q.Exec(ctx,
		`UPDATE leads SET owner_account_id=$2, pipeline_id=$3, stage_id=$4, contract_id=$5,
		   position = COALESCE((SELECT MAX(position)+1 FROM leads WHERE stage_id=$4),0)
		 WHERE id=$1`,
		leadID, ownerAccountID, pipelineID, stageID, contractID)
	return err
}

// TransferOwner reassigns a lead to a buyer without placing it in a pipeline.
func (r *Repository) TransferOwner(ctx context.Context, q database.Querier, leadID, buyerID int64, contractID *int64) error {
	_, err := q.Exec(ctx,
		`UPDATE leads SET owner_account_id=$2, contract_id=$3, pipeline_id=NULL, stage_id=NULL, position=0
		 WHERE id=$1`,
		leadID, buyerID, contractID)
	return err
}

func (r *Repository) SetStatus(ctx context.Context, q database.Querier, leadID int64, status string) error {
	_, err := q.Exec(ctx, `UPDATE leads SET status=$2 WHERE id=$1`, leadID, status)
	return err
}

// GetByID loads a lead (no visibility check).
func (r *Repository) GetByID(ctx context.Context, q database.Querier, leadID int64) (*Lead, error) {
	return scanLead(q.QueryRow(ctx, `SELECT `+leadCols+` FROM leads WHERE id=$1 AND `+leadNotDeleted, leadID))
}

// Get loads a lead enforcing account ownership + role visibility, with custom values.
func (r *Repository) Get(ctx context.Context, p *auth.Principal, leadID int64) (*Lead, error) {
	return r.GetByRef(ctx, p, strconv.FormatInt(leadID, 10))
}

// GetByRef loads a lead by numeric id or public_id UUID string.
func (r *Repository) GetByRef(ctx context.Context, p *auth.Principal, ref string) (*Lead, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, httpx.NotFound("lead not found")
	}
	var l *Lead
	var err error
	if leadID, parseErr := strconv.ParseInt(ref, 10, 64); parseErr == nil && leadID > 0 {
		l, err = scanLead(r.pool.QueryRow(ctx,
			`SELECT `+leadCols+` FROM leads WHERE id=$1 AND owner_account_id=$2 AND `+leadNotDeleted, leadID, p.AccountID))
	} else {
		l, err = r.GetByPublicID(ctx, r.pool, p.AccountID, ref)
	}
	if err != nil {
		return nil, err
	}
	if !r.CollaborationLeadAllowed(ctx, p, l) {
		return nil, httpx.NotFound("lead not found")
	}
	if !r.visible(ctx, p, l) {
		return nil, httpx.Forbidden("not permitted to view this lead")
	}
	if err := r.attachCustomValues(ctx, l); err != nil {
		return nil, err
	}
	if err := r.attachLeadNames(ctx, l); err != nil {
		return nil, err
	}
	if err := r.EnrichLeadEconomics(ctx, p.AccountType, l); err != nil {
		return nil, err
	}
	return l, nil
}

func (r *Repository) attachLeadNames(ctx context.Context, l *Lead) error {
	return r.pool.QueryRow(ctx,
		`SELECT CASE WHEN ba.type = 'buyer' THEN ba.name ELSE NULL END, rs.name,
		        u.full_name, u.prefs->>'avatar_url', pl.name, st.name
		 FROM leads l
		 LEFT JOIN accounts ba ON ba.id = l.owner_account_id AND ba.type = 'buyer'
		 LEFT JOIN routing_sources rs ON rs.slug = l.source AND rs.publisher_id = l.publisher_id
		 LEFT JOIN users u ON u.id = l.assigned_user_id
		 LEFT JOIN pipelines pl ON pl.id = l.pipeline_id
		 LEFT JOIN pipeline_stages st ON st.id = l.stage_id
		 WHERE l.id = $1`, l.ID).Scan(&l.BuyerName, &l.SourceName, &l.AssigneeName, &l.AssigneeAvatarURL, &l.PipelineName, &l.StageName)
}

func (r *Repository) attachCustomValues(ctx context.Context, l *Lead) error {
	rows, err := r.pool.Query(ctx, `SELECT custom_field_id, value FROM lead_custom_values WHERE lead_id=$1`, l.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	l.CustomValues = map[string]json.RawMessage{}
	for rows.Next() {
		var fid int64
		var val json.RawMessage
		if err := rows.Scan(&fid, &val); err != nil {
			return err
		}
		l.CustomValues[fmt.Sprintf("%d", fid)] = val
	}
	return rows.Err()
}

func (r *Repository) CollaborationLeadAllowed(ctx context.Context, p *auth.Principal, l *Lead) bool {
	pubID, ok := p.CollaborationPublisherID()
	if !ok {
		return true
	}
	if l.PublisherID != pubID || l.OwnerAccountID != p.AccountID {
		return false
	}
	if l.ContractID == nil {
		return true
	}
	var valid bool
	_ = r.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM contracts c
			WHERE c.id = $1 AND c.publisher_id = $2 AND c.buyer_id = $3
			  AND c.status = 'active' AND c.deleted_at IS NULL
		)`, *l.ContractID, pubID, p.AccountID).Scan(&valid)
	return valid
}

func (r *Repository) visible(ctx context.Context, p *auth.Principal, l *Lead) bool {
	switch p.Role {
	case "admin":
		return true
	case "user":
		return l.AssignedUserID != nil && *l.AssignedUserID == p.UserID
	case "follower":
		var ok bool
		_ = r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM lead_followers WHERE lead_id=$1 AND user_id=$2)`, l.ID, p.UserID).Scan(&ok)
		return ok
	}
	return false
}

type ListFilters struct {
	Status        string
	Source        string
	PipelineID    int64
	StageID       int64
	Assigned      int64
	Tag           string
	ActionOn      string
	ActionTZ      string
	ActionOverdue bool
	Conditions    []FilterCondition
	FilterTZ      string
	Search        string
}

type ListOptions struct {
	ListFilters
	Page    int
	Limit   int
	Sort    string
	SortDir string
	All     bool
}

var listSortCols = map[string]string{
	"created_at":    "l.created_at",
	"updated_at":    "l.updated_at",
	"first_name":    "l.first_name",
	"last_name":     "l.last_name",
	"phone":         "l.phone",
	"email":         "l.email",
	"source": "l.source",
	"source_name":    "rs.name",
	"status":        "l.status",
	"action_at":     "l.action_at",
	"buyer_name":     "buyer_name",
	"assignee_name":  "assignee_name",
	"pipeline_name":    "pl.name",
	"stage_name":       "st.name",
	"stage_entered_at": "stage_entered_at",
	"position":         "l.position",
}

const listFrom = ` FROM leads l
	LEFT JOIN accounts ba ON ba.id = l.owner_account_id AND ba.type = 'buyer'
	LEFT JOIN routing_sources rs ON rs.slug = l.source AND rs.publisher_id = l.publisher_id
	LEFT JOIN users u ON u.id = l.assigned_user_id
	LEFT JOIN pipelines pl ON pl.id = l.pipeline_id
	LEFT JOIN pipeline_stages st ON st.id = l.stage_id`

const listSelect = `l.id, l.public_id, l.owner_account_id, l.publisher_id, l.contract_id,
	l.first_name, l.last_name, l.phone, l.email, l.address, l.city, l.state, l.zip, l.source, l.external_id,
	l.cost, l.revenue,
	l.pipeline_id, l.stage_id, l.position, l.assigned_user_id, l.action_at, l.status,
	l.disqualification_reason_id, l.created_at, l.updated_at, l.tags,
	CASE WHEN ba.type = 'buyer' THEN ba.name ELSE NULL END AS buyer_name,
	rs.name AS source_name,
	u.full_name AS assignee_name,
	u.prefs->>'avatar_url' AS assignee_avatar_url,
	pl.name AS pipeline_name,
	st.name AS stage_name,
	COALESCE(
		(SELECT h.created_at FROM lead_stage_history h
		 WHERE h.lead_id = l.id AND h.to_stage_id = l.stage_id
		 ORDER BY h.created_at DESC, h.id DESC LIMIT 1),
		l.created_at
	) AS stage_entered_at`

func scanListLead(row pgx.Row) (*Lead, error) {
	l := &Lead{}
	err := row.Scan(&l.ID, &l.PublicID, &l.OwnerAccountID, &l.PublisherID, &l.ContractID,
		&l.FirstName, &l.LastName, &l.Phone, &l.Email, &l.Address, &l.City, &l.State, &l.Zip, &l.Source, &l.ExternalID,
		&l.Cost, &l.Revenue,
		&l.PipelineID, &l.StageID, &l.Position, &l.AssignedUserID, &l.ActionAt, &l.Status,
		&l.DisqReasonID, &l.CreatedAt, &l.UpdatedAt, &l.Tags, &l.BuyerName, &l.SourceName, &l.AssigneeName, &l.AssigneeAvatarURL,
		&l.PipelineName, &l.StageName, &l.StageEnteredAt)
	if err != nil {
		return nil, err
	}
	if l.Tags == nil {
		l.Tags = []string{}
	}
	return l, nil
}

func (r *Repository) listWhere(p *auth.Principal, f ListFilters) (string, []any) {
	args := []any{p.AccountID}
	where := "l.owner_account_id = $1 AND l.deleted_at IS NULL"
	add := func(cond string, val any) {
		args = append(args, val)
		where += fmt.Sprintf(" AND %s $%d", cond, len(args))
	}

	switch p.Role {
	case "user":
		args = append(args, p.UserID)
		where += fmt.Sprintf(" AND l.assigned_user_id = $%d", len(args))
	case "follower":
		args = append(args, p.UserID)
		where += fmt.Sprintf(" AND l.id IN (SELECT lead_id FROM lead_followers WHERE user_id = $%d)", len(args))
	}

	if len(f.Conditions) > 0 {
		conditions := append([]FilterCondition{}, f.Conditions...)
		extra := flatFiltersToConditions(ListFilters{
			Status: f.Status, Source: f.Source, PipelineID: f.PipelineID,
			StageID: f.StageID, Assigned: f.Assigned, Tag: f.Tag,
			ActionOn: f.ActionOn, ActionTZ: f.ActionTZ, ActionOverdue: f.ActionOverdue,
		})
		conditions = append(conditions, extra...)
		tz := f.FilterTZ
		if tz == "" {
			tz = f.ActionTZ
		}
		if tz == "" {
			tz = "UTC"
		}
		var err error
		where, args, err = appendCompiledFilters(where, args, conditions, FilterContext{UserID: p.UserID, TZ: tz})
		if err != nil {
			where += " AND false"
		}
	} else {
		if f.Status != "" {
			add("l.status =", f.Status)
		}
		if f.Source != "" {
			add("l.source =", f.Source)
		}
		if f.PipelineID != 0 {
			add("l.pipeline_id =", f.PipelineID)
		}
		if f.StageID != 0 {
			add("l.stage_id =", f.StageID)
		}
		if f.Assigned != 0 {
			add("l.assigned_user_id =", f.Assigned)
		}
		if f.Tag != "" {
			args = append(args, f.Tag)
			where += fmt.Sprintf(" AND $%d = ANY(l.tags)", len(args))
		}
		if f.ActionOverdue {
			where += " AND l.action_at IS NOT NULL AND l.action_at < now()"
		}
		if f.ActionOn != "" && f.ActionTZ != "" {
			args = append(args, f.ActionOn, f.ActionTZ)
			dateArg := len(args) - 1
			tzArg := len(args)
			where += fmt.Sprintf(
				" AND l.action_at >= ($%d::date AT TIME ZONE $%d) AND l.action_at < (($%d::date + interval '1 day') AT TIME ZONE $%d)",
				dateArg, tzArg, dateArg, tzArg,
			)
		}
	}

	if f.Search != "" {
		where, args = appendLeadSearch(where, args, f.Search)
	}
	where, args = collaboration.AppendLeadScope(p, where, args)
	return where, args
}

const leadStatusSearchLabel = `CASE l.status
	WHEN 'review' THEN 'in review'
	WHEN 'distributed' THEN 'distributed'
	WHEN 'returned' THEN 'returned'
	WHEN 'closed' THEN 'closed'
	ELSE l.status
END`

func appendLeadSearch(where string, args []any, term string) (string, []any) {
	term = strings.TrimSpace(term)
	if term == "" {
		return where, args
	}
	like := "%" + term + "%"
	args = append(args, like)
	n := len(args)
	where += fmt.Sprintf(` AND (
		l.first_name ILIKE $%d OR
		l.last_name ILIKE $%d OR
		(l.first_name || ' ' || l.last_name) ILIKE $%d OR
		l.email ILIKE $%d OR
		l.phone ILIKE $%d OR
		l.public_id::text ILIKE $%d OR
		l.address ILIKE $%d OR
		l.city ILIKE $%d OR
		l.state ILIKE $%d OR
		l.zip ILIKE $%d OR
		concat_ws(' ', l.address, l.city, l.state, l.zip) ILIKE $%d OR
		ba.name ILIKE $%d OR
		l.status ILIKE $%d OR
		(%s) ILIKE $%d
	)`, n, n, n, n, n, n, n, n, n, n, n, n, n, leadStatusSearchLabel, n)
	return where, args
}

func sortDirection(sortDir string) string {
	if sortDir == "desc" {
		return "DESC"
	}
	return "ASC"
}

func defaultListOrderBy() string {
	return "l.created_at DESC, l.id DESC"
}

func listOrderBy(sort, sortDir string) string {
	col, ok := listSortCols[sort]
	if !ok {
		return defaultListOrderBy()
	}
	return col + " " + sortDirection(sortDir) + " NULLS LAST, l.id DESC"
}

func (r *Repository) resolveListSort(ctx context.Context, accountID int64, sort, sortDir string, argLen int) (orderBy, extraJoin string, extraArgs []any) {
	if !strings.HasPrefix(sort, "custom_") {
		return listOrderBy(sort, sortDir), "", nil
	}
	fieldID, err := strconv.ParseInt(strings.TrimPrefix(sort, "custom_"), 10, 64)
	if err != nil {
		return defaultListOrderBy(), "", nil
	}
	var fieldType string
	err = r.pool.QueryRow(ctx,
		`SELECT type FROM custom_fields WHERE id=$1 AND account_id=$2`, fieldID, accountID).Scan(&fieldType)
	if err != nil {
		return defaultListOrderBy(), "", nil
	}
	dir := sortDirection(sortDir)
	var orderExpr string
	switch fieldType {
	case "number":
		orderExpr = "(sort_cv.value)::text::numeric"
	case "date", "datetime":
		orderExpr = "(sort_cv.value #>> '{}')::timestamptz"
	default:
		orderExpr = "sort_cv.value #>> '{}'"
	}
	joinArg := argLen + 1
	return orderExpr + " " + dir + " NULLS LAST, l.id DESC",
		fmt.Sprintf(" LEFT JOIN lead_custom_values sort_cv ON sort_cv.lead_id = l.id AND sort_cv.custom_field_id = $%d", joinArg),
		[]any{fieldID}
}

// List returns leads for the principal's account honoring role visibility.
func (r *Repository) List(ctx context.Context, p *auth.Principal, o ListOptions) (*ListResult, error) {
	where, args := r.listWhere(p, o.ListFilters)

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) `+listFrom+` WHERE `+where, args...).Scan(&total); err != nil {
		return nil, err
	}

	page := o.Page
	if page < 1 {
		page = 1
	}
	limit := o.Limit
	if o.All || limit <= 0 {
		limit = total
		if limit == 0 {
			limit = 1
		}
		page = 1
	}
	offset := (page - 1) * limit

	orderBy, extraJoin, sortArgs := r.resolveListSort(ctx, p.AccountID, o.Sort, o.SortDir, len(args))
	qArgs := append([]any{}, args...)
	qArgs = append(qArgs, sortArgs...)
	q := `SELECT ` + listSelect + listFrom + extraJoin + ` WHERE ` + where + ` ORDER BY ` + orderBy
	if !o.All && o.Limit > 0 {
		q += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(qArgs)+1, len(qArgs)+2)
		qArgs = append(qArgs, limit, offset)
	}

	rows, err := r.pool.Query(ctx, q, qArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Lead
	for rows.Next() {
		l, err := scanListLead(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items, err := r.attachCustomValuesBatch(ctx, out)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []Lead{}
	}
	if err := r.EnrichLeadEconomicsBatch(ctx, p.AccountType, items); err != nil {
		return nil, err
	}
	return &ListResult{Items: items, Total: total, Page: page, Limit: limit}, nil
}

// ListByAccount returns all leads for an account (publisher oversight; no role filter).
func (r *Repository) ListByAccount(ctx context.Context, accountID int64) ([]Lead, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+leadCols+` FROM leads WHERE owner_account_id=$1 AND `+leadNotDeleted+` ORDER BY stage_id, position, id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Lead
	for rows.Next() {
		l, err := scanLead(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return r.attachCustomValuesBatch(ctx, out)
}

func (r *Repository) attachCustomValuesBatch(ctx context.Context, leads []Lead) ([]Lead, error) {
	for i := range leads {
		leads[i].CustomValues = map[string]json.RawMessage{}
	}
	if len(leads) == 0 {
		return leads, nil
	}
	index := map[int64]int{}
	ids := make([]int64, len(leads))
	for i := range leads {
		ids[i] = leads[i].ID
		index[leads[i].ID] = i
	}
	rows, err := r.pool.Query(ctx,
		`SELECT lead_id, custom_field_id, value FROM lead_custom_values WHERE lead_id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var leadID, fid int64
		var val json.RawMessage
		if err := rows.Scan(&leadID, &fid, &val); err != nil {
			return nil, err
		}
		if i, ok := index[leadID]; ok {
			leads[i].CustomValues[fmt.Sprintf("%d", fid)] = val
		}
	}
	return leads, rows.Err()
}

// ── Updates ───────────────────────────────────────────────────────

func (r *Repository) UpdateBuiltins(ctx context.Context, accountID, leadID int64, fields map[string]*string) error {
	for f, v := range fields {
		if IsMoneyBuiltin(f) {
			continue
		}
		if !builtinFields[f] || v == nil {
			continue
		}
		sql := fmt.Sprintf(`UPDATE leads SET %s = $3 WHERE id = $1 AND owner_account_id = $2`, f)
		if _, err := r.pool.Exec(ctx, sql, leadID, accountID, *v); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) SetAssignee(ctx context.Context, accountID, leadID int64, userID *int64) error {
	return r.setAssignee(ctx, r.pool, accountID, leadID, userID)
}

func (r *Repository) setAssignee(ctx context.Context, q database.Querier, accountID, leadID int64, userID *int64) error {
	_, err := q.Exec(ctx, `UPDATE leads SET assigned_user_id=$3 WHERE id=$1 AND owner_account_id=$2`, leadID, accountID, userID)
	return err
}

func normalizeTags(tags []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	if out == nil {
		return []string{}
	}
	return out
}

func (r *Repository) SetTags(ctx context.Context, accountID, leadID int64, tags []string) error {
	return r.setTags(ctx, r.pool, accountID, leadID, tags)
}

func (r *Repository) setTags(ctx context.Context, q database.Querier, accountID, leadID int64, tags []string) error {
	normalized := normalizeTags(tags)
	_, err := q.Exec(ctx,
		`UPDATE leads SET tags=$3 WHERE id=$1 AND owner_account_id=$2`,
		leadID, accountID, normalized)
	return err
}

func (r *Repository) ListTagSuggestions(ctx context.Context, accountID int64) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT t FROM leads, unnest(tags) AS t
		 WHERE owner_account_id = $1 AND `+leadNotDeleted+` ORDER BY t`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if out == nil {
		return []string{}, rows.Err()
	}
	return out, rows.Err()
}

func (r *Repository) SetActionAt(ctx context.Context, q database.Querier, leadID int64, actionAt *time.Time) error {
	_, err := q.Exec(ctx, `UPDATE leads SET action_at=$2 WHERE id=$1`, leadID, actionAt)
	return err
}

func (r *Repository) SetDisqReason(ctx context.Context, q database.Querier, leadID, reasonID int64) error {
	_, err := q.Exec(ctx, `UPDATE leads SET disqualification_reason_id=$2 WHERE id=$1`, leadID, reasonID)
	return err
}

// StageInfo describes a destination stage and owning account.
type StageInfo struct {
	ID         int64
	PipelineID int64
	AccountID  int64
	StageType  string
}

func (r *Repository) GetStage(ctx context.Context, q database.Querier, stageID int64) (*StageInfo, error) {
	si := &StageInfo{}
	err := q.QueryRow(ctx,
		`SELECT st.id, st.pipeline_id, p.account_id, st.stage_type
		 FROM pipeline_stages st JOIN pipelines p ON p.id = st.pipeline_id
		 WHERE st.id = $1`, stageID).Scan(&si.ID, &si.PipelineID, &si.AccountID, &si.StageType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("stage not found")
	}
	return si, err
}

func (r *Repository) UpdateStage(ctx context.Context, q database.Querier, leadID, stageID int64) error {
	_, err := q.Exec(ctx,
		`UPDATE leads SET stage_id=$2, position=COALESCE((SELECT MAX(position)+1 FROM leads WHERE stage_id=$2),0)
		 WHERE id=$1`, leadID, stageID)
	return err
}

func (r *Repository) MoveToPublisher(ctx context.Context, q database.Querier, leadID, publisherID, pipelineID, stageID int64) error {
	_, err := q.Exec(ctx,
		`UPDATE leads SET owner_account_id=$2, pipeline_id=$3, stage_id=$4, contract_id=NULL, status='returned',
		   position=COALESCE((SELECT MAX(position)+1 FROM leads WHERE stage_id=$4),0)
		 WHERE id=$1`, leadID, publisherID, pipelineID, stageID)
	return err
}

func (r *Repository) InsertStageHistory(ctx context.Context, q database.Querier, leadID int64, fromStage *int64, toStage int64, userID int64, actionAt *time.Time, disqReason *int64) error {
	_, err := q.Exec(ctx,
		`INSERT INTO lead_stage_history(lead_id, from_stage_id, to_stage_id, moved_by_user_id, action_at_captured, disqualification_reason_id)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		leadID, fromStage, toStage, userID, actionAt, disqReason)
	return err
}

func (r *Repository) StageHistory(ctx context.Context, leadID int64) ([]StageHistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT h.id, h.from_stage_id, fs.name, h.to_stage_id, ts.name, u.full_name,
		        h.action_at_captured, dr.label, h.created_at
		 FROM lead_stage_history h
		 LEFT JOIN pipeline_stages fs ON fs.id = h.from_stage_id
		 LEFT JOIN pipeline_stages ts ON ts.id = h.to_stage_id
		 LEFT JOIN users u ON u.id = h.moved_by_user_id
		 LEFT JOIN disqualification_reasons dr ON dr.id = h.disqualification_reason_id
		 WHERE h.lead_id = $1 ORDER BY h.created_at DESC`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StageHistoryEntry
	for rows.Next() {
		var e StageHistoryEntry
		if err := rows.Scan(&e.ID, &e.FromStageID, &e.FromStageName, &e.ToStageID, &e.ToStageName,
			&e.MovedByName, &e.ActionAt, &e.DisqReason, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ── Notes & followers ─────────────────────────────────────────────

func (r *Repository) ListNotes(ctx context.Context, leadID int64) ([]Note, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT n.id, n.lead_id, n.user_id, COALESCE(u.full_name,''), n.body, n.created_at
		 FROM lead_notes n LEFT JOIN users u ON u.id = n.user_id
		 WHERE n.lead_id=$1 ORDER BY n.created_at DESC`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.LeadID, &n.UserID, &n.AuthorName, &n.Body, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *Repository) AddNote(ctx context.Context, leadID, userID int64, body string) (*Note, error) {
	n := &Note{}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO lead_notes(lead_id, user_id, body) VALUES ($1,$2,$3)
		 RETURNING id, lead_id, user_id, body, created_at`,
		leadID, userID, body).Scan(&n.ID, &n.LeadID, &n.UserID, &n.Body, &n.CreatedAt)
	return n, err
}

func (r *Repository) ListFollowers(ctx context.Context, leadID int64) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT user_id FROM lead_followers WHERE lead_id=$1`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repository) AddFollower(ctx context.Context, leadID, userID int64) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO lead_followers(lead_id, user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, leadID, userID)
	return err
}

func (r *Repository) RemoveFollower(ctx context.Context, leadID, userID int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM lead_followers WHERE lead_id=$1 AND user_id=$2`, leadID, userID)
	return err
}

// GetByExternalID finds an active lead by provider external_id.
func (r *Repository) GetByExternalID(ctx context.Context, q database.Querier, accountID int64, externalID string) (*Lead, error) {
	return scanLead(q.QueryRow(ctx,
		`SELECT `+leadCols+` FROM leads WHERE owner_account_id=$1 AND external_id=$2 AND `+leadNotDeleted,
		accountID, externalID))
}

// GetByPublicID finds an active lead by public_id UUID string.
func (r *Repository) GetByPublicID(ctx context.Context, q database.Querier, accountID int64, publicID string) (*Lead, error) {
	return scanLead(q.QueryRow(ctx,
		`SELECT `+leadCols+` FROM leads WHERE owner_account_id=$1 AND public_id=$2 AND `+leadNotDeleted,
		accountID, publicID))
}

// GetByPhone finds an active lead by phone.
func (r *Repository) GetByPhone(ctx context.Context, q database.Querier, accountID int64, phone string) (*Lead, error) {
	return scanLead(q.QueryRow(ctx,
		`SELECT `+leadCols+` FROM leads WHERE owner_account_id=$1 AND phone=$2 AND `+leadNotDeleted,
		accountID, phone))
}

// GetByEmail finds an active lead by email.
func (r *Repository) GetByEmail(ctx context.Context, q database.Querier, accountID int64, email string) (*Lead, error) {
	return scanLead(q.QueryRow(ctx,
		`SELECT `+leadCols+` FROM leads WHERE owner_account_id=$1 AND email=$2 AND `+leadNotDeleted,
		accountID, email))
}

// SetExternalID sets the provider external_id on a lead.
func (r *Repository) SetExternalID(ctx context.Context, q database.Querier, leadID int64, externalID string) error {
	_, err := q.Exec(ctx, `UPDATE leads SET external_id=$2 WHERE id=$1`, leadID, externalID)
	return err
}

// SoftDelete marks leads as deleted without removing rows.
func (r *Repository) SoftDelete(ctx context.Context, q database.Querier, accountID int64, leadID int64) error {
	ct, err := q.Exec(ctx,
		`UPDATE leads SET deleted_at=now() WHERE owner_account_id=$1 AND id=$2 AND `+leadNotDeleted,
		accountID, leadID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("lead not found")
	}
	return nil
}

// Delete removes leads owned by the account.
func (r *Repository) Delete(ctx context.Context, p *auth.Principal, leadIDs []int64) (int64, error) {
	where, args := r.listWhere(p, ListFilters{})
	args = append(args, leadIDs)
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM leads l WHERE `+where+` AND l.id = ANY($`+fmt.Sprint(len(args))+`)`, args...)
	if err != nil {
		if database.IsForeignKeyViolation(err) {
			return 0, httpx.BusinessRule("lead cannot be deleted while referenced by billing records")
		}
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// BulkSetAssignee sets assignee on leads owned by the account.
func (r *Repository) BulkSetAssignee(ctx context.Context, p *auth.Principal, leadIDs []int64, userID *int64) (int64, error) {
	where, args := r.listWhere(p, ListFilters{})
	userArg := len(args) + 1
	idsArg := len(args) + 2
	args = append(args, userID, leadIDs)
	tag, err := r.pool.Exec(ctx,
		`UPDATE leads l SET assigned_user_id=$`+fmt.Sprint(userArg)+` WHERE `+where+` AND l.id = ANY($`+fmt.Sprint(idsArg)+`)`,
		args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// BulkAddFollowers adds a follower to each lead.
func (r *Repository) BulkAddFollowers(ctx context.Context, p *auth.Principal, leadIDs []int64, userID int64) error {
	for _, leadID := range leadIDs {
		if _, err := r.Get(ctx, p, leadID); err != nil {
			return err
		}
		if err := r.AddFollower(ctx, leadID, userID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) StageName(ctx context.Context, q database.Querier, stageID *int64) string {
	if stageID == nil {
		return ""
	}
	var name string
	if err := q.QueryRow(ctx, `SELECT name FROM pipeline_stages WHERE id=$1`, *stageID).Scan(&name); err != nil {
		return ""
	}
	return name
}

func (r *Repository) CustomFieldNames(ctx context.Context, accountID int64, ids []int64) (map[string]string, error) {
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, name FROM custom_fields WHERE account_id=$1 AND id = ANY($2)`, accountID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[fmt.Sprintf("%d", id)] = name
	}
	return out, rows.Err()
}
