import { minutesToTimeHhmm, timeHhmmToMinutes } from "@/features/appointments/hooks";

export const SLOT_ROW_GRID =
  "grid grid-cols-[4rem_minmax(7.5rem,9rem)_minmax(7.5rem,9rem)_3rem_4.5rem] items-center gap-2";

export const CONTRACT_SLOT_ROW_GRID =
  "grid grid-cols-[1.25rem_4rem_minmax(6.5rem,8rem)_minmax(6.5rem,8rem)_3rem_3rem] items-center gap-2";

export const SLOT_CHECKBOX_CLASS =
  "h-4 w-4 shrink-0 rounded border-gray-300 bg-surface-card text-jade-600 focus:ring-jade-500/12";

export function slotEndTime(start: string, durationMin: number): string {
  return minutesToTimeHhmm(timeHhmmToMinutes(start) + durationMin);
}
