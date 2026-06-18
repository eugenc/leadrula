import { useEffect, useState } from "react";
import { Dropdown } from "@/components/ui/dropdown";
import { Input, Select } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import {
  buildDatetimeLocal,
  defaultDatetimeLocalParts,
  formatDatetimeForDisplay,
  parseDatetimeLocalParts,
  QUARTER_MINUTES,
  snapDatetimeLocalToQuarter,
  type DatetimeLocalParts,
} from "./customFieldDate";

const HOURS_12 = Array.from({ length: 12 }, (_, i) => i + 1);

function DatetimeControls({
  value,
  onChange,
  onBlur,
  disabled,
  className,
}: {
  value: string;
  onChange: (value: string) => void;
  onBlur?: () => void;
  disabled?: boolean;
  className?: string;
}) {
  const parts = value ? parseDatetimeLocalParts(value) : null;

  function emit(next: DatetimeLocalParts) {
    onChange(buildDatetimeLocal(next));
  }

  function update(patch: Partial<DatetimeLocalParts>) {
    const base = parts ?? defaultDatetimeLocalParts();
    emit({ ...base, ...patch });
  }

  return (
    <div className={cn("flex flex-wrap items-center gap-2", className)}>
      <Input
        type="date"
        value={parts?.date ?? ""}
        onChange={(e) => update({ date: e.target.value })}
        onBlur={onBlur}
        disabled={disabled}
        className="min-w-[140px] flex-1"
      />
      <Select
        value={parts ? String(parts.hour12) : ""}
        onChange={(e) => update({ hour12: Number(e.target.value) })}
        onBlur={onBlur}
        disabled={disabled}
        className="w-16 shrink-0"
        aria-label="Hour"
      >
        {!parts && <option value="">Hr</option>}
        {HOURS_12.map((h) => (
          <option key={h} value={h}>
            {String(h).padStart(2, "0")}
          </option>
        ))}
      </Select>
      <Select
        value={parts ? String(parts.minute) : ""}
        onChange={(e) => update({ minute: Number(e.target.value) })}
        onBlur={onBlur}
        disabled={disabled}
        className="w-16 shrink-0"
        aria-label="Minute"
      >
        {!parts && <option value="">Min</option>}
        {QUARTER_MINUTES.map((m) => (
          <option key={m} value={m}>
            {String(m).padStart(2, "0")}
          </option>
        ))}
      </Select>
      <Select
        value={parts?.period ?? ""}
        onChange={(e) => update({ period: e.target.value as "AM" | "PM" })}
        onBlur={onBlur}
        disabled={disabled}
        className="w-[72px] shrink-0"
        aria-label="AM or PM"
      >
        {!parts && <option value="">—</option>}
        <option value="AM">AM</option>
        <option value="PM">PM</option>
      </Select>
    </div>
  );
}

export function DatetimeFieldInput({
  value,
  onChange,
  onBlur,
  disabled,
  placeholder = "Set date & time",
  className,
  layout = "compact",
}: {
  value: string;
  onChange: (value: string) => void;
  onBlur?: () => void;
  disabled?: boolean;
  placeholder?: string;
  className?: string;
  layout?: "compact" | "inline";
}) {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!value) return;
    const snapped = snapDatetimeLocalToQuarter(value);
    if (snapped !== value) onChange(snapped);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- snap only when value changes
  }, [value]);

  if (layout === "inline") {
    return (
      <DatetimeControls
        value={value}
        onChange={onChange}
        onBlur={onBlur}
        disabled={disabled}
        className={className}
      />
    );
  }

  const display = value ? formatDatetimeForDisplay(value) : placeholder;

  return (
    <Dropdown
      open={open}
      onClose={() => {
        setOpen(false);
        onBlur?.();
      }}
      align="left"
      className="min-w-[320px] p-2"
      trigger={
        <button
          type="button"
          onClick={() => !disabled && setOpen((o) => !o)}
          disabled={disabled}
          className={cn(
            "min-w-0 flex-1 truncate text-left text-xs",
            !value && "text-gray-400",
            className
          )}
        >
          {display}
        </button>
      }
    >
      <DatetimeControls
        value={value}
        onChange={onChange}
        disabled={disabled}
      />
    </Dropdown>
  );
}
