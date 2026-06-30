import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/layout/IconButton";
import {
  DrawerBody,
  DrawerFooter,
  DrawerHeader,
  Sheet,
  drawerSubtitleClass,
  formFieldClass,
} from "@/components/ui/dialog";
import { Input, Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { TIMEZONES } from "@/lib/timezones";
import { cn } from "@/lib/utils";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  DEFAULT_WEEKLY_HOURS,
  firstValidStartInWindow,
  isStartValidForWindow,
  minutesToTimeHhmm,
  timeHhmmToMinutes,
  useBookingCalendar,
  useCalendarSlots,
  useCreateBookingCalendar,
  useCreateCalendarSlot,
  usePatchCalendarSlot,
  useSaveBookingCalendar,
  WEEKDAY_KEYS,
  WEEKDAYS,
} from "@/features/appointments/hooks";
import { CopyToWeekdaysPopover } from "@/features/appointments/CopyToWeekdaysPopover";
import { TimeFieldInput } from "@/features/appointments/TimeFieldInput";

type DayHours = { start: string; end: string };
type Schedule = Record<string, DayHours>;

type SlotRow = {
  id: number | string;
  weekday: number;
  start_time: string;
  duration_min: number;
  capacity: number;
};

type SlotDraft = {
  id: string;
  weekday: number;
  start_time: string;
  capacity: number;
};

type SlotParams = {
  start_time: string;
  capacity: number;
};

const FIRST_OPEN_DAY_ORDER = [1, 2, 3, 4, 5, 6, 0];
const SAVE_DEBOUNCE_MS = 500;
const DEFAULT_SLOT_DURATION_MIN = 30;
const MIN_SLOT_DURATION_MIN = 15;
const MAX_SLOT_DURATION_MIN = 180;
const SLOT_ROW_GRID =
  "grid grid-cols-[4rem_minmax(7.5rem,9rem)_minmax(7.5rem,9rem)_3rem_4.5rem] items-center gap-2";
// From+To share slot From+To+Cap width (equal columns); actions column aligns with slots.
const WORKING_HOURS_GRID =
  "grid grid-cols-[4rem_minmax(9.25rem,10.75rem)_minmax(9.25rem,10.75rem)_4.5rem] items-center gap-2";
export const AVAILABILITY_DRAWER_WIDTH = 520;

function scheduleHasInvalidHours(schedule: Schedule): boolean {
  return WEEKDAY_KEYS.some((key) => {
    const w = schedule[key];
    if (!w?.start) return false;
    if (!w.end) return true;
    return w.start >= w.end;
  });
}

function firstOpenWeekday(schedule: Schedule): number | null {
  for (const i of FIRST_OPEN_DAY_ORDER) {
    const w = schedule[WEEKDAY_KEYS[i]];
    if (w?.start && w.end && w.start < w.end) return i;
  }
  return null;
}

function isDayOpen(schedule: Schedule, weekday: number): boolean {
  const w = schedule[WEEKDAY_KEYS[weekday]];
  return !!(w?.start && w.end && w.start < w.end);
}

function openWeekdays(schedule: Schedule): number[] {
  return WEEKDAYS.map((_, i) => i).filter((i) => isDayOpen(schedule, i));
}

function slotEndTime(start: string, durationMin: number): string {
  return minutesToTimeHhmm(timeHhmmToMinutes(start) + durationMin);
}

function defaultAddSlotStart(
  daySlots: SlotRow[],
  dayStart: string,
  dayEnd: string,
  durationMin: number,
  bufferMin: number
): string | null {
  if (!daySlots.length) {
    return firstValidStartInWindow(dayStart, dayEnd, durationMin);
  }
  const sorted = [...daySlots].sort(
    (a, b) => timeHhmmToMinutes(a.start_time) - timeHhmmToMinutes(b.start_time)
  );
  const last = sorted[sorted.length - 1];
  const startMin =
    timeHhmmToMinutes(slotEndTime(last.start_time, durationMin)) + bufferMin;
  const candidate = minutesToTimeHhmm(startMin);
  if (isStartValidForWindow(candidate, durationMin, dayStart, dayEnd)) {
    return candidate;
  }
  return null;
}

function slotInsideWindow(start: string, durationMin: number, dayStart: string, dayEnd: string): boolean {
  const startMin = timeHhmmToMinutes(start);
  const endMin = startMin + durationMin;
  return startMin >= timeHhmmToMinutes(dayStart) && endMin <= timeHhmmToMinutes(dayEnd);
}

