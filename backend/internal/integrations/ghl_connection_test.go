package integrations

import (
	"testing"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
)

func TestValidateGHLConfigJSON_appointmentRequiresFields(t *testing.T) {
	err := providers.ValidateGHLConfigJSON(map[string]any{
		"location_id":        "loc1",
		"create_appointment": true,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestMergeGHLConfigDefaults(t *testing.T) {
	cfg := providers.MergeGHLConfigDefaults(map[string]any{"location_id": "x"})
	if cfg["create_contact"] != true {
		t.Fatal("expected create_contact true")
	}
	if _, ok := cfg["appointment_title_template"]; !ok {
		t.Fatal("expected default appointment_title_template")
	}
	if _, ok := cfg["appointment_datetime"]; !ok {
		t.Fatal("expected default appointment_datetime")
	}
}
