package webhooks

import (
	"strings"
	"testing"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
)

func TestGHLInboundFieldMapMerge_configOverridesDefaults(t *testing.T) {
	seen := map[string]bool{}
	type entry struct {
		sourceKey string
		builtin   string
	}
	var out []entry
	add := func(sourceKey, targetType string, builtinField *string, _ *int64) error {
		sourceKey = strings.TrimSpace(sourceKey)
		if sourceKey == "" || seen[sourceKey] {
			return nil
		}
		seen[sourceKey] = true
		if targetType == "builtin" && builtinField != nil {
			out = append(out, entry{sourceKey: sourceKey, builtin: *builtinField})
		}
		return nil
	}

	config := map[string]any{
		"contact_standard_fields": map[string]any{
			"firstName": map[string]any{
				"source_type":   "builtin",
				"builtin_field": "email",
			},
		},
	}
	for _, m := range providers.GHLInboundMapsFromConfig(config) {
		switch m.TargetType {
		case "builtin":
			if m.BuiltinField == nil {
				continue
			}
			bf := *m.BuiltinField
			if err := add(m.SourceKey, "builtin", &bf, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, f := range defaultGHLInboundFields {
		bf := f.BuiltinField
		if err := add(f.SourceKey, "builtin", &bf, nil); err != nil {
			t.Fatal(err)
		}
	}

	for _, e := range out {
		if e.sourceKey == "firstName" && e.builtin != "email" {
			t.Fatalf("firstName should map to email from config, got %q", e.builtin)
		}
	}
}
