// One-off helper: process a single integration_delivery_queue row.
// Usage: DATABASE_URL=... INTEGRATION_ENC_KEY=... go run ./scripts/run_delivery_job.go <queue_id>
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/integrations"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: go run ./scripts/run_delivery_job.go <queue_id>")
	}
	queueID, err := strconv.ParseInt(os.Args[1], 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	cfg := config.Load()
	encKey, err := hex.DecodeString(cfg.IntegrationEncKey)
	if err != nil || len(encKey) != 32 {
		log.Fatal("INTEGRATION_ENC_KEY must be 32-byte hex")
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	svc := integrations.NewService(pool, encKey, integrations.OAuthConfig{})
	if err := svc.ProcessJobByID(ctx, queueID); err != nil {
		log.Fatal(err)
	}
	fmt.Println("ok")
}