function slotsOverlap(aStart: string, aDur: number, bStart: string, bDur: number): boolean {
  const a0 = timeHhmmToMinutes(aStart);
  const a1 = a0 + aDur;
  const b0 = timeHhmmToMinutes(bStart);
  const b1 = b0 + bDur;
  return a0 < b1 && b0 < a1;
}

function draftId(): string {
  return crypto.randomUUID();
}

function schedulesEqual(a: Schedule, b: Schedule): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

function validateSlotChange(
  schedule: Schedule,
  weekday: number,
  start_time: string,
  duration_min: number,
  capacity: number,
  sameDay: { id: number | string; start_time: string; duration_min: number }[],
  excludeId?: number | string
): string | null {
  if (!isDayOpen(schedule, weekday)) {
    return "This day is closed";
  }
  if (duration_min < MIN_SLOT_DURATION_MIN || duration_min > MAX_SLOT_DURATION_MIN) {
    return `Duration must be between ${MIN_SLOT_DURATION_MIN} and ${MAX_SLOT_DURATION_MIN} minutes`;
  }
  if (capacity < 1 || capacity > 20) {
    return "Capacity must be between 1 and 20";
  }
  const day = schedule[WEEKDAY_KEYS[weekday]]!;
  if (!slotInsideWindow(start_time, duration_min, day.start, day.end)) {
    return "Slot must fall within working hours";
  }
  const others = sameDay.filter((s) => s.id !== excludeId);
  if (others.some((s) => slotsOverlap(start_time, duration_min, s.start_time, s.duration_min))) {
    return "Slot overlaps an existing slot on this day";
  }
  return null;
}

function validateAllSlotsDuration(
  schedule: Schedule,
  slots: { id: number | string; weekday: number; start_time: string; capacity: number }[],
  duration_min: number
): string | null {
  const withDur = slots.map((s) => ({ ...s, duration_min }));
  for (const s of withDur) {
    const err = validateSlotChange(
      schedule,
      s.weekday,
      s.start_time,
      duration_min,
      s.capacity,
      withDur.filter((x) => x.weekday === s.weekday),
      s.id
    );
    if (err) return err;
  }
  return null;
}

function copyDayHours(schedule: Schedule, fromWeekday: number, toWeekdays: number[]): Schedule {
  const source = schedule[WEEKDAY_KEYS[fromWeekday]] ?? { start: "", end: "" };
  const next = { ...schedule };
  for (const i of toWeekdays) {
    next[WEEKDAY_KEYS[i]] = { ...source };
  }
  return next;
}

