package providers

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
)

func fetchHubSpotPipelines(ctx context.Context, credentials []byte) ([]CRMPipeline, error) {
	token, err := oauthAccessToken(credentials)
	if err != nil {
		return nil, err
	}
	body, err := crmHTTPGet(ctx, hubspotBase+"/crm/v3/pipelines/deals", "Bearer "+token)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Results []struct {
			ID     string `json:"id"`
			Label  string `json:"label"`
			Stages []struct {
				ID       string            `json:"id"`
				Label    string            `json:"label"`
				Metadata map[string]string `json:"metadata"`
			} `json:"stages"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([]CRMPipeline, 0, len(parsed.Results))
	for _, p := range parsed.Results {
		stages := make([]CRMStage, 0, len(p.Stages))
		for i, s := range p.Stages {
			st := CRMStage{
				ExternalID: s.ID,
				Name:       s.Label,
				Position:   i,
			}
			applyHubSpotStageSignals(&st, s.Metadata)
			stages = append(stages, st)
		}
		out = append(out, CRMPipeline{
			ExternalID: p.ID,
			Name:       p.Label,
			Stages:     stages,
		})
	}
	return out, nil
}

func applyHubSpotStageSignals(st *CRMStage, metadata map[string]string) {
	if metadata == nil {
		return
	}
	isClosed := strings.EqualFold(metadata["isClosed"], "true")
	probStr := metadata["probability"]
	prob, _ := strconv.ParseFloat(probStr, 64)
	if isClosed {
		st.IsClosed = true
		if prob >= 1 || strings.Contains(strings.ToLower(st.Name), "won") {
			st.IsWon = true
		} else if prob == 0 || strings.Contains(strings.ToLower(st.Name), "lost") {
			st.IsClosedLost = true
		}
	}
}
