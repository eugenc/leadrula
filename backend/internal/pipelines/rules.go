package pipelines

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

type StageRule struct {
	ID              int64           `json:"id"`
	StageID         int64           `json:"stage_id"`
	Position        int             `json:"position"`
	ConditionLogic  string          `json:"condition_logic"`
	Conditions      json.RawMessage `json:"conditions"`
	Actions         json.RawMessage `json:"actions"`
	CreatedAt       time.Time       `json:"created_at"`
}

// RuleCondition checks a single lead/pipeline field against a value.
type RuleCondition struct {
	Domain string          `json:"domain"` // "lead" | "pipeline"
	Field  string          `json:"field"`  // e.g. "action_at", "status", "custom:foo", "previous_stage_id"
	Op     string          `json:"op"`     // eq, neq, gt, lt, contains, empty, not_empty
	Value  json.RawMessage `json:"value,omitempty"`
}

// RuleAction updates a single field on a domain (lead, pipeline, user).
type RuleAction struct {
	Verb   string          `json:"verb"`   // "update"
	Domain string          `json:"domain"` // "lead" | "pipeline" | "user"
	Field  string          `json:"field"`  // e.g. "status", "action_at", "stage_id", "assigned_user_id", "custom:foo"
	Value  json.RawMessage `json:"value,omitempty"`
}

var validLeadStatuses = map[string]bool{
	"review": true, "distributed": true, "returned": true, "closed": true,
}

func (s *Service) ListRules(ctx context.Context, p *auth.Principal, stageID int64) ([]StageRule, error) {
	if err := s.requireStage(ctx, p, stageID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, stage_id, position, condition_logic, conditions, actions, created_at
		 FROM stage_rules WHERE stage_id = $1 ORDER BY position, id`, stageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules, err := scanRules(rows)
	if err != nil {
		return nil, err
	}
	for i := range rules {
		if err := normalizeStageRuleJSON(&rules[i]); err != nil {
			return nil, err
		}
	}
	return rules, nil
}

func (s *Service) CreateRule(ctx context.Context, p *auth.Principal, stageID int64, logic string, conditions, actions json.RawMessage) (*StageRule, error) {
	if err := s.requireStage(ctx, p, stageID); err != nil {
		return nil, err
	}
	if err := validateRulePayload(logic, conditions, actions); err != nil {
		return nil, err
	}
	if err := s.validateRuleRefs(ctx, p, stageID, actions); err != nil {
		return nil, err
	}
	if logic == "" {
		logic = "and"
	}
	rule := &StageRule{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO stage_rules(stage_id, position, condition_logic, conditions, actions)
		 VALUES ($1, COALESCE((SELECT MAX(position)+1 FROM stage_rules WHERE stage_id=$1), 0), $2, $3, $4)
		 RETURNING id, stage_id, position, condition_logic, conditions, actions, created_at`,
		stageID, logic, conditions, actions).Scan(
		&rule.ID, &rule.StageID, &rule.Position, &rule.ConditionLogic,
		&rule.Conditions, &rule.Actions, &rule.CreatedAt)
	if err != nil {
		return nil, err
	}
	return rule, normalizeStageRuleJSON(rule)
}

