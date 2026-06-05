package pipelines

import (
	"context"
	"encoding/json"

	"github.com/echayko/leadrula/backend/internal/database"
)

// LeadEvalContext is an opaque evaluation context built from a lead snapshot.
// Use BuildLeadEvalContext to create one, then pass it to EvalConditions.
type LeadEvalContext = ruleEvalContext

// BuildLeadEvalContext loads a lead and its custom fields into an eval context
// so callers outside the pipelines package (e.g. outbound webhooks) can evaluate
// stage-rule-style conditions without duplicating the load logic.
func BuildLeadEvalContext(ctx context.Context, q database.Querier, accountID, leadID int64, fromStageID *int64) (*LeadEvalContext, error) {
	return buildEvalContext(ctx, q, accountID, leadID, fromStageID)
}

// EvalConditions evaluates a slice of RuleConditions against a LeadEvalContext.
// It mirrors matchConditions but is exported for use by the outbound webhook engine.
func EvalConditions(conds []RuleCondition, logic string, ec *LeadEvalContext) bool {
	if len(conds) == 0 {
		return true
	}
	return matchConditions(conds, logic, ec)
}

// ParseConditions deserialises a JSONB conditions array into []RuleCondition.
func ParseConditions(raw json.RawMessage) ([]RuleCondition, error) {
	return normalizeConditions(raw)
}
