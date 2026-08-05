package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/echayko/leadrula/backend/internal/customfields"
	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/internal/leads"
)

func (s *Service) RunWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.processJobs(ctx); err != nil {
				log.Printf("integrations worker: %v", err)
			}
		}
	}
}

func (s *Service) reclaimStaleProcessingJobs(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE integration_delivery_queue
		SET status = 'pending', next_attempt_at = now(), updated_at = now()
		WHERE status = 'processing'
		  AND updated_at < now() - interval '2 minutes'`)
	return err
}

func (s *Service) processJobs(ctx context.Context) error {
	if err := s.reclaimStaleProcessingJobs(ctx); err != nil {
		log.Printf("integrations worker: reclaim stale processing: %v", err)
	}
	rows, err := s.pool.Query(ctx, `
		UPDATE integration_delivery_queue
		SET status = 'processing', attempts = attempts + 1, updated_at = now()
		WHERE id IN (
			SELECT id FROM integration_delivery_queue
			WHERE status = 'pending'
			  AND next_attempt_at <= now()
			  AND attempts < max_attempts
			ORDER BY created_at
			LIMIT 10
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, connection_id, route_id, lead_id, payload, attempts, webhook_trigger_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var jobID, connID, leadID int64
		var routeID, triggerID *int64
		var payload json.RawMessage
		var attempts int
		if err := rows.Scan(&jobID, &connID, &routeID, &leadID, &payload, &attempts, &triggerID); err != nil {
			continue
		}
		go s.executeJob(context.Background(), jobID, connID, leadID, payload, attempts, triggerID)
	}
	return rows.Err()
}

