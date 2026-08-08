import type { ReturnRule, ReturnScheduleMode } from "@/types";

export type ReturnScheduleDraft = {
  mode: ReturnScheduleMode;
  delayValue: number;
  delayUnit: "minutes" | "hours" | "days";
  time: string;
  weekdays: number[];
};

export const DEFAULT_RETURN_SCHEDULE: ReturnScheduleDraft = {
  mode: "immediate",
  delayValue: 1,
  delayUnit: "hours",
  time: "09:00",
  weekdays: [1, 3, 5],
};

export function scheduleFromRule(rule: ReturnRule): ReturnScheduleDraft {
  const mode = rule.return_schedule_mode ?? "immediate";
  return {
    mode,
    delayValue: rule.return_delay_value ?? (rule.return_delay_seconds ? Math.max(1, Math.round(rule.return_delay_seconds / 3600)) : 1),
    delayUnit: rule.return_delay_unit ?? "hours",
    time: rule.return_time ?? "09:00",
    weekdays: rule.return_weekdays?.length ? [...rule.return_weekdays] : [1, 3, 5],
  };
}

export function scheduleToBody(draft: ReturnScheduleDraft): Record<string, unknown> {
  const body: Record<string, unknown> = { return_schedule_mode: draft.mode };
  if (draft.mode === "delay") {
    body.return_delay_value = draft.delayValue;
    body.return_delay_unit = draft.delayUnit;
  }
  if (draft.mode === "daily" || draft.mode === "weekly") {
    body.return_time = draft.time;
  }
  if (draft.mode === "weekly") {
    body.return_weekdays = draft.weekdays;
  }
  return body;
}

export function toggleWeekday(weekdays: number[], day: number): number[] {
  return weekdays.includes(day) ? weekdays.filter((d) => d !== day) : [...weekdays, day].sort((a, b) => a - b);
}
