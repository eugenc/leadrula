// Command bootstrap-platform creates the platform operator account and its first admin.
// Use -promote to move an existing user onto the platform account as admin.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/echayko/leadrula/backend/db"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/handlerid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()
	email := flag.String("email", os.Getenv("PLATFORM_EMAIL"), "platform admin email")
	password := flag.String("password", os.Getenv("PLATFORM_PASSWORD"), "platform admin password (not required with -promote)")
	name := flag.String("name", envOr("PLATFORM_NAME", "Platform"), "platform account name")
	adminName := flag.String("admin-name", envOr("PLATFORM_ADMIN_NAME", "Platform Admin"), "first admin full name")
	promote := flag.Bool("promote", false, "move an existing user to the platform account (keeps password)")
	flag.Parse()

	if *email == "" {
		log.Fatal("bootstrap-platform: email is required (flag or PLATFORM_EMAIL)")
	}
	*email = strings.TrimSpace(strings.ToLower(*email))

	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool, db.Migrations, db.Dir); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	platformID, err := ensurePlatformAccount(ctx, pool, *name)
	if err != nil {
		log.Fatalf("platform account: %v", err)
	}

	if *promote {
		if err := promoteUser(ctx, pool, platformID, *email); err != nil {
			log.Fatalf("promote: %v", err)
		}
		return
	}

	if *password == "" {
		log.Fatal("bootstrap-platform: password is required unless -promote is set")
	}

	var existing int64
	err = pool.QueryRow(ctx, `SELECT id FROM users WHERE email=$1`, *email).Scan(&existing)
	if err == nil {
		log.Printf("user %s already exists (id=%d); use -promote to attach to platform", *email, existing)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		log.Fatalf("lookup user: %v", err)
	}

	hash, err := auth.HashPassword(*password)
	if err != nil {
		log.Fatalf("hash: %v", err)
	}
	var userID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO users(account_id, email, password_hash, full_name, role)
		 VALUES ($1,$2,$3,$4,'admin') RETURNING id`,
		platformID, *email, hash, *adminName).Scan(&userID); err != nil {
		log.Fatalf("create platform admin: %v", err)
	}
	log.Printf("created platform admin id=%d email=%s", userID, *email)
}

func ensurePlatformAccount(ctx context.Context, pool *pgxpool.Pool, name string) (int64, error) {
	var platformID int64
	err := pool.QueryRow(ctx, `SELECT id FROM accounts WHERE type='platform' LIMIT 1`).Scan(&platformID)
	if err == nil {
		log.Printf("platform account already exists id=%d", platformID)
		return platformID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	hid := handlerid.Generate("PL")
	if err := pool.QueryRow(ctx,
		`INSERT INTO accounts(type, name, handler_id) VALUES ('platform', $1, $2) RETURNING id`,
		name, hid).Scan(&platformID); err != nil {
		return 0, err
	}
	log.Printf("created platform account id=%d handler_id=%s", platformID, hid)
	return platformID, nil
}

func promoteUser(ctx context.Context, pool *pgxpool.Pool, platformID int64, email string) error {
	var userID int64
	var oldAccountID int64
	var oldRole string
	err := pool.QueryRow(ctx,
		`SELECT u.id, u.account_id, u.role FROM users u WHERE u.email = $1`, email).
		Scan(&userID, &oldAccountID, &oldRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("user not found: " + email)
	}
	if err != nil {
		return err
	}

	var oldType string
	_ = pool.QueryRow(ctx, `SELECT type FROM accounts WHERE id = $1`, oldAccountID).Scan(&oldType)

	if oldAccountID == platformID && oldRole == "admin" {
		log.Printf("user %s (id=%d) is already platform admin", email, userID)
		return nil
	}

	ct, err := pool.Exec(ctx,
		`UPDATE users SET account_id = $1, role = 'admin', updated_at = now() WHERE id = $2`,
		platformID, userID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errors.New("update failed")
	}
	log.Printf("promoted user %s (id=%d) from %s account id=%d to platform admin (account id=%d)",
		email, userID, oldType, oldAccountID, platformID)
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
