package integrations

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

func railwayTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = railwayVar("DATABASE_PUBLIC_URL")
	}
	if url == "" {
		url = config.Load().DatabaseURL
	}
	pool, err := database.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func railwayVar(key string) string {
	out, err := exec.Command("railway", "variables", "--service", "leadrula", "--json").Output()
	if err != nil {
		return ""
	}
	var vars map[string]string
	if json.Unmarshal(out, &vars) != nil {
		return ""
	}
	return vars[key]
}

func railwayTestService(t *testing.T, pool *pgxpool.Pool) *Service {
	t.Helper()
	encHex := os.Getenv("INTEGRATION_ENC_KEY")
	if encHex == "" {
		encHex = railwayVar("INTEGRATION_ENC_KEY")
	}
	encKey, err := hex.DecodeString(encHex)
	if err != nil || len(encKey) != 32 {
		t.Skip("INTEGRATION_ENC_KEY not available (need 64 hex chars)")
	}
	cfg := config.Load()
	return NewService(pool, encKey, OAuthConfig{
		RedirectBase:       cfg.IntegrationOAuthRedirectBase,
		PipedriveClientID:  cfg.PipedriveClientID,
		PipedriveSecret:    cfg.PipedriveClientSecret,
		HubSpotClientID:    cfg.HubSpotClientID,
		HubSpotSecret:      cfg.HubSpotClientSecret,
		ZohoClientID:       cfg.ZohoCRMClientID,
		ZohoSecret:         cfg.ZohoCRMClientSecret,
		SalesforceClientID: cfg.SalesforceClientID,
		SalesforceSecret:   cfg.SalesforceClientSecret,
	})
}

func TestReclaimStaleProcessingJobs(t *testing.T) {
	pool := railwayTestPool(t)
	ctx := context.Background()

	var deliveryID int64
	var statusBefore string
	err := pool.QueryRow(ctx,
		`SELECT id, status::text FROM integration_delivery_queue ORDER BY created_at DESC LIMIT 1`,
	).Scan(&deliveryID, &statusBefore)
	if err != nil {
		t.Skip("no integration delivery in database")
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx,
			`UPDATE integration_delivery_queue SET status=$2::delivery_status, updated_at=now() WHERE id=$1`,
			deliveryID, statusBefore)
	})

	_, err = pool.Exec(ctx,
		`UPDATE integration_delivery_queue
		 SET status = 'processing', updated_at = now() - interval '3 minutes'
		 WHERE id = $1`, deliveryID)
	if err != nil {
		t.Fatalf("set processing: %v", err)
	}

	svc := &Service{pool: pool}
	if err := svc.reclaimStaleProcessingJobs(ctx); err != nil {
		t.Fatalf("reclaimStaleProcessingJobs: %v", err)
	}

	var statusAfter string
	if err := pool.QueryRow(ctx,
		`SELECT status::text FROM integration_delivery_queue WHERE id=$1`, deliveryID,
	).Scan(&statusAfter); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if statusAfter != "pending" {
		t.Fatalf("status = %q, want pending", statusAfter)
	}
}

func TestReclaimStaleProcessingJobs_leavesRecentProcessing(t *testing.T) {
	pool := railwayTestPool(t)
	ctx := context.Background()

	var deliveryID int64
	var statusBefore string
	err := pool.QueryRow(ctx,
		`SELECT id, status::text FROM integration_delivery_queue ORDER BY created_at DESC LIMIT 1`,
	).Scan(&deliveryID, &statusBefore)
	if err != nil {
		t.Skip("no integration delivery in database")
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx,
			`UPDATE integration_delivery_queue SET status=$2::delivery_status, updated_at=now() WHERE id=$1`,
			deliveryID, statusBefore)
	})

	_, err = pool.Exec(ctx,
		`UPDATE integration_delivery_queue
		 SET status = 'processing', updated_at = now()
		 WHERE id = $1`, deliveryID)
	if err != nil {
		t.Fatalf("set processing: %v", err)
	}

	svc := &Service{pool: pool}
	if err := svc.reclaimStaleProcessingJobs(ctx); err != nil {
		t.Fatalf("reclaimStaleProcessingJobs: %v", err)
	}

	var statusAfter string
	if err := pool.QueryRow(ctx,
		`SELECT status::text FROM integration_delivery_queue WHERE id=$1`, deliveryID,
	).Scan(&statusAfter); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if statusAfter != "processing" {
		t.Fatalf("status = %q, want processing", statusAfter)
	}
}

func TestProcessJobsOnce_pendingDeliveryCompletes(t *testing.T) {
	if os.Getenv("RUN_RAILWAY_INTEGRATION") != "1" {
		t.Skip("set RUN_RAILWAY_INTEGRATION=1 to run against Railway PG")
	}

	pool := railwayTestPool(t)
	ctx := context.Background()
	svc := railwayTestService(t, pool)

	const deliveryID int64 = 37
	var statusBefore string
	if err := pool.QueryRow(ctx,
		`SELECT status::text FROM integration_delivery_queue WHERE id=$1`, deliveryID,
	).Scan(&statusBefore); err != nil {
		t.Fatalf("read delivery: %v", err)
	}

	_, err := pool.Exec(ctx,
		`UPDATE integration_delivery_queue
		 SET status = 'pending', next_attempt_at = now(), updated_at = now()
		 WHERE id = $1`, deliveryID)
	if err != nil {
		t.Fatalf("reset pending: %v", err)
	}

	if err := svc.processJobs(ctx); err != nil {
		t.Fatalf("processJobs: %v", err)
	}

	deadline := time.Now().Add(25 * time.Second)
	var statusAfter string
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		if err := pool.QueryRow(ctx,
			`SELECT status::text FROM integration_delivery_queue WHERE id=$1`, deliveryID,
		).Scan(&statusAfter); err != nil {
			t.Fatalf("read status: %v", err)
		}
		if statusAfter != "processing" && statusAfter != "pending" {
			break
		}
	}

	if statusAfter == "processing" || statusAfter == "pending" {
		_, _ = pool.Exec(ctx,
			`UPDATE integration_delivery_queue SET status=$2::delivery_status, updated_at=now() WHERE id=$1`,
			deliveryID, statusBefore)
		t.Fatalf("delivery still %q after 25s", statusAfter)
	}
	t.Logf("delivery %d finished with status %q", deliveryID, statusAfter)
}
