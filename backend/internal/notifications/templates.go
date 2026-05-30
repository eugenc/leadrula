package notifications

import (
	"fmt"
	"html"
)

type emailParams struct {
	baseURL  string
	greeting string
	body     string
	ctaText  string
	ctaURL   string
	footer   string
}

func renderEmail(p emailParams) string {
	logoURL := p.baseURL + "/leadrula-logo-light.png"
	greeting := html.EscapeString(p.greeting)
	body := html.EscapeString(p.body)
	ctaText := html.EscapeString(p.ctaText)
	ctaURL := html.EscapeString(p.ctaURL)
	footer := html.EscapeString(p.footer)

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="margin:0;padding:0;background-color:#F4F5F7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#F4F5F7;padding:32px 16px;">
<tr><td align="center">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:520px;background-color:#FFFFFF;border-radius:8px;border:1px solid #EAECF0;">
<tr><td style="padding:32px 32px 24px;text-align:center;border-bottom:1px solid #EAECF0;">
<img src="%s" alt="LeadRula" width="160" style="display:inline-block;max-width:160px;height:auto;" />
</td></tr>
<tr><td style="padding:32px;">
<p style="margin:0 0 16px;font-size:16px;line-height:24px;color:#475467;">%s</p>
<p style="margin:0 0 24px;font-size:15px;line-height:22px;color:#475467;">%s</p>
<table role="presentation" cellpadding="0" cellspacing="0"><tr><td style="border-radius:6px;background-color:#1A9268;">
<a href="%s" style="display:inline-block;padding:12px 24px;font-size:15px;font-weight:600;color:#FFFFFF;text-decoration:none;">%s</a>
</td></tr></table>
<p style="margin:24px 0 0;font-size:13px;line-height:20px;color:#667085;">%s</p>
</td></tr>
<tr><td style="padding:20px 32px;border-top:1px solid #EAECF0;text-align:center;">
<p style="margin:0;font-size:12px;line-height:18px;color:#98A2B3;">%s</p>
</td></tr>
</table>
</td></tr>
</table>
</body>
</html>`, logoURL, greeting, body, ctaURL, ctaText, footer, "LeadRula")
}

func inviteEmail(baseURL, fullName, link string) string {
	greeting := "Hi,"
	if fullName != "" {
		greeting = "Hi " + fullName + ","
	}
	return renderEmail(emailParams{
		baseURL:  baseURL,
		greeting: greeting,
		body:     "You have been invited to join LeadRula. Click the button below to accept your invite and create your password.",
		ctaText:  "Accept your invite",
		ctaURL:   link,
		footer:   "This link expires in 72 hours. If you did not expect this email, you can ignore it.",
	})
}

func passwordResetEmail(baseURL, fullName, link string) string {
	greeting := "Hi,"
	if fullName != "" {
		greeting = "Hi " + fullName + ","
	}
	return renderEmail(emailParams{
		baseURL:  baseURL,
		greeting: greeting,
		body:     "We received a request to reset your LeadRula password. Click the button below to choose a new password.",
		ctaText:  "Reset password",
		ctaURL:   link,
		footer:   "This link expires in 2 hours. If you did not request a reset, you can ignore this email.",
	})
}
