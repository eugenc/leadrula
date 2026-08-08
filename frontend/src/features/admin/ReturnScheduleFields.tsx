import { PAYOUT_WEEKDAYS } from "@/features/admin/contractCompensation";
import {
  DEFAULT_RETURN_SCHEDULE,
  type ReturnScheduleDraft,
  toggleWeekday,
} from "@/features/admin/returnSchedule";
import type { ReturnScheduleMode } from "@/types";
import { Input, Select } from "@/components/ui/input";
import { cn } from "@/lib/utils";

type Props = {
  value: ReturnScheduleDraft;
  onChange: (value: ReturnScheduleDraft) => void;
  compact?: boolean;
};

const MODES: { value: ReturnScheduleMode; label: string }[] = [
  { value: "immediate", label: "Immediately" },
  { value: "delay", label: "After delay" },
  { value: "daily", label: "Every day at" },
  { value: "weekly", label: "On selected days at" },
];

const choiceInputClass =
  "h-4 w-4 shrink-0 border-gray-300 bg-surface-card text-jade-600 focus:ring-jade-500/12";
const radioClass = choiceInputClass;
const checkboxClass = cn(choiceInputClass, "rounded");

export function ReturnScheduleFields({ value, onChange, compact = false }: Props) {
  function set<K extends keyof ReturnScheduleDraft>(key: K, next: ReturnScheduleDraft[K]) {
    onChange({ ...value, [key]: next });
  }

  return (
    <div className={compact ? "space-y-2" : "space-y-2.5"}>
      <div className="text-xs font-semibold text-gray-500">Return timing</div>
      <div className="space-y-2">
        {MODES.map((mode) => {
          const hasInlineControls =
            (mode.value === "delay" && value.mode === "delay") ||
            (mode.value === "daily" && value.mode === "daily") ||
            (mode.value === "weekly" && value.mode === "weekly");

          return (
            <label
              key={mode.value}
              className={cn(
                "flex flex-wrap items-center gap-2 text-sm text-gray-700",
                hasInlineControls && "min-h-9"
              )}
            >
              <input
                type="radio"
                className={radioClass}
                name={`return-schedule-${compact ? "compact" : "full"}`}
                checked={value.mode === mode.value}
                onChange={() => onChange({ ...DEFAULT_RETURN_SCHEDULE, ...value, mode: mode.value })}
              />
              <span className="min-w-[110px]">{mode.label}</span>
              {mode.value === "delay" && value.mode === "delay" && (
                <>
                  <Input
                    type="number"
                    min={1}
                    className="w-20"
                    value={value.delayValue}
                    onChange={(e) => set("delayValue", Math.max(1, Number(e.target.value) || 1))}
                  />
                  <Select
                    value={value.delayUnit}
                    onChange={(e) => set("delayUnit", e.target.value as ReturnScheduleDraft["delayUnit"])}
                    className="w-auto min-w-[7.5rem]"
                  >
                    <option value="minutes">minutes</option>
                    <option value="hours">hours</option>
                    <option value="days">days</option>
                  </Select>
                </>
              )}
              {mode.value === "daily" && value.mode === "daily" && (
                <Input
                  type="time"
                  className="w-[9.5rem]"
                  value={value.time}
                  onChange={(e) => set("time", e.target.value)}
                />
              )}
              {mode.value === "weekly" && value.mode === "weekly" && (
                <>
                  <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
                    {PAYOUT_WEEKDAYS.map((d) => (
                      <label key={d.value} className="inline-flex items-center gap-1 text-xs text-gray-600">
                        <input
                          type="checkbox"
                          className={checkboxClass}
                          checked={value.weekdays.includes(d.value)}
                          onChange={() => set("weekdays", toggleWeekday(value.weekdays, d.value))}
                        />
                        {d.label.slice(0, 3)}
                      </label>
                    ))}
                  </div>
                  <Input
                    type="time"
                    className="w-[9.5rem]"
                    value={value.time}
                    onChange={(e) => set("time", e.target.value)}
                  />
                </>
              )}
            </label>
          );
        })}
      </div>
    </div>
  );
}
