import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { TIMEZONES } from "@/lib/timezones";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  DEFAULT_WEEKLY_HOURS,
  timeOptions15,
  useBuyerAppointmentSlots,
  useCopyBuyerSlots,
  useCreateBuyerSlot,
  usePatchBuyerSlot,
  useSaveBuyerAvailability,
  WEEKDAY_KEYS,
  WEEKDAYS,
} from "@/features/appointments/hooks";
import type { BuyerAvailability } from "@/types";

export function BuyerAvailabilityEditor({
  availability,
  readOnly = false,
}: {
  availability: BuyerAvailability;
  readOnly?: boolean;
}) {
  const saveAvail = useSaveBuyerAvailability();
  const { data: slots = [] } = useBuyerAppointmentSlots();
  const createSlot = useCreateBuyerSlot();
  const patchSlot = usePatchBuyerSlot();
  const copySlots = useCopyBuyerSlots();

  const [schedule, setSchedule] = useState(availability.schedule ?? {});
  const [timezone, setTimezone] = useState(availability.timezone);
  const [bufferMin, setBufferMin] = useState(availability.buffer_min);
  const [activeDay, setActiveDay] = useState(1);
  const [newStart, setNewStart] = useState("09:00");
  const [newDuration, setNewDuration] = useState(30);
  const [newCapacity, setNewCapacity] = useState(1);
  const [copyTargets, setCopyTargets] = useState<number[]>([]);

  const times = timeOptions15();

  function saveHours() {
    saveAvail.mutate(
      { schedule, timezone, buffer_min: bufferMin },
      {
        onSuccess: () => toast.success("Availability saved"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function addSlot() {
    createSlot.mutate(
      { weekday: activeDay, start_time: newStart, duration_min: newDuration, capacity: newCapacity },
      {
        onSuccess: () => toast.success("Slot added"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function runCopy() {
    if (!copyTargets.length) return;
    copySlots.mutate(
      { from_weekday: activeDay, to_weekdays: copyTargets },
      {
        onSuccess: () => toast.success("Slots copied"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  const daySlots = slots.filter((s) => s.weekday === activeDay && !s.disabled_at);

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
        <div className="mt-2 space-y-1">
          {WEEKDAY_KEYS.map((key, i) => {
            const w = schedule[key] ?? { start: "", end: "" };
            return (
              <div key={key} className="grid grid-cols-[4rem_1fr_1fr] items-center gap-2 text-sm">
                <span className="font-medium text-gray-600">{WEEKDAYS[i]}</span>
                <Select
                  value={w.start || ""}
                  disabled={readOnly}
                  onChange={(e) => setSchedule({ ...schedule, [key]: { ...w, start: e.target.value } })}
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
                  onChange={(e) => setSchedule({ ...schedule, [key]: { ...w, end: e.target.value } })}
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
      </div>

      <div>
        <SectionLabel>Booking slots</SectionLabel>
        <div className="mb-3 flex flex-wrap gap-1">
          {WEEKDAYS.map((label, i) => (
            <button
              key={label}
              type="button"
              onClick={() => setActiveDay(i)}
              className={`rounded px-2 py-1 text-sm font-semibold ${
                activeDay === i ? "bg-jade-500 text-white" : "bg-gray-100 text-gray-600"
              }`}
            >
              {label}
            </button>
          ))}
        </div>

        <div className="space-y-1">
          {daySlots.map((s) => (
            <div
              key={s.id}
              className="flex items-center justify-between rounded border border-gray-100 px-3 py-2 text-sm"
            >
              <span>
                {s.start_time.slice(0, 5)} · {s.duration_min} min · capacity {s.capacity}
              </span>
              {!readOnly && (
                <Button
                  variant="secondary"
                  className="h-7 text-xs"
                  onClick={() =>
                    patchSlot.mutate({ id: s.id, body: { disabled: true } }, {
                      onSuccess: () => toast.success("Slot disabled"),
                    })
                  }
                >
                  Disable
                </Button>
              )}
            </div>
          ))}
          {!daySlots.length && <p className="text-sm text-gray-400">No slots for this day.</p>}
        </div>

        {!readOnly && (
          <>
            <div className="mt-3 grid grid-cols-4 gap-2">
              <div>
                <Label>Start</Label>
                <Select value={newStart} onChange={(e) => setNewStart(e.target.value)}>
                  {times.map((t) => (
                    <option key={t} value={t}>
                      {t}
                    </option>
                  ))}
                </Select>
              </div>
              <div>
                <Label>Duration (min)</Label>
                <Input type="number" value={newDuration} onChange={(e) => setNewDuration(Number(e.target.value))} />
              </div>
              <div>
                <Label>Capacity</Label>
                <Input type="number" value={newCapacity} onChange={(e) => setNewCapacity(Number(e.target.value))} />
              </div>
              <div className="flex items-end">
                <Button type="button" onClick={addSlot} disabled={createSlot.isPending}>
                  Add slot
                </Button>
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
              <Button type="button" variant="secondary" onClick={runCopy} disabled={copySlots.isPending}>
                Copy slots
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

export function BuyerSetupWizard({ onComplete }: { onComplete: () => void }) {
  const saveAvail = useSaveBuyerAvailability();
  const createSlot = useCreateBuyerSlot();
  const [step, setStep] = useState(0);
  const [timezone, setTimezone] = useState("America/New_York");
  const [slotStart, setSlotStart] = useState("09:00");

  function finish() {
    saveAvail.mutate(
      { schedule: DEFAULT_WEEKLY_HOURS, timezone, buffer_min: 0 },
      {
        onSuccess: () => {
          createSlot.mutate(
            { weekday: 1, start_time: slotStart, duration_min: 30, capacity: 1 },
            {
              onSuccess: () => {
                toast.success("Availability configured");
                onComplete();
              },
              onError: (e) => toast.error(errorMessage(e)),
            }
          );
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  return (
    <div className="mx-auto max-w-lg space-y-4 rounded-lg border border-gray-100 bg-white p-6">
      <h2 className="text-lg font-bold text-gray-800">Set up appointments</h2>
      {step === 0 && (
        <>
          <Label>Calendar timezone</Label>
          <Select value={timezone} onChange={(e) => setTimezone(e.target.value)}>
            {TIMEZONES.map((tz) => (
              <option key={tz} value={tz}>
                {tz}
              </option>
            ))}
          </Select>
          <Button type="button" onClick={() => setStep(1)}>
            Next
          </Button>
        </>
      )}
      {step === 1 && (
        <>
          <p className="text-sm text-gray-600">We&apos;ll start with Mon–Fri 9:00–17:00 working hours. You can refine later.</p>
          <Button type="button" onClick={() => setStep(2)}>
            Next
          </Button>
        </>
      )}
      {step === 2 && (
        <>
          <Label>First Monday slot start time</Label>
          <Select value={slotStart} onChange={(e) => setSlotStart(e.target.value)}>
            {timeOptions15().map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </Select>
          <Button type="button" onClick={finish} disabled={saveAvail.isPending || createSlot.isPending}>
            Finish setup
          </Button>
        </>
      )}
    </div>
  );
}
