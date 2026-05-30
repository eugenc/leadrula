package pipelines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

// fieldKind drives which operators apply and how values are coerced.
type fieldKind string

const (
	kindText     fieldKind = "text"
	kindNumber   fieldKind = "number"
	kindDate     fieldKind = "date"
	kindStatus   fieldKind = "status"
	kindStage    fieldKind = "stage"
	kindUser     fieldKind = "user"
	kindDisq     fieldKind = "disq"
	kindCheckbox fieldKind = "checkbox"
	kindTags     fieldKind = "tags"
)

// leadTextBuiltins are lead columns that conditions/actions treat as plain text.
// Kept local to avoid importing the leads package (leads imports pipelines).
var leadTextBuiltins = map[string]bool{
	"first_name": true, "last_name": true, "phone": true, "email": true,
	"address": true, "city": true, "state": true, "zip": true, "source": true,
}

var knownOps = map[string]bool{
	"eq": true, "neq": true, "gt": true, "lt": true,
	"contains": true, "empty": true, "not_empty": true,
}

func customFieldKind(t string) fieldKind {
	switch t {
	case "number":
		return kindNumber
	case "date", "datetime":
		return kindDate
	case "checkbox":
		return kindCheckbox
	default:
		return kindText // text, dropdown
	}
}

// conditionFieldKind resolves the kind for a condition's domain/field, using the
// account's custom-field definitions for custom:{key} fields.
func conditionFieldKind(domain, field string, customByKey map[string]customFieldDef) (fieldKind, bool) {
	switch domain {
	case "lead":
		if strings.HasPrefix(field, "custom:") {
			d, ok := customByKey[strings.TrimPrefix(field, "custom:")]
			return d.kind, ok
		}
		switch field {
		case "status":
			return kindStatus, true
		case "action_at":
			return kindDate, true
		case "assigned_user_id":
			return kindUser, true
		case "disqualification_reason_id":
			return kindDisq, true
		case "tags":
			return kindTags, true
		}
		if leadTextBuiltins[field] {
			return kindText, true
		}
	case "pipeline":
		switch field {
		case "previous_stage_id":
			return kindStage, true
		case "days_in_previous_stage":
			return kindNumber, true
		}
	}
	return "", false
}

type customFieldDef struct {
	id   int64
	kind fieldKind
}

// ruleEvalContext is the lead + pipeline snapshot a rule is evaluated against.
type ruleEvalContext struct {
	builtins     map[string]*string
	status       string
	actionAt     *time.Time
	assignedUser *int64
	disqReason   *int64
	tags         []string
	stageID      *int64
	createdAt    time.Time
	customByKey  map[string]customFieldDef
	customByID   map[int64]json.RawMessage
	prevStageID  *int64
	daysInPrev   float64
}

func buildEvalContext(ctx context.Context, q database.Querier, accountID, leadID int64, fromStageID *int64) (*ruleEvalContext, error) {
	ec := &ruleEvalContext{
		builtins:    map[string]*string{},
		customByKey: map[string]customFieldDef{},
		customByID:  map[int64]json.RawMessage{},
		prevStageID: fromStageID,
	}

	var firstName, lastName string
	var phone, email, address, city, state, zip, source *string
	err := q.QueryRow(ctx,
		`SELECT first_name, last_name, phone, email, address, city, state, zip, source,
		        status, action_at, assigned_user_id, disqualification_reason_id, tags, stage_id, created_at
		 FROM leads WHERE id=$1`, leadID).Scan(
		&firstName, &lastName, &phone, &email, &address, &city, &state, &zip, &source,
		&ec.status, &ec.actionAt, &ec.assignedUser, &ec.disqReason, &ec.tags, &ec.stageID, &ec.createdAt)
	if err != nil {
		return nil, err
	}
	ec.builtins["first_name"] = &firstName
	ec.builtins["last_name"] = &lastName
	ec.builtins["phone"] = phone
	ec.builtins["email"] = email
	ec.builtins["address"] = address
	ec.builtins["city"] = city
	ec.builtins["state"] = state
	ec.builtins["zip"] = zip
	ec.builtins["source"] = source

	defRows, err := q.Query(ctx, `SELECT id, field_key, type FROM custom_fields WHERE account_id=$1`, accountID)
	if err != nil {
		return nil, err
	}
	for defRows.Next() {
		var id int64
		var key, ftype string
		if err := defRows.Scan(&id, &key, &ftype); err != nil {
			defRows.Close()
			return nil, err
		}
		ec.customByKey[key] = customFieldDef{id: id, kind: customFieldKind(ftype)}
	}
	defRows.Close()
	if err := defRows.Err(); err != nil {
		return nil, err
	}

	valRows, err := q.Query(ctx, `SELECT custom_field_id, value FROM lead_custom_values WHERE lead_id=$1`, leadID)
	if err != nil {
		return nil, err
	}
	for valRows.Next() {
		var fid int64
		var val json.RawMessage
		if err := valRows.Scan(&fid, &val); err != nil {
			valRows.Close()
			return nil, err
		}
		ec.customByID[fid] = val
	}
	valRows.Close()
	if err := valRows.Err(); err != nil {
		return nil, err
	}

	if fromStageID != nil {
		var enteredAt time.Time
		err := q.QueryRow(ctx,
			`SELECT created_at FROM lead_stage_history
			 WHERE lead_id=$1 AND to_stage_id=$2 ORDER BY created_at DESC, id DESC LIMIT 1`,
			leadID, *fromStageID).Scan(&enteredAt)
		switch {
		case err == nil:
			ec.daysInPrev = time.Since(enteredAt).Hours() / 24
		case errors.Is(err, pgx.ErrNoRows):
			ec.daysInPrev = time.Since(ec.createdAt).Hours() / 24
		default:
			return nil, err
		}
	}
	return ec, nil
}

