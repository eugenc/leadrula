package webhooks

import "testing"

func TestSyncOutboundSecretValue_signDisabled(t *testing.T) {
	got, err := syncOutboundSecretValue(false, "existing-secret")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("expected empty secret, got %q", got)
	}
}

func TestSyncOutboundSecretValue_keepsExisting(t *testing.T) {
	got, err := syncOutboundSecretValue(true, "existing-secret")
	if err != nil {
		t.Fatal(err)
	}
	if got != "existing-secret" {
		t.Fatalf("expected existing secret preserved, got %q", got)
	}
}

func TestSyncOutboundSecretValue_generatesWhenEnabled(t *testing.T) {
	got, err := syncOutboundSecretValue(true, "")
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("expected generated secret")
	}
	if len(got) != 64 {
		t.Fatalf("expected 64-char hex secret, got len %d", len(got))
	}
}