function WorkingHoursGrid({
  schedule,
  onChange,
  onBlur,
  readOnly = false,
}: {
  schedule: Schedule;
  onChange: (schedule: Schedule) => void;
  onBlur?: () => void;
  readOnly?: boolean;
}) {
  return (
    <div className="space-y-1">
      <div className={cn(WORKING_HOURS_GRID, "px-0")}>
        <span />
        <span className="text-xs font-medium text-gray-400">From</span>
        <span className="text-xs font-medium text-gray-400">To</span>
        <span />
      </div>
      {WEEKDAY_KEYS.map((key, i) => {
        const w = schedule[key] ?? { start: "", end: "" };
        return (
          <div key={key} className={WORKING_HOURS_GRID}>
            <span className="text-sm font-medium text-gray-700">{WEEKDAYS[i]}</span>
            <div className="min-w-0 w-full">
              <TimeFieldInput
                value={w.start}
                placeholder="Closed"
                allowClear
                disabled={readOnly}
                onBlur={onBlur}
                onChange={(start) =>
                  onChange({
                    ...schedule,
                    [key]: start ? { start, end: w.end } : { start: "", end: "" },
                  })
                }
              />
            </div>
            <div className="min-w-0 w-full">
              <TimeFieldInput
                value={w.end}
                placeholder="—"
                disabled={readOnly || !w.start}
                minTime={w.start}
                onBlur={onBlur}
                onChange={(end) => onChange({ ...schedule, [key]: { ...w, end } })}
              />
            </div>
            <div className="flex items-center gap-0.5">
              <CopyToWeekdaysPopover
                sourceWeekday={i}
                disabled={readOnly}
                onApply={(toWeekdays) => {
                  onChange(copyDayHours(schedule, i, toWeekdays));
                  onBlur?.();
                }}
              />
              {!readOnly && w.start && (
                <IconButton
                  variant="danger"
                  aria-label="Close day"
                  onClick={() => {
                    onChange({ ...schedule, [key]: { start: "", end: "" } });
                    onBlur?.();
                  }}
                >
                  <Trash2 className="h-4 w-4" />
                </IconButton>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

function SlotEditableRow({
  schedule,
  slot,
  globalDurationMin,
  dayLabel,
  copyButton,
  readOnly,
  onRemove,
  onBlurSave,
}: {
  schedule: Schedule;
  slot: SlotRow;
  globalDurationMin: number;
  dayLabel?: string;
  copyButton?: ReactNode;
  readOnly: boolean;
  onRemove: () => void;
  onBlurSave: (params: SlotParams, revert: () => void) => void;
}) {
  const dayHours = schedule[WEEKDAY_KEYS[slot.weekday]]!;
  const maxFrom = minutesToTimeHhmm(timeHhmmToMinutes(dayHours.end) - globalDurationMin);
  const [from, setFrom] = useState(slot.start_time);
  const [capacity, setCapacity] = useState(slot.capacity);

  useEffect(() => {
    setFrom(slot.start_time);
    setCapacity(slot.capacity);
  }, [slot.start_time, slot.capacity]);

  function revert() {
    setFrom(slot.start_time);
    setCapacity(slot.capacity);
  }

  function commit() {
    if (from === slot.start_time && capacity === slot.capacity) return;
    onBlurSave({ start_time: from, capacity }, revert);
  }

  return (
    <div className={SLOT_ROW_GRID}>
      <span className="text-sm font-medium text-gray-700">{dayLabel ?? ""}</span>
      <div className="min-w-0 w-full">
        <TimeFieldInput
          value={from}
          disabled={readOnly}
          minTime={dayHours.start}
          maxTime={maxFrom}
          onBlur={commit}
          onChange={(next) => {
            if (!next) return;
            if (!isStartValidForWindow(next, globalDurationMin, dayHours.start, dayHours.end)) {
              const first = firstValidStartInWindow(dayHours.start, dayHours.end, globalDurationMin);
              if (first) setFrom(first);
              return;
            }
            setFrom(next);
          }}
        />
      </div>
      <div className="min-w-0 w-full">
        <TimeFieldInput
          value={slotEndTime(from, globalDurationMin)}
          disabled
          onChange={() => {}}
        />
      </div>
      <div className="min-w-0 w-full">
        <Input
          type="number"
          min={1}
          max={20}
          disabled={readOnly}
          value={capacity}
          className="px-2 text-center"
          onBlur={commit}
          onChange={(e) => setCapacity(Number(e.target.value))}
        />
      </div>
      {!readOnly && (
        <div className="flex items-center gap-0.5">
          {copyButton}
          <IconButton variant="danger" aria-label="Remove slot" onClick={onRemove}>
            <Trash2 className="h-4 w-4" />
          </IconButton>
        </div>
      )}
    </div>
  );
}

function BookingSlotsPanel({
  schedule,
  slots,
  globalDurationMin,
  onGlobalDurationChange,
  bufferMin,
  onBufferMinChange,
  onBufferBlur,
  readOnly = false,
  onAdd,
  onEdit,
  onRemove,
  onCopy,
  savePending = false,
}: {
  schedule: Schedule;
  slots: SlotRow[];
  globalDurationMin: number;
  onGlobalDurationChange: (value: number) => void;
  bufferMin: number;
  onBufferMinChange: (value: number) => void;
  onBufferBlur?: () => void;
  readOnly?: boolean;
  onAdd: (
    params: { weekday: number; start_time: string; capacity: number },
    onDone: (ok: boolean) => void
  ) => void;
  onEdit: (id: number | string, params: SlotParams, onDone: (ok: boolean) => void) => void;
  onRemove: (id: number | string) => void;
  onCopy: (slot: SlotRow, toWeekdays: number[]) => void;
  savePending?: boolean;
}) {
  const days = openWeekdays(schedule);
  const [durationDraft, setDurationDraft] = useState(String(globalDurationMin));

  useEffect(() => {
    setDurationDraft(String(globalDurationMin));
  }, [globalDurationMin]);

  function addSlot(weekday: number) {
    const dayHours = schedule[WEEKDAY_KEYS[weekday]]!;
    const daySlots = slots.filter((s) => s.weekday === weekday);
    const from = defaultAddSlotStart(
      daySlots,
      dayHours.start,
      dayHours.end,
      globalDurationMin,
      bufferMin
    );
    if (!from) {
      toast.error("No slot fits in working hours");
      return;
    }
    onAdd({ weekday, start_time: from, capacity: 1 }, () => {});
  }

  function handleDurationInput(raw: string) {
    setDurationDraft(raw);
    const next = Number(raw);
    if (
      raw !== "" &&
      !Number.isNaN(next) &&
      next >= MIN_SLOT_DURATION_MIN &&
      next <= MAX_SLOT_DURATION_MIN
    ) {
      onGlobalDurationChange(next);
    }
  }

  function handleDurationBlur() {
    const trimmed = durationDraft.trim();
    if (!trimmed) {
      setDurationDraft(String(globalDurationMin));
      return;
    }
    const next = Number(trimmed);
    if (Number.isNaN(next)) {
      setDurationDraft(String(globalDurationMin));
      return;
    }
    const clamped = Math.min(
      MAX_SLOT_DURATION_MIN,
      Math.max(MIN_SLOT_DURATION_MIN, Math.round(next))
    );
    setDurationDraft(String(clamped));
    if (clamped !== globalDurationMin) {
      onGlobalDurationChange(clamped);
    }
  }

  if (!days.length) {
    return (
      <p className="text-sm text-gray-500">Set working hours above to add booking slots.</p>
    );
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-3 md:grid-cols-2">
        <div>
          <Label>Duration (min)</Label>
          <Input
            type="number"
            min={MIN_SLOT_DURATION_MIN}
            max={MAX_SLOT_DURATION_MIN}
            disabled={readOnly}
            value={durationDraft}
            onChange={(e) => handleDurationInput(e.target.value)}
            onBlur={handleDurationBlur}
          />
        </div>
        <div>
          <Label>Buffer after appointments (min)</Label>
          <Input
            type="number"
            min={0}
            max={60}
            disabled={readOnly}
            value={bufferMin}
            onChange={(e) => onBufferMinChange(Number(e.target.value))}
            onBlur={onBufferBlur}
          />
        </div>
      </div>

      <div className="space-y-1">
        {slots.length > 0 && (
          <div className={cn(SLOT_ROW_GRID, "px-0")}>
            <span />
            <span className="text-xs font-medium text-gray-400">From</span>
            <span className="text-xs font-medium text-gray-400">To</span>
            <span className="text-xs font-medium text-gray-400">Cap</span>
            <span />
          </div>
        )}
        {days.map((weekday) => {
          const daySlots = slots.filter((s) => s.weekday === weekday);

          if (readOnly && !daySlots.length) return null;

          if (!daySlots.length) {
            if (readOnly) return null;
            return (
              <div key={weekday} className={SLOT_ROW_GRID}>
                <span className="text-sm font-medium text-gray-700">{WEEKDAYS[weekday]}</span>
                <span className="col-span-2 text-sm text-gray-400">No slots for this day.</span>
                <span />
                <IconButton
                  aria-label="Add slot"
                  disabled={savePending}
                  onClick={() => addSlot(weekday)}
                >
                  <Plus className="h-4 w-4" />
                </IconButton>
              </div>
            );
          }

          return (
            <div key={weekday} className="space-y-1">
              {daySlots.map((s, i) => {
                const isFirst = i === 0;
                return (
                  <SlotEditableRow
                    key={s.id}
                    schedule={schedule}
                    slot={s}
                    globalDurationMin={globalDurationMin}
                    dayLabel={isFirst ? WEEKDAYS[weekday] : undefined}
                    copyButton={
                      !readOnly ? (
                        <CopyToWeekdaysPopover
                          sourceWeekday={s.weekday}
                          targetWeekdays={openWeekdays(schedule)}
                          disabled={savePending}
                          onApply={(toWeekdays) => onCopy(s, toWeekdays)}
                        />
                      ) : undefined
                    }
                    readOnly={readOnly}
                    onRemove={() => onRemove(s.id)}
                    onBlurSave={(params, revert) => {
                      onEdit(s.id, params, (ok) => {
                        if (!ok) revert();
                      });
                    }}
                  />
                );
              })}
              {!readOnly && (
                <div className={SLOT_ROW_GRID}>
                  <span />
                  <span className="col-span-3" />
                  <IconButton
                    aria-label="Add slot"
                    disabled={savePending}
                    onClick={() => addSlot(weekday)}
                  >
                    <Plus className="h-4 w-4" />
                  </IconButton>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

export function BuyerAvailabilityEditor({
  calendarId,
  readOnly = false,
}: {
  calendarId: number;
  readOnly?: boolean;
}) {
  const { data: calendar } = useBookingCalendar(calendarId);
  const saveAvail = useSaveBookingCalendar(calendarId);
  const { data: slots = [] } = useCalendarSlots(calendarId);
  const createSlot = useCreateCalendarSlot(calendarId);
  const patchSlot = usePatchCalendarSlot(calendarId);

  const [schedule, setSchedule] = useState<Schedule>({});
  const [timezone, setTimezone] = useState("America/New_York");
  const [location, setLocation] = useState("");
  const [bufferMin, setBufferMin] = useState(0);
  const [slotDurationMin, setSlotDurationMin] = useState(DEFAULT_SLOT_DURATION_MIN);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const savedSnapshot = useRef<{ schedule: Schedule; timezone: string; location: string; bufferMin: number } | null>(null);
  const stateRef = useRef({ schedule, timezone, location, bufferMin });
  const slotsInitialized = useRef(false);

  useEffect(() => {
    stateRef.current = { schedule, timezone, location, bufferMin };
  }, [schedule, timezone, location, bufferMin]);

  useEffect(() => {
    if (!calendar) return;
    const sched = (calendar.schedule as Schedule) ?? {};
    setSchedule(sched);
    setTimezone(calendar.timezone);
    setLocation(calendar.location ?? "");
    setBufferMin(calendar.buffer_min);
    savedSnapshot.current = {
      schedule: sched,
      timezone: calendar.timezone,
      location: calendar.location ?? "",
      bufferMin: calendar.buffer_min,
    };
    slotsInitialized.current = false;
  }, [calendar]);

  useEffect(() => {
    if (slotsInitialized.current) return;
    const active = slots.filter((s) => !s.disabled_at);
    if (!active.length) return;
    setSlotDurationMin(active[0].duration_min);
    slotsInitialized.current = true;
  }, [slots]);

  const persistIfDirty = useCallback(() => {
    if (readOnly || !calendar) return;
    const { schedule: sched, timezone: tz, location: loc, bufferMin: buf } = stateRef.current;
    if (scheduleHasInvalidHours(sched)) return;
    const snap = savedSnapshot.current;
    if (
      snap &&
      snap.timezone === tz &&
      snap.location === loc &&
      snap.bufferMin === buf &&
      schedulesEqual(snap.schedule, sched)
    ) {
      return;
    }
    saveAvail.mutate(
      { schedule: sched, timezone: tz, location: loc, buffer_min: buf },
      {
        onSuccess: () => {
          savedSnapshot.current = { schedule: sched, timezone: tz, location: loc, bufferMin: buf };
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }, [readOnly, calendar, saveAvail]);

  const scheduleSave = useCallback(() => {
    if (readOnly || !calendar) return;
    clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(persistIfDirty, SAVE_DEBOUNCE_MS);
  }, [readOnly, calendar, persistIfDirty]);

  const flushSave = useCallback(() => {
    clearTimeout(saveTimer.current);
    saveTimer.current = undefined;
    persistIfDirty();
  }, [persistIfDirty]);

  useEffect(() => () => flushSave(), [flushSave]);

  function handleBlurSave() {
    scheduleSave();
  }

  if (!calendar) return null;

  function addSlot(
    params: {
      weekday: number;
      start_time: string;
      capacity: number;
    },
    onDone: (ok: boolean) => void
  ) {
    const err = validateSlotChange(
      schedule,
      params.weekday,
      params.start_time,
      slotDurationMin,
      params.capacity,
      slots
        .filter((s) => !s.disabled_at && s.weekday === params.weekday)
        .map((s) => ({ id: s.id, start_time: s.start_time, duration_min: slotDurationMin }))
    );
    if (err) {
      toast.error(err);
      onDone(false);
      return;
    }
    createSlot.mutate(
      { weekday: params.weekday, start_time: params.start_time, duration_min: slotDurationMin, capacity: params.capacity },
      {
        onSuccess: () => onDone(true),
        onError: (e) => {
          toast.error(errorMessage(e));
          onDone(false);
        },
      }
    );
  }

  function editSlot(id: number | string, params: SlotParams, onDone: (ok: boolean) => void) {
    const slot = slots.find((s) => s.id === id && !s.disabled_at);
    if (!slot) {
      onDone(false);
      return;
    }
    const err = validateSlotChange(
      schedule,
      slot.weekday,
      params.start_time,
      slotDurationMin,
      params.capacity,
      slots
        .filter((s) => !s.disabled_at && s.weekday === slot.weekday)
        .map((s) => ({ id: s.id, start_time: s.start_time, duration_min: slotDurationMin })),
      id
    );
    if (err) {
      toast.error(err);
      onDone(false);
      return;
    }
    patchSlot.mutate(
      {
        id: id as number,
        body: {
          start_time: params.start_time,
          duration_min: slotDurationMin,
          capacity: params.capacity,
        },
      },
      {
        onSuccess: () => onDone(true),
        onError: (e) => {
          toast.error(errorMessage(e));
          onDone(false);
        },
      }
    );
  }

  function changeGlobalDuration(next: number) {
    if (next < MIN_SLOT_DURATION_MIN || next > MAX_SLOT_DURATION_MIN) return;
    const rows = slots.filter((s) => !s.disabled_at);
    const err = validateAllSlotsDuration(
      schedule,
      rows.map((s) => ({ id: s.id, weekday: s.weekday, start_time: s.start_time, capacity: s.capacity })),
      next
    );
    if (err) {
      toast.error(err);
      return;
    }
    setSlotDurationMin(next);
    for (const s of rows) {
      if (s.duration_min === next) continue;
      patchSlot.mutate({ id: s.id, body: { duration_min: next } });
    }
  }

  async function copySlot(slot: SlotRow, toWeekdays: number[]) {
    const closed = toWeekdays.filter((d) => !isDayOpen(schedule, d));
    if (closed.length) {
      toast.error(`Cannot copy to closed days: ${closed.map((d) => WEEKDAYS[d]).join(", ")}`);
      return;
    }
    let copied = 0;
    for (const target of toWeekdays) {
      if (target === slot.weekday) continue;
      const err = validateSlotChange(
        schedule,
        target,
        slot.start_time,
        slotDurationMin,
        slot.capacity,
        slots
          .filter((s) => !s.disabled_at && s.weekday === target)
          .map((s) => ({ id: s.id, start_time: s.start_time, duration_min: slotDurationMin }))
      );
      if (err) {
        toast.error(`${WEEKDAYS[target]}: ${err}`);
        continue;
      }
      try {
        await createSlot.mutateAsync({
          weekday: target,
          start_time: slot.start_time,
          duration_min: slotDurationMin,
          capacity: slot.capacity,
        });
        copied++;
      } catch (e) {
        toast.error(`${WEEKDAYS[target]}: ${errorMessage(e)}`);
      }
    }
    if (copied) toast.success(copied === 1 ? "Slot copied" : "Slots copied");
  }

  const slotRows: SlotRow[] = slots
    .filter((s) => !s.disabled_at)
    .map((s) => ({
      id: s.id,
      weekday: s.weekday,
      start_time: s.start_time,
      duration_min: s.duration_min,
      capacity: s.capacity,
    }));

  return (
    <div className="w-fit max-w-full space-y-6">
      <div>
        <Label>Calendar timezone</Label>
        <Select
          value={timezone}
          disabled={readOnly}
          onChange={(e) => setTimezone(e.target.value)}
          onBlur={handleBlurSave}
        >
          {TIMEZONES.map((tz) => (
            <option key={tz} value={tz}>
              {tz}
            </option>
          ))}
        </Select>
      </div>

      <div>
        <Label>Location</Label>
        <Input
          value={location}
          disabled={readOnly}
          placeholder="e.g. Orlando showroom"
          onChange={(e) => {
            const v = e.target.value;
            setLocation(v);
            stateRef.current = { ...stateRef.current, location: v };
            scheduleSave();
          }}
          onBlur={handleBlurSave}
        />
        <p className="mt-1 text-xs text-gray-400">Shown to publishers when booking appointments.</p>
      </div>

      <div>
        <SectionLabel>Working hours</SectionLabel>
        <div className="mt-2">
          <WorkingHoursGrid
            schedule={schedule}
            onChange={setSchedule}
            onBlur={handleBlurSave}
            readOnly={readOnly}
          />
        </div>
      </div>

      <div>
        <SectionLabel>Booking slots</SectionLabel>
        <div className="mt-2">
          <BookingSlotsPanel
            schedule={schedule}
            slots={slotRows}
            globalDurationMin={slotDurationMin}
            onGlobalDurationChange={changeGlobalDuration}
            bufferMin={bufferMin}
            onBufferMinChange={setBufferMin}
            onBufferBlur={handleBlurSave}
            readOnly={readOnly}
            onAdd={addSlot}
            onEdit={editSlot}
            onRemove={(id) =>
              patchSlot.mutate(
                { id: id as number, body: { disabled: true } },
                {
                  onError: (e) => toast.error(errorMessage(e)),
                }
              )
            }
            onCopy={copySlot}
            savePending={createSlot.isPending || patchSlot.isPending}
          />
        </div>
      </div>
    </div>
  );
}

function wizardHasData(
  step: number,
  name: string,
  schedule: Schedule,
  slotDrafts: SlotDraft[],
  calendarId: number | null
): boolean {
  if (step > 0 || calendarId) return true;
  if (name.trim()) return true;
  if (slotDrafts.length) return true;
  return !schedulesEqual(schedule, DEFAULT_WEEKLY_HOURS);
}

export function BuyerSetupWizard({
  open,
  onOpenChange,
  onComplete,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onComplete: (calendarId: number) => void;
}) {
  const createCalendar = useCreateBookingCalendar();
  const [calendarId, setCalendarId] = useState<number | null>(null);
  const saveAvail = useSaveBookingCalendar(calendarId ?? 0);
  const createSlot = useCreateCalendarSlot(calendarId ?? 0);
  const [step, setStep] = useState(0);
  const [name, setName] = useState("");
  const [timezone, setTimezone] = useState("America/New_York");
  const [schedule, setSchedule] = useState<Schedule>({ ...DEFAULT_WEEKLY_HOURS });
  const [slotDrafts, setSlotDrafts] = useState<SlotDraft[]>([]);
  const [slotDurationMin, setSlotDurationMin] = useState(DEFAULT_SLOT_DURATION_MIN);
  const [bufferMin, setBufferMin] = useState(0);
  const [finishing, setFinishing] = useState(false);

  function requestClose() {
    if (wizardHasData(step, name, schedule, slotDrafts, calendarId)) {
      if (!window.confirm("Discard calendar setup?")) return;
    }
    onOpenChange(false);
  }

  function goToSlotStep() {
    if (firstOpenWeekday(schedule) === null) {
      toast.error("Set working hours for at least one day");
      return;
    }
    setStep(2);
  }

  function validateDraftAdd(
    weekday: number,
    start_time: string,
    capacity: number,
    drafts: SlotDraft[],
    excludeId?: string
  ): string | null {
    return validateSlotChange(
      schedule,
      weekday,
      start_time,
      slotDurationMin,
      capacity,
      drafts
        .filter((d) => d.weekday === weekday)
        .map((d) => ({ id: d.id, start_time: d.start_time, duration_min: slotDurationMin })),
      excludeId
    );
  }

  function changeWizardDuration(next: number) {
    if (next < MIN_SLOT_DURATION_MIN || next > MAX_SLOT_DURATION_MIN) return;
    const err = validateAllSlotsDuration(
      schedule,
      slotDrafts.map((d) => ({
        id: d.id,
        weekday: d.weekday,
        start_time: d.start_time,
        capacity: d.capacity,
      })),
      next
    );
    if (err) {
      toast.error(err);
      return;
    }
    setSlotDurationMin(next);
  }

  function addDraft(params: {
    weekday: number;
    start_time: string;
    capacity: number;
  }): boolean {
    const err = validateDraftAdd(
      params.weekday,
      params.start_time,
      params.capacity,
      slotDrafts
    );
    if (err) {
      toast.error(err);
      return false;
    }
    setSlotDrafts((prev) => [...prev, { id: draftId(), ...params }]);
    return true;
  }

  function updateDraft(id: number | string, params: SlotParams): boolean {
    const slot = slotDrafts.find((d) => d.id === id);
    if (!slot) return false;
    const err = validateDraftAdd(
      slot.weekday,
      params.start_time,
      params.capacity,
      slotDrafts,
      id as string
    );
    if (err) {
      toast.error(err);
      return false;
    }
    setSlotDrafts((prev) =>
      prev.map((d) =>
        d.id === id
          ? {
              ...d,
              start_time: params.start_time,
              capacity: params.capacity,
            }
          : d
      )
    );
    return true;
  }

  function copySingleDraft(slot: SlotRow, toWeekdays: number[]) {
    const closed = toWeekdays.filter((d) => !isDayOpen(schedule, d));
    if (closed.length) {
      toast.error(`Cannot copy to closed days: ${closed.map((d) => WEEKDAYS[d]).join(", ")}`);
      return;
    }

    const next = [...slotDrafts];
    let copied = 0;
    for (const target of toWeekdays) {
      if (target === slot.weekday) continue;
      const err = validateDraftAdd(target, slot.start_time, slot.capacity, next);
      if (err) {
        toast.error(`${WEEKDAYS[target]}: ${err}`);
        continue;
      }
      next.push({
        id: draftId(),
        weekday: target,
        start_time: slot.start_time,
        capacity: slot.capacity,
      });
      copied++;
    }
    if (!copied) return;
    setSlotDrafts(next);
    toast.success(copied === 1 ? "Slot copied" : "Slots copied");
  }

  async function finish() {
    if (!calendarId) return;
    if (firstOpenWeekday(schedule) === null) {
      toast.error("Set working hours for at least one day");
      return;
    }

    setFinishing(true);
    try {
      await saveAvail.mutateAsync({ schedule, timezone, buffer_min: bufferMin });
      for (const draft of slotDrafts) {
        await createSlot.mutateAsync({
          weekday: draft.weekday,
          start_time: draft.start_time,
          duration_min: slotDurationMin,
          capacity: draft.capacity,
        });
      }
      toast.success("Calendar configured");
      onComplete(calendarId);
      onOpenChange(false);
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setFinishing(false);
    }
  }

  async function startWizard() {
    const trimmed = name.trim();
    if (!trimmed) {
      toast.error("Calendar name is required");
      return;
    }
    try {
      const cal = await createCalendar.mutateAsync({ name: trimmed, timezone });
      setCalendarId(cal.id);
      setStep(1);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  const draftRows: SlotRow[] = slotDrafts.map((d) => ({
    id: d.id,
    weekday: d.weekday,
    start_time: d.start_time,
    duration_min: slotDurationMin,
    capacity: d.capacity,
  }));

  return (
    <Sheet open={open} onClose={requestClose} width={AVAILABILITY_DRAWER_WIDTH}>
      <DrawerHeader title="Set up calendar" onClose={requestClose} />
      <DrawerBody>
        {step === 0 && (
          <div className={cn("space-y-4", formFieldClass)}>
            <div>
              <Label>Calendar name</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Sales team" />
            </div>
            <div>
              <Label>Timezone</Label>
              <Select value={timezone} onChange={(e) => setTimezone(e.target.value)}>
                {TIMEZONES.map((tz) => (
                  <option key={tz} value={tz}>
                    {tz}
                  </option>
                ))}
              </Select>
            </div>
          </div>
        )}
        {step === 1 && (
          <div className={cn("space-y-4", formFieldClass)}>
            <p className={drawerSubtitleClass}>
              Set working hours for each day. Closed days won&apos;t accept bookings.
            </p>
            <WorkingHoursGrid schedule={schedule} onChange={setSchedule} />
          </div>
        )}
        {step === 2 && (
          <div className={cn("space-y-4", formFieldClass)}>
            <p className={drawerSubtitleClass}>
              Add booking slots for each day. You can skip this and add slots later.
            </p>
            <BookingSlotsPanel
              schedule={schedule}
              slots={draftRows}
              globalDurationMin={slotDurationMin}
              onGlobalDurationChange={changeWizardDuration}
              bufferMin={bufferMin}
              onBufferMinChange={setBufferMin}
              onAdd={(params, onDone) => onDone(addDraft(params))}
              onEdit={(id, params, onDone) => onDone(updateDraft(id, params))}
              onRemove={(id) => setSlotDrafts((prev) => prev.filter((d) => d.id !== id))}
              onCopy={copySingleDraft}
            />
          </div>
        )}
      </DrawerBody>
      <DrawerFooter className="flex justify-end gap-2">
        <Button type="button" variant="secondary" onClick={requestClose}>
          Cancel
        </Button>
        {step > 0 && (
          <Button type="button" variant="secondary" onClick={() => setStep((s) => s - 1)}>
            Back
          </Button>
        )}
        {step === 0 && (
          <Button type="button" onClick={startWizard} disabled={createCalendar.isPending}>
            Next
          </Button>
        )}
        {step === 1 && (
          <Button
            type="button"
            onClick={() => {
              if (scheduleHasInvalidHours(schedule)) {
                toast.error("End time must be after start time for open days");
                return;
              }
              goToSlotStep();
            }}
          >
            Next
          </Button>
        )}
        {step === 2 && (
          <Button
            type="button"
            onClick={finish}
            disabled={finishing || !calendarId || saveAvail.isPending || createSlot.isPending}
          >
            Finish setup
          </Button>
        )}
      </DrawerFooter>
    </Sheet>
  );
}
