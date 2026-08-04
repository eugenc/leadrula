package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

func fetchPipedrivePipelines(ctx context.Context, credentials []byte) ([]CRMPipeline, error) {
	token, err := oauthAccessToken(credentials)
	if err != nil {
		return nil, err
	}
	body, err := crmHTTPGet(ctx, pipedriveBase+"/pipelines?include_stages=1", "Bearer "+token)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Success bool `json:"success"`
		Data    []struct {
			ID     int    `json:"id"`
			Name   string `json:"name"`
			Stages []struct {
				ID              int     `json:"id"`
				Name            string  `json:"name"`
				OrderNr         int     `json:"order_nr"`
				DealProbability int     `json:"deal_probability"`
				RottenFlag      bool    `json:"rotten_flag"`
				ActiveFlag      bool    `json:"active_flag"`
			} `json:"stages"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if !parsed.Success {
		return nil, fmt.Errorf("pipedrive pipelines request failed")
	}
	out := make([]CRMPipeline, 0, len(parsed.Data))
	for _, p := range parsed.Data {
		stages := make([]CRMStage, 0, len(p.Stages))
		for i, s := range p.Stages {
			st := CRMStage{
				ExternalID: strconv.Itoa(s.ID),
				Name:       s.Name,
				Position:   i,
			}
			if s.DealProbability >= 100 {
				st.IsWon = true
				st.IsClosed = true
			} else if s.DealProbability == 0 && !s.ActiveFlag {
				st.IsClosedLost = true
				st.IsClosed = true
			}
			stages = append(stages, st)
		}
		out = append(out, CRMPipeline{
			ExternalID: strconv.Itoa(p.ID),
			Name:       p.Name,
			Stages:     stages,
		})
	}
	return out, nil
}
