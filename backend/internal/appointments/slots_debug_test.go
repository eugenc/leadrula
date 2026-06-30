package appointments_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/echayko/leadrula/backend/internal/appointments"
	"github.com/echayko/leadrula/backend/internal/database"
)

func TestDebugListFreeSlots(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil { t.Fatal(err) }
	svc := appointments.NewService(pool, nil, nil, nil)
	slots, err := svc.ListFreeSlots(ctx, 1, 5, "2026-07-01")
	if err != nil {
		t.Fatalf("ERROR: %T %v", err, err)
	}
	fmt.Printf("OK: %d slots\n", len(slots))
}
