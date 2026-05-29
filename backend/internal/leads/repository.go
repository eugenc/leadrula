package leads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
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
	"address": true, "city": true, "state": true, "zip": true, "campaign_name": true,
}

const leadCols = `id, public_id, owner_account_id, publisher_id, contract_id,
	first_name, last_name, phone, email, address, city, state, zip, campaign_name,
	pipeline_id, stage_id, position, assigned_user_id, action_at, status,
	disqualification_reason_id, created_at, updated_at`

func scanLead(row pgx.Row) (*Lead, error) {
	l := &Lead{}
	err := row.Scan(&l.ID, &l.PublicID, &l.OwnerAccountID, &l.PublisherID, &l.ContractID,
		&l.FirstName, &l.LastName, &l.Phone, &l.Email, &l.Address, &l.City, &l.State, &l.Zip, &l.CampaignName,
		&l.PipelineID, &l.StageID, &l.Position, &l.AssignedUserID, &l.ActionAt, &l.Status,
		&l.DisqReasonID, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.NotFound("lead not found")
		}
		return nil, err
	}
	return l, nil
}

// InsertLead creates a lead in review status owned by the publisher.
func (r *Repository) InsertLead(ctx context.Context, q database.Querier, ownerAccountID, publisherID int64, campaignName string, rawPayload []byte) (int64, string, error) {
	var id int64
	var publicID string
	var cn interface{}
	if campaignName != "" {
		cn = campaignName
	}
	if len(rawPayload) == 0 {
		rawPayload = []byte("{}")
	}
	err := q.QueryRow(ctx,
		`INSERT INTO leads(owner_account_id, publisher_id, campaign_name, raw_payload, status)
		 VALUES ($1,$2,$3,$4,'review') RETURNING id, public_id`,
		ownerAccountID, publisherID, cn, rawPayload).Scan(&id, &publicID)
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
	_, err := q.Exec(ctx,
		`INSERT INTO lead_custom_values(lead_id, custom_field_id, value) VALUES ($1,$2,$3)
		 ON CONFLICT (lead_id, custom_field_id) DO UPDATE SET value = EXCLUDED.value`,
		leadID, customFieldID, valueJSON)
	return err
}

// PlaceInPipeline assigns owner + pipeline/stage + contract.
func (r *Repository) PlaceInPipeline(ctx context.Context, q database.Querier, leadID, ownerAccountID, pipelineID, stageID, contractID int64) error {
	_, err := q.Exec(ctx,
		`UPDATE leads SET owner_account_id=$2, pipeline_id=$3, stage_id=$4, contract_id=$5,
		   position = COALESCE((SELECT MAX(position)+1 FROM leads WHERE stage_id=$4),0)
		 WHERE id=$1`,
		leadID, ownerAccountID, pipelineID, stageID, contractID)
	return err
}

func (r *Repository) SetStatus(ctx context.Context, q database.Querier, leadID int64, status string) error {
	_, err := q.Exec(ctx, `UPDATE leads SET status=$2 WHERE id=$1`, leadID, status)
	return err
}

// GetByID loads a lead (no visibility check).
func (r *Repository) GetByID(ctx context.Context, q database.Querier, leadID int64) (*Lead, error) {
	return scanLead(q.QueryRow(ctx, `SELECT `+leadCols+` FROM leads WHERE id=$1`, leadID))
}

// Get loads a lead enforcing account ownership + role visibility, with custom values.
func (r *Repository) Get(ctx context.Context, p *auth.Principal, leadID int64) (*Lead, error) {
	l, err := scanLead(r.pool.QueryRow(ctx,
		`SELECT `+leadCols+` FROM leads WHERE id=$1 AND owner_account_id=$2`, leadID, p.AccountID))
	if err != nil {
		return nil, err
	}
	if !r.visible(ctx, p, l) {
		return nil, httpx.Forbidden("not permitted to view this lead")
	}
	if err := r.attachCustomValues(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
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
	Status     string
	Campaign   string
	PipelineID int64
	StageID    int64
	Assigned   int64
}

// List returns leads for the principal's account honoring role visibility.
func (r *Repository) List(ctx context.Context, p *auth.Principal, f ListFilters) ([]Lead, error) {
	args := []any{p.AccountID}
	where := "owner_account_id = $1"
	add := func(cond string, val any) {
		args = append(args, val)
		where += fmt.Sprintf(" AND %s $%d", cond, len(args))
	}
	if f.Status != "" {
		add("status =", f.Status)
	}
	if f.Campaign != "" {
		add("campaign_name =", f.Campaign)
	}
	if f.PipelineID != 0 {
		add("pipeline_id =", f.PipelineID)
	}
	if f.StageID != 0 {
		add("stage_id =", f.StageID)
	}
	if f.Assigned != 0 {
		add("assigned_user_id =", f.Assigned)
	}
	// role visibility
	switch p.Role {
	case "user":
		args = append(args, p.UserID)
		where += fmt.Sprintf(" AND assigned_user_id = $%d", len(args))
	case "follower":
		args = append(args, p.UserID)
		where += fmt.Sprintf(" AND id IN (SELECT lead_id FROM lead_followers WHERE user_id = $%d)", len(args))
	}

	rows, err := r.pool.Query(ctx, `SELECT `+leadCols+` FROM leads WHERE `+where+` ORDER BY stage_id, position, id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Lead
	var ids []int64
	for rows.Next() {
		l, err := scanLead(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *l)
		ids = append(ids, l.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return r.attachCustomValuesBatch(ctx, out)
}

// ListByAccount returns all leads for an account (publisher oversight; no role filter).
func (r *Repository) ListByAccount(ctx context.Context, accountID int64) ([]Lead, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+leadCols+` FROM leads WHERE owner_account_id=$1 ORDER BY stage_id, position, id`, accountID)
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
	_, err := r.pool.Exec(ctx, `UPDATE leads SET assigned_user_id=$3 WHERE id=$1 AND owner_account_id=$2`, leadID, accountID, userID)
	return err
}

func (r *Repository) SetActionAt(ctx context.Context, q database.Querier, leadID int64, actionAt *time.Time) error {
	_, err := q.Exec(ctx, `UPDATE leads SET action_at=$2 WHERE id=$1`, leadID, actionAt)
	return err
}

func (r *Repository) SetDisqReason(ctx context.Context, q database.Querier, leadID, reasonID int64) error {
	_, err := q.Exec(ctx, `UPDATE leads SET disqualification_reason_id=$2 WHERE id=$1`, leadID, reasonID)
	return err
}

// StageInfo describes a destination stage's prompts and owning account.
type StageInfo struct {
	ID                     int64
	PipelineID             int64
	AccountID              int64
	PromptActionDatetime   bool
	PromptDisqualification bool
}

func (r *Repository) GetStage(ctx context.Context, q database.Querier, stageID int64) (*StageInfo, error) {
	si := &StageInfo{}
	err := q.QueryRow(ctx,
		`SELECT st.id, st.pipeline_id, p.account_id, st.prompt_action_datetime, st.prompt_disqualification
		 FROM pipeline_stages st JOIN pipelines p ON p.id = st.pipeline_id
		 WHERE st.id = $1`, stageID).Scan(&si.ID, &si.PipelineID, &si.AccountID, &si.PromptActionDatetime, &si.PromptDisqualification)
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

// AssignedUserIDForLead returns the assignee (for notifications), if any.
func (r *Repository) ReasonBelongsToAccount(ctx context.Context, q database.Querier, accountID, reasonID int64) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM disqualification_reasons WHERE id=$1 AND account_id=$2)`, reasonID, accountID).Scan(&ok)
	return ok, err
}
