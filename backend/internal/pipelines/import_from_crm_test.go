package pipelines

import "testing"

func TestResolveImportPipelineName_noConflict(t *testing.T) {
	existing := map[string]bool{"other": true}
	name, renamed := resolveImportPipelineName("Sales", "HubSpot", existing)
	if renamed || name != "Sales" {
		t.Fatalf("got name=%q renamed=%v", name, renamed)
	}
}

func TestResolveImportPipelineName_providerSuffix(t *testing.T) {
	existing := map[string]bool{"sales": true}
	name, renamed := resolveImportPipelineName("Sales", "HubSpot", existing)
	if !renamed || name != "Sales (HubSpot)" {
		t.Fatalf("got name=%q renamed=%v", name, renamed)
	}
}

func TestResolveImportPipelineName_numericSuffix(t *testing.T) {
	existing := map[string]bool{
		"sales":            true,
		"sales (hubspot)":  true,
	}
	name, renamed := resolveImportPipelineName("Sales", "HubSpot", existing)
	if !renamed || name != "Sales (HubSpot) (2)" {
		t.Fatalf("got name=%q renamed=%v", name, renamed)
	}
}
