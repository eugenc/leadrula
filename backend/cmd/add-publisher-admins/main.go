// Command add-publisher-admins creates publisher admin users (idempotent by email).
package main

import (
	"context"
	"flag"
	"log"
	"strings"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
)

func main() {
	password := flag.String("password", "", "password for new admins (required)")
	flag.Parse()
	if *password == "" {
		log.Fatal("add-publisher-admins: -password is required")
	}

	emails := flag.Args()
	if len(emails) == 0 {
		log.Fatal("add-publisher-admins: pass one or more admin emails as arguments")
	}

	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	var publisherID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts WHERE type='publisher' LIMIT 1`).Scan(&publisherID); err != nil {
		log.Fatalf("publisher: %v", err)
	}

	hash, err := auth.HashPassword(*password)
	if err != nil {
		log.Fatalf("hash: %v", err)
	}

	for _, email := range emails {
		email = strings.TrimSpace(strings.ToLower(email))
		if email == "" {
			continue
		}
		name := nameFromEmail(email)

		tag, err := pool.Exec(ctx,
			`INSERT INTO users(account_id, email, password_hash, full_name, role)
			 VALUES ($1,$2,$3,$4,'admin')
			 ON CONFLICT (email) DO UPDATE
			 SET account_id = EXCLUDED.account_id,
			     password_hash = EXCLUDED.password_hash,
			     full_name = EXCLUDED.full_name,
			     role = 'admin',
			     is_active = true`,
			publisherID, email, hash, name)
		if err != nil {
			log.Fatalf("user %s: %v", email, err)
		}
		if tag.RowsAffected() == 1 {
			log.Printf("created admin %s", email)
		} else {
			log.Printf("updated admin %s", email)
		}
	}
}

func nameFromEmail(email string) string {
	local := strings.Split(email, "@")[0]
	local = strings.ReplaceAll(local, ".", " ")
	local = strings.ReplaceAll(local, "_", " ")
	return strings.Title(local)
}
