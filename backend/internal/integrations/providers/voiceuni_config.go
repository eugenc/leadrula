package providers

import (
	"encoding/json"
	"strings"
)

func DefaultVoiceUniOutboundFieldMap() []SunbaseFieldMapEntry {
	return []SunbaseFieldMapEntry{
		{DestKey: "external_id", SourceType: "builtin", BuiltinField: strPtr("external_id")},
		{DestKey: "first_name", SourceType: "builtin", BuiltinField: strPtr("first_name")},
		{DestKey: "last_name", SourceType: "builtin", BuiltinField: strPtr("last_name")},
		{DestKey: "phone", SourceType: "builtin", BuiltinField: strPtr("phone")},
		{DestKey: "email", SourceType: "builtin", BuiltinField: strPtr("email")},
		{DestKey: "address", SourceType: "builtin", BuiltinField: strPtr("address")},
		{DestKey: "city", SourceType: "builtin", BuiltinField: strPtr("city")},
		{DestKey: "state", SourceType: "builtin", BuiltinField: strPtr("state")},
		{DestKey: "zip", SourceType: "builtin", BuiltinField: strPtr("zip")},
		{DestKey: "source", SourceType: "builtin", BuiltinField: strPtr("source")},
	}
}

func VoiceUniOutboundFieldMapFromConfig(config map[string]any) []SunbaseFieldMapEntry {
	if config == nil {
		return DefaultVoiceUniOutboundFieldMap()
	}
	raw, ok := config["outbound_field_map"]
	if !ok {
		return DefaultVoiceUniOutboundFieldMap()
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return DefaultVoiceUniOutboundFieldMap()
	}
	var entries []SunbaseFieldMapEntry
	if err := json.Unmarshal(b, &entries); err != nil || len(entries) == 0 {
		return DefaultVoiceUniOutboundFieldMap()
	}
	return entries
}

func VoiceUniSourceSlug(config map[string]any) string {
	if config == nil {
		return ""
	}
	if s, ok := config["source_slug"].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func VoiceUniCallSourceSlug(config map[string]any) string {
	if config == nil {
		return ""
	}
	if s, ok := config["call_source_slug"].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func MergeVoiceUniConfigDefaults(config map[string]any) map[string]any {
	if config == nil {
		config = map[string]any{}
	}
	if _, ok := config["outbound_field_map"]; !ok {
		config["outbound_field_map"] = DefaultVoiceUniOutboundFieldMap()
	}
	return config
}
