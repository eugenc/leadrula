package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/pipelines"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

// RouteCondition checks lead fields or inbound payload fields before applying a route branch.
type RouteCondition struct {
	Domain string          `json:"domain"` // "lead" | "payload"
	Field  string          `json:"field"`
	Op     string          `json:"op"`
	Value  json.RawMessage `json:"value,omitempty"`
}

// RouteBranch is one conditional destination rule inside a route.
type RouteBranch struct {
	Name             string          `json:"name"`
	Position         int             `json:"position"`
	ConditionLogic   string          `json:"condition_logic"`
	Conditions       json.RawMessage `json:"conditions"`
	Destination      string          `json:"destination"`
	Delivery         string          `json:"delivery"`
	TargetPipelineID *int64          `json:"target_pipeline_id"`
	TargetStageID    *int64          `json:"target_stage_id"`
	ContractID       *int64          `json:"contract_id"`
	CompensationID   *int64          `json:"compensation_id"`
	DestWebhookID    *int64          `json:"dest_webhook_id"`
}

func parseRouteConditions(raw json.RawMessage) ([]RouteCondition, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var conds []RouteCondition
	if err := json.Unmarshal(raw, &conds); err != nil {
		return nil, err
	}
	return conds, nil
}

func parseRouteBranches(raw json.RawMessage) ([]RouteBranch, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var branches []RouteBranch
	if err := json.Unmarshal(raw, &branches); err != nil {
		return nil, err
	}
	return branches, nil
}

func normalizeConditionLogic(logic string) string {
	if logic == "or" {
		return "or"
	}
	return "and"
}

func flattenPayloadMap(raw map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range raw {
		if k == "custom" {
			continue
		}
		out[k] = v
	}
	if custom, ok := raw["custom"].(map[string]any); ok {
		for k, v := range custom {
			out[k] = v
		}
	}
	return out
}

func loadLeadPayloadFlat(ctx context.Context, q database.Querier, leadID int64) (map[string]any, error) {
	var raw json.RawMessage
	err := q.QueryRow(ctx, `SELECT raw_payload FROM leads WHERE id=$1`, leadID).Scan(&raw)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]any{}, nil
	}
	return flattenPayloadMap(m), nil
}

func payloadFieldText(flat map[string]any, field string) (string, bool) {
	v, ok := flat[field]
	if !ok {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x), true
	case float64:
		if x == float64(int64(x)) {
			return strings.TrimSpace(strconv.FormatInt(int64(x), 10)), true
		}
		return strings.TrimSpace(strconv.FormatFloat(x, 'f', -1, 64)), true
	case bool:
		if x {
			return "true", true
		}
		return "false", true
	default:
		b, _ := json.Marshal(v)
		return strings.Trim(string(b), `"`), true
	}
}

func payloadExpected(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return strings.Trim(string(raw), `"`)
}

func evalPayloadCondition(c RouteCondition, flat map[string]any) bool {
	s, present := payloadFieldText(flat, c.Field)
	switch c.Op {
	case "eq":
		return present && s == payloadExpected(c.Value)
	case "neq":
		return !present || s != payloadExpected(c.Value)
	case "contains":
		return present && strings.Contains(strings.ToLower(s), strings.ToLower(payloadExpected(c.Value)))
	case "empty":
		return !present || s == ""
	case "not_empty":
		return present && s != ""
	default:
		return false
	}
}

func evalRouteCondition(c RouteCondition, leadCtx *pipelines.LeadEvalContext, flat map[string]any) bool {
	if c.Domain == "payload" {
		return evalPayloadCondition(c, flat)
	}
	rc := pipelines.RuleCondition{
		Domain: "lead",
		Field:  c.Field,
		Op:     c.Op,
		Value:  c.Value,
	}
	return pipelines.EvalConditions([]pipelines.RuleCondition{rc}, "and", leadCtx)
}

func evalRouteConditions(conds []RouteCondition, logic string, leadCtx *pipelines.LeadEvalContext, flat map[string]any) bool {
	if len(conds) == 0 {
		return true
	}
	logic = normalizeConditionLogic(logic)
	for _, c := range conds {
		ok := evalRouteCondition(c, leadCtx, flat)
		if logic == "or" && ok {
			return true
		}
		if logic == "and" && !ok {
			return false
		}
	}
	if logic == "or" {
		return false
	}
	return true
}