func (e *ruleEvalContext) evalCondition(c RuleCondition) bool {
	kind, ok := conditionFieldKind(c.Domain, c.Field, e.customByKey)
	if !ok {
		return false
	}
	switch kind {
	case kindText:
		s, present := e.textValue(c.Field)
		return evalText(s, present, c.Op, c.Value)
	case kindStatus:
		return evalEnumStr(e.status, c.Op, c.Value)
	case kindNumber:
		n, present := e.numberValue(c.Domain, c.Field)
		return evalNumber(n, present, c.Op, c.Value)
	case kindDate:
		return evalDate(e.dateValue(c.Field), c.Op, c.Value)
	case kindUser:
		return evalIntRef(e.assignedUser, c.Op, c.Value)
	case kindStage:
		return evalIntRef(e.prevStageID, c.Op, c.Value)
	case kindDisq:
		return evalIntRef(e.disqReason, c.Op, c.Value)
	case kindCheckbox:
		b, present := e.checkboxValue(c.Field)
		return evalCheckbox(b, present, c.Op, c.Value)
	case kindTags:
		return evalTags(e.tags, c.Op, c.Value)
	}
	return false
}

func (e *ruleEvalContext) customRaw(field string) (json.RawMessage, bool) {
	d, ok := e.customByKey[strings.TrimPrefix(field, "custom:")]
	if !ok {
		return nil, false
	}
	raw, ok := e.customByID[d.id]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	return raw, true
}

func (e *ruleEvalContext) textValue(field string) (string, bool) {
	if strings.HasPrefix(field, "custom:") {
		raw, ok := e.customRaw(field)
		if !ok {
			return "", false
		}
		return rawToString(raw), true
	}
	if p := e.builtins[field]; p != nil {
		return *p, true
	}
	return "", false
}

func (e *ruleEvalContext) numberValue(domain, field string) (float64, bool) {
	if domain == "pipeline" && field == "days_in_previous_stage" {
		if e.prevStageID == nil {
			return 0, false
		}
		return e.daysInPrev, true
	}
	raw, ok := e.customRaw(field)
	if !ok {
		return 0, false
	}
	return rawToFloat(raw)
}

func (e *ruleEvalContext) dateValue(field string) *time.Time {
	if field == "action_at" {
		return e.actionAt
	}
	raw, ok := e.customRaw(field)
	if !ok {
		return nil
	}
	var s string
	if json.Unmarshal(raw, &s) != nil || s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func (e *ruleEvalContext) checkboxValue(field string) (bool, bool) {
	raw, ok := e.customRaw(field)
	if !ok {
		return false, false
	}
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return b, true
	}
	return false, false
}

// ── operator evaluation ────────────────────────────────────────────

