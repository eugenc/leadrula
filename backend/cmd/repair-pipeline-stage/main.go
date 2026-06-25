// Command repair-pipeline-stage diagnoses and repairs leads whose stage_id
// does not belong to pipeline_id (board "Unplaced" state).
//
// Run against Railway production Postgres:
//
//	railway run go run ./cmd/repair-pipeline-stage -first Eugene -last Tester
//	railway run go run ./cmd/repair-pipeline-stage -repair
//	railway run go run ./cmd/repair-pipeline-stage -clear-stale-publisher-tracking
//	railway run go run ./cmd/repair-pipeline-stage -clear-stale-publisher-tracking -repair
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/database"
)

const repairSQL = `
WITH mismatched AS (
    SELECT l.id AS lead_id,
           l.pipeline_id,
           l.stage_id AS old_stage_id,
           l.contract_id,
           l.owner_account_id,
           ps.name AS old_stage_name
    FROM leads l
    JOIN pipeline_stages ps ON ps.id = l.stage_id
    JOIN accounts oa ON oa.id = l.owner_account_id
    WHERE l.pipeline_id IS NOT NULL
      AND l.stage_id IS NOT NULL
      AND l.pipeline_id <> ps.pipeline_id
      AND l.deleted_at IS NULL
      AND oa.type IN ('buyer', 'publisher')
),
repaired AS (
    SELECT m.lead_id,
           COALESCE(
               (SELECT p.buyer_target_stage_id
                FROM contract_participations p
                JOIN pipeline_stages bps ON bps.id = p.buyer_target_stage_id
                WHERE p.contract_id = m.contract_id
                  AND p.buyer_id = m.owner_account_id
                  AND p.status = 'active'
                  AND p.buyer_target_stage_id IS NOT NULL
                  AND bps.pipeline_id = m.pipeline_id
                ORDER BY p.id
                LIMIT 1),
               (SELECT ps2.id
                FROM pipeline_stages ps2
                WHERE ps2.pipeline_id = m.pipeline_id
                  AND ps2.name = m.old_stage_name
                ORDER BY ps2.position, ps2.id
                LIMIT 1),
               (SELECT ps3.id
                FROM pipeline_stages ps3
                WHERE ps3.pipeline_id = m.pipeline_id
                ORDER BY ps3.position, ps3.id
                LIMIT 1)
           ) AS new_stage_id
    FROM mismatched m
)
UPDATE leads l
SET stage_id = r.new_stage_id
FROM repaired r
WHERE l.id = r.lead_id
  AND r.new_stage_id IS NOT NULL
`

const publisherMirrorSQL = `
UPDATE leads l
SET publisher_stage_id = COALESCE(
        (SELECT csm.publisher_stage_id
         FROM contract_stage_maps csm
         WHERE csm.contract_id = l.contract_id
           AND csm.buyer_stage_id = l.stage_id
         ORDER BY CASE WHEN csm.participation_id IS NOT NULL THEN 0 ELSE 1 END,
                  csm.id
         LIMIT 1),
        (SELECT c.source_stage_id FROM contracts c WHERE c.id = l.contract_id),
        l.publisher_stage_id
    ),
    publisher_pipeline_id = COALESCE(
        l.publisher_pipeline_id,
        (SELECT c.source_pipeline_id FROM contracts c WHERE c.id = l.contract_id)
    )
WHERE l.contract_id IS NOT NULL
  AND l.deleted_at IS NULL
  AND l.publisher_pipeline_id IS NOT NULL
  AND l.stage_id IS NOT NULL
  AND EXISTS (
      SELECT 1 FROM pipeline_stages ps
      WHERE ps.id = l.stage_id AND ps.pipeline_id = l.pipeline_id
  )
`

const clearStalePublisherTrackingSQL = `
UPDATE leads
SET publisher_pipeline_id = NULL, publisher_stage_id = NULL
WHERE owner_account_id <> publisher_id
  AND status = 'distributed'
  AND publisher_pipeline_id IS NOT NULL
  AND deleted_at IS NULL
`