func branchConditionsMatch(ctx context.Context, q database.Querier, accountID, leadID int64, branch *RouteBranch, payloadFlat map[string]any) (bool, error) {
	conds, err := parseRouteConditions(branch.Conditions)
	if err != nil {
		return false, err
	}
	if len(conds) == 0 {
		return true, nil
	}

	var leadCtx *pipelines.LeadEvalContext
	flat := payloadFlat
	needsLead := false
	needsPayload := flat != nil
	for _, c := range conds {
		if c.Domain == "payload" {
			needsPayload = true
		} else {
			needsLead = true
		}
	}
	if needsLead {
		leadCtx, err = pipelines.BuildLeadEvalContext(ctx, q, accountID, leadID, nil)
		if err != nil {
			return false, err
		}
	}
	if needsPayload && flat == nil {
		flat, err = loadLeadPayloadFlat(ctx, q, leadID)
		if err != nil {
			return false, err
		}
	}
	if flat == nil {
		flat = map[string]any{}
	}
	logic := branch.ConditionLogic
	if logic == "" {
		logic = "and"
	}
	return evalRouteConditions(conds, logic, leadCtx, flat), nil
}

// PickMatchingBranch returns the first branch whose conditions pass, ordered by position.
func PickMatchingBranch(ctx context.Context, q database.Querier, accountID, leadID int64, rt *Route, payloadFlat map[string]any) (*RouteBranch, int, error) {
	branches, err := parseRouteBranches(rt.Branches)
	if err != nil {
		return nil, 0, err
	}
	if len(branches) == 0 {
		return nil, 0, nil
	}
	sort.Slice(branches, func(i, j int) bool {
		if branches[i].Position != branches[j].Position {
			return branches[i].Position < branches[j].Position
		}
		return i < j
	})
	for _, b := range branches {
		branch := b
		ok, err := branchConditionsMatch(ctx, q, accountID, leadID, &branch, payloadFlat)
		if err != nil {
			return nil, 0, err
		}
		if ok {
			return &branch, branch.Position, nil
		}
	}
	return nil, 0, nil
}

// RouteForApply overlays a matched branch destination onto a route copy for ApplyRoute.
func RouteForApply(rt *Route, branch *RouteBranch) *Route {
	if rt == nil || branch == nil {
		return rt
	}
	out := *rt
	out.Destination = branch.Destination
	out.Delivery = branch.Delivery
	out.TargetPipelineID = branch.TargetPipelineID
	out.TargetStageID = branch.TargetStageID
	out.ContractID = branch.ContractID
	out.CompensationID = branch.CompensationID
	out.DestWebhookID = branch.DestWebhookID
	out.MatchedBranchPosition = branch.Position
	return &out
}

// MatchOriginRoutes finds the first route+branch match among candidates for an origin.
func MatchOriginRoutes(ctx context.Context, q database.Querier, accountID, leadID int64, routes []Route, payloadFlat map[string]any) (*Route, error) {
	for i := range routes {
		branch, _, err := PickMatchingBranch(ctx, q, accountID, leadID, &routes[i], payloadFlat)
		if err != nil {
			return nil, err
		}
		if branch != nil {
			return RouteForApply(&routes[i], branch), nil
		}
	}
	return nil, nil
}

func validateBranchConditions(logic string, raw json.RawMessage) error {
	if logic != "" && logic != "and" && logic != "or" {
		return httpx.Validation("condition_logic must be and or or")
	}
	if _, err := parseRouteConditions(raw); err != nil {
		return httpx.Validation("invalid conditions")
	}
	return nil
}

func validateRouteBranches(branches []RouteBranch, publisherOwned bool) error {
	if len(branches) == 0 {
		return httpx.Validation("at least one branch is required")
	}
	seen := map[int]bool{}
	seenNames := map[string]bool{}
	for i := range branches {
		b := &branches[i]
		if b.Position < 0 {
			return httpx.Validation("branch position must be >= 0")
		}
		if seen[b.Position] {
			return httpx.Validation("duplicate branch position")
		}
		seen[b.Position] = true
		name := strings.TrimSpace(b.Name)
		if len(name) > 100 {
			return httpx.Validation("branch name must be 100 characters or less")
		}
		if name != "" {
			key := strings.ToLower(name)
			if seenNames[key] {
				return httpx.Validation("duplicate branch name")
			}
			seenNames[key] = true
		}
		logic := b.ConditionLogic
		if logic == "" {
			logic = "and"
		}
		conds := b.Conditions
		if conds == nil {
			conds = json.RawMessage(`[]`)
		}
		if err := validateBranchConditions(logic, conds); err != nil {
			return err
		}
		p := CreateRouteParams{
			Destination:      b.Destination,
			Delivery:         b.Delivery,
			TargetPipelineID: b.TargetPipelineID,
			TargetStageID:    b.TargetStageID,
			ContractID:       b.ContractID,
			CompensationID:   b.CompensationID,
			DestWebhookID:    b.DestWebhookID,
		}
		if err := validateBranchDestination(p, publisherOwned); err != nil {
			return err
		}
	}
	return nil
}

