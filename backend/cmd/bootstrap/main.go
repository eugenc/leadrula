// Command bootstrap creates the single publisher account and its first admin.
// Idempotent: re-running will not duplicate the publisher or admin.
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/echayko/leadrula/backend/db"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
)

func main() {
	cfg := config.Load()
	email := flag.String("email", os.Getenv("BOOTSTRAP_EMAIL"), "first admin email")
	password := flag.String("password", os.Getenv("BOOTSTRAP_PASSWORD"), "first admin password")
	name := flag.String("name", envOr("BOOTSTRAP_NAME", "Publisher Admin"), "first admin full name")
	pubName := flag.String("publisher", "HQ Publisher", "publisher account name")
	flag.Parse()

	if *email == "" || *password == "" {
		log.Fatal("bootstrap: email and password are required (flags or BOOTSTRAP_EMAIL/BOOTSTRAP_PASSWORD)")
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool, db.Migrations, db.Dir); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// publisher account (one only)
	var publisherID int64
	err = pool.QueryRow(ctx, `SELECT id FROM accounts WHERE type='publisher' LIMIT 1`).Scan(&publisherID)
	if err != nil {
		if err := pool.QueryRow(ctx,
			`INSERT INTO accounts(type, name) VALUES ('publisher', $1) RETURNING id`, *pubName).Scan(&publisherID); err != nil {
			log.Fatalf("create publisher: %v", err)
		}
		log.Printf("created publisher account id=%d", publisherID)
	} else {
		log.Printf("publisher already exists id=%d", publisherID)
	}

	// admin user
	var existing int64
	err = pool.QueryRow(ctx, `SELECT id FROM users WHERE email=$1`, *email).Scan(&existing)
	if err == nil {
		log.Printf("user %s already exists (id=%d); nothing to do", *email, existing)
		return
	}
	hash, err := auth.HashPassword(*password)
	if err != nil {
		log.Fatalf("hash: %v", err)
	}
	var userID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO users(account_id, email, password_hash, full_name, role)
		 VALUES ($1,$2,$3,$4,'admin') RETURNING id`,
		publisherID, *email, hash, *name).Scan(&userID); err != nil {
		log.Fatalf("create admin: %v", err)
	}
	log.Printf("created publisher admin id=%d email=%s", userID, *email)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
