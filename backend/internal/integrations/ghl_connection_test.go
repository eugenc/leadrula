package integrations

import (
	"testing"

	"github.com/echayko/leadrula/backend/internal/integrations/providers"
)

func testEncKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func TestGhlCredentialsHavePIT(t *testing.T) {
	key := testEncKey(t)
	plain := []byte(`{"private_integration_token":"pit-abc"}`)
	enc, err := encrypt(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	if !ghlCredentialsHavePIT(key, enc) {
		t.Fatal("expected true for valid encrypted PIT")
	}
	if ghlCredentialsHavePIT(key, nil) {
		t.Fatal("expected false for empty credentials")
	}
	emptyEnc, err := encrypt(key, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if ghlCredentialsHavePIT(key, emptyEnc) {
		t.Fatal("expected false for empty JSON credentials")
	}
}

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
	csf, ok := cfg["contact_standard_fields"].(map[string]any)
	if !ok || len(csf) != 9 {
		t.Fatalf("expected 9 default contact_standard_fields, got %#v", cfg["contact_standard_fields"])
	}
	if first, ok := csf["firstName"].(map[string]any); !ok || first["builtin_field"] != "first_name" {
		t.Fatalf("expected firstName -> first_name default, got %#v", csf["firstName"])
	}
}
