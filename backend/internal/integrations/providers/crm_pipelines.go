package providers

import (
	"context"
	"fmt"
	"strings"
)

// CRMPipeline is a normalized pipeline fetched from an external CRM.
type CRMPipeline struct {
	ExternalID string     `json:"external_id"`
	Name       string     `json:"name"`
	Stages     []CRMStage `json:"stages"`
}

// CRMStage is a normalized stage within a CRM pipeline.
type CRMStage struct {
	ExternalID   string `json:"external_id"`
	Name         string `json:"name"`
	Position     int    `json:"position"`
	IsWon        bool   `json:"is_won,omitempty"`
	IsClosedLost bool   `json:"is_closed_lost,omitempty"`
	IsClosed     bool   `json:"is_closed,omitempty"`
}

var crmPipelineProviders = map[string]bool{
	"ghl":       true,
	"pipedrive": true,
	"hubspot":   true,
	"zoho_crm":  true,
}

// CRMPipelineImportSupported reports whether a provider slug supports pipeline import.
func CRMPipelineImportSupported(slug string) bool {
	return crmPipelineProviders[slug]
}

// FetchCRMPipelines loads pipelines from a connected CRM provider.
func FetchCRMPipelines(ctx context.Context, slug string, credentials []byte, config map[string]any) ([]CRMPipeline, error) {
	switch slug {
	case "ghl":
		locationID, _ := config["location_id"].(string)
		if locationID == "" {
			return nil, fmt.Errorf("location_id is required")
		}
		token, err := ParseGHLCredentials(credentials)
		if err != nil {
			return nil, err
		}
		pipelines, err := FetchGHLPipelines(ctx, token, locationID)
		if err != nil {
			return nil, err
		}
		return ghlPipelinesToCRM(pipelines), nil
	case "pipedrive":
		return fetchPipedrivePipelines(ctx, credentials)
	case "hubspot":
		return fetchHubSpotPipelines(ctx, credentials)
	case "zoho_crm":
		return fetchZohoCRMPipelines(ctx, credentials, config)
	case "salesforce", "sunbase":
		return nil, fmt.Errorf("%s does not support pipeline import", slug)
	default:
		return nil, fmt.Errorf("pipeline import not supported for %s", slug)
	}
}

func ghlPipelinesToCRM(pipelines []GHLPipeline) []CRMPipeline {
	out := make([]CRMPipeline, 0, len(pipelines))
	for _, p := range pipelines {
		stages := make([]CRMStage, 0, len(p.Stages))
		for i, s := range p.Stages {
			st := CRMStage{
				ExternalID: s.ID,
				Name:       s.Name,
				Position:   i,
			}
			if i == len(p.Stages)-1 {
				applyWonNameHeuristic(&st)
			}
			stages = append(stages, st)
		}
		out = append(out, CRMPipeline{
			ExternalID: p.ID,
			Name:       p.Name,
			Stages:     stages,
		})
	}
	return out
}

func applyWonNameHeuristic(st *CRMStage) {
	n := strings.ToLower(strings.TrimSpace(st.Name))
	switch n {
	case "won", "closed won", "sold", "closed-won", "deal won":
		st.IsWon = true
	}
}

// InferCRMStageType maps CRM stage signals to a Leadrula stage_type value.
func InferCRMStageType(stage CRMStage, isLast bool) string {
	if stage.IsWon {
		return "won"
	}
	if stage.IsClosedLost || (stage.IsClosed && !stage.IsWon) {
		return "disqualification"
	}
	if isLast {
		applyWonNameHeuristic(&stage)
		if stage.IsWon {
			return "won"
		}
	}
	return "standard"
}

func ProviderDisplayName(slug string) string {
	switch slug {
	case "ghl":
		return "GHL"
	case "pipedrive":
		return "Pipedrive"
	case "hubspot":
		return "HubSpot"
	case "zoho_crm":
		return "Zoho CRM"
	default:
		return slug
	}
}
