package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func fetchZohoCRMPipelines(ctx context.Context, credentials []byte, config map[string]any) ([]CRMPipeline, error) {
	token, err := oauthAccessToken(credentials)
	if err != nil {
		return nil, err
	}
	domain, _ := config["api_domain"].(string)
	if domain == "" {
		domain = "com"
	}
	base := "https://www.zohoapis." + strings.TrimPrefix(domain, ".")
	authHeader := "Zoho-oauthtoken " + token

	layoutBody, err := crmHTTPGet(ctx, base+"/crm/v6/settings/layouts?module=Deals", authHeader)
	if err != nil {
		return nil, err
	}
	var layouts struct {
		Layouts []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Visible bool   `json:"visible"`
		} `json:"layouts"`
	}
	if err := json.Unmarshal(layoutBody, &layouts); err != nil {
		return nil, err
	}
	if len(layouts.Layouts) == 0 {
		return nil, fmt.Errorf("no deals layouts found in zoho crm")
	}
	layoutID := layouts.Layouts[0].ID
	for _, l := range layouts.Layouts {
		if l.Visible {
			layoutID = l.ID
			break
		}
	}

	pipeBody, err := crmHTTPGet(ctx, base+"/crm/v6/settings/pipeline?layout_id="+layoutID, authHeader)
	if err != nil {
		return nil, err
	}
	var pipelineResp struct {
		Pipeline []struct {
			DisplayValue string `json:"display_value"`
			Default      bool   `json:"default"`
			Maps         []struct {
				ID           string `json:"id"`
				DisplayValue string `json:"display_value"`
				Sequence     int    `json:"sequence_number"`
				Probability  int    `json:"probability"`
				ForecastType string `json:"forecast_type"`
			} `json:"maps"`
		} `json:"pipeline"`
	}
	if err := json.Unmarshal(pipeBody, &pipelineResp); err != nil {
		return nil, err
	}
	out := make([]CRMPipeline, 0, len(pipelineResp.Pipeline))
	for pi, p := range pipelineResp.Pipeline {
		stages := make([]CRMStage, 0, len(p.Maps))
		for i, s := range p.Maps {
			st := CRMStage{
				ExternalID: s.ID,
				Name:       s.DisplayValue,
				Position:   i,
			}
			if s.Probability >= 100 || strings.EqualFold(s.ForecastType, "Closed Won") {
				st.IsWon = true
				st.IsClosed = true
			} else if s.Probability == 0 && (strings.EqualFold(s.ForecastType, "Closed Lost") || strings.Contains(strings.ToLower(s.DisplayValue), "lost")) {
				st.IsClosedLost = true
				st.IsClosed = true
			}
			stages = append(stages, st)
		}
		name := p.DisplayValue
		if name == "" {
			name = fmt.Sprintf("Deals Pipeline %d", pi+1)
		}
		out = append(out, CRMPipeline{
			ExternalID: layoutID + ":" + name,
			Name:       name,
			Stages:     stages,
		})
	}
	return out, nil
}
