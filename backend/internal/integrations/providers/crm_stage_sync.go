package providers

import (
	"fmt"
	"strings"
)

// InboundStageSyncDiagnosis explains why inbound CRM stage sync did or did not run.
type InboundStageSyncDiagnosis struct {
	CanSync       bool
	SkipReason    string
	TargetStageID int64
	CRMPipelineID string
	CRMStageID    string
}

// DiagnoseCRMInboundStageSync checks whether a flattened webhook payload can move a lead stage.
func DiagnoseCRMInboundStageSync(slug string, flat map[string]any, cfg InboundStageSyncConfig, currentStageID *int64) InboundStageSyncDiagnosis {
	if !cfg.Enabled {
		return InboundStageSyncDiagnosis{SkipReason: "inbound stage sync disabled"}
	}
	if cfg.LeadrulaPipelineID <= 0 || strings.TrimSpace(cfg.CRMPipelineID) == "" {
		return InboundStageSyncDiagnosis{SkipReason: "inbound sync pipeline not configured"}
	}
	if !hasCompleteCRMInboundStageMap(cfg.PipelineStageMap, cfg.LeadrulaPipelineID) {
		return InboundStageSyncDiagnosis{SkipReason: "stage map incomplete for sync pipeline"}
	}
	crmPipelineID, crmStageID := CRMInboundPipelineStage(slug, flat)
	if crmPipelineID == "" || crmStageID == "" {
		return InboundStageSyncDiagnosis{SkipReason: "payload missing pipelineId or pipelineStageId"}
	}
	if crmPipelineID != cfg.CRMPipelineID {
		return InboundStageSyncDiagnosis{
			SkipReason:    fmt.Sprintf("pipeline %s does not match configured sync pipeline", crmPipelineID),
			CRMPipelineID: crmPipelineID,
			CRMStageID:    crmStageID,
		}
	}
	targetStageID, ok := ResolveCRMLeadrulaStage(cfg.PipelineStageMap, cfg.LeadrulaPipelineID, crmPipelineID, crmStageID)
	if !ok {
		return InboundStageSyncDiagnosis{
			SkipReason:    fmt.Sprintf("no Leadrula stage mapped for CRM stage %s", crmStageID),
			CRMPipelineID: crmPipelineID,
			CRMStageID:    crmStageID,
		}
	}
	if currentStageID != nil && *currentStageID == targetStageID {
		return InboundStageSyncDiagnosis{
			SkipReason:    "lead already at target stage",
			TargetStageID: targetStageID,
			CRMPipelineID: crmPipelineID,
			CRMStageID:    crmStageID,
		}
	}
	return InboundStageSyncDiagnosis{
		CanSync:       true,
		TargetStageID: targetStageID,
		CRMPipelineID: crmPipelineID,
		CRMStageID:    crmStageID,
	}
}

// CRMPipelineStageMapEntry is the normalized pipeline/stage map entry for all CRMs.
type CRMPipelineStageMapEntry struct {
	LeadrulaPipelineID int64  `json:"leadrula_pipeline_id"`
	LeadrulaStageID    int64  `json:"leadrula_stage_id"`
	CRMPipelineID      string `json:"crm_pipeline_id,omitempty"`
	CRMStageID         string `json:"crm_stage_id,omitempty"`
	CRMStageName       string `json:"crm_stage_name,omitempty"`
	GHLPipelineID      string `json:"ghl_pipeline_id,omitempty"`
	GHLPipelineStageID string `json:"ghl_pipeline_stage_id,omitempty"`
	GHLStageName       string `json:"ghl_stage_name,omitempty"`
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

func entryCRMStageName(e CRMPipelineStageMapEntry) string {
	if name := strings.TrimSpace(e.CRMStageName); name != "" {
		return name
	}
	return strings.TrimSpace(e.GHLStageName)
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

// HasCRMStageMapEntry reports whether the stage map links a Leadrula stage to a CRM pipeline.
func HasCRMStageMapEntry(entries []CRMPipelineStageMapEntry, lrPipelineID int64, crmPipelineID string, lrStageID int64) bool {
	crmPipelineID = strings.TrimSpace(crmPipelineID)
	if lrPipelineID <= 0 || crmPipelineID == "" || lrStageID <= 0 {
		return false
	}
	for _, e := range entries {
		if e.LeadrulaPipelineID != lrPipelineID || e.LeadrulaStageID != lrStageID {
			continue
		}
		if entryCRMPipelineID(e) == crmPipelineID && entryCRMStageID(e) != "" {
			return true
		}
	}
	return false
}

// ResolveCRMLeadrulaStageByGHLStageName maps a stored GHL stage name to a Leadrula stage ID.
func ResolveCRMLeadrulaStageByGHLStageName(entries []CRMPipelineStageMapEntry, lrPipelineID int64, crmPipelineID, stageName string) (stageID int64, ok bool) {
	crmPipelineID = strings.TrimSpace(crmPipelineID)
	stageName = strings.TrimSpace(stageName)
	if lrPipelineID <= 0 || crmPipelineID == "" || stageName == "" {
		return 0, false
	}
	want := strings.ToLower(stageName)
	for _, e := range entries {
		if e.LeadrulaPipelineID != lrPipelineID {
			continue
		}
		if entryCRMPipelineID(e) != crmPipelineID {
			continue
		}
		if strings.ToLower(strings.TrimSpace(entryCRMStageName(e))) != want {
			continue
		}
		if e.LeadrulaStageID > 0 {
			return e.LeadrulaStageID, true
		}
	}
	return 0, false
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
