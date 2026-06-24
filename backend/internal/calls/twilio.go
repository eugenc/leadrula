package calls

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// dialLeg is a single buyer destination in a simuldial tier.
type dialLeg struct {
	LegID       int64
	Destination string
}

// twiMLDial builds the <Dial> for a simuldial tier. action drives the waterfall
// continuation; each number reports per-leg status to leg-status.
func (s *Service) twiMLDial(callID int64, legs []dialLeg, callerID string, timeoutSec int, record bool, tier int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><Response>`)
	action := fmt.Sprintf("%s/webhooks/twilio/continue?call=%d&tier=%d", s.webhookBase, callID, tier)
	recAttr := ""
	if record {
		recAttr = fmt.Sprintf(` record="record-from-answer-dual" recordingStatusCallback="%s/webhooks/twilio/recording?call=%d"`, s.webhookBase, callID)
	}
	callerAttr := ""
	if callerID != "" {
		callerAttr = fmt.Sprintf(` callerId="%s"`, xmlEscape(callerID))
	}
	b.WriteString(fmt.Sprintf(`<Dial timeout="%d" action="%s"%s%s>`, timeoutSec, xmlEscape(action), callerAttr, recAttr))
	for _, leg := range legs {
		cb := fmt.Sprintf("%s/webhooks/twilio/leg-status?leg=%d", s.webhookBase, leg.LegID)
		b.WriteString(fmt.Sprintf(`<Number statusCallback="%s" statusCallbackEvent="initiated ringing answered completed">%s</Number>`,
			xmlEscape(cb), xmlEscape(leg.Destination)))
	}
	b.WriteString(`</Dial></Response>`)
	return b.String()
}

// twiMLHold keeps the caller on ringback while no buyer is eligible. Twilio
// repeats <Play loop="0"> indefinitely, so the caller never hears a hangup.
func twiMLHold() string {
	return `<?xml version="1.0" encoding="UTF-8"?><Response>` +
		`<Play loop="0">http://com.twilio.sounds.music.s3.amazonaws.com/MARKOVICHAMP-Borghestral.mp3</Play>` +
		`</Response>`
}

func twiMLHangup() string {
	return `<?xml version="1.0" encoding="UTF-8"?><Response><Hangup/></Response>`
}

func twiMLReject() string {
	return `<?xml version="1.0" encoding="UTF-8"?><Response><Reject reason="busy"/></Response>`
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// validateTwilioSignature verifies X-Twilio-Signature for a form POST.
// signature = base64(HMAC-SHA1(authToken, fullURL + sorted(key+value)...)).
func validateTwilioSignature(authToken, fullURL string, form url.Values, signature string) bool {
	if authToken == "" || signature == "" {
		return false
	}
	keys := make([]string, 0, len(form))
	for k := range form {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(fullURL)
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(form.Get(k))
	}
	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(b.String()))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// fetchRecording streams a Twilio recording using the publisher's account creds.
func fetchRecording(ctx context.Context, accountSID, authToken, recordingURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, recordingURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(accountSID, authToken)
	client := &http.Client{}
	return client.Do(req)
}

// postForm performs an RTB ping (or any form POST) and returns status + body.
func postForm(ctx context.Context, endpoint string, headers map[string]string, body url.Values) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	return resp.StatusCode, data, nil
}
