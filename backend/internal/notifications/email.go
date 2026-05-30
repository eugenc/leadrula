// Package notifications handles in-app notifications and transactional email.
package notifications

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
)

// EmailSender sends transactional email via the Mailgun Messages API.
// If MAILGUN_API_KEY is not configured, it logs the message instead (dev mode).
type EmailSender struct {
	apiKey  string
	domain  string
	from    string
	apiBase string
	baseURL string
	client  *http.Client
}

func NewEmailSender(apiKey, domain, from, apiBase, baseURL string) *EmailSender {
	return &EmailSender{
		apiKey:  apiKey,
		domain:  domain,
		from:    from,
		apiBase: strings.TrimRight(apiBase, "/"),
		baseURL: baseURL,
		client:  http.DefaultClient,
	}
}

func (e *EmailSender) send(to, subject, body string) error {
	if e.apiKey == "" {
		log.Printf("[email:dev] to=%s subject=%q\n%s", to, subject, body)
		return nil
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, field := range []struct{ name, value string }{
		{"from", e.from},
		{"to", to},
		{"subject", subject},
		{"html", body},
	} {
		if err := w.WriteField(field.name, field.value); err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
		return err
	}

	url := fmt.Sprintf("%s/v3/%s/messages", e.apiBase, e.domain)
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Basic "+basicAuth("api", e.apiKey))

	resp, err := e.client.Do(req)
	if err != nil {
		log.Printf("email send failed to %s: %v", to, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("email send failed to %s: status=%d body=%s", to, resp.StatusCode, strings.TrimSpace(string(respBody)))
		return fmt.Errorf("mailgun returned status %d", resp.StatusCode)
	}
	return nil
}

func basicAuth(user, pass string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
}

func (e *EmailSender) SendInvite(to, fullName, token string) error {
	link := fmt.Sprintf("%s/invite/accept?token=%s", e.baseURL, token)
	body := inviteEmail(e.baseURL, fullName, link)
	return e.send(to, "You're invited to LeadRula", body)
}

func (e *EmailSender) SendPasswordReset(to, fullName, token string) error {
	link := fmt.Sprintf("%s/reset?token=%s", e.baseURL, token)
	body := passwordResetEmail(e.baseURL, fullName, link)
	return e.send(to, "Reset your LeadRula password", body)
}
