package providers

import (
	"encoding/json"
	"fmt"
)

func oauthAccessToken(credentials []byte) (string, error) {
	var creds struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(credentials, &creds); err != nil || creds.AccessToken == "" {
		return "", fmt.Errorf("access_token required")
	}
	return creds.AccessToken, nil
}
