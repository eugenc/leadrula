package leads

import (
	"context"
	"testing"
)

func TestTryApplyWebhookOriginRoute_noMatch(t *testing.T) {
	pool := connectLeadsTestDB(t)
	repo := NewRepository(pool)
	svc := &Service{repo: repo}

	ok := svc.TryApplyWebhookOriginRoute(context.Background(), 1, 999999999, 1, nil)
	if ok {
		t.Fatal("expected false when no webhook-origin route matches")
	}
}

func TestTryApplyConnectionOriginRoute_noMatch(t *testing.T) {
	pool := connectLeadsTestDB(t)
	repo := NewRepository(pool)
	svc := &Service{repo: repo}

	ok := svc.TryApplyConnectionOriginRoute(context.Background(), 1, 999999999, 1, nil)
	if ok {
		t.Fatal("expected false when no integration-origin route matches")
	}
}

func TestTryApplyWebhookOriginRoute_invalidArgs(t *testing.T) {
	pool := connectLeadsTestDB(t)
	repo := NewRepository(pool)
	svc := &Service{repo: repo}

	if svc.TryApplyWebhookOriginRoute(context.Background(), 1, 0, 1, nil) {
		t.Fatal("expected false for zero webhook id")
	}
	if svc.TryApplyWebhookOriginRoute(context.Background(), 1, 1, 0, nil) {
		t.Fatal("expected false for zero lead id")
	}
	if (*Service)(nil).TryApplyWebhookOriginRoute(context.Background(), 1, 1, 1, nil) {
		t.Fatal("expected false for nil service")
	}
}

func TestTryApplyConnectionOriginRoute_invalidArgs(t *testing.T) {
	pool := connectLeadsTestDB(t)
	repo := NewRepository(pool)
	svc := &Service{repo: repo}

	if svc.TryApplyConnectionOriginRoute(context.Background(), 1, 0, 1, nil) {
		t.Fatal("expected false for zero connection id")
	}
	if svc.TryApplyConnectionOriginRoute(context.Background(), 1, 1, 0, nil) {
		t.Fatal("expected false for zero lead id")
	}
	if (*Service)(nil).TryApplyConnectionOriginRoute(context.Background(), 1, 1, 1, nil) {
		t.Fatal("expected false for nil service")
	}
}
