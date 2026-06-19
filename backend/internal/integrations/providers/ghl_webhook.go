package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func parseGHLDeliveryMode(config map[string]any) string {
	return ParseGHLDeliveryModeFromConfig(config)
}

func ParseGHLDeliveryModeFromConfig(config map[string]any) string {
	if config == nil {
		return "api"
	}
	if s, ok := config["delivery_mode"].(string); ok {
		if strings.EqualFold(strings.TrimSpace(s), "webhook") {
			return "webhook"
		}
	}
	return "api"
}

func ghlWebhookURLValid(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func lookupGHLStage(mapEntries []GHLPipelineStageMapEntry, pipelineID, stageID int64) (string, string) {
	for _, e := range mapEntries {
		if e.LeadrulaPipelineID == pipelineID && e.LeadrulaStageID == stageID {
			return e.GHLPipelineID, e.GHLPipelineStageID
		}
	}
	return "", ""
}

func buildGHLWebhookPayload(cfg GHLConfig, payload DeliveryPayload) map[string]any {
	body := buildGHLContactBody(cfg, payload)
	delete(body, "locationId")
	delete(body, "tags")
	if payload.PipelineID != 0 {
		body["leadrula_pipeline_id"] = payload.PipelineID
	}
	if payload.StageID != 0 {
		body["leadrula_stage_id"] = payload.StageID
	}
	if pid, sid := lookupGHLStage(cfg.PipelineStageMap, payload.PipelineID, payload.StageID); pid != "" {
		body["ghl_pipeline_id"] = pid
		body["ghl_pipeline_stage_id"] = sid
	}
	appendLeadID(body, payload)
	return body
}

func ghlDeliverWebhook(ctx context.Context, webhookURL string, body map[string]any) (*DeliveryResult, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	mapped := AnyMapToMapped(body)
	result := &DeliveryResult{
		Request: marshalRequestLog(mapped),
	}
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	result.HTTPStatus = resp.StatusCode
	result.Raw = raw
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		if msg == "" {
			return result, fmt.Errorf("ghl webhook returned %d", resp.StatusCode)
		}
		return result, fmt.Errorf("ghl webhook returned %d: %s", resp.StatusCode, msg)
	}
	return result, nil
}

func ghlWebhookTestPayload() DeliveryPayload {
	return DeliveryPayload{
		LeadID:    "00000000-0000-0000-0000-000000000001",
		FirstName: "Leadrula",
		LastName:  "Test",
		Phone:     "+15555550100",
		Email:     "test@leadrula.example",
		Address:   "123 Test St",
		City:      "Miami",
		State:     "FL",
		Zip:       "33101",
		Source:    "leadrula_test",
	}
}

func ghlTestWebhook(ctx context.Context, cfg GHLConfig) error {
	body := buildGHLWebhookPayload(cfg, ghlWebhookTestPayload())
	_, err := ghlDeliverWebhook(ctx, cfg.WebhookURL, body)
	return err
}
