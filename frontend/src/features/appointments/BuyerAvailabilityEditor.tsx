import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { drawerSubtitleClass, drawerTitleClass, formFieldClass } from "@/components/ui/dialog";
import { Input, Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { TIMEZONES } from "@/lib/timezones";
import { cn } from "@/lib/utils";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  DEFAULT_WEEKLY_HOURS,
  firstValidStartInWindow,
  formatTimeHhmm12,
  isStartValidForWindow,
  timeOptions15,
  useBookingCalendar,
  useCalendarSlots,
  useCopyCalendarSlots,
  useCreateBookingCalendar,
  useCreateCalendarSlot,
  usePatchCalendarSlot,
  useSaveBookingCalendar,
  WEEKDAY_KEYS,
  WEEKDAYS,
} from "@/features/appointments/hooks";
import { TimeOfDayPicker } from "@/features/appointments/TimeOfDayPicker";

type DayHours = { start: string; end: string };
type Schedule = Record<string, DayHours>;

type SlotRow = {
  id: number | string;
  start_time: string;
  duration_min: number;
  capacity: number;
};

type SlotDraft = {
  id: string;
  weekday: number;
  start_time: string;
  duration_min: number;
  capacity: number;
};

const FIRST_OPEN_DAY_ORDER = [1, 2, 3, 4, 5, 6, 0];

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

function timeToMinutes(t: string): number {
  const [h, m] = t.split(":").map(Number);
  return h * 60 + m;
}

function slotInsideWindow(start: string, durationMin: number, dayStart: string, dayEnd: string): boolean {
  const startMin = timeToMinutes(start);
  const endMin = startMin + durationMin;
  return startMin >= timeToMinutes(dayStart) && endMin <= timeToMinutes(dayEnd);
}

function slotsOverlap(aStart: string, aDur: number, bStart: string, bDur: number): boolean {
  const a0 = timeToMinutes(aStart);
  const a1 = a0 + aDur;
  const b0 = timeToMinutes(bStart);
  const b1 = b0 + bDur;
  return a0 < b1 && b0 < a1;
}

function draftId(): string {
  return crypto.randomUUID();
}