// ProcessJobByID runs a single queue row synchronously (used by ops scripts and retries).
func (s *Service) ProcessJobByID(ctx context.Context, jobID int64) error {
	var connID, leadID int64
	var routeID, triggerID *int64
	var payload json.RawMessage
	var attempts int
	err := s.pool.QueryRow(ctx,
		`UPDATE integration_delivery_queue
		 SET status = 'processing', attempts = attempts + 1, updated_at = now()
		 WHERE id = $1 AND status IN ('pending', 'dead')
		 RETURNING connection_id, route_id, lead_id, payload, attempts, webhook_trigger_id`,
		jobID).Scan(&connID, &routeID, &leadID, &payload, &attempts, &triggerID)
	if err != nil {
		return fmt.Errorf("queue job %d: %w", jobID, err)
	}
	s.executeJob(ctx, jobID, connID, leadID, payload, attempts, triggerID)

	var status string
	var lastErr *string
	if err := s.pool.QueryRow(ctx,
		`SELECT status::text, last_error FROM integration_delivery_queue WHERE id = $1`, jobID).
		Scan(&status, &lastErr); err != nil {
		return err
	}
	if status != "success" {
		msg := "delivery failed"
		if lastErr != nil && *lastErr != "" {
			msg = *lastErr
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func (s *Service) executeJob(ctx context.Context, jobID, connID, leadID int64, payload json.RawMessage, attempts int, triggerID *int64) {
	defer func() {
		if r := recover(); r != nil {
			s.markFailed(ctx, jobID, attempts, fmt.Sprintf("worker panic: %v", r))
		}
	}()

	start := time.Now()

	var encCredentials []byte
	var providerSlug string
	var config json.RawMessage
	var status string
	if err := s.pool.QueryRow(ctx,
		`SELECT c.credentials, p.slug, c.config, c.status
		 FROM integration_connections c
		 JOIN integration_providers p ON p.id = c.provider_id
		 WHERE c.id = $1`, connID).Scan(&encCredentials, &providerSlug, &config, &status); err != nil {
		s.markFailed(ctx, jobID, attempts, "connection not found: "+err.Error())
		return
	}
	if status != "active" {
		s.markFailed(ctx, jobID, attempts, "connection not active")
		return
	}

	var connConfig map[string]any
	_ = json.Unmarshal(config, &connConfig)

	var credentials []byte
	var err error
	if _, oauthOK := oauthProviders[providerSlug]; oauthOK {
		credentials, err = s.refreshOAuthToken(ctx, connID, providerSlug, connConfig)
	} else if len(encCredentials) > 0 {
		credentials, err = decrypt(s.encKey, encCredentials)
	}
	if err != nil {
		s.markFailed(ctx, jobID, attempts, "credential load failed: "+err.Error())
		return
	}

	p, ok := s.providers[providerSlug]
	if !ok {
		s.markFailed(ctx, jobID, attempts, "unknown provider: "+providerSlug)
		return
	}

	if leadID != 0 && (providerSlug == "sunbase" || providerSlug == "ghl") {
		repo := leads.NewRepository(s.pool)
		if refreshed, err := leads.RefreshDeliveryPayload(ctx, s.pool, repo, leadID, payload); err == nil {
			payload = refreshed
		}
	}

	var result *providers.DeliveryResult
	if providerSlug == "webhook" {
		result, err = providers.DeliverWebhook(ctx, credentials, payload)
	} else {
		var dp providers.DeliveryPayload
		_ = json.Unmarshal(payload, &dp)
		var raw map[string]any
		_ = json.Unmarshal(payload, &raw)
		if cfg, ok := raw["_config"].(map[string]any); ok {
			dp.Config = cfg
		}
		for k, v := range connConfig {
			if dp.Config == nil {
				dp.Config = map[string]any{}
			}
			if _, exists := dp.Config[k]; !exists {
				dp.Config[k] = v
			}
		}
		if providerSlug == "sunbase" || providerSlug == "ghl" {
			var accountID int64
			var accountTimezone string
			if err := s.pool.QueryRow(ctx,
				`SELECT ic.account_id, COALESCE(NULLIF(a.timezone, ''), 'America/New_York')
				 FROM integration_connections ic
				 JOIN accounts a ON a.id = ic.account_id
				 WHERE ic.id=$1`, connID).Scan(&accountID, &accountTimezone); err == nil {
				if dp.Config == nil {
					dp.Config = map[string]any{}
				}
				dp.Config["account_timezone"] = accountTimezone
				if types, err := customfields.FieldTypesByAccount(ctx, s.pool, accountID); err == nil && len(types) > 0 {
					dp.Config["custom_field_types"] = types
				}
			}
		}
		result, err = p.Deliver(ctx, credentials, dp)
	}
	duration := int(time.Since(start).Milliseconds())

	reqLog := payload
	httpStatus := 0
	var respRaw []byte
	if result != nil {
		if len(result.Request) > 0 {
			reqLog = result.Request
		}
		httpStatus = result.HTTPStatus
		respRaw = result.Raw
	}

	if err != nil {
		s.logAttempt(ctx, jobID, attempts, "failed", httpStatus, reqLog, respRaw, duration, err.Error())
		if providers.IsDeliverySkipped(err) {
			s.markSkipped(ctx, jobID, err.Error())
			return
		}
		s.markFailed(ctx, jobID, attempts, err.Error())
		return
	}
	extID := ""
	if result != nil {
		extID = result.ExternalID
	}
	s.logAttempt(ctx, jobID, attempts, "success", httpStatus, reqLog, respRaw, duration, "")
	s.markSuccess(ctx, jobID, extID)
	_, _ = s.pool.Exec(ctx, `UPDATE integration_connections SET last_used_at = now(), last_error = NULL WHERE id = $1`, connID)
	if leadID != 0 && len(respRaw) > 0 {
		if triggerID != nil {
			s.applyResponseMap(ctx, *triggerID, leadID, respRaw)
		} else if providerSlug == "sunbase" {
			s.applyConnectionResponseMap(ctx, connID, leadID, respRaw)
		}
	}
	if leadID != 0 && extID != "" {
		repo := leads.NewRepository(s.pool)
		_ = repo.SetExternalID(ctx, s.pool, leadID, extID)
	}
}

func (s *Service) markSuccess(ctx context.Context, jobID int64, externalID string) {
	if _, err := s.pool.Exec(ctx,
		`UPDATE integration_delivery_queue
		 SET status = 'success', external_id = $2, delivered_at = now(), updated_at = now()
		 WHERE id = $1`, jobID, externalID); err != nil {
		log.Printf("integrations worker: markSuccess job %d: %v", jobID, err)
	}
}

func (s *Service) markSkipped(ctx context.Context, jobID int64, errMsg string) {
	if _, err := s.pool.Exec(ctx,
		`UPDATE integration_delivery_queue
		 SET status = 'dead'::delivery_status, attempts = max_attempts, last_error = $2, updated_at = now()
		 WHERE id = $1`, jobID, errMsg); err != nil {
		log.Printf("integrations worker: markSkipped job %d: %v", jobID, err)
	}
}

func (s *Service) markFailed(ctx context.Context, jobID int64, attempts int, errMsg string) {
	backoff := []time.Duration{30 * time.Second, 2 * time.Minute, 8 * time.Minute, 30 * time.Minute, 2 * time.Hour}
	next := time.Now().Add(30 * time.Second)
	if attempts-1 < len(backoff) {
		next = time.Now().Add(backoff[attempts-1])
	}
	if _, err := s.pool.Exec(ctx,
		`UPDATE integration_delivery_queue
		 SET status = CASE WHEN attempts >= max_attempts THEN 'dead'::delivery_status ELSE 'pending'::delivery_status END,
		     next_attempt_at = $2, last_error = $3, updated_at = now()
		 WHERE id = $1`, jobID, next, errMsg); err != nil {
		log.Printf("integrations worker: markFailed job %d: %v", jobID, err)
	}
	if _, err := s.pool.Exec(ctx,
		`UPDATE integration_connections SET last_error = $2 WHERE id = (
			SELECT connection_id FROM integration_delivery_queue WHERE id = $1)`, jobID, errMsg); err != nil {
		log.Printf("integrations worker: markFailed connection update job %d: %v", jobID, err)
	}
}

func (s *Service) logAttempt(ctx context.Context, jobID int64, attempt int, status string, httpStatus int, req, resp []byte, duration int, errMsg string) {
	_, _ = s.pool.Exec(ctx,
		`INSERT INTO integration_delivery_logs
		   (queue_item_id, attempt_number, status, http_status, request_body, response_body, duration_ms, error)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''))`,
		jobID, attempt, status, httpStatus, req, truncate(resp, 4096), duration, errMsg)
}
