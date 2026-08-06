// Command credit-buyer adds wallet balance to a buyer by name (dev/admin ops).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/echayko/leadrula/backend/internal/billing"
	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
)

func main() {
	name := flag.String("name", "", "buyer account name (partial match, case-insensitive)")
	amount := flag.Float64("amount", 0, "USD amount to credit")
	desc := flag.String("desc", "Manual admin credit", "transaction description")
	dryRun := flag.Bool("dry-run", false, "print match only, do not credit")
	flag.Parse()

	if *name == "" || *amount <= 0 {
		log.Fatal("usage: credit-buyer -name \"Nova Structa\" -amount 500")
	}

	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	var buyerID int64
	var buyerName string
	err = pool.QueryRow(ctx,
		`SELECT id, name FROM accounts WHERE type='buyer' AND name ILIKE $1 ORDER BY id LIMIT 1`,
		"%"+strings.TrimSpace(*name)+"%").Scan(&buyerID, &buyerName)
	if err != nil {
		// Fuzzy: any word from the search string must appear in the name.
		words := strings.Fields(strings.ToLower(strings.TrimSpace(*name)))
		if len(words) > 0 {
			var conds []string
			var args []any
			for i, w := range words {
				conds = append(conds, fmt.Sprintf("name ILIKE $%d", i+1))
				args = append(args, "%"+w+"%")
			}
			q := fmt.Sprintf(`SELECT id, name FROM accounts WHERE type='buyer' AND %s ORDER BY id LIMIT 1`,
				strings.Join(conds, " AND "))
			err = pool.QueryRow(ctx, q, args...).Scan(&buyerID, &buyerName)
		}
	}
	if err != nil {
		rows, qerr := pool.Query(ctx, `SELECT id, name, type FROM accounts ORDER BY type, name`)
		if qerr == nil {
			fmt.Printf("buyer not found matching %q. accounts:\n", *name)
			for rows.Next() {
				var id int64
				var n, typ string
				_ = rows.Scan(&id, &n, &typ)
				fmt.Printf("  id=%d type=%s name=%q\n", id, typ, n)
			}
			rows.Close()
		} else {
			fmt.Printf("list accounts failed: %v\n", qerr)
		}
		log.Fatalf("buyer not found matching %q: %v", *name, err)
	}

	svc := billing.NewService(pool, nil, nil, nil, nil, "")
	balBefore, _ := svc.GetBalance(ctx, buyerID)
	fmt.Printf("buyer id=%d name=%q balance_before=%.2f\n", buyerID, buyerName, balBefore)

	if *dryRun {
		return
	}

	txn, err := svc.Topup(ctx, buyerID, *amount, *desc)
	if err != nil {
		log.Fatalf("topup: %v", err)
	}
	balAfter, _ := svc.GetBalance(ctx, buyerID)
	fmt.Printf("credited %.2f txn_id=%d balance_after=%.2f\n", *amount, txn.ID, balAfter)
}
