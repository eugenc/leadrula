import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Dropdown } from "@/components/ui/dropdown";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { cn } from "@/lib/utils";
import { QUARTER_MINUTES } from "@/features/leads/customFieldDate";
import {
  buildTimeHhmm,
  formatTimeHhmm12,
  parseTimeHhmm,
  timeHhmmToMinutes,
  type TimeHhmmParts,
} from "@/features/appointments/hooks";

const HOURS_12 = Array.from({ length: 12 }, (_, i) => i + 1);
const DEFAULT_PREVIEW = "09:00";

const base =
  "w-full rounded-md border border-gray-200 bg-surface-card text-md text-gray-800 outline-none transition-[border-color,box-shadow] placeholder:text-gray-300 hover:border-gray-300 focus:border-jade-500 focus:ring-[3px] focus:ring-jade-500/12 disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-400";

function chipClass(active: boolean, disabled?: boolean) {
  return cn(
    "rounded-md border px-2 py-1 text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-40",
    active
      ? "border-jade-500 bg-jade-500 text-white"
      : "border-gray-100 bg-surface-card text-gray-700 hover:bg-jade-50 hover:text-jade-700"
  );
}

function isTimeInRange(time: string, minTime?: string, maxTime?: string): boolean {
  const t = timeHhmmToMinutes(time);
  if (minTime !== undefined && t <= timeHhmmToMinutes(minTime)) return false;
  if (maxTime !== undefined && t > timeHhmmToMinutes(maxTime)) return false;
  return true;
}

function partsValid(parts: TimeHhmmParts, minTime?: string, maxTime?: string): boolean {
  return isTimeInRange(buildTimeHhmm(parts), minTime, maxTime);
}

function TimePickerPanel({
  value,
  onChange,
  onClose,
  disabled,
  minTime,
  maxTime,
  allowClear,
}: {
  value: string;
  onChange: (value: string) => void;
  onClose?: () => void;
  disabled?: boolean;
  minTime?: string;
  maxTime?: string;
  allowClear?: boolean;
}) {
  const displayValue = value || DEFAULT_PREVIEW;
  const parts = parseTimeHhmm(displayValue);

  function pick(next: string) {
    onChange(next);
  }

  function update(patch: Partial<TimeHhmmParts>) {
    const next = { ...parts, ...patch };
    if (partsValid(next, minTime, maxTime)) {
      pick(buildTimeHhmm(next));
      return;
    }
    if (patch.hour12 !== undefined || patch.period !== undefined) {
      for (const m of QUARTER_MINUTES) {
        for (const p of ["AM", "PM"] as const) {
          const candidate = { ...parts, ...patch, minute: m, period: p };
          if (partsValid(candidate, minTime, maxTime)) {
            pick(buildTimeHhmm(candidate));
            return;
          }
        }
      }
    }
  }

  function hourDisabled(h: number): boolean {
    return !QUARTER_MINUTES.some((m) =>
      (["AM", "PM"] as const).some((p) => partsValid({ hour12: h, minute: m, period: p }, minTime, maxTime))
    );
  }

  function minuteDisabled(m: number): boolean {
    return !(["AM", "PM"] as const).some((p) => partsValid({ ...parts, minute: m, period: p }, minTime, maxTime));
  }

  function periodDisabled(p: "AM" | "PM"): boolean {
    return !partsValid({ ...parts, period: p }, minTime, maxTime);
  }

  return (
    <div className="w-[260px] space-y-2 p-1">
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

      {allowClear && (
        <Button
          type="button"
          variant="secondary"
          className="h-8 w-full text-xs"
          disabled={disabled}
          onClick={() => {
            onChange("");
            onClose?.();
          }}
        >
          Closed
        </Button>
      )}
    </div>
  );
}

export function TimeFieldInput({
  value,
  onChange,
  onBlur,
  placeholder = "—",
  disabled,
  minTime,
  maxTime,
  allowClear,
  className,
}: {
  value: string;
  onChange: (value: string) => void;
  onBlur?: () => void;
  placeholder?: string;
  disabled?: boolean;
  minTime?: string;
  maxTime?: string;
  allowClear?: boolean;
  className?: string;
}) {
  const [open, setOpen] = useState(false);

  function close() {
    setOpen(false);
    onBlur?.();
  }

  return (
    <div className="min-w-0 w-full">
      <Dropdown
        open={open}
        onClose={close}
        align="left"
        className="p-0"
        trigger={
          <button
            type="button"
            disabled={disabled}
            onClick={() => !disabled && setOpen((o) => !o)}
            className={cn(base, "h-9 w-full truncate px-3 text-left text-sm", !value && "text-gray-400", className)}
          >
            {value ? formatTimeHhmm12(value) : placeholder}
          </button>
        }
      >
        <TimePickerPanel
          value={value}
          onChange={onChange}
          onClose={close}
          disabled={disabled}
          minTime={minTime}
          maxTime={maxTime}
          allowClear={allowClear}
        />
      </Dropdown>
    </div>
  );
}
