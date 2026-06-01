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

export function stageTypeLabel(type: StageType): string {
  return STAGE_TYPES.find((t) => t.value === type)?.label ?? type;
}