func validateBranchDestination(p CreateRouteParams, publisherOwned bool) error {
	if p.Destination != "contract" && p.Destination != "pipeline" && p.Destination != "webhook" && p.Destination != "integration" {
		return httpx.Validation("destination must be contract, pipeline, webhook, or integration")
	}
	if !publisherOwned {
		if p.Destination == "contract" {
			return httpx.Validation("contract destination is publisher-only")
		}
	}
	if p.Destination == "contract" || p.Destination == "pipeline" {
		if p.Delivery != "leads" && p.Delivery != "leads_pipeline" && p.Delivery != "" {
			return httpx.Validation("delivery must be leads or leads_pipeline")
		}
	}
	switch p.Destination {
	case "contract":
		if p.ContractID == nil || *p.ContractID == 0 {
			return httpx.Validation("contract_id is required for contract destination")
		}
	case "pipeline":
		delivery := p.Delivery
		if delivery == "" {
			delivery = "leads_pipeline"
		}
		if delivery == "leads_pipeline" {
			if p.TargetPipelineID == nil || p.TargetStageID == nil {
				return httpx.Validation("target_pipeline_id and target_stage_id are required for pipeline destination")
			}
		}
	case "webhook":
		if p.DestWebhookID == nil || *p.DestWebhookID == 0 {
			return httpx.Validation("dest_webhook_id is required for webhook destination")
		}
	}
	return nil
}

func defaultBranchFromParams(p CreateRouteParams) RouteBranch {
	delivery := p.Delivery
	if delivery == "" {
		if p.Destination == "integration" || p.Destination == "webhook" {
			delivery = "leads"
		} else {
			delivery = "leads_pipeline"
		}
	}
	return RouteBranch{
		Name:             "Route 1",
		Position:         0,
		ConditionLogic:   "and",
		Conditions:       json.RawMessage(`[]`),
		Destination:      p.Destination,
		Delivery:         delivery,
		TargetPipelineID: p.TargetPipelineID,
		TargetStageID:    p.TargetStageID,
		ContractID:       p.ContractID,
		CompensationID:   p.CompensationID,
		DestWebhookID:    p.DestWebhookID,
	}
}

func normalizeRouteBranches(p *CreateRouteParams) error {
	if len(p.Branches) == 0 {
		if p.Destination == "" {
			return httpx.Validation("at least one branch is required")
		}
		p.Branches = []RouteBranch{defaultBranchFromParams(*p)}
	}
	for i := range p.Branches {
		b := &p.Branches[i]
		if b.ConditionLogic == "" {
			b.ConditionLogic = "and"
		}
		if b.Conditions == nil {
			b.Conditions = json.RawMessage(`[]`)
		}
		if b.Delivery == "" {
			if b.Destination == "integration" || b.Destination == "webhook" {
				b.Delivery = "leads"
			} else {
				b.Delivery = "leads_pipeline"
			}
		}
		if strings.TrimSpace(b.Name) == "" {
			b.Name = fmt.Sprintf("Route %d", b.Position+1)
		} else {
			b.Name = strings.TrimSpace(b.Name)
		}
	}
	first := p.Branches[0]
	p.Destination = first.Destination
	p.Delivery = first.Delivery
	p.TargetPipelineID = first.TargetPipelineID
	p.TargetStageID = first.TargetStageID
	p.ContractID = first.ContractID
	p.CompensationID = first.CompensationID
	p.DestWebhookID = first.DestWebhookID
	return nil
}

func branchesToJSON(branches []RouteBranch) (json.RawMessage, error) {
	b, err := json.Marshal(branches)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
