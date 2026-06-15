import type { StageType } from "@/types";

export const STAGE_TYPES: { value: StageType; label: string; description: string }[] = [
  { value: "standard", label: "Standard", description: "No prompt on move" },
  { value: "action", label: "Action", description: "Require action date & time" },
  { value: "disqualification", label: "Disqualification", description: "Require disqualification reason" },
  { value: "won", label: "Won", description: "Sets lead status to closed on entry" },
];

export function stageNeedsPrompt(type: StageType): boolean {
  return type === "action" || type === "disqualification";
}

/** True when the API rejected a stage move because prompt fields were not sent. */
export function stagePromptMissingError(code: string, message: string, type: StageType): boolean {
  if (code !== "business_rule" || !stageNeedsPrompt(type)) return false;
  const msg = message.toLowerCase();
  if (type === "action") return msg.includes("action date");
  return msg.includes("disqualification reason");
}

export function stageTypeLabel(type: StageType): string {
  return STAGE_TYPES.find((t) => t.value === type)?.label ?? type;
}
