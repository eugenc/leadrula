package providers

import (
	"context"
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

	_, err := doSunbaseRequest(context.Background(), http.MethodPost, srv.URL)
	if err == nil {
		t.Fatal("expected error for unable to find site response")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unable to find site") {
		t.Fatalf("error = %q, want unable to find site", err.Error())
	}
}

func TestDoSunbaseRequest_successWithUUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"uuid":"cust-4f70f8df-7639-442c-be5c-d44efa235210"}`))
	}))
	t.Cleanup(srv.Close)

	res, err := doSunbaseRequest(context.Background(), http.MethodPost, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExternalID != "cust-4f70f8df-7639-442c-be5c-d44efa235210" {
		t.Fatalf("external_id = %q", res.ExternalID)
	}
}

func TestDoSunbaseRequest_successWithPlainTextID(t *testing.T) {
	body := "Identified sunbrightsolarusa\nsuccessfully Inserted lead Chayko a27665862c664a6e8d6bd6906ad53aa9 sunbrightsolarusa"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	res, err := doSunbaseRequest(context.Background(), http.MethodPost, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExternalID != "a27665862c664a6e8d6bd6906ad53aa9" {
		t.Fatalf("external_id = %q", res.ExternalID)
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
