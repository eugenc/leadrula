// Command seed-demo populates a development database with buyers, pipelines,
// contracts, routing campaigns and sample leads. Safe to skip if already seeded.
package main

import (
	"context"
	"log"

	"github.com/echayko/leadrula/backend/db"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

var disqReasons = []string{
	"Cancelled @ Door", "Not Interested", "Credit Declined", "Cancelled Instal",
	"NQ-ROOF", "NQ-Shading", "NQ-TH/Trailer/Apt", "NQ-Has Solar", "NQ-Selling/Moved",
	"NQ-Homestead Utility", "No Contact", "NQ-Low Bill", "NQ-Renter",
}

func main() {
	cfg := config.Load()
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool, db.Migrations, db.Dir); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	var publisherID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts WHERE type='publisher' LIMIT 1`).Scan(&publisherID); err != nil {
		log.Fatal("seed-demo: no publisher account; run cmd/bootstrap first")
	}

	var buyerCount int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM accounts WHERE type='buyer'`).Scan(&buyerCount)
	if buyerCount > 0 {
		log.Printf("seed-demo: %d buyers already exist; skipping (drop schema to reseed)", buyerCount)
		return
	}

	// Publisher pipeline
	pubPipeline := insertPipeline(ctx, pool, publisherID, "Lead Distribution")
	pubNew := insertStage(ctx, pool, pubPipeline, "New", 0, true, false)
	_ = insertStage(ctx, pool, pubPipeline, "Qualifying", 1, true, false)
	pubReady := insertStage(ctx, pool, pubPipeline, "Ready to Distribute", 2, false, false)
	_ = insertStage(ctx, pool, pubPipeline, "Distributed", 3, false, false)
	pubReturned := insertStage(ctx, pool, pubPipeline, "Returned", 4, false, false)

	seedReasons(ctx, pool, publisherID)
	insertCustomField(ctx, pool, publisherID, "Utility Provider", "utility_provider", "text")

	// Buyers
	solar := insertAccount(ctx, pool, "buyer", "Solar Pros")
	roofing := insertAccount(ctx, pool, "buyer", "Roofing Co")

	for _, b := range []struct {
		id           int64
		campaign     string
		adminEmail   string
		startBalance float64
	}{
		{solar, "solar_ontario_q2", "admin@solarpros.test", 500},
		{roofing, "roofing_gta", "admin@roofingco.test", 250},
	} {
		seedBuyer(ctx, pool, publisherID, b.id, b.campaign, b.adminEmail, b.startBalance,
			pubPipeline, pubReady, pubReturned)
	}

	// An unmatched lead in the intake queue (publisher review)
	leadID, _ := insertLead(ctx, pool, publisherID, publisherID, "unknown_campaign", "Pat", "Queue",
		"+15550000000", &pubPipeline, &pubNew, "review")
	if _, err := pool.Exec(ctx,
		`INSERT INTO lead_intake_queue(lead_id, raw_payload, campaign_name) VALUES ($1,'{"campaign_name":"unknown_campaign"}','unknown_campaign')`,
		leadID); err != nil {
		log.Fatalf("queue insert: %v", err)
	}

	log.Println("seed-demo complete")
}

