package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

func crmHTTPGet(ctx context.Context, url, authHeader string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authHeader)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("crm api returned %d: %s", res.StatusCode, trimCRMErrorBody(body))
	}
	return body, nil
}

func trimCRMErrorBody(body []byte) string {
	s := string(body)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
