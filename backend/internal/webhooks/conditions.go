package webhooks

import (
	"encoding/json"
	"strings"
)

// PayloadCondition checks one inbound JSON field against a value.
type PayloadCondition struct {
	Field string          `json:"field"`
	Op    string          `json:"op"`
	Value json.RawMessage `json:"value,omitempty"`
}

func payloadFieldValue(flat map[string]any, field string) (string, bool) {
	if v, ok := flat[field]; ok {
		return toText(v), true
	}
	return "", false
}

func evalPayloadConditions(conds []PayloadCondition, logic string, flat map[string]any) bool {
	if len(conds) == 0 {
		return true
	}
	if logic != "or" {
		logic = "and"
	}
	for _, c := range conds {
		ok := evalPayloadCondition(c, flat)
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

func evalPayloadCondition(c PayloadCondition, flat map[string]any) bool {
	s, present := payloadFieldValue(flat, c.Field)
	switch c.Op {
	case "eq":
		return present && s == payloadCondExpected(c.Value)
	case "neq":
		return !present || s != payloadCondExpected(c.Value)
	case "contains":
		return present && strings.Contains(strings.ToLower(s), strings.ToLower(payloadCondExpected(c.Value)))
	case "empty":
		return !present || s == ""
	case "not_empty":
		return present && s != ""
	default:
		return false
	}
}

func payloadCondExpected(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return strings.Trim(string(raw), `"`)
}

func parsePayloadConditions(raw json.RawMessage) ([]PayloadCondition, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var conds []PayloadCondition
	if err := json.Unmarshal(raw, &conds); err != nil {
		return nil, err
	}
	return conds, nil
}
