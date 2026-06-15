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
