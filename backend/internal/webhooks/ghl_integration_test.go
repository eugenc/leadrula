package webhooks

import (
	"testing"
)

func TestParseGHLWebhookIDs_fromConfig(t *testing.T) {
	ids := ParseGHLWebhookIDs(map[string]any{
		"inbound_webhook_slug": "acme-ghl-abc",
		"inbound_webhook_id":   float64(42),
		"webhook_ids": map[string]any{
			"inbound": float64(42),
		},
	})
	if ids.Inbound != 42 || ids.InboundSlug != "acme-ghl-abc" {
		t.Fatalf("ids=%+v", ids)
	}
}

func TestGHLBaseSlug(t *testing.T) {
	slug, err := ghlBaseSlug("Acme Solar", "abc12345-uuid")
	if err != nil || slug == "" {
		t.Fatalf("slug=%q err=%v", slug, err)
	}
}
