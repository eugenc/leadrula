package webhooks

import "testing"

func TestVerifySecretForWebhook(t *testing.T) {
	_, full, hash, prefix, err := generateSecret()
	if err != nil {
		t.Fatal(err)
	}
	wh := &resolvedWebhook{
		SecretHash:            &hash,
		SecretPrefix:          prefix,
		InboundSecretRequired: true,
	}
	if !verifySecretForWebhook(wh, full) {
		t.Fatal("expected valid secret")
	}
	if verifySecretForWebhook(wh, "bad-prefix.bad-secret") {
		t.Fatal("expected invalid secret")
	}
}

func TestVerifySecretForWebhook_missingHash(t *testing.T) {
	wh := &resolvedWebhook{
		InboundSecretRequired: true,
		SecretPrefix:          "abc",
	}
	if verifySecretForWebhook(wh, "abc.secret") {
		t.Fatal("expected false when secret hash is missing")
	}
}
