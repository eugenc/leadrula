// Package notifications handles in-app notifications and transactional email.
package notifications

import (
	"fmt"
	"log"
	"net/smtp"
)

// EmailSender sends transactional email over SMTP (Mailgun in production).
// If SMTP is not configured, it logs the message instead (dev mode).
type EmailSender struct {
	host    string
	port    string
	user    string
	pass    string
	from    string
	baseURL string
}

func NewEmailSender(host, port, user, pass, from, baseURL string) *EmailSender {
	return &EmailSender{host: host, port: port, user: user, pass: pass, from: from, baseURL: baseURL}
}

func (e *EmailSender) send(to, subject, body string) error {
	if e.host == "" {
		log.Printf("[email:dev] to=%s subject=%q\n%s", to, subject, body)
		return nil
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		e.from, to, subject, body)
	addr := e.host + ":" + e.port
	authClient := smtp.PlainAuth("", e.user, e.pass, e.host)
	if err := smtp.SendMail(addr, authClient, e.from, []string{to}, []byte(msg)); err != nil {
		log.Printf("email send failed to %s: %v", to, err)
		return err
	}
	return nil
}

func (e *EmailSender) SendInvite(to, token string) error {
	link := fmt.Sprintf("%s/invite/accept?token=%s", e.baseURL, token)
	body := fmt.Sprintf(`<p>You have been invited to the Lead Distribution CRM.</p>
<p><a href="%s">Accept your invite</a></p>`, link)
	return e.send(to, "You're invited to LeadRula", body)
}

func (e *EmailSender) SendPasswordReset(to, token string) error {
	link := fmt.Sprintf("%s/reset?token=%s", e.baseURL, token)
	body := fmt.Sprintf(`<p>Reset your password using the link below:</p>
<p><a href="%s">Reset password</a></p>`, link)
	return e.send(to, "Reset your LeadRula password", body)
}
