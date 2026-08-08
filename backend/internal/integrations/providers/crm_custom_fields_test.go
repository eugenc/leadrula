package providers

import "testing"

func TestMapCRMFieldType(t *testing.T) {
	tests := []struct {
		provider string
		dataType string
		want     string
	}{
		{"ghl", "TEXT", "text"},
		{"ghl", "numerical", "number"},
		{"ghl", "date", "date"},
		{"ghl", "single_options", "dropdown"},
		{"hubspot", "number", "number"},
		{"hubspot", "booleancheckbox", "checkbox"},
		{"salesforce", "currency", "number"},
		{"salesforce", "picklist", "dropdown"},
		{"pipedrive", "monetary", "number"},
		{"pipedrive", "enum", "dropdown"},
		{"zoho_crm", "percent", "number"},
		{"zoho_crm", "multiselectpicklist", "dropdown"},
	}
	for _, tc := range tests {
		if got := MapCRMFieldType(tc.provider, tc.dataType); got != tc.want {
			t.Errorf("MapCRMFieldType(%q, %q) = %q, want %q", tc.provider, tc.dataType, got, tc.want)
		}
	}
}

func TestCRMInboundSourceKey(t *testing.T) {
	field := CRMCustomField{ID: "1", FieldKey: "solar_type", Name: "Solar Type"}
	if got := CRMInboundSourceKey("pipedrive", field); got != "current.solar_type" {
		t.Fatalf("pipedrive key = %q", got)
	}
	if got := CRMInboundSourceKey("zoho_crm", field); got != "data.solar_type" {
		t.Fatalf("zoho key = %q", got)
	}
	if got := CRMInboundSourceKey("hubspot", field); got != "solar_type" {
		t.Fatalf("hubspot key = %q", got)
	}
}

func TestSlugFieldKey(t *testing.T) {
	if got := SlugFieldKey("  Solar Type! "); got != "solar_type" {
		t.Fatalf("SlugFieldKey = %q", got)
	}
}

func TestPrepareCRMInboundFlat_hubspot(t *testing.T) {
	flat := PrepareCRMInboundFlat("hubspot", map[string]any{
		"propertyName":  "custom_score",
		"propertyValue": "42",
		"properties": map[string]any{
			"firstname": "Ada",
		},
	})
	if flat["custom_score"] != "42" {
		t.Fatalf("expected custom_score, got %v", flat["custom_score"])
	}
	if flat["firstname"] != "Ada" {
		t.Fatalf("expected firstname, got %v", flat["firstname"])
	}
}

func TestPrepareCRMInboundFlat_pipedrive(t *testing.T) {
	flat := PrepareCRMInboundFlat("pipedrive", map[string]any{
		"current": map[string]any{
			"abc123": "value",
			"custom_fields": map[string]any{
				"xyz": "nested",
			},
		},
	})
	if flat["current.abc123"] != "value" {
		t.Fatalf("expected current.abc123, got %v", flat["current.abc123"])
	}
	if flat["current.xyz"] != "nested" {
		t.Fatalf("expected current.xyz, got %v", flat["current.xyz"])
	}
}

func TestCRMCustomFieldsSupported(t *testing.T) {
	if !CRMCustomFieldsSupported("hubspot") {
		t.Fatal("hubspot should be supported")
	}
	if CRMCustomFieldsSupported("stripe") {
		t.Fatal("stripe should not be supported")
	}
}
