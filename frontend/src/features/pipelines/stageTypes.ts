import type { StageType } from "@/types";
import {
  buildDatetimeLocal,
  defaultDatetimeLocalParts,
  isoToDatetimeLocal,
} from "@/features/leads/customFieldDate";

export const STAGE_TYPES: { value: StageType; label: string; description: string }[] = [
  { value: "standard", label: "Standard", description: "No prompt on move" },
  { value: "action", label: "Action", description: "Require Action Date & Time" },
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

/** Action Date & Time applies only on Action pipeline stages. */
export function showActionAtForStage(stageType: StageType | undefined): boolean {
  return stageType === "action";
}

/**
 * Initial datetime-local value for the stage-move prompt. When moving from one
 * action stage to another, prefill the lead's existing action time, or the
 * current time (snapped) when it has none. Other moves start empty.
 */
export function initialActionAtForStageMove(
  fromStageType: StageType | undefined,
  toStageType: StageType | undefined,
  leadActionAt: string | null | undefined
): string {
  if (fromStageType !== "action" || toStageType !== "action") return "";
  if (leadActionAt) return isoToDatetimeLocal(leadActionAt);
  return buildDatetimeLocal(defaultDatetimeLocalParts());
}
