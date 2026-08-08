package providers

import (
	"encoding/json"
	"strings"
)

// GHLInboundMapEntry is a GHL payload key mapped to a Leadrula lead field (inbound webhook).
type GHLInboundMapEntry struct {
	SourceKey     string
	TargetType    string
	BuiltinField  *string
	CustomFieldID *int64
}

type ghlStandardInboundSpec struct {
	fieldKey string
	ghlKeys  []string
}

var ghlOpportunityStandardInbound = []ghlStandardInboundSpec{
	{fieldKey: "monetary_value", ghlKeys: []string{"monetaryValue"}},
	{fieldKey: "assigned_user_id", ghlKeys: []string{"assignedTo"}},
	{fieldKey: "status", ghlKeys: []string{"status"}},
}

var ghlAppointmentStandardInbound = []ghlStandardInboundSpec{
	{fieldKey: "description", ghlKeys: []string{"description"}},
	{fieldKey: "address", ghlKeys: []string{"address"}},
	{fieldKey: "assigned_user_id", ghlKeys: []string{"assignedUserId"}},
	{fieldKey: "meeting_location_type", ghlKeys: []string{"meetingLocationType"}},
}

var ghlContactStandardInbound = []ghlStandardInboundSpec{
	{fieldKey: "firstName", ghlKeys: []string{"firstName", "first_name"}},
	{fieldKey: "lastName", ghlKeys: []string{"lastName", "last_name"}},
	{fieldKey: "phone", ghlKeys: []string{"phone"}},
	{fieldKey: "email", ghlKeys: []string{"email"}},
	{fieldKey: "address1", ghlKeys: []string{"address1", "address"}},
	{fieldKey: "city", ghlKeys: []string{"city"}},
	{fieldKey: "state", ghlKeys: []string{"state"}},
	{fieldKey: "postalCode", ghlKeys: []string{"postalCode", "zip"}},
	{fieldKey: "source", ghlKeys: []string{"source"}},
}

// GHLInboundMapsFromConfig inverts outbound field mapping for GHL inbound webhooks.
func GHLInboundMapsFromConfig(config map[string]any) []GHLInboundMapEntry {
	if config == nil {
		return nil
	}
	out := invertOutboundFieldMap(ghlOutboundFieldMapFromConfig(config))
	out = append(out, invertStandardFields(config["opportunity_standard_fields"], ghlOpportunityStandardInbound)...)
	out = append(out, invertStandardFields(config["appointment_standard_fields"], ghlAppointmentStandardInbound)...)
	out = append(out, invertStandardFields(ghlContactStandardFieldsForInbound(config), ghlContactStandardInbound)...)
	return out
}

func invertOutboundFieldMap(entries []SunbaseFieldMapEntry) []GHLInboundMapEntry {
	var out []GHLInboundMapEntry
	for _, e := range entries {
		if e.DestKey == "" || e.SourceType == "static" {
			continue
		}
		var base GHLInboundMapEntry
		switch e.SourceType {
		case "builtin":
			if e.BuiltinField == nil || strings.TrimSpace(*e.BuiltinField) == "" {
				continue
			}
			bf := strings.TrimSpace(*e.BuiltinField)
			base.TargetType = "builtin"
			base.BuiltinField = &bf
		case "custom":
			if e.CustomFieldID == nil || *e.CustomFieldID <= 0 {
				continue
			}
			id := *e.CustomFieldID
			base.TargetType = "custom"
			base.CustomFieldID = &id
		default:
			continue
		}
		out = append(out, ghlInboundMapAliases(e, base)...)
	}
	return out
}

func ghlInboundMapAliases(e SunbaseFieldMapEntry, base GHLInboundMapEntry) []GHLInboundMapEntry {
	keys := ghlInboundSourceKeys(e)
	out := make([]GHLInboundMapEntry, 0, len(keys))
	for _, key := range keys {
		entry := base
		entry.SourceKey = key
		out = append(out, entry)
	}
	return out
}

func ghlInboundSourceKeys(e SunbaseFieldMapEntry) []string {
	destKey := strings.TrimSpace(e.DestKey)
	if destKey == "" {
		return nil
	}
	keys := []string{destKey}
	if e.GHLFieldName != nil {
		if name := strings.TrimSpace(*e.GHLFieldName); name != "" && name != destKey {
			keys = append(keys, name)
		}
	}
	if ghlFieldModel(e) == "opportunity" && !strings.Contains(destKey, ".") {
		keys = append(keys, "opportunity."+destKey)
	}
	return keys
}

func ghlContactStandardFieldsForInbound(config map[string]any) any {
	if config == nil {
		return defaultGHLContactStandardFieldsMap()
	}
	if _, ok := config["contact_standard_fields"]; !ok {
		return defaultGHLContactStandardFieldsMap()
	}
	return config["contact_standard_fields"]
}

func invertStandardFields(raw any, specs []ghlStandardInboundSpec) []GHLInboundMapEntry {
	fields := parseGHLStandardFieldsMap(raw)
	if len(fields) == 0 {
		return nil
	}
	var out []GHLInboundMapEntry
	for _, spec := range specs {
		fs, ok := fields[spec.fieldKey]
		if !ok || !ghlFieldSourceSet(fs) || fs.SourceType == "static" {
			continue
		}
		switch fs.SourceType {
		case "builtin":
			if fs.BuiltinField == nil {
				continue
			}
			bf := strings.TrimSpace(*fs.BuiltinField)
			for _, key := range spec.ghlKeys {
				b := bf
				out = append(out, GHLInboundMapEntry{
					SourceKey:    key,
					TargetType:   "builtin",
					BuiltinField: &b,
				})
			}
		case "custom":
			if fs.CustomFieldID == nil {
				continue
			}
			id := *fs.CustomFieldID
			for _, key := range spec.ghlKeys {
				cid := id
				out = append(out, GHLInboundMapEntry{
					SourceKey:     key,
					TargetType:    "custom",
					CustomFieldID: &cid,
				})
			}
		}
	}
	return out
}

func parseGHLStandardFieldsMap(raw any) map[string]GHLFieldSource {
	out := map[string]GHLFieldSource{}
	if raw == nil {
		return out
	}
	m, ok := raw.(map[string]any)
	if !ok {
		b, err := json.Marshal(raw)
		if err != nil {
			return out
		}
		if json.Unmarshal(b, &m) != nil {
			return out
		}
	}
	for k, v := range m {
		if k == "duration_minutes" {
			continue
		}
		fs := parseGHLFieldSource(v)
		if ghlFieldSourceSet(fs) {
			out[k] = fs
		}
	}
	return out
}