func evalText(s string, present bool, op string, cv json.RawMessage) bool {
	switch op {
	case "empty":
		return !present || strings.TrimSpace(s) == ""
	case "not_empty":
		return present && strings.TrimSpace(s) != ""
	}
	target := rawToString(cv)
	switch op {
	case "eq":
		return strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(target))
	case "neq":
		return !strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(target))
	case "contains":
		return strings.Contains(strings.ToLower(s), strings.ToLower(target))
	}
	return false
}

func evalEnumStr(s, op string, cv json.RawMessage) bool {
	target := rawToString(cv)
	switch op {
	case "eq":
		return s == target
	case "neq":
		return s != target
	}
	return false
}

func evalNumber(n float64, present bool, op string, cv json.RawMessage) bool {
	if !present {
		return false
	}
	target, ok := rawToFloat(cv)
	if !ok {
		return false
	}
	switch op {
	case "eq":
		return n == target
	case "gt":
		return n > target
	case "lt":
		return n < target
	}
	return false
}

func evalDate(t *time.Time, op string, cv json.RawMessage) bool {
	switch op {
	case "empty":
		return t == nil
	case "not_empty":
		return t != nil
	}
	if t == nil {
		return false
	}
	ref := time.Now().UTC().AddDate(0, 0, daysFromValue(cv))
	leadT := t.UTC()
	switch op {
	case "eq":
		return sameDay(leadT, ref)
	case "gt":
		return leadT.After(ref)
	case "lt":
		return leadT.Before(ref)
	}
	return false
}

func evalIntRef(id *int64, op string, cv json.RawMessage) bool {
	switch op {
	case "empty":
		return id == nil
	case "not_empty":
		return id != nil
	}
	target, ok := rawToInt(cv)
	switch op {
	case "eq":
		return id != nil && ok && *id == target
	case "neq":
		return !(id != nil && ok && *id == target)
	}
	return false
}

func evalCheckbox(b, present bool, op string, cv json.RawMessage) bool {
	if op != "eq" || !present {
		return false
	}
	var target bool
	if json.Unmarshal(cv, &target) != nil {
		target = rawToString(cv) == "true"
	}
	return b == target
}

func evalTags(tags []string, op string, cv json.RawMessage) bool {
	if op != "contains" {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(rawToString(cv)))
	for _, t := range tags {
		if strings.ToLower(strings.TrimSpace(t)) == target {
			return true
		}
	}
	return false
}

// ── value coercion helpers ──────────────────────────────────────────

func rawToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		if b {
			return "true"
		}
		return "false"
	}
	return strings.Trim(string(raw), `"`)
}

func rawToFloat(raw json.RawMessage) (float64, bool) {
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return f, true
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v, true
		}
	}
	return 0, false
}

func rawToInt(raw json.RawMessage) (int64, bool) {
	f, ok := rawToFloat(raw)
	if !ok {
		return 0, false
	}
	return int64(f), true
}

func daysFromValue(raw json.RawMessage) int {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) == nil {
		if d, ok := obj["days"]; ok {
			if v, ok := rawToFloat(d); ok {
				return int(v)
			}
		}
	}
	if f, ok := rawToFloat(raw); ok {
		return int(f)
	}
	return 0
}

func isNullRaw(raw json.RawMessage) bool {
	return len(raw) == 0 || string(raw) == "null"
}

