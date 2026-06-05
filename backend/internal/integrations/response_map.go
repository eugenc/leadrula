package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// responseMapEntry is one row of a trigger's response_map config.
type responseMapEntry struct {
	ResponseKey   string  `json:"response_key"`
	TargetType    string  `json:"target_type"`    // "builtin" or "custom"
	BuiltinField  *string `json:"builtin_field"`  // e.g. "external_id"
	CustomFieldID *int64  `json:"custom_field_id"`
}

// validBuiltinFields mirrors the whitelist in leads/repository.go.
var validBuiltinFields = map[string]bool{
	"first_name": true, "last_name": true, "phone": true, "email": true,
	"address": true, "city": true, "state": true, "zip": true, "source": true,
	"external_id": true,
}

// applyResponseMap reads the trigger's response_map config, extracts values from
// the HTTP response body, and writes them back to the lead.
func (s *Service) applyResponseMap(ctx context.Context, triggerID, leadID int64, respRaw []byte) {
	var rawMap json.RawMessage
	var accountID int64
	if err := s.pool.QueryRow(ctx,
		`SELECT t.response_map, w.account_id
		 FROM webhook_outbound_triggers t
		 JOIN webhooks w ON w.id = t.webhook_id
		 WHERE t.id = $1`, triggerID).Scan(&rawMap, &accountID); err != nil {
		log.Printf("response_map: load trigger %d: %v", triggerID, err)
		return
	}
	if len(rawMap) == 0 || string(rawMap) == "null" || string(rawMap) == "[]" {
		return
	}

	var entries []responseMapEntry
	if err := json.Unmarshal(rawMap, &entries); err != nil {
		log.Printf("response_map: parse trigger %d: %v", triggerID, err)
		return
	}
	if len(entries) == 0 {
		return
	}

	var respObj map[string]any
	if err := json.Unmarshal(respRaw, &respObj); err != nil {
		log.Printf("response_map: parse response for trigger %d: %v", triggerID, err)
		return
	}

	for _, e := range entries {
		val, ok := resolveDotPath(respObj, e.ResponseKey)
		if !ok {
			continue
		}
		switch e.TargetType {
		case "builtin":
			if e.BuiltinField == nil || !validBuiltinFields[*e.BuiltinField] {
				log.Printf("response_map: trigger %d unknown builtin field %v", triggerID, e.BuiltinField)
				continue
			}
			sql := fmt.Sprintf(`UPDATE leads SET %s = $3 WHERE id = $1 AND owner_account_id = $2`, *e.BuiltinField)
			if _, err := s.pool.Exec(ctx, sql, leadID, accountID, val); err != nil {
				log.Printf("response_map: trigger %d write builtin %s: %v", triggerID, *e.BuiltinField, err)
			}
		case "custom":
			if e.CustomFieldID == nil {
				continue
			}
			valJSON, _ := json.Marshal(val)
			if _, err := s.pool.Exec(ctx,
				`INSERT INTO lead_custom_values(lead_id, custom_field_id, value) VALUES ($1,$2,$3)
				 ON CONFLICT (lead_id, custom_field_id) DO UPDATE SET value = EXCLUDED.value`,
				leadID, *e.CustomFieldID, valJSON); err != nil {
				log.Printf("response_map: trigger %d write custom %d: %v", triggerID, *e.CustomFieldID, err)
			}
		}
	}
}

// resolveDotPath traverses a nested map[string]any using dot-separated keys.
// Returns the string representation of the value and whether it was found.
func resolveDotPath(obj map[string]any, path string) (string, bool) {
	parts := strings.SplitN(path, ".", 2)
	val, ok := obj[parts[0]]
	if !ok {
		return "", false
	}
	if len(parts) == 1 {
		return anyToString(val), true
	}
	nested, ok := val.(map[string]any)
	if !ok {
		return "", false
	}
	return resolveDotPath(nested, parts[1])
}

func anyToString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%g", x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		b, _ := json.Marshal(v)
		return strings.Trim(string(b), `"`)
	}
}