function WorkingHoursGrid({
  schedule,
  onChange,
  readOnly = false,
}: {
  schedule: Schedule;
  onChange: (schedule: Schedule) => void;
  readOnly?: boolean;
}) {
  const times = timeOptions15();

  return (
    <div className="space-y-1">
      {WEEKDAY_KEYS.map((key, i) => {
        const w = schedule[key] ?? { start: "", end: "" };
        return (
          <div key={key} className="grid grid-cols-[4rem_1fr_1fr] items-center gap-2">
            <span className="text-sm font-medium text-gray-700">{WEEKDAYS[i]}</span>
            <Select
              value={w.start || ""}
              disabled={readOnly}
              onChange={(e) => onChange({ ...schedule, [key]: { ...w, start: e.target.value } })}
            >
              <option value="">Closed</option>
              {times.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </Select>
            <Select
              value={w.end || ""}
              disabled={readOnly}
              onChange={(e) => onChange({ ...schedule, [key]: { ...w, end: e.target.value } })}
            >
              <option value="">—</option>
              {times.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </Select>
          </div>
        );
      })}
    </div>
  );
}

function BookingSlotsPanel({
  schedule,
  activeDay,
  onActiveDayChange,
  slots,
  readOnly = false,
  onAdd,
  onRemove,
  onCopy,
  addPending = false,
  copyPending = false,
}: {
  schedule: Schedule;
  activeDay: number;
  onActiveDayChange: (day: number) => void;
  slots: SlotRow[];
  readOnly?: boolean;
  onAdd: (params: { start_time: string; duration_min: number; capacity: number }) => void;
  onRemove: (id: number | string) => void;
  onCopy: (toWeekdays: number[]) => void;
  addPending?: boolean;
  copyPending?: boolean;
}) {
  const [newStart, setNewStart] = useState("09:00");
  const [newDuration, setNewDuration] = useState(30);
  const [newCapacity, setNewCapacity] = useState(1);
  const [copyTargets, setCopyTargets] = useState<number[]>([]);
  const prevDayRef = useRef(activeDay);
  const dayOpen = isDayOpen(schedule, activeDay);
  const dayHours = schedule[WEEKDAY_KEYS[activeDay]];

  useEffect(() => {
    if (!dayOpen || !dayHours?.start || !dayHours?.end) return;
    const first = firstValidStartInWindow(dayHours.start, dayHours.end, newDuration);
    if (!first) return;
    if (prevDayRef.current !== activeDay) {
      prevDayRef.current = activeDay;
      setNewStart(first);
      return;
    }
    setNewStart((prev) =>
      isStartValidForWindow(prev, newDuration, dayHours.start, dayHours.end) ? prev : first
    );
  }, [activeDay, dayHours?.start, dayHours?.end, dayOpen, newDuration]);

  const canAddSlot =
    dayOpen &&
    dayHours?.start &&
    dayHours?.end &&
    firstValidStartInWindow(dayHours.start, dayHours.end, newDuration) !== null;

  return (
    <>
      <div className="mb-3 flex flex-wrap gap-1">
        {WEEKDAYS.map((label, i) => (
          <button
            key={label}
            type="button"
            onClick={() => onActiveDayChange(i)}
            className={cn(
              "rounded-md border px-2 py-1 text-sm font-semibold transition-colors",
              activeDay === i
                ? "border-jade-500 bg-jade-500 text-white"
                : "border-gray-100 bg-surface-card text-gray-700 hover:bg-jade-50 hover:text-jade-700"
            )}
          >
            {label}
          </button>
        ))}
      </div>

      {!dayOpen ? (
        <p className="text-sm text-gray-500">
          This day is closed — set working hours in the previous step.
        </p>
      ) : (
        <>
          <div className="space-y-1">
            {slots.map((s) => (
              <div
                key={s.id}
                className="flex items-center justify-between rounded border border-gray-100 px-3 py-2 text-sm"
              >
                <span>
                  {formatTimeHhmm12(s.start_time)} · {s.duration_min} min · capacity {s.capacity}
                </span>
                {!readOnly && (
                  <Button
                    variant="secondary"
                    className="h-7 text-xs"
                    onClick={() => onRemove(s.id)}
                  >
                    Remove
                  </Button>
                )}
              </div>
            ))}
            {!slots.length && <p className="text-sm text-gray-400">No slots for this day.</p>}
          </div>

          {!readOnly && (
            <>
              <div className="mt-3 space-y-3">
                <div>
                  <Label>Start</Label>
                  {dayHours?.start && dayHours?.end && (
                    <TimeOfDayPicker
                      value={newStart}
                      onChange={setNewStart}
                      dayStart={dayHours.start}
                      dayEnd={dayHours.end}
                      durationMin={newDuration}
                    />
                  )}
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <div>
                    <Label>Duration (min)</Label>
                    <Input
                      type="number"
                      min={15}
                      max={240}
                      value={newDuration}
                      onChange={(e) => setNewDuration(Number(e.target.value))}
                    />
                  </div>
                  <div>
                    <Label>Capacity</Label>
                    <Input
                      type="number"
                      min={1}
                      max={20}
                      value={newCapacity}
                      onChange={(e) => setNewCapacity(Number(e.target.value))}
                    />
                  </div>
                  <div className="flex items-end">
                    <Button
                      type="button"
                      onClick={() =>
                        onAdd({ start_time: newStart, duration_min: newDuration, capacity: newCapacity })
                      }
                      disabled={addPending || !canAddSlot}
                    >
                      Add slot
                    </Button>
                  </div>
                </div>
              </div>

              <div className="mt-4 flex flex-wrap items-end gap-2 border-t border-gray-100 pt-3">
                <span className="text-sm text-gray-500">Copy {WEEKDAYS[activeDay]} to:</span>
                {WEEKDAYS.map((label, i) =>
                  i === activeDay ? null : (
                    <label key={label} className="flex items-center gap-1 text-sm">
                      <input
                        type="checkbox"
                        checked={copyTargets.includes(i)}
                        onChange={(e) =>
                          setCopyTargets((prev) =>
                            e.target.checked ? [...prev, i] : prev.filter((d) => d !== i)
                          )
                        }
                      />
                      {label}
                    </label>
                  )
                )}
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => {
                    if (!copyTargets.length) return;
                    onCopy(copyTargets);
                    setCopyTargets([]);
                  }}
                  disabled={copyPending || !copyTargets.length}
                >
                  Copy slots
                </Button>
              </div>
            </>
          )}
        </>
      )}
    </>
  );
}

function WizardNav({
  showBack,
  onBack,
  onNext,
  nextLabel,
  nextDisabled,
}: {
  showBack: boolean;
  onBack: () => void;
  onNext: () => void;
  nextLabel: string;
  nextDisabled?: boolean;
}) {
  return (
    <div className="flex gap-2">
      {showBack && (
        <Button type="button" variant="secondary" onClick={onBack}>
          Back
        </Button>
      )}
      <Button type="button" onClick={onNext} disabled={nextDisabled}>
        {nextLabel}
      </Button>
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
  const copySlots = useCopyCalendarSlots(calendarId);

  const [schedule, setSchedule] = useState<Schedule>({});
  const [timezone, setTimezone] = useState("America/New_York");
  const [bufferMin, setBufferMin] = useState(0);
  const [activeDay, setActiveDay] = useState(1);

  useEffect(() => {
    if (!calendar) return;
    setSchedule((calendar.schedule as Schedule) ?? {});
    setTimezone(calendar.timezone);
    setBufferMin(calendar.buffer_min);
  }, [calendar]);

  if (!calendar) return null;

  function saveHours() {
    saveAvail.mutate(
      { schedule, timezone, buffer_min: bufferMin },
      {
        onSuccess: () => toast.success("Availability saved"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function addSlot(params: { start_time: string; duration_min: number; capacity: number }) {
    createSlot.mutate(
      { weekday: activeDay, ...params },
      {
        onSuccess: () => toast.success("Slot added"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function runCopy(toWeekdays: number[]) {
    copySlots.mutate(
      { from_weekday: activeDay, to_weekdays: toWeekdays },
      {
        onSuccess: () => toast.success("Slots copied"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  const daySlots: SlotRow[] = slots
    .filter((s) => s.weekday === activeDay && !s.disabled_at)
    .map((s) => ({
      id: s.id,
      start_time: s.start_time,
      duration_min: s.duration_min,
      capacity: s.capacity,
    }));

  return (
    <div className="space-y-6">
      <div className="grid gap-3 md:grid-cols-3">
        <div>
          <Label>Calendar timezone</Label>
          <Select value={timezone} disabled={readOnly} onChange={(e) => setTimezone(e.target.value)}>
            {TIMEZONES.map((tz) => (
              <option key={tz} value={tz}>
                {tz}
              </option>
            ))}
          </Select>
        </div>
        <div>
          <Label>Buffer after appointments (min)</Label>
          <Input
            type="number"
            min={0}
            max={60}
            disabled={readOnly}
            value={bufferMin}
            onChange={(e) => setBufferMin(Number(e.target.value))}
          />
        </div>
        {!readOnly && (
          <div className="flex items-end">
            <Button type="button" onClick={saveHours} disabled={saveAvail.isPending}>
              Save hours & buffer
            </Button>
          </div>
        )}
      </div>

      <div>
        <SectionLabel>Working hours</SectionLabel>
        <div className="mt-2">
          <WorkingHoursGrid schedule={schedule} onChange={setSchedule} readOnly={readOnly} />
        </div>
      </div>

      <div>
        <SectionLabel>Booking slots</SectionLabel>
        <div className="mt-2">
          <BookingSlotsPanel
            schedule={schedule}
            activeDay={activeDay}
            onActiveDayChange={setActiveDay}
            slots={daySlots}
            readOnly={readOnly}
            onAdd={addSlot}
            onRemove={(id) =>
              patchSlot.mutate({ id: id as number, body: { disabled: true } }, {
                onSuccess: () => toast.success("Slot disabled"),
                onError: (e) => toast.error(errorMessage(e)),
              })
            }
            onCopy={runCopy}
            addPending={createSlot.isPending}
            copyPending={copySlots.isPending}
          />
        </div>
      </div>
    </div>
  );
}

export function BuyerSetupWizard({ onComplete }: { onComplete: (calendarId: number) => void }) {
  const createCalendar = useCreateBookingCalendar();
  const [calendarId, setCalendarId] = useState<number | null>(null);
  const saveAvail = useSaveBookingCalendar(calendarId ?? 0);
  const createSlot = useCreateCalendarSlot(calendarId ?? 0);
  const [step, setStep] = useState(0);
  const [name, setName] = useState("");
  const [timezone, setTimezone] = useState("America/New_York");
  const [schedule, setSchedule] = useState<Schedule>({ ...DEFAULT_WEEKLY_HOURS });
  const [activeDay, setActiveDay] = useState(1);
  const [slotDrafts, setSlotDrafts] = useState<SlotDraft[]>([]);
  const [finishing, setFinishing] = useState(false);
  const hasEnteredSlotStep = useRef(false);

  function goToSlotStep() {
    const weekday = firstOpenWeekday(schedule);
    if (weekday === null) {
      toast.error("Set working hours for at least one day");
      return;
    }
    if (!hasEnteredSlotStep.current) {
      setActiveDay(weekday);
      hasEnteredSlotStep.current = true;
    }
    setStep(2);
  }

  function validateDraftAdd(
    weekday: number,
    start_time: string,
    duration_min: number,
    capacity: number,
    drafts: SlotDraft[]
  ): string | null {
    if (!isDayOpen(schedule, weekday)) {
      return "This day is closed";
    }
    if (duration_min < 15 || duration_min > 240) {
      return "Duration must be between 15 and 240 minutes";
    }
    if (capacity < 1 || capacity > 20) {
      return "Capacity must be between 1 and 20";
    }
    const day = schedule[WEEKDAY_KEYS[weekday]]!;
    if (!slotInsideWindow(start_time, duration_min, day.start, day.end)) {
      return "Slot must fall within working hours";
    }
    const sameDay = drafts.filter((d) => d.weekday === weekday);
    if (sameDay.some((d) => slotsOverlap(start_time, duration_min, d.start_time, d.duration_min))) {
      return "Slot overlaps an existing slot on this day";
    }
    return null;
  }

  function addDraft(params: { start_time: string; duration_min: number; capacity: number }) {
    const err = validateDraftAdd(activeDay, params.start_time, params.duration_min, params.capacity, slotDrafts);
    if (err) {
      toast.error(err);
      return;
    }
    setSlotDrafts((prev) => [
      ...prev,
      { id: draftId(), weekday: activeDay, ...params },
    ]);
  }

  function copyDrafts(toWeekdays: number[]) {
    const source = slotDrafts.filter((d) => d.weekday === activeDay);
    if (!source.length) {
      toast.error("No slots to copy on this day");
      return;
    }

    const closed = toWeekdays.filter((d) => !isDayOpen(schedule, d));
    if (closed.length) {
      toast.error(`Cannot copy to closed days: ${closed.map((d) => WEEKDAYS[d]).join(", ")}`);
      return;
    }

    const next = [...slotDrafts];
    for (const target of toWeekdays) {
      for (const s of source) {
        const err = validateDraftAdd(target, s.start_time, s.duration_min, s.capacity, next);
        if (err) {
          toast.error(`${WEEKDAYS[target]}: ${err}`);
          continue;
        }
        next.push({
          id: draftId(),
          weekday: target,
          start_time: s.start_time,
          duration_min: s.duration_min,
          capacity: s.capacity,
        });
      }
    }
    setSlotDrafts(next);
    toast.success("Slots copied");
  }

  async function finish() {
    if (!calendarId) return;
    if (firstOpenWeekday(schedule) === null) {
      toast.error("Set working hours for at least one day");
      return;
    }

    setFinishing(true);
    try {
      await saveAvail.mutateAsync({ schedule, timezone, buffer_min: 0 });
      for (const draft of slotDrafts) {
        await createSlot.mutateAsync({
          weekday: draft.weekday,
          start_time: draft.start_time,
          duration_min: draft.duration_min,
          capacity: draft.capacity,
        });
      }
      toast.success("Calendar configured");
      onComplete(calendarId);
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

  const dayDrafts: SlotRow[] = slotDrafts
    .filter((d) => d.weekday === activeDay)
    .map((d) => ({
      id: d.id,
      start_time: d.start_time,
      duration_min: d.duration_min,
      capacity: d.capacity,
    }));

  return (
    <div className="mx-auto max-w-lg space-y-4 rounded-lg border border-gray-100 bg-surface-card p-6 font-sans">
      <h2 className={drawerTitleClass}>Set up calendar</h2>
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
          <WizardNav
            showBack={false}
            onBack={() => {}}
            onNext={startWizard}
            nextLabel="Next"
            nextDisabled={createCalendar.isPending}
          />
        </div>
      )}
      {step === 1 && (
        <div className={cn("space-y-4", formFieldClass)}>
          <p className={drawerSubtitleClass}>
            Set working hours for each day. Closed days won&apos;t accept bookings.
          </p>
          <WorkingHoursGrid schedule={schedule} onChange={setSchedule} />
          <WizardNav
            showBack
            onBack={() => setStep(0)}
            onNext={() => {
              if (scheduleHasInvalidHours(schedule)) {
                toast.error("End time must be after start time for open days");
                return;
              }
              goToSlotStep();
            }}
            nextLabel="Next"
          />
        </div>
      )}
      {step === 2 && (
        <div className={cn("space-y-4", formFieldClass)}>
          <p className={drawerSubtitleClass}>
            Add booking slots for each day. You can skip this and add slots later.
          </p>
          <BookingSlotsPanel
            schedule={schedule}
            activeDay={activeDay}
            onActiveDayChange={setActiveDay}
            slots={dayDrafts}
            onAdd={addDraft}
            onRemove={(id) => setSlotDrafts((prev) => prev.filter((d) => d.id !== id))}
            onCopy={copyDrafts}
          />
          <WizardNav
            showBack
            onBack={() => setStep(1)}
            onNext={finish}
            nextLabel="Finish setup"
            nextDisabled={finishing || !calendarId || saveAvail.isPending || createSlot.isPending}
          />
        </div>
      )}
    </div>
  );
}
