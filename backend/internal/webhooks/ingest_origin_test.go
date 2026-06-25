package webhooks

import (
	"context"
	"testing"
)

type mockInboundOriginApplier struct {
	connCalls []int64
	whCalls   []int64
	connOK    bool
}

func (m *mockInboundOriginApplier) TryApplyConnectionOriginRoute(_ context.Context, _, connectionID, _ int64, _ map[string]any) bool {
	m.connCalls = append(m.connCalls, connectionID)
	return m.connOK
}

func (m *mockInboundOriginApplier) TryApplyWebhookOriginRoute(_ context.Context, _, webhookID, _ int64, _ map[string]any) bool {
	m.whCalls = append(m.whCalls, webhookID)
	return false
}

func TestApplyInboundOriginRoutes_webhookOnly(t *testing.T) {
	mock := &mockInboundOriginApplier{}
	webhook := Webhook{ID: 10, AccountID: 1}

	applyInboundOriginRoutes(context.Background(), mock, webhook, 99, nil)

	if len(mock.connCalls) != 0 {
		t.Fatalf("connection calls = %v, want none", mock.connCalls)
	}
	if len(mock.whCalls) != 1 || mock.whCalls[0] != 10 {
		t.Fatalf("webhook calls = %v, want [10]", mock.whCalls)
	}
}

func TestApplyInboundOriginRoutes_integrationMatched(t *testing.T) {
	connID := int64(55)
	mock := &mockInboundOriginApplier{connOK: true}
	webhook := Webhook{ID: 10, AccountID: 1, IntegrationConnectionID: &connID}

	applyInboundOriginRoutes(context.Background(), mock, webhook, 99, map[string]any{"status": "Booked"})

	if len(mock.connCalls) != 1 || mock.connCalls[0] != 55 {
		t.Fatalf("connection calls = %v, want [55]", mock.connCalls)
	}
	if len(mock.whCalls) != 0 {
		t.Fatalf("webhook calls = %v, want none when integration route matched", mock.whCalls)
	}
}

func TestApplyInboundOriginRoutes_integrationFallbackToWebhook(t *testing.T) {
	connID := int64(55)
	mock := &mockInboundOriginApplier{connOK: false}
	webhook := Webhook{ID: 10, AccountID: 1, IntegrationConnectionID: &connID}

	applyInboundOriginRoutes(context.Background(), mock, webhook, 99, nil)

	if len(mock.connCalls) != 1 || mock.connCalls[0] != 55 {
		t.Fatalf("connection calls = %v, want [55]", mock.connCalls)
	}
	if len(mock.whCalls) != 1 || mock.whCalls[0] != 10 {
		t.Fatalf("webhook calls = %v, want [10]", mock.whCalls)
	}
}

func TestApplyInboundOriginRoutes_nilService(t *testing.T) {
	webhook := Webhook{ID: 10, AccountID: 1}
	applyInboundOriginRoutes(context.Background(), nil, webhook, 99, nil)
}

func TestApplyInboundOriginRoutes_zeroLeadID(t *testing.T) {
	mock := &mockInboundOriginApplier{}
	webhook := Webhook{ID: 10, AccountID: 1}
	applyInboundOriginRoutes(context.Background(), mock, webhook, 0, nil)
	if len(mock.connCalls) != 0 || len(mock.whCalls) != 0 {
		t.Fatal("expected no calls for zero lead id")
	}
}

func TestLeadIDsForOriginRoutes_dedupesSameLead(t *testing.T) {
	leadID := int64(42)
	results := []ActionResult{
		{LeadInternalID: leadID},
		{LeadInternalID: leadID},
		{LeadInternalID: 99},
	}
	ids := leadIDsForOriginRoutes(results)
	if len(ids) != 2 {
		t.Fatalf("ids = %v, want 2 unique lead ids", ids)
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	if !seen[leadID] || !seen[99] {
		t.Fatalf("ids = %v, want %d and 99", ids, leadID)
	}
}
