package providers

import (
	"strings"
)

// CRMPipelineStageMapEntry is the normalized pipeline/stage map entry for all CRMs.
type CRMPipelineStageMapEntry struct {
	LeadrulaPipelineID int64  `json:"leadrula_pipeline_id"`
	LeadrulaStageID    int64  `json:"leadrula_stage_id"`
	CRMPipelineID      string `json:"crm_pipeline_id,omitempty"`
	CRMStageID         string `json:"crm_stage_id,omitempty"`
	GHLPipelineID      string `json:"ghl_pipeline_id,omitempty"`
	GHLPipelineStageID string `json:"ghl_pipeline_stage_id,omitempty"`
}

type InboundStageSyncConfig struct {
	Enabled            bool
	LeadrulaPipelineID int64
	CRMPipelineID      string
	PipelineStageMap   []CRMPipelineStageMapEntry
}

func entryCRMPipelineID(e CRMPipelineStageMapEntry) string {
	if id := strings.TrimSpace(e.CRMPipelineID); id != "" {
		return id
	}
	return strings.TrimSpace(e.GHLPipelineID)
}

func entryCRMStageID(e CRMPipelineStageMapEntry) string {
	if id := strings.TrimSpace(e.CRMStageID); id != "" {
		return id
	}
	return strings.TrimSpace(e.GHLPipelineStageID)
}

func NormalizePipelineStageMapEntries(entries []GHLPipelineStageMapEntry) []CRMPipelineStageMapEntry {
	out := make([]CRMPipelineStageMapEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, mapEntry(e))
	}
	return out
}

func NormalizePipelineStageMapFromRaw(raw any, deliveryMode string) []CRMPipelineStageMapEntry {
	return NormalizePipelineStageMapEntries(parsePipelineStageMap(raw, deliveryMode))
}

// ParseInboundStageSync reads inbound stage sync settings from a connection config map.
func ParseInboundStageSync(config map[string]any) InboundStageSyncConfig {
	if config == nil {
		return InboundStageSyncConfig{}
	}
	out := InboundStageSyncConfig{
		Enabled:            boolFromAny(config["inbound_stage_sync_enabled"]),
		LeadrulaPipelineID: ghlInt64FromAny(config["inbound_sync_leadrula_pipeline_id"]),
		PipelineStageMap:   NormalizePipelineStageMapFromRaw(config["pipeline_stage_map"], ParseGHLDeliveryModeFromConfig(config)),
	}
	if s, ok := config["inbound_sync_crm_pipeline_id"].(string); ok && strings.TrimSpace(s) != "" {
		out.CRMPipelineID = strings.TrimSpace(s)
	} else if s, ok := config["inbound_sync_ghl_pipeline_id"].(string); ok {
		out.CRMPipelineID = strings.TrimSpace(s)
	}
	return out
}

func boolFromAny(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(x, "true")
	default:
		return false
	}
}

// CRMInboundStageSyncReady reports whether inbound CRM stage auto-sync should run.
func CRMInboundStageSyncReady(cfg InboundStageSyncConfig) bool {
	if !cfg.Enabled {
		return false
	}
	if cfg.LeadrulaPipelineID <= 0 || strings.TrimSpace(cfg.CRMPipelineID) == "" {
		return false
	}
	return hasCompleteCRMInboundStageMap(cfg.PipelineStageMap, cfg.LeadrulaPipelineID)
}

func hasCompleteCRMInboundStageMap(entries []CRMPipelineStageMapEntry, lrPipelineID int64) bool {
	for _, e := range entries {
		if e.LeadrulaPipelineID != lrPipelineID {
			continue
		}
		if e.LeadrulaStageID > 0 && entryCRMPipelineID(e) != "" && entryCRMStageID(e) != "" {
			return true
		}
	}
	return false
}

// ResolveCRMLeadrulaStage maps a CRM pipeline/stage to a Leadrula stage ID.
func ResolveCRMLeadrulaStage(entries []CRMPipelineStageMapEntry, lrPipelineID int64, crmPipelineID, crmStageID string) (stageID int64, ok bool) {
	crmPipelineID = strings.TrimSpace(crmPipelineID)
	crmStageID = strings.TrimSpace(crmStageID)
	if lrPipelineID <= 0 || crmPipelineID == "" || crmStageID == "" {
		return 0, false
	}
	for _, e := range entries {
		if e.LeadrulaPipelineID != lrPipelineID {
			continue
		}
		if entryCRMPipelineID(e) == crmPipelineID && entryCRMStageID(e) == crmStageID {
			return e.LeadrulaStageID, e.LeadrulaStageID > 0
		}
	}
	return 0, false
}

