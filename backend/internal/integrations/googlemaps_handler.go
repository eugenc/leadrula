package integrations

import (
	"net/http"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/integrations/googlemaps"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func (h *Handler) googleMapsStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	connected, err := h.svc.HasGoogleMapsConnection(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"connected": connected})
}

func (h *Handler) googleMapsAutocomplete(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	apiKey, err := h.svc.GoogleMapsAPIKey(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	input := r.URL.Query().Get("input")
	sessionToken := r.URL.Query().Get("session_token")
	suggestions, err := googlemaps.Autocomplete(r.Context(), apiKey, input, sessionToken)
	if err != nil {
		httpx.WriteError(w, httpx.Validation("google places autocomplete failed: "+err.Error()))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"suggestions": suggestions})
}

func (h *Handler) googleMapsPlaceDetails(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	apiKey, err := h.svc.GoogleMapsAPIKey(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var body struct {
		PlaceID string `json:"place_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	details, err := googlemaps.PlaceDetails(r.Context(), apiKey, body.PlaceID)
	if err != nil {
		httpx.WriteError(w, httpx.Validation("google places details failed: "+err.Error()))
		return
	}
	httpx.JSON(w, http.StatusOK, details)
}
