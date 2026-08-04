package integrations

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

type OAuthConfig struct {
	RedirectBase       string
	PipedriveClientID  string
	PipedriveSecret    string
	HubSpotClientID    string
	HubSpotSecret      string
	ZohoClientID       string
	ZohoSecret         string
	SalesforceClientID string
	SalesforceSecret   string
}

type oauthProviderMeta struct {
	authURL      string
	tokenURL     string
	scopes       []string
	clientID     func(OAuthConfig) string
	clientSecret func(OAuthConfig) string
	extraAuth    func(connConfig map[string]any) url.Values
}

var oauthProviders = map[string]oauthProviderMeta{
	"pipedrive": {
		authURL:      "https://oauth.pipedrive.com/oauth/authorize",
		tokenURL:     "https://oauth.pipedrive.com/oauth/token",
		scopes:       []string{"base"},
		clientID:     func(c OAuthConfig) string { return c.PipedriveClientID },
		clientSecret: func(c OAuthConfig) string { return c.PipedriveSecret },
	},
	"hubspot": {
		authURL:      "https://app.hubspot.com/oauth/authorize",
		tokenURL:     "https://api.hubapi.com/oauth/v1/token",
		scopes:       []string{"crm.objects.contacts.write", "crm.objects.contacts.read"},
		clientID:     func(c OAuthConfig) string { return c.HubSpotClientID },
		clientSecret: func(c OAuthConfig) string { return c.HubSpotSecret },
	},
	"zoho_crm": {
		authURL:      "https://accounts.zoho.com/oauth/v2/auth",
		tokenURL:     "https://accounts.zoho.com/oauth/v2/token",
		scopes:       []string{"ZohoCRM.modules.leads.CREATE", "ZohoCRM.modules.leads.READ"},
		clientID:     func(c OAuthConfig) string { return c.ZohoClientID },
		clientSecret: func(c OAuthConfig) string { return c.ZohoSecret },
		extraAuth: func(cfg map[string]any) url.Values {
			return url.Values{"access_type": {"offline"}, "prompt": {"consent"}}
		},
	},
	"salesforce": {
		authURL:      "https://login.salesforce.com/services/oauth2/authorize",
		tokenURL:     "https://login.salesforce.com/services/oauth2/token",
		scopes:       []string{"api", "refresh_token"},
		clientID:     func(c OAuthConfig) string { return c.SalesforceClientID },
		clientSecret: func(c OAuthConfig) string { return c.SalesforceSecret },
	},
}

type pkceBundle struct {
	Verifier  string `json:"v"`
	ConnID    int64  `json:"c"`
	Namespace string `json:"ns"`
}

var (
	pkceMu    sync.Mutex
	pkceCache = map[string]pkceBundle{}
)

func zohoAccountsHost(domain string) string {
	if domain == "" {
		domain = "com"
	}
	return "https://accounts.zoho." + strings.TrimPrefix(domain, ".")
}

