package contracts

import (
	"context"
	"testing"

	"github.com/echayko/leadrula/backend/internal/database"
)

func TestListWithNullDescription(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()
	svc := NewService(pool)
	_, err = svc.List(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
}