func (s *Service) UpdateRule(ctx context.Context, p *auth.Principal, ruleID int64, logic *string, conditions, actions json.RawMessage) (*StageRule, error) {
	stageID, err := s.ruleStageID(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	if err := s.requireStage(ctx, p, stageID); err != nil {
		return nil, err
	}
	if logic != nil || conditions != nil || actions != nil {
		cur, err := s.getRule(ctx, ruleID)
		if err != nil {
			return nil, err
		}
		useLogic := cur.ConditionLogic
		if logic != nil {
			useLogic = *logic
		}
		useConds := cur.Conditions
		if conditions != nil {
			useConds = conditions
		}
		useActs := cur.Actions
		if actions != nil {
			useActs = actions
		}
		if err := validateRulePayload(useLogic, useConds, useActs); err != nil {
			return nil, err
		}
		if err := s.validateRuleRefs(ctx, p, stageID, useActs); err != nil {
			return nil, err
		}
	}
	rule := &StageRule{}
	err = s.pool.QueryRow(ctx,
		`UPDATE stage_rules SET
		   condition_logic = COALESCE($2, condition_logic),
		   conditions = COALESCE($3, conditions),
		   actions = COALESCE($4, actions)
		 WHERE id = $1
		 RETURNING id, stage_id, position, condition_logic, conditions, actions, created_at`,
		ruleID, logic, conditions, actions).Scan(
		&rule.ID, &rule.StageID, &rule.Position, &rule.ConditionLogic,
		&rule.Conditions, &rule.Actions, &rule.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("rule not found")
	}
	if err != nil {
		return nil, err
	}
	return rule, normalizeStageRuleJSON(rule)
}

func (s *Service) DeleteRule(ctx context.Context, p *auth.Principal, ruleID int64) error {
	stageID, err := s.ruleStageID(ctx, ruleID)
	if err != nil {
		return err
	}
	if err := s.requireStage(ctx, p, stageID); err != nil {
		return err
	}
	ct, err := s.pool.Exec(ctx, `DELETE FROM stage_rules WHERE id = $1`, ruleID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("rule not found")
	}
	return nil
}

func (s *Service) ReorderRules(ctx context.Context, p *auth.Principal, stageID int64, orderedRuleIDs []int64) error {
	if err := s.requireStage(ctx, p, stageID); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE stage_rules SET position = position + 100000 WHERE stage_id = $1`, stageID); err != nil {
		return err
	}
	for i, id := range orderedRuleIDs {
		if _, err := tx.Exec(ctx,
			`UPDATE stage_rules SET position = $3 WHERE id = $1 AND stage_id = $2`,
			id, stageID, i); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// EvaluateStageRules runs the first matching rule for a stage entry move.
// enteredStageID is the stage just entered (whose rules we run); fromStageID is
// the stage the lead was in before this move (for pipeline conditions).
func (s *Service) EvaluateStageRules(ctx context.Context, q database.Querier, accountID, userID, leadID, enteredStageID int64, fromStageID *int64) error {
	rows, err := q.Query(ctx,
		`SELECT id, stage_id, position, condition_logic, conditions, actions, created_at
		 FROM stage_rules WHERE stage_id = $1 ORDER BY position, id`, enteredStageID)
	if err != nil {
		return err
	}
	rules, err := scanRules(rows)
	rows.Close()
	if err != nil || len(rules) == 0 {
		return err
	}

	ec, err := buildEvalContext(ctx, q, accountID, leadID, fromStageID)
	if err != nil {
		return err
	}
	currentStageID := enteredStageID
	if ec.stageID != nil {
		currentStageID = *ec.stageID
	}

	for _, rule := range rules {
		conds, err := normalizeConditions(rule.Conditions)
		if err != nil {
			continue
		}
		if !matchConditions(conds, rule.ConditionLogic, ec) {
			continue
		}
		acts, err := normalizeActions(rule.Actions)
		if err != nil {
			return err
		}
		for _, a := range acts {
			if err := s.applyAction(ctx, q, accountID, userID, leadID, &currentStageID, ec, a); err != nil {
				return err
			}
		}
		return nil
	}
	return nil
}

func matchConditions(conds []RuleCondition, logic string, ec *ruleEvalContext) bool {
	if len(conds) == 0 {
		return true
	}
	if logic == "or" {
		for _, c := range conds {
			if ec.evalCondition(c) {
				return true
			}
		}
		return false
	}
	for _, c := range conds {
		if !ec.evalCondition(c) {
			return false
		}
	}
	return true
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func normalizeConditions(raw json.RawMessage) ([]RuleCondition, error) {
	if isNullRaw(raw) {
		return nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, httpx.Validation("invalid conditions json")
	}
	out := make([]RuleCondition, 0, len(items))
	for _, item := range items {
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(item, &probe); err != nil {
			return nil, httpx.Validation("invalid condition")
		}
		if _, legacy := probe["type"]; legacy {
			out = append(out, legacyCondition(item))
			continue
		}
		var c RuleCondition
		if err := json.Unmarshal(item, &c); err != nil {
			return nil, httpx.Validation("invalid condition")
		}
		out = append(out, c)
	}
	return out, nil
}

func legacyCondition(item json.RawMessage) RuleCondition {
	var l struct {
		Op   string `json:"op"`
		Days int    `json:"days"`
	}
	_ = json.Unmarshal(item, &l)
	val, _ := json.Marshal(map[string]int{"days": l.Days})
	return RuleCondition{Domain: "lead", Field: "action_at", Op: l.Op, Value: val}
}

func normalizeActions(raw json.RawMessage) ([]RuleAction, error) {
	if isNullRaw(raw) {
		return nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, httpx.Validation("invalid actions json")
	}
	out := make([]RuleAction, 0, len(items))
	for _, item := range items {
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(item, &probe); err != nil {
			return nil, httpx.Validation("invalid action")
		}
		_, hasType := probe["type"]
		_, hasVerb := probe["verb"]
		if hasType && !hasVerb {
			a, err := legacyAction(item)
			if err != nil {
				return nil, err
			}
			out = append(out, a)
			continue
		}
		var a RuleAction
		if err := json.Unmarshal(item, &a); err != nil {
			return nil, httpx.Validation("invalid action")
		}
		out = append(out, a)
	}
	return out, nil
}

func legacyAction(item json.RawMessage) (RuleAction, error) {
	var l struct {
		Type    string `json:"type"`
		StageID int64  `json:"stage_id"`
		Mode    string `json:"mode"`
		Days    int    `json:"days"`
		UserID  int64  `json:"user_id"`
		Status  string `json:"status"`
	}
	_ = json.Unmarshal(item, &l)
	switch l.Type {
	case "move_to_stage":
		v, _ := json.Marshal(l.StageID)
		return RuleAction{Verb: "update", Domain: "pipeline", Field: "stage_id", Value: v}, nil
	case "set_action_date":
		v, _ := json.Marshal(map[string]any{"mode": l.Mode, "days": l.Days})
		return RuleAction{Verb: "update", Domain: "lead", Field: "action_at", Value: v}, nil
	case "assign_user":
		v, _ := json.Marshal(l.UserID)
		return RuleAction{Verb: "update", Domain: "user", Field: "assigned_user_id", Value: v}, nil
	case "clear_assignee":
		return RuleAction{Verb: "update", Domain: "user", Field: "assigned_user_id", Value: json.RawMessage("null")}, nil
	case "set_status":
		v, _ := json.Marshal(l.Status)
		return RuleAction{Verb: "update", Domain: "lead", Field: "status", Value: v}, nil
	}
	return RuleAction{}, httpx.Validation("unknown legacy action")
}

func normalizeStageRuleJSON(r *StageRule) error {
	conds, err := normalizeConditions(r.Conditions)
	if err != nil {
		return err
	}
	if conds == nil {
		conds = []RuleCondition{}
	}
	cb, err := json.Marshal(conds)
	if err != nil {
		return err
	}
	r.Conditions = cb

	acts, err := normalizeActions(r.Actions)
	if err != nil {
		return err
	}
	if acts == nil {
		acts = []RuleAction{}
	}
	ab, err := json.Marshal(acts)
	if err != nil {
		return err
	}
	r.Actions = ab
	return nil
}

func validateRulePayload(logic string, conditions, actions json.RawMessage) error {
	if logic != "and" && logic != "or" {
		return httpx.Validation("condition_logic must be and or or")
	}
	conds, err := normalizeConditions(conditions)
	if err != nil {
		return err
	}
	for _, c := range conds {
		if c.Domain != "lead" && c.Domain != "pipeline" {
			return httpx.Validation("invalid condition domain")
		}
		if c.Field == "" {
			return httpx.Validation("condition field required")
		}
		if !knownOps[c.Op] {
			return httpx.Validation("invalid condition operator")
		}
	}
	acts, err := normalizeActions(actions)
	if err != nil {
		return err
	}
	if len(acts) == 0 {
		return httpx.Validation("rule must have at least one action")
	}
	for _, a := range acts {
		if a.Verb != "" && a.Verb != "update" {
			return httpx.Validation("unsupported action verb")
		}
		switch a.Domain {
		case "lead", "pipeline", "user":
		default:
			return httpx.Validation("invalid action domain")
		}
		if a.Field == "" {
			return httpx.Validation("action field required")
		}
	}
	return nil
}

func (s *Service) validateRuleRefs(ctx context.Context, p *auth.Principal, stageID int64, actions json.RawMessage) error {
	acts, err := normalizeActions(actions)
	if err != nil {
		return err
	}
	customByKey, err := loadCustomByKey(ctx, s.pool, p.AccountID)
	if err != nil {
		return err
	}
	for _, a := range acts {
		if srcField, isRef := actionFromFieldRef(a.Value); isRef {
			if err := validateFromFieldRef(customByKey, a.Domain, a.Field, srcField); err != nil {
				return err
			}
			continue
		}
		switch {
		case a.Domain == "pipeline" && a.Field == "stage_id":
			sid, ok := rawToInt(a.Value)
			if !ok {
				return httpx.Validation("stage_id required")
			}
			if err := s.requireStage(ctx, p, sid); err != nil {
				return err
			}
		case a.Domain == "user" && a.Field == "assigned_user_id" && !isNullRaw(a.Value):
			if uid, ok := rawToInt(a.Value); ok && uid != 0 {
				inAcc, err := userInAccount(ctx, s.pool, p.AccountID, uid)
				if err != nil {
					return err
				}
				if !inAcc {
					return httpx.Validation("invalid user for assign action")
				}
			}
		case a.Domain == "lead" && a.Field == "status":
			if !validLeadStatuses[rawToString(a.Value)] {
				return httpx.Validation("invalid status for rule action")
			}
		case a.Domain == "lead" && a.Field == "action_at":
			if _, err := resolveActionAtValue(a.Value); err != nil {
				return err
			}
		case a.Domain == "lead" && a.Field == "disqualification_reason_id" && !isNullRaw(a.Value):
			if rid, ok := rawToInt(a.Value); ok {
				owned, err := s.reasonBelongsToRulePipeline(ctx, stageID, rid)
				if err != nil {
					return err
				}
				if !owned {
					return httpx.Validation("invalid disqualification reason")
				}
			}
		case a.Domain == "lead" && strings.HasPrefix(a.Field, "custom:"):
			key := strings.TrimPrefix(a.Field, "custom:")
			var ok bool
			if err := s.pool.QueryRow(ctx,
				`SELECT EXISTS(SELECT 1 FROM custom_fields WHERE field_key=$1 AND account_id=$2)`, key, p.AccountID).Scan(&ok); err != nil {
				return err
			}
			if !ok {
				return httpx.Validation("unknown custom field")
			}
		}
	}
	return nil
}

func (s *Service) assertStageOwned(ctx context.Context, accountID, stageID int64) error {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM pipeline_stages st JOIN pipelines p ON p.id = st.pipeline_id
		   WHERE st.id = $1 AND p.account_id = $2)`, stageID, accountID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.NotFound("stage not found")
	}
	return nil
}