func (s *Service) OAuthStartURL(ctx context.Context, accountID int64, providerSlug, connectionName string, connConfig map[string]any, namespace string) (string, int64, error) {
	meta, ok := oauthProviders[providerSlug]
	if !ok {
		return "", 0, httpx.Validation("oauth not supported for provider: " + providerSlug)
	}
	clientID := meta.clientID(s.oauth)
	if clientID == "" {
		return "", 0, httpx.BusinessRule("oauth client not configured for " + providerSlug)
	}
	var providerID int64
	if err := s.pool.QueryRow(ctx,
		`SELECT id FROM integration_providers WHERE slug = $1`, providerSlug).Scan(&providerID); err != nil {
		return "", 0, httpx.NotFound("provider not found")
	}
	verifier, challenge, state, err := pkceMaterial()
	if err != nil {
		return "", 0, err
	}
	configJSON, _ := json.Marshal(connConfig)
	var connID int64
	err = s.pool.QueryRow(ctx,
		`INSERT INTO integration_connections (account_id, provider_id, name, config, status, oauth_state)
		 VALUES ($1, $2, $3, $4, 'pending_oauth', $5)
		 ON CONFLICT (account_id, provider_id, name) DO UPDATE
		   SET oauth_state = EXCLUDED.oauth_state, config = EXCLUDED.config, status = 'pending_oauth', updated_at = now()
		 RETURNING id`,
		accountID, providerID, connectionName, configJSON, state).Scan(&connID)
	if err != nil {
		return "", 0, err
	}
	s.storePKCE(state, verifier, connID, namespace)

	authURL := meta.authURL
	if providerSlug == "zoho_crm" {
		domain, _ := connConfig["api_domain"].(string)
		authURL = zohoAccountsHost(domain) + "/oauth/v2/auth"
	}
	redirectURI := s.oauthCallbackURL(namespace, providerSlug)
	q := url.Values{
		"client_id":             {clientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"scope":                 {strings.Join(meta.scopes, " ")},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	if meta.extraAuth != nil {
		for k, v := range meta.extraAuth(connConfig) {
			q[k] = v
		}
	}
	return authURL + "?" + q.Encode(), connID, nil
}

func (s *Service) OAuthCallback(ctx context.Context, namespace, providerSlug, code, state string) (int64, error) {
	connID, verifier, ns, err := s.loadPKCE(state)
	if err != nil {
		return 0, err
	}
	if ns != "" {
		namespace = ns
	}
	meta, ok := oauthProviders[providerSlug]
	if !ok {
		return 0, httpx.Validation("unknown oauth provider")
	}
	var configJSON []byte
	if err := s.pool.QueryRow(ctx,
		`SELECT config FROM integration_connections WHERE id = $1 AND oauth_state = $2`,
		connID, state).Scan(&configJSON); err != nil {
		return 0, httpx.NotFound("connection not found")
	}
	var connConfig map[string]any
	_ = json.Unmarshal(configJSON, &connConfig)

	tokenURL := meta.tokenURL
	if providerSlug == "zoho_crm" {
		domain, _ := connConfig["api_domain"].(string)
		tokenURL = zohoAccountsHost(domain) + "/oauth/v2/token"
	}
	redirectURI := s.oauthCallbackURL(namespace, providerSlug)
	body := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {meta.clientID(s.oauth)},
		"client_secret": {meta.clientSecret(s.oauth)},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("token exchange failed: %s", string(raw))
	}
	var tok map[string]any
	if err := json.Unmarshal(raw, &tok); err != nil {
		return 0, err
	}
	access, _ := tok["access_token"].(string)
	if access == "" {
		return 0, fmt.Errorf("no access_token in response")
	}
	refresh, _ := tok["refresh_token"].(string)
	expiresIn, _ := tok["expires_in"].(float64)
	var expiresAt *time.Time
	if expiresIn > 0 {
		t := time.Now().Add(time.Duration(expiresIn) * time.Second)
		expiresAt = &t
	}
	if providerSlug == "salesforce" {
		if iu, ok := tok["instance_url"].(string); ok {
			connConfig["instance_url"] = iu
			configJSON, _ = json.Marshal(connConfig)
		}
	}
	encAccess, err := encrypt(s.encKey, []byte(access))
	if err != nil {
		return 0, err
	}
	var encRefresh []byte
	if refresh != "" {
		encRefresh, err = encrypt(s.encKey, []byte(refresh))
		if err != nil {
			return 0, err
		}
	}
	encRaw, _ := encrypt(s.encKey, raw)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx,
		`UPDATE integration_connections SET status = 'active', oauth_state = NULL, config = $2, updated_at = now() WHERE id = $1`,
		connID, configJSON)
	if err != nil {
		return 0, err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO integration_oauth_tokens (connection_id, access_token, refresh_token, expires_at, raw)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (connection_id) DO UPDATE
		   SET access_token = EXCLUDED.access_token, refresh_token = EXCLUDED.refresh_token,
		       expires_at = EXCLUDED.expires_at, raw = EXCLUDED.raw, updated_at = now()`,
		connID, encAccess, encRefresh, expiresAt, encRaw)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return connID, nil
}

func (s *Service) refreshOAuthToken(ctx context.Context, connectionID int64, providerSlug string, connConfig map[string]any) ([]byte, error) {
	meta, ok := oauthProviders[providerSlug]
	if !ok {
		return nil, fmt.Errorf("no oauth for %s", providerSlug)
	}
	var encAccess, encRefresh []byte
	var expiresAt *time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT access_token, refresh_token, expires_at FROM integration_oauth_tokens WHERE connection_id = $1`,
		connectionID).Scan(&encAccess, &encRefresh, &expiresAt)
	if err != nil {
		return nil, err
	}
	access, err := decrypt(s.encKey, encAccess)
	if err != nil {
		return nil, err
	}
	if expiresAt != nil && time.Now().Before(expiresAt.Add(-30*time.Second)) {
		return s.oauthCredsJSON(string(access), connConfig)
	}
	refresh, err := decrypt(s.encKey, encRefresh)
	if err != nil || len(refresh) == 0 {
		return s.oauthCredsJSON(string(access), connConfig)
	}
	tokenURL := meta.tokenURL
	if providerSlug == "zoho_crm" {
		domain, _ := connConfig["api_domain"].(string)
		tokenURL = zohoAccountsHost(domain) + "/oauth/v2/token"
	}
	body := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {string(refresh)},
		"client_id":     {meta.clientID(s.oauth)},
		"client_secret": {meta.clientSecret(s.oauth)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("refresh failed: %s", string(raw))
	}
	var tok map[string]any
	_ = json.Unmarshal(raw, &tok)
	newAccess, _ := tok["access_token"].(string)
	if newAccess == "" {
		return nil, fmt.Errorf("refresh missing access_token")
	}
	expiresIn, _ := tok["expires_in"].(float64)
	var newExpires *time.Time
	if expiresIn > 0 {
		t := time.Now().Add(time.Duration(expiresIn) * time.Second)
		newExpires = &t
	}
	encNew, _ := encrypt(s.encKey, []byte(newAccess))
	_, _ = s.pool.Exec(ctx,
		`UPDATE integration_oauth_tokens SET access_token = $2, expires_at = $3, updated_at = now() WHERE connection_id = $1`,
		connectionID, encNew, newExpires)
	return s.oauthCredsJSON(newAccess, connConfig)
}

