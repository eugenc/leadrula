package integrations

import (
	"testing"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func TestVoiceUniFlattenPayload(t *testing.T) {
	flat := voiceuniFlattenPayload(map[string]any{
		"external_id":   "vu-1",
		"first_name":    "Jane",
		"connection_id": "conn-uuid",
		"custom":        map[string]any{"utility_provider": "Acme"},
	})
	if flat["utility_provider"] != "Acme" {
		t.Fatalf("custom flatten = %v", flat["utility_provider"])
	}
	if _, ok := flat["connection_id"]; ok {
		t.Fatal("connection_id should be stripped")
	}
}

func TestVoiceUniHasIdentity(t *testing.T) {
	if voiceuniHasIdentity(map[string]any{"external_id": "x"}) {
		t.Fatal("external_id alone is not identity")
	}
	if !voiceuniHasIdentity(map[string]any{"phone": "+15551234567"}) {
		t.Fatal("phone should count as identity")
	}
}

func TestIngestVoiceUni_missingExternalID(t *testing.T) {
	svc := &Service{}
	_, err := svc.IngestVoiceUni(t.Context(), 1, VoiceUniIngestParams{
		Raw: map[string]any{"first_name": "Jane", "phone": "+1"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	appErr, ok := err.(*httpx.AppError)
	if !ok || appErr.Code != httpx.CodeValidation {
		t.Fatalf("err = %v", err)
	}
}
