import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
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
  buildOptimisticSlot,
  firstValidStartInWindow,
  invalidateCalendarSlots,
  isOptimisticSlotId,
  isStartValidForWindow,
  minutesToTimeHhmm,
  setCalendarSlotsCache,
  timeHhmmToMinutes,
  useBookingCalendar,
  useCalendarSlots,
  useCreateBookingCalendar,
  useCreateCalendarSlot,
  useCreatePublisherBookingCalendar,
  useCreatePublisherCalendarSlot,
  useCopyCalendarSlots,
  useCopyPublisherCalendarSlots,
  usePatchCalendarSlot,
  usePatchPublisherCalendarSlot,
  usePublisherBookingCalendar,
  usePublisherCalendarSlots,
  useSaveBookingCalendar,
  useSavePublisherBookingCalendar,
  type CalendarOwner,
  type SlotMutationMeta,
  WEEKDAY_KEYS,
  WEEKDAYS,
} from "@/features/appointments/hooks";
import { CopyToWeekdaysPopover } from "@/features/appointments/CopyToWeekdaysPopover";
import { TimeFieldInput } from "@/features/appointments/TimeFieldInput";
import { Spinner } from "@/components/ui/misc";
import type { BuyerAppointmentSlot } from "@/types";

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
const DEFAULT_SLOT_CAPACITY = 1;
const MIN_SLOT_CAPACITY = 1;
const MAX_SLOT_CAPACITY = 20;
const SLOT_ROW_GRID =
  "grid grid-cols-[4rem_minmax(7.5rem,9rem)_minmax(7.5rem,9rem)_3rem_4.5rem] items-center gap-2";
// From+To share slot From+To+Cap width (equal columns); actions column aligns with slots.
const WORKING_HOURS_GRID =
  "grid grid-cols-[4rem_minmax(9.25rem,10.75rem)_minmax(9.25rem,10.75rem)_4.5rem] items-center gap-2";
export const AVAILABILITY_DRAWER_WIDTH = 520;
const SLOT_SKIP_OPTIMISTIC = { meta: { skipOptimistic: true } satisfies SlotMutationMeta };

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

function clampDefaultCapacity(value: number): number {
  return Math.min(MAX_SLOT_CAPACITY, Math.max(MIN_SLOT_CAPACITY, Math.round(value)));
}

function generateDaySlotStarts(
  dayStart: string,
  dayEnd: string,
  durationMin: number,
  bufferMin: number
): string[] {
  const starts: string[] = [];
  let current = firstValidStartInWindow(dayStart, dayEnd, durationMin);
  while (current && isStartValidForWindow(current, durationMin, dayStart, dayEnd)) {
    starts.push(current);
    const nextMin = timeHhmmToMinutes(slotEndTime(current, durationMin)) + bufferMin;
    current = minutesToTimeHhmm(nextMin);
  }
  return starts;
}

function generateAllSlotStarts(
  schedule: Schedule,
  durationMin: number,
  bufferMin: number
): { weekday: number; start_time: string }[] {
  const result: { weekday: number; start_time: string }[] = [];
  for (const weekday of openWeekdays(schedule)) {
    const day = schedule[WEEKDAY_KEYS[weekday]]!;
    for (const start_time of generateDaySlotStarts(day.start, day.end, durationMin, bufferMin)) {
      result.push({ weekday, start_time });
    }
  }
  return result;
}