func seedBuyer(ctx context.Context, pool *pgxpool.Pool, publisherID, buyerID int64, campaign, adminEmail string, startBalance float64, pubPipeline, pubReady, pubReturned int64) {
	// admin user
	hash, _ := auth.HashPassword("password123")
	_, _ = pool.Exec(ctx,
		`INSERT INTO users(account_id, email, password_hash, full_name, role) VALUES ($1,$2,$3,'Buyer Admin','admin')`,
		buyerID, adminEmail, hash)

	// pipeline + stages
	pipe := insertPipeline(ctx, pool, buyerID, "Sales")
	newLead := insertStage(ctx, pool, pipe, "New Lead", 0, true, false)
	_ = insertStage(ctx, pool, pipe, "Contacted", 1, true, false)
	appt := insertStage(ctx, pool, pipe, "Appointment Set", 2, true, false)
	_ = insertStage(ctx, pool, pipe, "Sold", 3, false, false)
	missed := insertStage(ctx, pool, pipe, "Missed Appointment", 4, false, false)
	_ = insertStage(ctx, pool, pipe, "Disqualified", 5, false, true)

	seedReasons(ctx, pool, buyerID)
	insertCustomField(ctx, pool, buyerID, "Utility Provider", "utility_provider", "text")

	// balance
	_, _ = pool.Exec(ctx, `INSERT INTO buyer_balances(buyer_id, balance) VALUES ($1,$2)
		ON CONFLICT (buyer_id) DO UPDATE SET balance=EXCLUDED.balance`, buyerID, startBalance)

	// contract
	var contractID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO contracts(publisher_id, buyer_id, name, source_pipeline_id, source_stage_id,
		    buyer_pipeline_id, return_stage_id, rate_per_lead)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		publisherID, buyerID, "Contract", pubPipeline, pubReady, pipe, pubReturned, 25.00).Scan(&contractID); err != nil {
		log.Fatalf("contract: %v", err)
	}
	// return rule: Missed Appointment returns the lead
	_, _ = pool.Exec(ctx, `INSERT INTO contract_return_rules(contract_id, buyer_stage_id) VALUES ($1,$2)`, contractID, missed)

	// routing campaign + field map
	var campaignID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO routing_campaigns(publisher_id, campaign_name, target_pipeline_id, target_stage_id)
		 VALUES ($1,$2,$3,$4) RETURNING id`,
		publisherID, campaign, pipe, newLead).Scan(&campaignID); err != nil {
		log.Fatalf("campaign: %v", err)
	}
	for _, fm := range []struct{ src, field string }{
		{"phone_number", "phone"}, {"fname", "first_name"}, {"lname", "last_name"}, {"email", "email"},
	} {
		field := fm.field
		_, _ = pool.Exec(ctx,
			`INSERT INTO routing_field_map(campaign_id, source_key, target_type, builtin_field) VALUES ($1,$2,'builtin',$3)`,
			campaignID, fm.src, field)
	}

	// sample distributed leads
	for i, nm := range [][2]string{{"Jane", "Doe"}, {"John", "Smith"}, {"Maria", "Lopez"}} {
		stage := newLead
		if i == 1 {
			stage = appt
		}
		lid, _ := insertLead(ctx, pool, buyerID, publisherID, campaign, nm[0], nm[1], "+1555000111"+itoa(i), &pipe, &stage, "distributed")
		_, _ = pool.Exec(ctx, `UPDATE leads SET contract_id=$2 WHERE id=$1`, lid, contractID)
		// debit
		debit(ctx, pool, buyerID, 25.00, lid, contractID)
	}
}

func seedReasons(ctx context.Context, pool *pgxpool.Pool, accountID int64) {
	for i, label := range disqReasons {
		_, _ = pool.Exec(ctx,
			`INSERT INTO disqualification_reasons(account_id, label, position) VALUES ($1,$2,$3)`,
			accountID, label, i)
	}
}

func insertAccount(ctx context.Context, pool *pgxpool.Pool, atype, name string) int64 {
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO accounts(type, name) VALUES ($1,$2) RETURNING id`, atype, name).Scan(&id); err != nil {
		log.Fatalf("account %s: %v", name, err)
	}
	return id
}

func insertPipeline(ctx context.Context, pool *pgxpool.Pool, accountID int64, name string) int64 {
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO pipelines(account_id, name) VALUES ($1,$2) RETURNING id`, accountID, name).Scan(&id); err != nil {
		log.Fatalf("pipeline %s: %v", name, err)
	}
	return id
}

func insertStage(ctx context.Context, pool *pgxpool.Pool, pipelineID int64, name string, pos int, promptAction, promptDisq bool) int64 {
	var id int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO pipeline_stages(pipeline_id, name, position, prompt_action_datetime, prompt_disqualification)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		pipelineID, name, pos, promptAction, promptDisq).Scan(&id); err != nil {
		log.Fatalf("stage %s: %v", name, err)
	}
	return id
}

func insertCustomField(ctx context.Context, pool *pgxpool.Pool, accountID int64, name, key, ftype string) int64 {
	var id int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO custom_fields(account_id, name, field_key, type) VALUES ($1,$2,$3,$4) RETURNING id`,
		accountID, name, key, ftype).Scan(&id); err != nil {
		log.Fatalf("custom field %s: %v", name, err)
	}
	return id
}

func insertLead(ctx context.Context, pool *pgxpool.Pool, ownerID, publisherID int64, campaign, first, last, phone string, pipelineID, stageID *int64, status string) (int64, string) {
	var id int64
	var publicID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO leads(owner_account_id, publisher_id, campaign_name, first_name, last_name, phone, pipeline_id, stage_id, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id, public_id`,
		ownerID, publisherID, campaign, first, last, phone, pipelineID, stageID, status).Scan(&id, &publicID); err != nil {
		log.Fatalf("lead %s %s: %v", first, last, err)
	}
	return id, publicID
}

func debit(ctx context.Context, pool *pgxpool.Pool, buyerID int64, amount float64, leadID, contractID int64) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("debit begin: %v", err)
	}
	defer tx.Rollback(ctx)
	var bal float64
	_ = tx.QueryRow(ctx, `SELECT balance::float8 FROM buyer_balances WHERE buyer_id=$1 FOR UPDATE`, buyerID).Scan(&bal)
	newBal := bal - amount
	_, _ = tx.Exec(ctx, `UPDATE buyer_balances SET balance=$2 WHERE buyer_id=$1`, buyerID, newBal)
	_, _ = tx.Exec(ctx,
		`INSERT INTO transactions(buyer_id, lead_id, contract_id, type, amount, balance_after, description)
		 VALUES ($1,$2,$3,'debit',$4,$5,'demo lead distributed')`,
		buyerID, leadID, contractID, -amount, newBal)
	_ = tx.Commit(ctx)
}

func itoa(i int) string { return string(rune('0' + i)) }
