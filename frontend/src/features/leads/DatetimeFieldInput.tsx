import { useEffect, useMemo, useState } from "react";
import {
  addMonths,
  eachDayOfInterval,
  endOfMonth,
  endOfWeek,
  format,
  isSameDay,
  isSameMonth,
  isToday,
  parse,
  startOfMonth,
  startOfWeek,
} from "date-fns";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { Dropdown } from "@/components/ui/dropdown";
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
const WEEKDAYS = ["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"] as const;

function chipClass(active: boolean, disabled?: boolean) {
  return cn(
    "rounded-md border px-2 py-1 text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-40",
    active
      ? "border-jade-500 bg-jade-500 text-white"
      : "border-gray-100 bg-surface-card text-gray-700 hover:bg-jade-50 hover:text-jade-700"
  );
}

function partsToDate(parts: DatetimeLocalParts): Date {
  return parse(parts.date, "yyyy-MM-dd", new Date());
}

function DatetimePickerPanel({
  value,
  onChange,
  disabled,
  className,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  className?: string;
}) {
  const parts = value ? parseDatetimeLocalParts(value) : defaultDatetimeLocalParts();
  const selectedDate = partsToDate(parts!);
  const [viewMonth, setViewMonth] = useState(() => startOfMonth(selectedDate));

  useEffect(() => {
    setViewMonth(startOfMonth(selectedDate));
  }, [parts!.date]);

  const calendarDays = useMemo(() => {
    const monthStart = startOfMonth(viewMonth);
    const monthEnd = endOfMonth(viewMonth);
    return eachDayOfInterval({
      start: startOfWeek(monthStart, { weekStartsOn: 0 }),
      end: endOfWeek(monthEnd, { weekStartsOn: 0 }),
    });
  }, [viewMonth]);

  function emit(next: DatetimeLocalParts) {
    onChange(buildDatetimeLocal(next));
  }

  function update(patch: Partial<DatetimeLocalParts>) {
    emit({ ...parts!, ...patch });
  }

  function selectDay(day: Date) {
    update({ date: format(day, "yyyy-MM-dd") });
  }

  return (
    <div className={cn("w-[280px]", className)}>
      <div className="mb-3 flex items-center justify-between">
        <button
          type="button"
          disabled={disabled}
          onClick={() => setViewMonth((m) => addMonths(m, -1))}
          className="rounded-md p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 disabled:opacity-40"
          aria-label="Previous month"
        >
          <ChevronLeft className="h-4 w-4" />
        </button>
        <span className="text-sm font-semibold text-gray-800">{format(viewMonth, "MMMM yyyy")}</span>
        <button
          type="button"
          disabled={disabled}
          onClick={() => setViewMonth((m) => addMonths(m, 1))}
          className="rounded-md p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 disabled:opacity-40"
          aria-label="Next month"
        >
          <ChevronRight className="h-4 w-4" />
        </button>
      </div>

      <div className="mb-1 grid grid-cols-7 gap-0.5">
        {WEEKDAYS.map((d) => (
          <div key={d} className="py-1 text-center text-[10px] font-medium uppercase text-gray-400">
            {d}
          </div>
        ))}
      </div>

      <div className="mb-4 grid grid-cols-7 gap-0.5">
        {calendarDays.map((day) => {
          const inMonth = isSameMonth(day, viewMonth);
          const selected = isSameDay(day, selectedDate);
          const today = isToday(day);
          return (
            <button
              key={day.toISOString()}
              type="button"
              disabled={disabled}
              onClick={() => selectDay(day)}
              className={cn(
                "h-8 rounded-md text-xs font-medium transition-colors",
                !inMonth && "text-gray-300",
                inMonth && !selected && "text-gray-700 hover:bg-jade-50 hover:text-jade-700",
                selected && "bg-jade-500 text-white",
                today && !selected && "ring-1 ring-inset ring-jade-300"
              )}
            >
              {format(day, "d")}
            </button>
          );
        })}
      </div>

      <div className="space-y-2 border-t border-gray-100 pt-3">
        <div>
          <p className="mb-1.5 text-[10px] font-medium uppercase tracking-wide text-gray-400">Hour</p>
          <div className="grid grid-cols-6 gap-1">
            {HOURS_12.map((h) => (
              <button
                key={h}
                type="button"
                disabled={disabled}
                onClick={() => update({ hour12: h })}
                className={chipClass(parts!.hour12 === h, disabled)}
              >
                {String(h).padStart(2, "0")}
              </button>
            ))}
          </div>
        </div>

        <div>
          <p className="mb-1.5 text-[10px] font-medium uppercase tracking-wide text-gray-400">Minute</p>
          <div className="grid grid-cols-4 gap-1">
            {QUARTER_MINUTES.map((m) => (
              <button
                key={m}
                type="button"
                disabled={disabled}
                onClick={() => update({ minute: m })}
                className={chipClass(parts!.minute === m, disabled)}
              >
                {String(m).padStart(2, "0")}
              </button>
            ))}
          </div>
        </div>

        <div>
          <p className="mb-1.5 text-[10px] font-medium uppercase tracking-wide text-gray-400">Period</p>
          <div className="grid grid-cols-2 gap-1">
            {(["AM", "PM"] as const).map((p) => (
              <button
                key={p}
                type="button"
                disabled={disabled}
                onClick={() => update({ period: p })}
                className={chipClass(parts!.period === p, disabled)}
              >
                {p}
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

export function DatetimeFieldInput({
  value,
  onChange,
  onBlur,
  disabled,
  placeholder,
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
      <DatetimePickerPanel
        value={value}
        onChange={onChange}
        disabled={disabled}
        className={className}
      />
    );
  }

  const emptyLabel = placeholder ?? (
    <>
      <span className="hidden lg:inline">Click to Set Date & Time</span>
      <span className="lg:hidden">Tap to Set Date & Time</span>
    </>
  );

  return (
    <Dropdown
      open={open}
      onClose={() => {
        setOpen(false);
        onBlur?.();
      }}
      align="left"
      className="p-3"
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
          {value ? formatDatetimeForDisplay(value) : emptyLabel}
        </button>
      }
    >
      <DatetimePickerPanel value={value} onChange={onChange} disabled={disabled} />
    </Dropdown>
  );
}
