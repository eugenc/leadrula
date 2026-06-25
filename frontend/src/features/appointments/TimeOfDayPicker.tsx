import { cn } from "@/lib/utils";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { QUARTER_MINUTES } from "@/features/leads/customFieldDate";
import {
  buildTimeHhmm,
  firstValidStartInWindow,
  isStartValidForWindow,
  parseTimeHhmm,
  type TimeHhmmParts,
} from "@/features/appointments/hooks";

const HOURS_12 = Array.from({ length: 12 }, (_, i) => i + 1);

function chipClass(active: boolean, disabled?: boolean) {
  return cn(
    "rounded-md border px-2 py-1 text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-40",
    active
      ? "border-jade-500 bg-jade-500 text-white"
      : "border-gray-100 bg-surface-card text-gray-700 hover:bg-jade-50 hover:text-jade-700"
  );
}

function isPartsValid(
  parts: TimeHhmmParts,
  dayStart: string,
  dayEnd: string,
  durationMin: number
): boolean {
  return isStartValidForWindow(buildTimeHhmm(parts), durationMin, dayStart, dayEnd);
}

export function TimeOfDayPicker({
  value,
  onChange,
  dayStart,
  dayEnd,
  durationMin,
  disabled,
}: {
  value: string;
  onChange: (value: string) => void;
  dayStart: string;
  dayEnd: string;
  durationMin: number;
  disabled?: boolean;
}) {
  const parts = parseTimeHhmm(value);
  const firstValid = firstValidStartInWindow(dayStart, dayEnd, durationMin);

  if (!firstValid) {
    return (
      <p className="text-sm text-gray-500">
        No slot fits in working hours — shorten duration or widen hours.
      </p>
    );
  }

  function update(patch: Partial<TimeHhmmParts>) {
    const next = { ...parts, ...patch };
    if (isPartsValid(next, dayStart, dayEnd, durationMin)) {
      onChange(buildTimeHhmm(next));
      return;
    }
    if (patch.hour12 !== undefined || patch.period !== undefined) {
      for (const m of QUARTER_MINUTES) {
        for (const p of ["AM", "PM"] as const) {
          const candidate = { ...parts, ...patch, minute: m, period: p };
          if (isPartsValid(candidate, dayStart, dayEnd, durationMin)) {
            onChange(buildTimeHhmm(candidate));
            return;
          }
        }
      }
    }
  }

  function hourDisabled(h: number): boolean {
    return !QUARTER_MINUTES.some((m) =>
      (["AM", "PM"] as const).some((p) => isPartsValid({ hour12: h, minute: m, period: p }, dayStart, dayEnd, durationMin))
    );
  }

  function minuteDisabled(m: number): boolean {
    return !(["AM", "PM"] as const).some((p) =>
      isPartsValid({ ...parts, minute: m, period: p }, dayStart, dayEnd, durationMin)
    );
  }

  function periodDisabled(p: "AM" | "PM"): boolean {
    return !isPartsValid({ ...parts, period: p }, dayStart, dayEnd, durationMin);
  }

  return (
    <div className="space-y-2 rounded-md border border-gray-100 p-3">
      <div>
        <SectionLabel className="mb-1.5">Hour</SectionLabel>
        <div className="grid grid-cols-6 gap-1">
          {HOURS_12.map((h) => (
            <button
              key={h}
              type="button"
              disabled={disabled || hourDisabled(h)}
              onClick={() => update({ hour12: h })}
              className={chipClass(parts.hour12 === h, disabled || hourDisabled(h))}
            >
              {String(h).padStart(2, "0")}
            </button>
          ))}
        </div>
      </div>

      <div>
        <SectionLabel className="mb-1.5">Minute</SectionLabel>
        <div className="grid grid-cols-4 gap-1">
          {QUARTER_MINUTES.map((m) => (
            <button
              key={m}
              type="button"
              disabled={disabled || minuteDisabled(m)}
              onClick={() => update({ minute: m })}
              className={chipClass(parts.minute === m, disabled || minuteDisabled(m))}
            >
              {String(m).padStart(2, "0")}
            </button>
          ))}
        </div>
      </div>

      <div>
        <SectionLabel className="mb-1.5">Period</SectionLabel>
        <div className="grid grid-cols-2 gap-1">
          {(["AM", "PM"] as const).map((p) => (
            <button
              key={p}
              type="button"
              disabled={disabled || periodDisabled(p)}
              onClick={() => update({ period: p })}
              className={chipClass(parts.period === p, disabled || periodDisabled(p))}
            >
              {p}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