func resolveActionDateValue(raw json.RawMessage) (*time.Time, error) {
	if isNullRaw(raw) {
		return nil, nil
	}
	var v struct {
		Mode string `json:"mode"`
		Days int    `json:"days"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, httpx.Validation("invalid action date value")
	}
	now := time.Now().UTC()
	switch v.Mode {
	case "today":
		t := now
		return &t, nil
	case "plus_days":
		t := now.AddDate(0, 0, v.Days)
		return &t, nil
	case "clear", "":
		return nil, nil
	default:
		return nil, httpx.Validation("invalid set_action_date mode")
	}
}

func cleanTags(tags []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || seen[strings.ToLower(t)] {
			continue
		}
		seen[strings.ToLower(t)] = true
		out = append(out, t)
	}
	return out
}

// ── action application ──────────────────────────────────────────────

// applyAction performs one Update action inside the move transaction. For
// pipeline stage moves it updates currentStageID and records history.
func (s *Service) applyAction(ctx context.Context, q database.Querier, accountID, userID, leadID int64, currentStageID *int64, a RuleAction) error {
	switch a.Domain {
	case "pipeline":
		if a.Field != "stage_id" {
			return httpx.Validation("unsupported pipeline action field")
		}
		stageID, ok := rawToInt(a.Value)
		if !ok || stageID == 0 {
			return httpx.Validation("stage_id required")
		}
		if err := s.assertStageOwnedTx(ctx, q, accountID, stageID); err != nil {
			return err
		}
		from := *currentStageID
		if err := moveLeadStage(ctx, q, leadID, stageID); err != nil {
			return err
		}
		if err := insertHistory(ctx, q, leadID, &from, stageID, userID, nil, nil); err != nil {
			return err
		}
		*currentStageID = stageID
		return nil
	case "user":
		if a.Field != "assigned_user_id" {
			return httpx.Validation("unsupported user action field")
		}
		if isNullRaw(a.Value) {
			_, err := q.Exec(ctx, `UPDATE leads SET assigned_user_id=NULL WHERE id=$1`, leadID)
			return err
		}
		uid, ok := rawToInt(a.Value)
		if !ok || uid == 0 {
			_, err := q.Exec(ctx, `UPDATE leads SET assigned_user_id=NULL WHERE id=$1`, leadID)
			return err
		}
		inAcc, err := userInAccount(ctx, q, accountID, uid)
		if err != nil {
			return err
		}
		if !inAcc {
			return httpx.Validation("invalid user for assign action")
		}
		_, err = q.Exec(ctx, `UPDATE leads SET assigned_user_id=$2 WHERE id=$1`, leadID, uid)
		return err
	case "lead":
		return applyLeadUpdate(ctx, q, accountID, leadID, a)
	}
	return httpx.Validation("unknown action domain")
}

func applyLeadUpdate(ctx context.Context, q database.Querier, accountID, leadID int64, a RuleAction) error {
	if strings.HasPrefix(a.Field, "custom:") {
		key := strings.TrimPrefix(a.Field, "custom:")
		var fid int64
		if err := q.QueryRow(ctx,
			`SELECT id FROM custom_fields WHERE field_key=$1 AND account_id=$2`, key, accountID).Scan(&fid); err != nil {
			return httpx.Validation("unknown custom field")
		}
		val := a.Value
		if len(val) == 0 {
			val = json.RawMessage("null")
		}
		_, err := q.Exec(ctx,
			`INSERT INTO lead_custom_values(lead_id, custom_field_id, value) VALUES ($1,$2,$3)
			 ON CONFLICT (lead_id, custom_field_id) DO UPDATE SET value = EXCLUDED.value`,
			leadID, fid, []byte(val))
		return err
	}
	switch a.Field {
	case "status":
		st := rawToString(a.Value)
		if !validLeadStatuses[st] {
			return httpx.Validation("invalid status for rule action")
		}
		_, err := q.Exec(ctx, `UPDATE leads SET status=$2 WHERE id=$1`, leadID, st)
		return err
	case "action_at":
		t, err := resolveActionDateValue(a.Value)
		if err != nil {
			return err
		}
		_, err = q.Exec(ctx, `UPDATE leads SET action_at=$2 WHERE id=$1`, leadID, t)
		return err
	case "disqualification_reason_id":
		if isNullRaw(a.Value) {
			_, err := q.Exec(ctx, `UPDATE leads SET disqualification_reason_id=NULL WHERE id=$1`, leadID)
			return err
		}
		rid, ok := rawToInt(a.Value)
		if !ok {
			return httpx.Validation("invalid disqualification reason")
		}
		var owned bool
		if err := q.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM disqualification_reasons WHERE id=$1 AND account_id=$2)`, rid, accountID).Scan(&owned); err != nil {
			return err
		}
		if !owned {
			return httpx.Validation("invalid disqualification reason")
		}
		_, err := q.Exec(ctx, `UPDATE leads SET disqualification_reason_id=$2 WHERE id=$1`, leadID, rid)
		return err
	case "tags":
		var tags []string
		if len(a.Value) > 0 {
			_ = json.Unmarshal(a.Value, &tags)
		}
		_, err := q.Exec(ctx, `UPDATE leads SET tags=$2 WHERE id=$1`, leadID, cleanTags(tags))
		return err
	}
	if leadTextBuiltins[a.Field] {
		sql := fmt.Sprintf(`UPDATE leads SET %s=$2 WHERE id=$1`, a.Field)
		_, err := q.Exec(ctx, sql, leadID, rawToString(a.Value))
		return err
	}
	return httpx.Validation("unsupported lead action field")
}
