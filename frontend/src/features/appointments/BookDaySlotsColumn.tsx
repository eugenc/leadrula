import { format } from "date-fns";
import { errorMessage } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/misc";
import type { AppointmentDaySlot, AppointmentDayWorkingHours, AppointmentFreeSlot } from "@/types";

function formatSlotTime(iso: string) {
  return format(new Date(iso), "h:mm a");
}

function formatHhmmTime(hhmm: string) {
  const [hStr, mStr] = hhmm.split(":");
  const h = Number(hStr);
  const m = Number(mStr);
  const period = h >= 12 ? "PM" : "AM";
  const hour12 = h % 12 || 12;
  return `${hour12}:${String(m).padStart(2, "0")} ${period}`;
}

export function BookDaySlotsColumn({
  selectedDate,
  loading,
  error,
  freeSlots,
  booked,
  workingHours,
  onBookSlot,
  onCustomTime,
}: {
  selectedDate: string;
  loading: boolean;
  error: unknown;
  freeSlots: AppointmentFreeSlot[];
  booked: AppointmentDaySlot[];
  workingHours: AppointmentDayWorkingHours | null;
  onBookSlot: (slot: AppointmentFreeSlot) => void;
  onCustomTime: () => void;
}) {
  const showNoHours =
    !loading && !workingHours && freeSlots.length === 0 && booked.length === 0;

  return (
    <div className="w-48 shrink-0 border-l border-gray-100 pl-3">
      <div className="mb-2 text-sm font-bold text-gray-800">
        {format(new Date(selectedDate + "T12:00:00"), "EEE, MMM d")}
      </div>

      {showNoHours && (
        <p className="mb-3 text-xs text-gray-400">No hours configured for this day.</p>
      )}

      {workingHours && (
        <div className="mb-3">
          <div className="mb-1 text-xs font-semibold uppercase tracking-wide text-gray-400">Hours</div>
          <div className="text-xs text-gray-500">
            {formatHhmmTime(workingHours.start)} – {formatHhmmTime(workingHours.end)}
          </div>
        </div>
      )}

      {booked.length > 0 && (
        <div className="mb-3">
          <div className="mb-1 text-xs font-semibold uppercase tracking-wide text-amber-600">Booked</div>
          <div className="space-y-0.5">
            {booked.map((b) => (
              <div key={b.slot_start} className="text-xs text-amber-700">
                {formatSlotTime(b.slot_start)} · {b.duration_min}m
              </div>
            ))}
          </div>
        </div>
      )}

      {loading ? (
        <Spinner className="h-5 w-5" />
      ) : error ? (
        <p className="text-sm text-red-600">{errorMessage(error)}</p>
      ) : !freeSlots.length ? (
        <p className="text-sm text-gray-400">No free slots.</p>
      ) : (
        <div className="space-y-1">
          <div className="mb-1 text-xs font-semibold uppercase tracking-wide text-jade-700">Available</div>
          {freeSlots.map((s) => (
            <button
              key={s.slot_start}
              type="button"
              onClick={() => onBookSlot(s)}
              className="flex w-full items-center justify-between rounded border border-gray-100 px-2 py-2 text-left text-sm hover:bg-jade-50"
            >
              <span>{formatSlotTime(s.slot_start)}</span>
              <span className="text-xs font-semibold text-jade-700">{s.remaining_capacity} free</span>
            </button>
          ))}
        </div>
      )}

      <Button type="button" variant="outline" size="sm" className="mt-3 w-full" onClick={onCustomTime}>
        Custom time
      </Button>
    </div>
  );
}
