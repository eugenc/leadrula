package integrations

import (
	"net/http"

	"github.com/echayko/leadrula/backend/internal/apikeys"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

// RegisterPublicRoutes mounts API-key VoiceUni ingest under /api/v1.
func (h *Handler) RegisterPublicRoutes(r chi.Router, apikeysSvc *apikeys.Service) {
	r.With(apikeysSvc.RequireLeadsWrite).Post("/api/v1/integrations/voiceuni/ingest", h.publicVoiceUniIngest)
}

func (h *Handler) publicVoiceUniIngest(w http.ResponseWriter, r *http.Request) {
	acct := auth.APIKeyAccountFromContext(r.Context())
	if acct == nil || acct.AccountType != "publisher" {
		httpx.Err(w, http.StatusForbidden, httpx.CodeForbidden, "publisher API key required")
		return
	}
	var raw map[string]any
	if !httpx.DecodeJSON(w, r, &raw) {
		return
	}
	connectionID := ""
	if v, ok := raw["connection_id"].(string); ok {
		connectionID = v
	}
	externalID := ""
	if v, ok := raw["external_id"].(string); ok {
		externalID = v
	}
	res, err := h.svc.IngestVoiceUni(r.Context(), acct.AccountID, VoiceUniIngestParams{
		ConnectionPublicID: connectionID,
		ExternalID:         externalID,
		Raw:                raw,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	status := http.StatusOK
	if res.Created {
		status = http.StatusCreated
	}
	httpx.JSON(w, status, map[string]any{"data": res})
}