const stalePublisherTrackingCountSQL = `
SELECT COUNT(*)
FROM leads
WHERE owner_account_id <> publisher_id
  AND status = 'distributed'
  AND publisher_pipeline_id IS NOT NULL
  AND deleted_at IS NULL
`

func main() {
	first := flag.String("first", "", "lead first name (ILIKE)")
	last := flag.String("last", "", "lead last name (ILIKE)")
	repair := flag.Bool("repair", false, "repair all pipeline/stage mismatches")
	clearStale := flag.Bool("clear-stale-publisher-tracking", false, "clear stale publisher_pipeline_id on buyer-owned distributed leads")
	flag.Parse()

	ctx := context.Background()
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required (use: railway run go run ./cmd/repair-pipeline-stage ...)")
	}
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	var has0073 bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = '0073_repair_lead_stage_pipeline_mismatch')`).Scan(&has0073); err != nil {
		log.Fatalf("check migration 0073: %v", err)
	}
	fmt.Printf("migration 0073_repair_lead_stage_pipeline_mismatch applied: %v\n", has0073)

	var mismatchCount int
	if err := pool.QueryRow(ctx, mismatchCountSQL).Scan(&mismatchCount); err != nil {
		log.Fatalf("count mismatches: %v", err)
	}
	fmt.Printf("leads with pipeline/stage mismatch: %d\n", mismatchCount)

	var staleCount int
	if err := pool.QueryRow(ctx, stalePublisherTrackingCountSQL).Scan(&staleCount); err != nil {
		log.Fatalf("count stale publisher tracking: %v", err)
	}
	fmt.Printf("buyer-owned distributed leads with stale publisher tracking: %d\n", staleCount)

	if *first != "" || *last != "" {
		if err := diagnoseLead(ctx, pool, *first, *last); err != nil {
			log.Fatalf("diagnose: %v", err)
		}
	}

	if *repair && mismatchCount > 0 {
		tag, err := pool.Exec(ctx, repairSQL)
		if err != nil {
			log.Fatalf("repair: %v", err)
		}
		fmt.Printf("repaired %d lead(s)\n", tag.RowsAffected())
		tag, err = pool.Exec(ctx, publisherMirrorSQL)
		if err != nil {
			log.Fatalf("publisher mirror sync: %v", err)
		}
		fmt.Printf("publisher mirror rows updated: %d\n", tag.RowsAffected())
		var after int
		if err := pool.QueryRow(ctx, mismatchCountSQL).Scan(&after); err != nil {
			log.Fatalf("count after repair: %v", err)
		}
		fmt.Printf("remaining mismatches: %d\n", after)
		if after > 0 {
			os.Exit(1)
		}
	} else if *repair && mismatchCount == 0 {
		fmt.Println("no pipeline/stage mismatches to repair")
	}

	if *clearStale {
		if staleCount == 0 {
			fmt.Println("no stale publisher tracking to clear")
			return
		}
		if !*repair {
			fmt.Printf("dry run: would clear publisher tracking on %d lead(s) (pass -repair to apply)\n", staleCount)
			return
		}
		tag, err := pool.Exec(ctx, clearStalePublisherTrackingSQL)
		if err != nil {
			log.Fatalf("clear stale publisher tracking: %v", err)
		}
		fmt.Printf("cleared publisher tracking on %d lead(s)\n", tag.RowsAffected())
		var after int
		if err := pool.QueryRow(ctx, stalePublisherTrackingCountSQL).Scan(&after); err != nil {
			log.Fatalf("count after clear: %v", err)
		}
		fmt.Printf("remaining stale publisher tracking: %d\n", after)
	}
}

const mismatchCountSQL = `
SELECT COUNT(*)
FROM leads l
JOIN pipeline_stages ps ON ps.id = l.stage_id
JOIN accounts oa ON oa.id = l.owner_account_id
WHERE l.pipeline_id IS NOT NULL
  AND l.stage_id IS NOT NULL
  AND l.pipeline_id <> ps.pipeline_id
  AND l.deleted_at IS NULL
  AND oa.type IN ('buyer', 'publisher')`

func diagnoseLead(ctx context.Context, pool database.Querier, first, last string) error {
	rows, err := pool.Query(ctx,
		`SELECT l.id, l.first_name, l.last_name,
		        l.pipeline_id, pl.name,
		        l.stage_id, ps.name, ps.pipeline_id,
		        l.publisher_pipeline_id, l.publisher_stage_id,
		        l.owner_account_id, l.publisher_id, l.status::text
		 FROM leads l
		 LEFT JOIN pipelines pl ON pl.id = l.pipeline_id
		 LEFT JOIN pipeline_stages ps ON ps.id = l.stage_id
		 WHERE l.deleted_at IS NULL
		   AND ($1 = '' OR l.first_name ILIKE $1)
		   AND ($2 = '' OR l.last_name ILIKE $2)
		 ORDER BY l.id`,
		like(first), like(last))
	if err != nil {
		return err
	}
	defer rows.Close()

	var found bool
	for rows.Next() {
		found = true
		var (
			id, pipelineID, stageID, stagePipelineID                 *int64
			pubPipelineID, pubStageID                                *int64
			ownerID, publisherID                                     int64
			firstName, lastName, pipelineName, stageName, status     string
		)
		if err := rows.Scan(
			&id, &firstName, &lastName,
			&pipelineID, &pipelineName,
			&stageID, &stageName, &stagePipelineID,
			&pubPipelineID, &pubStageID,
			&ownerID, &publisherID, &status,
		); err != nil {
			return err
		}
		mismatch := pipelineID != nil && stageID != nil && stagePipelineID != nil && *pipelineID != *stagePipelineID
		fmt.Printf("\n--- lead id=%d %s %s status=%s ---\n", *id, firstName, lastName, status)
		fmt.Printf("pipeline_id=%v (%s) stage_id=%v (%s) stage_pipeline_id=%v\n",
			ptr(pipelineID), pipelineName, ptr(stageID), stageName, ptr(stagePipelineID))
		fmt.Printf("publisher_pipeline_id=%v publisher_stage_id=%v owner=%d publisher=%d\n",
			ptr(pubPipelineID), ptr(pubStageID), ownerID, publisherID)
		if mismatch {
			fmt.Println("UNPLACED: stage does not belong to pipeline_id")
		} else if stageID == nil {
			fmt.Println("UNPLACED: stage_id is null")
		} else {
			fmt.Println("pipeline/stage consistent")
		}

		hrows, err := pool.Query(ctx,
			`SELECT h.created_at, h.from_stage_id, h.to_stage_id, h.actor_type, h.actor_label
			 FROM lead_stage_history h WHERE h.lead_id = $1
			 ORDER BY h.created_at DESC LIMIT 10`, *id)
		if err != nil {
			return err
		}
		fmt.Println("recent stage history:")
		for hrows.Next() {
			var fromID, toID *int64
			var createdAt time.Time
			var actorType string
			var actorLabel *string
			if err := hrows.Scan(&createdAt, &fromID, &toID, &actorType, &actorLabel); err != nil {
				hrows.Close()
				return err
			}
			fmt.Printf("  %s from=%s to=%s actor=%s/%s\n", createdAt.UTC().Format(time.RFC3339), ptr(fromID), ptr(toID), actorType, ptr(actorLabel))
		}
		hrows.Close()

		if stageID != nil {
			srows, err := pool.Query(ctx,
				`SELECT sr.id, sr.position, sr.actions::text
				 FROM stage_rules sr WHERE sr.stage_id = $1 ORDER BY sr.position, sr.id`, *stageID)
			if err != nil {
				return err
			}
			fmt.Printf("stage rules on stage_id=%d:\n", *stageID)
			for srows.Next() {
				var ruleID, pos int64
				var actions string
				if err := srows.Scan(&ruleID, &pos, &actions); err != nil {
					srows.Close()
					return err
				}
				fmt.Printf("  rule id=%d pos=%d actions=%s\n", ruleID, pos, actions)
			}
			srows.Close()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("no leads matching first=%q last=%q", first, last)
	}
	return nil
}

func like(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return s
}

func ptr[T any](p *T) string {
	if p == nil {
		return "null"
	}
	return fmt.Sprint(*p)
}
