package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDoSunbaseRequest_unableToFindSiteOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Unable to find site [sunbrightsolarusa]"))
	}))
	t.Cleanup(srv.Close)

	res, err := doSunbaseRequest(context.Background(), http.MethodPost, srv.URL, map[string]string{"schema_name": "sunbrightsolarusa"})
	if err == nil {
		t.Fatal("expected error for unable to find site response")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unable to find site") {
		t.Fatalf("error = %q, want unable to find site", err.Error())
	}
	if res == nil || len(res.Request) == 0 {
		t.Fatal("expected request log on error")
	}
	var reqLog DeliveryRequestLog
	if json.Unmarshal(res.Request, &reqLog) != nil || reqLog.Mapped["schema_name"] != "sunbrightsolarusa" {
		t.Fatalf("request log mapped = %v", reqLog.Mapped)
	}
	if res.HTTPStatus != http.StatusOK {
		t.Fatalf("http status = %d", res.HTTPStatus)
	}
}

func TestDoSunbaseRequest_successWithUUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"uuid":"cust-4f70f8df-7639-442c-be5c-d44efa235210"}`))
	}))
	t.Cleanup(srv.Close)

	res, err := doSunbaseRequest(context.Background(), http.MethodPost, srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExternalID != "cust-4f70f8df-7639-442c-be5c-d44efa235210" {
		t.Fatalf("external_id = %q", res.ExternalID)
	}
	if len(res.Request) == 0 {
		t.Fatal("expected request log")
	}
}

func TestDoSunbaseRequest_successWithPlainTextID(t *testing.T) {
	body := "Identified sunbrightsolarusa\nsuccessfully Inserted lead Chayko a27665862c664a6e8d6bd6906ad53aa9 sunbrightsolarusa"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	res, err := doSunbaseRequest(context.Background(), http.MethodPost, srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExternalID != "a27665862c664a6e8d6bd6906ad53aa9" {
		t.Fatalf("external_id = %q", res.ExternalID)
	}
}

func TestNormalizeSunbaseExternalID(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"a27665862c664a6e8d6bd6906ad53aa9", "a27665862c664a6e8d6bd6906ad53aa9"},
		{"cust-4f70f8df-7639-442c-be5c-d44efa235210", "4f70f8df7639442cbe5cd44efa235210"},
		{"4f70f8df-7639-442c-be5c-d44efa235210", "4f70f8df7639442cbe5cd44efa235210"},
		{"", ""},
		{"not-an-id", "not-an-id"},
	}
	for _, tc := range tests {
		if got := NormalizeSunbaseExternalID(tc.in); got != tc.want {
			t.Fatalf("NormalizeSunbaseExternalID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSunbaseExternalIDCandidates(t *testing.T) {
	cust := "cust-4f70f8df-7639-442c-be5c-d44efa235210"
	hex := "4f70f8df7639442cbe5cd44efa235210"
	candidates := SunbaseExternalIDCandidates(cust)
	if len(candidates) < 2 {
		t.Fatalf("expected multiple candidates, got %v", candidates)
	}
	found := map[string]bool{}
	for _, c := range candidates {
		found[c] = true
	}
	if !found[cust] || !found[hex] {
		t.Fatalf("candidates = %v, want cust and hex forms", candidates)
	}
}

func TestParseSunbaseExternalID(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "plain text success",
			body: "Identified sunbrightsolarusa\nsuccessfully Inserted lead Chayko a27665862c664a6e8d6bd6906ad53aa9 sunbrightsolarusa",
			want: "a27665862c664a6e8d6bd6906ad53aa9",
		},
		{
			name: "json cust uuid",
			body: `{"uuid":"cust-4f70f8df-7639-442c-be5c-d44efa235210"}`,
			want: "cust-4f70f8df-7639-442c-be5c-d44efa235210",
		},
		{
			name: "json hex id",
			body: `{"id":"a27665862c664a6e8d6bd6906ad53aa9"}`,
			want: "a27665862c664a6e8d6bd6906ad53aa9",
		},
		{
			name: "error body",
			body: "Unable to find site [sunbrightsolarusa]",
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseSunbaseExternalID([]byte(tc.body))
			if got != tc.want {
				t.Fatalf("parseSunbaseExternalID() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildDeliveryRequestLog_queryParams(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/post?first_name=Eugene&dob=1988%2F22%2F12", nil)
	if err != nil {
		t.Fatal(err)
	}
	mapped := map[string]string{"first_name": "Eugene", "dob": "1988/22/12"}
	raw := BuildDeliveryRequestLog(mapped, req, nil)
	var log DeliveryRequestLog
	if err := json.Unmarshal(raw, &log); err != nil {
		t.Fatal(err)
	}
	if log.Mapped["dob"] != "1988/22/12" {
		t.Fatalf("mapped dob = %q", log.Mapped["dob"])
	}
	if log.HTTP.Method != http.MethodPost {
		t.Fatalf("method = %q", log.HTTP.Method)
	}
	if !strings.Contains(log.HTTP.URL, "first_name=Eugene") {
		t.Fatalf("url = %q", log.HTTP.URL)
	}
}
