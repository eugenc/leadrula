package providers

import (
	"context"
	"encoding/json"
)

type GHLProvider struct{}

func (p *GHLProvider) Slug() string { return "ghl" }

func (p *GHLProvider) Deliver(ctx context.Context, credentials []byte, payload DeliveryPayload) (*DeliveryResult, error) {
	cfg, err := ParseGHLConfig(payload.Config)
	if err != nil {
		return nil, err
	}

	if cfg.DeliveryMode == "webhook" {
		if len(cfg.PipelineStageMap) == 0 || !MatchesGHLWebhookTrigger(cfg.PipelineStageMap, payload.PipelineID, payload.StageID) {
			return &DeliveryResult{}, nil
		}
		body := buildGHLWebhookPayload(cfg, payload)
		return ghlDeliverWebhook(ctx, cfg.WebhookURL, body)
	}

	token, err := ParseGHLCredentials(credentials)
	if err != nil {
		return nil, err
	}

	contactID, result, err := ghlUpsertContact(ctx, token, cfg, payload)
	if err != nil {
		return result, err
	}

	if cfg.CreateOpportunity {
		ghlPipelineID, ghlStageID, err := resolveGHLStage(cfg.PipelineStageMap, payload.PipelineID, payload.StageID)
		if err != nil {
			return result, err
		}
		oppResult, err := ghlCreateOpportunity(ctx, token, cfg, contactID, ghlPipelineID, ghlStageID, payload)
		if err != nil {
			if oppResult != nil {
				result.HTTPStatus = oppResult.HTTPStatus
				result.Raw = oppResult.Raw
				result.Request = oppResult.Request
			}
			return result, err
		}
		result.HTTPStatus = oppResult.HTTPStatus
		result.Raw = oppResult.Raw
	}

	if cfg.CreateAppointment {
		apptResult, err := ghlCreateAppointment(ctx, token, cfg, contactID, payload)
		if err != nil {
			if apptResult != nil {
				result.HTTPStatus = apptResult.HTTPStatus
				result.Raw = apptResult.Raw
				result.Request = apptResult.Request
			}
			return result, err
		}
		result.HTTPStatus = apptResult.HTTPStatus
		result.Raw = apptResult.Raw
	}

	result.ExternalID = contactID
	return result, nil
}

func (p *GHLProvider) ValidateCredentials(ctx context.Context, credentials []byte, config map[string]any) error {
	cfg, err := ParseGHLConfig(config)
	if err != nil {
		return err
	}
	if cfg.DeliveryMode == "webhook" {
		return nil
	}
	if _, err := ParseGHLCredentials(credentials); err != nil {
		return err
	}
	return nil
}

func (p *GHLProvider) TestConnection(ctx context.Context, credentials []byte, config map[string]any) error {
	cfg, err := ParseGHLConfigForTest(config)
	if err != nil {
		return err
	}
	if cfg.DeliveryMode == "webhook" {
		return ghlTestWebhook(ctx, cfg)
	}
	token, err := ParseGHLCredentials(credentials)
	if err != nil {
		return err
	}
	return ghlTestConnection(ctx, token, cfg.LocationID)
}

func ValidateGHLConfigJSON(config map[string]any) error {
	_, err := ParseGHLConfig(config)
	return err
}

func MergeGHLConfigDefaults(config map[string]any) map[string]any {
	if config == nil {
		config = map[string]any{}
	}
	if _, ok := config["delivery_mode"]; !ok {
		config["delivery_mode"] = "api"
	}
	loc, _ := config["location_id"].(string)
	defaults := DefaultGHLConnectionConfig(loc)
	for k, v := range defaults {
		if _, ok := config[k]; !ok {
			config[k] = v
		}
	}
	if _, ok := config["create_contact"]; !ok {
		config["create_contact"] = true
	}
	return config
}

func GHLConfigFromJSON(raw json.RawMessage) map[string]any {
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return MergeGHLConfigDefaults(out)
}
