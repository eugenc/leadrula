/** Diagnose whether a GHL inbound webhook payload can trigger Leadrula stage sync. */
export type GhlStageSyncDiagnosis = {
  status: "ready" | "warning" | "ok";
  message: string;
};

const PIPELINE_KEYS = ["pipelineId", "pipeline_id"];
const STAGE_KEYS = ["pipelineStageId", "pipeline_stage_id", "stageId"];
const STAGE_NAME_KEYS = ["pipeline_stage", "pipleline_stage", "pippleine_stage", "stage_name"];

function pickString(payload: Record<string, unknown>, keys: string[]): string {
  for (const key of keys) {
    const v = payload[key];
    if (typeof v === "string" && v.trim()) return v.trim();
    if (typeof v === "number") return String(v);
  }
  return "";
}

export function diagnoseGhlInboundStageSyncPayload(payload: unknown): GhlStageSyncDiagnosis {
  if (!payload || typeof payload !== "object") {
    return {
      status: "warning",
      message: "No payload — stage sync requires pipelineId and pipelineStageId from GHL.",
    };
  }
  const flat = payload as Record<string, unknown>;
  const pipelineId = pickString(flat, PIPELINE_KEYS);
  const stageId = pickString(flat, STAGE_KEYS);
  const stageName = pickString(flat, STAGE_NAME_KEYS);
  const contactId = pickString(flat, ["contact_id", "contactId", "id"]);

  const missing: string[] = [];
  if (!contactId) missing.push("contact_id");
  if (!pipelineId) missing.push("pipelineId");
  if (!stageId) missing.push("pipelineStageId");

  if (missing.length > 0) {
    const stageHint =
      !stageId && stageName
        ? ` Found stage name "${stageName}" — sync will match it against stored GHL stage names in your map if pipelineStageId is missing.`
        : "";
    return {
      status: "warning",
      message: `Stage sync will not run — missing ${missing.join(", ")}.${stageHint} Webhook capture can still succeed without moving the lead.`,
    };
  }

  return {
    status: "ready",
    message: `Stage sync fields present (pipeline ${pipelineId}, stage ${stageId}). Lead moves only if these IDs match your stage map.`,
  };
}