// CRMInboundPipelineStage reads CRM pipeline and stage IDs from a flattened webhook payload.
func CRMInboundPipelineStage(slug string, flat map[string]any) (crmPipelineID, crmStageID string) {
	switch slug {
	case "ghl":
		return GHLInboundPipelineStage(flat)
	case "pipedrive":
		return pipedriveInboundPipelineStage(flat)
	case "hubspot":
		return hubspotInboundPipelineStage(flat)
	case "zoho_crm":
		return zohoInboundPipelineStage(flat)
	default:
		return "", ""
	}
}

// CRMInboundContactID reads the CRM contact/person ID from a flattened webhook payload.
func CRMInboundContactID(slug string, flat map[string]any) string {
	switch slug {
	case "ghl":
		return ghlInboundContactID(flat)
	case "pipedrive":
		return pipedriveInboundContactID(flat)
	case "hubspot":
		return hubspotInboundContactID(flat)
	case "zoho_crm":
		return zohoInboundContactID(flat)
	default:
		return ""
	}
}

func ghlInboundContactID(flat map[string]any) string {
	for _, key := range []string{"contact_id", "contactId", "id"} {
		if v, ok := flat[key]; ok {
			if s := strings.TrimSpace(toGHLText(v)); s != "" && s != "null" {
				return s
			}
		}
	}
	return ""
}

func pipedriveInboundPipelineStage(flat map[string]any) (string, string) {
	if pid := strings.TrimSpace(toGHLText(flat["current.pipeline_id"])); pid != "" {
		return pid, strings.TrimSpace(toGHLText(flat["current.stage_id"]))
	}
	current, _ := flat["current"].(map[string]any)
	if current == nil {
		current = flat
	}
	return strings.TrimSpace(toGHLText(current["pipeline_id"])), strings.TrimSpace(toGHLText(current["stage_id"]))
}

func pipedriveInboundContactID(flat map[string]any) string {
	if id := strings.TrimSpace(toGHLText(flat["current.person_id"])); id != "" {
		return id
	}
	current, _ := flat["current"].(map[string]any)
	if current == nil {
		current = flat
	}
	if id := strings.TrimSpace(toGHLText(current["person_id"])); id != "" {
		return id
	}
	return strings.TrimSpace(toGHLText(current["personId"]))
}

func hubspotInboundPipelineStage(flat map[string]any) (string, string) {
	// HubSpot deal.propertyChange sends propertyName=dealstage; pipeline from association or properties.
	if prop, _ := flat["propertyName"].(string); prop != "" && prop != "dealstage" {
		return "", ""
	}
	stageID := strings.TrimSpace(toGHLText(flat["propertyValue"]))
	if stageID == "" {
		stageID = strings.TrimSpace(toGHLText(flat["dealstage"]))
	}
	pipelineID := strings.TrimSpace(toGHLText(flat["pipeline"]))
	if pipelineID == "" {
		pipelineID = strings.TrimSpace(toGHLText(flat["pipeline_id"]))
	}
	return pipelineID, stageID
}

func hubspotInboundContactID(flat map[string]any) string {
	if id := strings.TrimSpace(toGHLText(flat["objectId"])); id != "" {
		return id
	}
	return strings.TrimSpace(toGHLText(flat["contact_id"]))
}

func zohoInboundPipelineStage(flat map[string]any) (string, string) {
	data, _ := flat["data"].(map[string]any)
	if data == nil {
		data = flat
	}
	pipelineID := strings.TrimSpace(toGHLText(data["Pipeline"]))
	if pipelineID == "" {
		pipelineID = strings.TrimSpace(toGHLText(data["pipeline_id"]))
	}
	stageID := strings.TrimSpace(toGHLText(data["Stage"]))
	if stageID == "" {
		stageID = strings.TrimSpace(toGHLText(data["stage_id"]))
	}
	return pipelineID, stageID
}

func zohoInboundContactID(flat map[string]any) string {
	data, _ := flat["data"].(map[string]any)
	if data == nil {
		data = flat
	}
	if id := strings.TrimSpace(toGHLText(data["Contact_Name"])); id != "" {
		return id
	}
	return strings.TrimSpace(toGHLText(data["contact_id"]))
}

// StageMapCRMStageToLeadrula builds crm_stage_id -> leadrula_stage_id for a pipeline.
func StageMapCRMStageToLeadrula(entries []CRMPipelineStageMapEntry, lrPipelineID int64, crmPipelineID string) map[string]int64 {
	out := map[string]int64{}
	for _, e := range entries {
		if e.LeadrulaPipelineID != lrPipelineID {
			continue
		}
		if crmPipelineID != "" && entryCRMPipelineID(e) != crmPipelineID {
			continue
		}
		sid := entryCRMStageID(e)
		if sid != "" && e.LeadrulaStageID > 0 {
			out[sid] = e.LeadrulaStageID
		}
	}
	return out
}