function buildGenerateConfirmMessage(
  existingCount: number,
  newCount: number,
  durationMin: number,
  bufferMin: number,
  capacity: number
): string {
  const settings = `${durationMin} min, ${bufferMin} min buffer, capacity ${capacity}`;
  if (existingCount > 0) {
    return `Replace all ${existingCount} booking slot${existingCount === 1 ? "" : "s"} and generate ${newCount} new slot${newCount === 1 ? "" : "s"} from working hours? (${settings})`;
  }
  return `Generate ${newCount} booking slot${newCount === 1 ? "" : "s"} from working hours? (${settings})`;
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

  const pending = isOptimisticSlotId(slot.id);
  const rowDisabled = readOnly || pending;

  return (
    <div className={SLOT_ROW_GRID}>
      <span className="text-sm font-medium text-gray-700">{dayLabel ?? ""}</span>
      <div className="min-w-0 w-full">
        <TimeFieldInput
          value={from}
          disabled={rowDisabled}
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
          disabled={rowDisabled}
          value={capacity}
          className="px-2 text-center"
          onBlur={commit}
          onChange={(e) => setCapacity(Number(e.target.value))}
        />
      </div>
      {!readOnly && (
        <div className="flex items-center gap-0.5">
          {copyButton}
          <IconButton
            variant="danger"
            aria-label="Remove slot"
            disabled={pending}
            onClick={onRemove}
          >
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
  onClearAll,
  onGenerateAll,
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
  onClearAll?: () => void;
  onGenerateAll?: (defaultCapacity: number) => void;
  savePending?: boolean;
}) {
  const days = openWeekdays(schedule);
  const [durationDraft, setDurationDraft] = useState(String(globalDurationMin));
  const [defaultCapacityDraft, setDefaultCapacityDraft] = useState(String(DEFAULT_SLOT_CAPACITY));

  useEffect(() => {
    setDurationDraft(String(globalDurationMin));
  }, [globalDurationMin]);

  function defaultCapacity(): number {
    const parsed = Number(defaultCapacityDraft);
    if (Number.isNaN(parsed)) return DEFAULT_SLOT_CAPACITY;
    return clampDefaultCapacity(parsed);
  }

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
    onAdd({ weekday, start_time: from, capacity: defaultCapacity() }, () => {});
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

  function handleDefaultCapacityInput(raw: string) {
    setDefaultCapacityDraft(raw);
  }

  function handleDefaultCapacityBlur() {
    const trimmed = defaultCapacityDraft.trim();
    if (!trimmed) {
      setDefaultCapacityDraft(String(DEFAULT_SLOT_CAPACITY));
      return;
    }
    const next = Number(trimmed);
    if (Number.isNaN(next)) {
      setDefaultCapacityDraft(String(DEFAULT_SLOT_CAPACITY));
      return;
    }
    setDefaultCapacityDraft(String(clampDefaultCapacity(next)));
  }

  if (!days.length) {
    return (
      <p className="text-sm text-gray-500">Set working hours above to add booking slots.</p>
    );
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-3 md:grid-cols-3">
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
          <Label>Buffer (After)</Label>
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
        <div>
          <Label>Default capacity</Label>
          <Input
            type="number"
            min={MIN_SLOT_CAPACITY}
            max={MAX_SLOT_CAPACITY}
            disabled={readOnly}
            value={defaultCapacityDraft}
            onChange={(e) => handleDefaultCapacityInput(e.target.value)}
            onBlur={handleDefaultCapacityBlur}
          />
        </div>
      </div>

      <div className="space-y-1">
        {(onGenerateAll || (slots.length > 0 && onClearAll)) && (
          <div className="flex justify-end gap-2">
            {onGenerateAll && (
              <Button
                type="button"
                variant="secondary"
                size="sm"
                disabled={savePending}
                onClick={() => onGenerateAll(defaultCapacity())}
              >
                Generate slots
              </Button>
            )}
            {slots.length > 0 && onClearAll && (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                disabled={savePending}
                onClick={onClearAll}
              >
                Clear all
              </Button>
            )}
          </div>
        )}
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
  owner = "buyer",
}: {
  calendarId: number;
  readOnly?: boolean;
  owner?: CalendarOwner;
}) {
  const qc = useQueryClient();
  const isPublisher = owner === "publisher";
  const buyerCal = useBookingCalendar(isPublisher ? null : calendarId);
  const pubCal = usePublisherBookingCalendar(isPublisher ? calendarId : null);
  const calendar = isPublisher ? pubCal.data : buyerCal.data;
  const calendarLoading = isPublisher ? pubCal.isLoading : buyerCal.isLoading;
  const saveBuyer = useSaveBookingCalendar(calendarId);
  const savePub = useSavePublisherBookingCalendar(calendarId);
  const saveAvail = isPublisher ? savePub : saveBuyer;
  const buyerSlots = useCalendarSlots(isPublisher ? null : calendarId);
  const pubSlots = usePublisherCalendarSlots(isPublisher ? calendarId : null);
  const slotsData = isPublisher ? pubSlots.data : buyerSlots.data;
  const emptySlots = useMemo(() => [] as BuyerAppointmentSlot[], []);
  const slots = slotsData ?? emptySlots;
  const createBuyerSlot = useCreateCalendarSlot(calendarId);
  const createPubSlot = useCreatePublisherCalendarSlot(calendarId);
  const createSlot = isPublisher ? createPubSlot : createBuyerSlot;
  const patchBuyerSlot = usePatchCalendarSlot(calendarId);
  const patchPubSlot = usePatchPublisherCalendarSlot(calendarId);
  const patchSlot = isPublisher ? patchPubSlot : patchBuyerSlot;
  const copyBuyerSlots = useCopyCalendarSlots(calendarId);
  const copyPubSlots = useCopyPublisherCalendarSlots(calendarId);
  const copySlots = isPublisher ? copyPubSlots : copyBuyerSlots;

  const [name, setName] = useState("");
  const [schedule, setSchedule] = useState<Schedule>({});
  const [timezone, setTimezone] = useState("America/New_York");
  const [location, setLocation] = useState("");
  const [bufferMin, setBufferMin] = useState(0);
  const [slotDurationMin, setSlotDurationMin] = useState(DEFAULT_SLOT_DURATION_MIN);
  const [bulkSlotOp, setBulkSlotOp] = useState(false);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const savedSnapshot = useRef<{ name: string; schedule: Schedule; timezone: string; location: string; bufferMin: number } | null>(null);
  const stateRef = useRef({ name, schedule, timezone, location, bufferMin });
  const slotsInitialized = useRef(false);
  const calendarLoadedRef = useRef(false);
  const lastHydratedKey = useRef<string | null>(null);

  useEffect(() => {
    stateRef.current = { name, schedule, timezone, location, bufferMin };
  }, [name, schedule, timezone, location, bufferMin]);

  useEffect(() => {
    lastHydratedKey.current = null;
    calendarLoadedRef.current = false;
  }, [calendarId]);

  useEffect(() => {
    if (!calendar) {
      calendarLoadedRef.current = false;
      return;
    }
    if (saveTimer.current) return;
    const hydrationKey = `${calendar.id}:${calendar.updated_at}`;
    if (lastHydratedKey.current === hydrationKey) return;
    lastHydratedKey.current = hydrationKey;

    const sched = (calendar.schedule as Schedule) ?? {};
    setName(calendar.name);
    setSchedule(sched);
    setTimezone(calendar.timezone);
    setLocation(calendar.location ?? "");
    setBufferMin(calendar.buffer_min);
    savedSnapshot.current = {
      name: calendar.name,
      schedule: sched,
      timezone: calendar.timezone,
      location: calendar.location ?? "",
      bufferMin: calendar.buffer_min,
    };
    calendarLoadedRef.current = true;
    slotsInitialized.current = false;
  }, [calendar?.id, calendar?.updated_at]);

  useEffect(() => {
    if (slotsInitialized.current) return;
    const active = slots.filter((s) => !s.disabled_at);
    if (!active.length) return;
    setSlotDurationMin(active[0].duration_min);
    slotsInitialized.current = true;
  }, [slots]);

  const persistIfDirty = useCallback(() => {
    if (readOnly || !calendarLoadedRef.current) return;
    const { name: calName, schedule: sched, timezone: tz, location: loc, bufferMin: buf } = stateRef.current;
    if (scheduleHasInvalidHours(sched)) return;
    const trimmedName = calName.trim();
    if (!trimmedName) {
      toast.error("Calendar name is required");
      return;
    }
    const snap = savedSnapshot.current;
    if (
      snap &&
      snap.name === trimmedName &&
      snap.timezone === tz &&
      snap.location === loc &&
      snap.bufferMin === buf &&
      schedulesEqual(snap.schedule, sched)
    ) {
      return;
    }
    saveAvail.mutate(
      { name: trimmedName, schedule: sched, timezone: tz, location: loc, buffer_min: buf },
      {
        onSuccess: (saved) => {
          const savedSched = (saved.schedule as Schedule) ?? {};
          savedSnapshot.current = {
            name: saved.name,
            schedule: savedSched,
            timezone: saved.timezone,
            location: saved.location ?? "",
            bufferMin: saved.buffer_min,
          };
          lastHydratedKey.current = `${saved.id}:${saved.updated_at}`;
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }, [readOnly, saveAvail]);

  const persistIfDirtyRef = useRef(persistIfDirty);
  persistIfDirtyRef.current = persistIfDirty;

  const scheduleSave = useCallback(() => {
    if (readOnly || !calendarLoadedRef.current) return;
    clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(persistIfDirty, SAVE_DEBOUNCE_MS);
  }, [readOnly, persistIfDirty]);

  useEffect(() => {
    return () => {
      clearTimeout(saveTimer.current);
      saveTimer.current = undefined;
      persistIfDirtyRef.current();
    };
  }, []);

  function handleBlurSave() {
    scheduleSave();
  }

  if (calendarLoading || !calendar) {
    return <Spinner className="mx-auto h-6 w-6" />;
  }

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
    if (isOptimisticSlotId(id)) {
      onDone(false);
      return;
    }
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
    const targets = toWeekdays.filter((target) => target !== slot.weekday);
    if (!targets.length) return;

    const sourceSlots = slots.filter((s) => !s.disabled_at && s.weekday === slot.weekday);
    if (!sourceSlots.length) return;

    const optimisticAdds = targets.flatMap((target) =>
      sourceSlots.map((s) =>
        buildOptimisticSlot(calendarId, calendar.account_id, {
          weekday: target,
          start_time: s.start_time,
          duration_min: slotDurationMin,
          capacity: s.capacity,
        })
      )
    );
    setCalendarSlotsCache(qc, isPublisher ? "publisher" : "buyer", calendarId, [
      ...slots,
      ...optimisticAdds,
    ]);

    setBulkSlotOp(true);
    try {
      await copySlots.mutateAsync({ from_weekday: slot.weekday, to_weekdays: targets });
      toast.success(
        targets.length === 1
          ? `${WEEKDAYS[slot.weekday]} copied to ${WEEKDAYS[targets[0]!]}`
          : `${WEEKDAYS[slot.weekday]} copied to ${targets.length} days`
      );
    } catch (e) {
      toast.error(errorMessage(e));
      invalidateCalendarSlots(qc, isPublisher ? "publisher" : "buyer", calendarId);
    } finally {
      setBulkSlotOp(false);
    }
  }

  async function clearAllSlots() {
    const active = slots.filter((s) => !s.disabled_at);
    if (!active.length) return;
    if (!window.confirm(`Remove all ${active.length} booking slots?`)) return;

    const now = new Date().toISOString();
    setCalendarSlotsCache(
      qc,
      isPublisher ? "publisher" : "buyer",
      calendarId,
      slots.map((s) => (!s.disabled_at ? { ...s, disabled_at: now } : s))
    );

    setBulkSlotOp(true);
    let cleared = 0;
    try {
      for (const s of active) {
        if (isOptimisticSlotId(s.id)) {
          cleared++;
          continue;
        }
        try {
          await patchSlot.mutateAsync({ id: s.id, body: { disabled: true } }, SLOT_SKIP_OPTIMISTIC);
          cleared++;
        } catch (e) {
          toast.error(`${WEEKDAYS[s.weekday]} ${s.start_time}: ${errorMessage(e)}`);
        }
      }
      if (cleared === active.length) toast.success("All booking slots cleared");
    } finally {
      invalidateCalendarSlots(qc, isPublisher ? "publisher" : "buyer", calendarId);
      setBulkSlotOp(false);
    }
  }

  async function generateAllSlots(defaultCapacity: number) {
    const capacity = clampDefaultCapacity(defaultCapacity);
    const generated = generateAllSlotStarts(schedule, slotDurationMin, bufferMin);
    if (!generated.length) {
      toast.error("No slots fit in working hours");
      return;
    }
    const active = slots.filter((s) => !s.disabled_at);
    const msg = buildGenerateConfirmMessage(
      active.length,
      generated.length,
      slotDurationMin,
      bufferMin,
      capacity
    );
    if (!window.confirm(msg)) return;

    const now = new Date().toISOString();
    const optimisticNew = generated.map((slot) =>
      buildOptimisticSlot(calendarId, calendar.account_id, {
        weekday: slot.weekday,
        start_time: slot.start_time,
        duration_min: slotDurationMin,
        capacity,
      })
    );
    setCalendarSlotsCache(qc, isPublisher ? "publisher" : "buyer", calendarId, [
      ...slots.map((s) => (!s.disabled_at ? { ...s, disabled_at: now } : s)),
      ...optimisticNew,
    ]);

    setBulkSlotOp(true);
    try {
      for (const s of active) {
        if (isOptimisticSlotId(s.id)) continue;
        try {
          await patchSlot.mutateAsync({ id: s.id, body: { disabled: true } }, SLOT_SKIP_OPTIMISTIC);
        } catch (e) {
          toast.error(`${WEEKDAYS[s.weekday]} ${s.start_time}: ${errorMessage(e)}`);
        }
      }

      let created = 0;
      for (const slot of generated) {
        try {
          await createSlot.mutateAsync(
            {
              weekday: slot.weekday,
              start_time: slot.start_time,
              duration_min: slotDurationMin,
              capacity,
            },
            SLOT_SKIP_OPTIMISTIC
          );
          created++;
        } catch (e) {
          toast.error(`${WEEKDAYS[slot.weekday]} ${slot.start_time}: ${errorMessage(e)}`);
        }
      }

      if (created === generated.length) {
        toast.success(
          created === 1 ? "1 booking slot generated" : `${created} booking slots generated`
        );
      } else if (created > 0) {
        toast.success(`${created} of ${generated.length} booking slots generated`);
      }
    } finally {
      invalidateCalendarSlots(qc, isPublisher ? "publisher" : "buyer", calendarId);
      setBulkSlotOp(false);
    }
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
        <Label>Calendar name</Label>
        <Input
          value={name}
          disabled={readOnly}
          placeholder="e.g. Sales team"
          onChange={(e) => setName(e.target.value)}
          onBlur={handleBlurSave}
        />
      </div>

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
          onChange={(e) => setLocation(e.target.value)}
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
            onRemove={(id) => {
              if (isOptimisticSlotId(id)) return;
              patchSlot.mutate(
                { id: id as number, body: { disabled: true } },
                {
                  onError: (e) => toast.error(errorMessage(e)),
                }
              );
            }}
            onCopy={copySlot}
            onClearAll={readOnly ? undefined : clearAllSlots}
            onGenerateAll={readOnly ? undefined : generateAllSlots}
            savePending={bulkSlotOp}
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
  owner = "buyer",
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onComplete: (calendarId: number) => void;
  owner?: CalendarOwner;
}) {
  const isPublisher = owner === "publisher";
  const createBuyerCalendar = useCreateBookingCalendar();
  const createPubCalendar = useCreatePublisherBookingCalendar();
  const createCalendar = isPublisher ? createPubCalendar : createBuyerCalendar;
  const [calendarId, setCalendarId] = useState<number | null>(null);
  const saveBuyer = useSaveBookingCalendar(calendarId ?? 0);
  const savePub = useSavePublisherBookingCalendar(calendarId ?? 0);
  const saveAvail = isPublisher ? savePub : saveBuyer;
  const createBuyerSlot = useCreateCalendarSlot(calendarId ?? 0);
  const createPubSlot = useCreatePublisherCalendarSlot(calendarId ?? 0);
  const createSlot = isPublisher ? createPubSlot : createBuyerSlot;
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

  function generateAllDrafts(defaultCapacity: number) {
    const capacity = clampDefaultCapacity(defaultCapacity);
    const generated = generateAllSlotStarts(schedule, slotDurationMin, bufferMin);
    if (!generated.length) {
      toast.error("No slots fit in working hours");
      return;
    }
    const msg = buildGenerateConfirmMessage(
      slotDrafts.length,
      generated.length,
      slotDurationMin,
      bufferMin,
      capacity
    );
    if (!window.confirm(msg)) return;
    setSlotDrafts(
      generated.map((slot) => ({
        id: draftId(),
        weekday: slot.weekday,
        start_time: slot.start_time,
        capacity,
      }))
    );
    toast.success(
      generated.length === 1
        ? "1 booking slot generated"
        : `${generated.length} booking slots generated`
    );
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
              onClearAll={() => setSlotDrafts([])}
              onGenerateAll={generateAllDrafts}
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