func (s *Service) assertStageOwnedTx(ctx context.Context, q database.Querier, accountID, stageID int64) error {
	var ok bool
	err := q.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM pipeline_stages st JOIN pipelines p ON p.id = st.pipeline_id
		   WHERE st.id = $1 AND p.account_id = $2)`, stageID, accountID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.NotFound("stage not found")
	}
	return nil
}

func userInAccount(ctx context.Context, q database.Querier, accountID, userID int64) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND account_id=$2 AND is_active)`,
		userID, accountID).Scan(&ok)
	return ok, err
}

func moveLeadStage(ctx context.Context, q database.Querier, leadID, stageID int64) error {
	_, err := q.Exec(ctx,
		`UPDATE leads SET stage_id=$2, position=COALESCE((SELECT MAX(position)+1 FROM leads WHERE stage_id=$2),0)
		 WHERE id=$1`, leadID, stageID)
	return err
}

func insertHistory(ctx context.Context, q database.Querier, leadID int64, fromStage *int64, toStage, userID int64, actionAt *time.Time, disq *int64) error {
	_, err := q.Exec(ctx,
		`INSERT INTO lead_stage_history(lead_id, from_stage_id, to_stage_id, moved_by_user_id, action_at_captured, disqualification_reason_id)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		leadID, fromStage, toStage, userID, actionAt, disq)
	return err
}

func (s *Service) ruleStageID(ctx context.Context, ruleID int64) (int64, error) {
	var stageID int64
	err := s.pool.QueryRow(ctx, `SELECT stage_id FROM stage_rules WHERE id=$1`, ruleID).Scan(&stageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, httpx.NotFound("rule not found")
	}
	return stageID, err
}

func (s *Service) getRule(ctx context.Context, ruleID int64) (*StageRule, error) {
	rule := &StageRule{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, stage_id, position, condition_logic, conditions, actions, created_at
		 FROM stage_rules WHERE id = $1`, ruleID).Scan(
		&rule.ID, &rule.StageID, &rule.Position, &rule.ConditionLogic,
		&rule.Conditions, &rule.Actions, &rule.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("rule not found")
	}
	return rule, err
}

func scanRules(rows pgx.Rows) ([]StageRule, error) {
	var out []StageRule
	for rows.Next() {
		var r StageRule
		if err := rows.Scan(&r.ID, &r.StageID, &r.Position, &r.ConditionLogic, &r.Conditions, &r.Actions, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
