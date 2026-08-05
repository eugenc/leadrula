/** Diagnose whether a GHL inbound webhook payload can trigger Leadrula stage sync. */
export type GhlStageSyncDiagnosis = {
  status: "ready" | "warning" | "ok";
  message: string;
};

const PIPELINE_KEYS = [
  "pipelineId",
  "pipeline_id",
  "opportunity.pipelineId",
  "opportunity.pipeline_id",
];
const STAGE_KEYS = [
  "pipelineStageId",
  "pipeline_stage_id",
  "stageId",
  "opportunity.pipelineStageId",
  "opportunity.pipeline_stage_id",
];
const STAGE_NAME_KEYS = [
  "pipeline_stage",
  "pipleline_stage",
  "pippleine_stage",
  "stage_name",
  "stageName",
  "opportunity.pipeline_stage_name",
  "opportunity.stageName",
];
const CONTACT_KEYS = ["contact_id", "contactId", "contact.id", "contact.contactId", "contact.contact_id", "id"];

function pickString(payload: Record<string, unknown>, keys: string[]): string {
  for (const key of keys) {
    const v = payload[key];
    if (typeof v === "string" && v.trim()) return v.trim();
    if (typeof v === "number") return String(v);
  }
  return "";
}

function normalizeGhlPayload(payload: Record<string, unknown>): Record<string, unknown> {
  const out = { ...payload };
  const copyIfEmpty = (dest: string, sources: string[]) => {
    if (pickString(out, [dest])) return;
    for (const src of sources) {
      const val = pickString(out, [src]);
      if (val) {
        out[dest] = val;
        return;
      }
    }
  };
  copyIfEmpty("contactId", ["contact_id", "contact.id", "contact.contactId", "contact.contact_id"]);
  copyIfEmpty("pipelineId", ["pipeline_id", "opportunity.pipelineId", "opportunity.pipeline_id"]);
  copyIfEmpty("pipeline_id", ["pipelineId", "opportunity.pipeline_id", "opportunity.pipelineId"]);
  copyIfEmpty("pipelineStageId", [
    "pipeline_stage_id",
    "stageId",
    "opportunity.pipelineStageId",
    "opportunity.pipeline_stage_id",
  ]);
  copyIfEmpty("pipleline_stage", [
    "pipeline_stage",
    "pippleine_stage",
    "stage_name",
    "stageName",
    "opportunity.pipeline_stage_name",
    "opportunity.stageName",
  ]);
  return out;
}

export function diagnoseGhlInboundStageSyncPayload(payload: unknown): GhlStageSyncDiagnosis {
  if (!payload || typeof payload !== "object") {
    return {
      status: "warning",
      message:
        "No payload — GHL default webhook should include contactId, pipeline_id, and pipleline_stage.",
    };
  }
  const flat = normalizeGhlPayload(payload as Record<string, unknown>);
  const pipelineId = pickString(flat, PIPELINE_KEYS);
  const stageId = pickString(flat, STAGE_KEYS);
  const stageName = pickString(flat, STAGE_NAME_KEYS);
  const contactId = pickString(flat, CONTACT_KEYS);

  const missing: string[] = [];
  if (!contactId) missing.push("contactId");
  if (!pipelineId) missing.push("pipeline_id");
  if (!stageId && !stageName) missing.push("pipleline_stage");

  if (missing.length === 0) {
    if (stageId) {
      return {
        status: "ready",
        message: `GHL default payload ready (pipeline ${pipelineId}, stage id ${stageId}). Lead moves if the stage map matches.`,
      };
    }
    return {
      status: "ready",
      message: `GHL default payload ready — stage name "${stageName}" (pipeline ${pipelineId}). Lead moves if the name matches your stage map.`,
    };
  }

  return {
    status: "warning",
    message: `Stage sync will not run — missing ${missing.join(", ")} from GHL default payload. Webhook capture can still succeed without moving the lead.`,
  };
}