func (s *Service) oauthCredsJSON(access string, connConfig map[string]any) ([]byte, error) {
	creds := map[string]any{"access_token": access}
	if iu, ok := connConfig["instance_url"].(string); ok {
		creds["instance_url"] = iu
	}
	if d, ok := connConfig["api_domain"].(string); ok {
		creds["api_domain"] = d
	}
	return json.Marshal(creds)
}

func (s *Service) oauthCallbackURL(namespace, provider string) string {
	base := strings.TrimRight(s.oauth.RedirectBase, "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	return base + "/" + namespace + "/integrations/oauth/" + provider + "/callback"
}

func pkceMaterial() (verifier, challenge, state string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	verifier = base64URLEncode(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64URLEncode(h[:])
	sb := make([]byte, 16)
	_, _ = rand.Read(sb)
	state = base64URLEncode(sb)
	return
}

func base64URLEncode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func (s *Service) storePKCE(state, verifier string, connID int64, namespace string) {
	pkceMu.Lock()
	pkceCache[state] = pkceBundle{Verifier: verifier, ConnID: connID, Namespace: namespace}
	pkceMu.Unlock()
}

func (s *Service) loadPKCE(state string) (connID int64, verifier, namespace string, err error) {
	pkceMu.Lock()
	b, ok := pkceCache[state]
	delete(pkceCache, state)
	pkceMu.Unlock()
	if !ok {
		return 0, "", "", httpx.NotFound("invalid oauth state")
	}
	return b.ConnID, b.Verifier, b.Namespace, nil
}
