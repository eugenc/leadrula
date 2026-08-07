import { useEffect, useState } from "react";
import { Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { Button } from "@/components/ui/button";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { useBuyerCalendars, useSetContractAppointmentCalendar } from "@/features/appointments/hooks";

export function BuyerContractCalendarSection({
  contractId,
  appointmentCalendarId,
}: {
  contractId: number;
  appointmentCalendarId?: number | null;
}) {
  const { data: calendars = [], isLoading } = useBuyerCalendars();
  const save = useSetContractAppointmentCalendar();
  const [calendarId, setCalendarId] = useState(appointmentCalendarId ?? 0);

  useEffect(() => {
    setCalendarId(appointmentCalendarId ?? 0);
  }, [appointmentCalendarId]);

  if (isLoading) return <p className="text-sm text-gray-400">Loading calendars…</p>;
  if (!calendars.length) {
    return (
      <p className="text-sm text-amber-700">
        Create a booking calendar under Calendars before attaching one to this contract.
      </p>
    );
  }

  function submit() {
    if (!calendarId) {
      toast.error("Select a calendar");
      return;
    }
    save.mutate(
      { contractId, appointment_calendar_id: calendarId },
      {
        onSuccess: () => toast.success("Calendar saved"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  return (
    <div className="mt-4 space-y-3 border-t border-gray-100 pt-4">
      <SectionLabel>Booking calendar</SectionLabel>
      <p className="text-xs text-gray-400">
        Your calendar — you can book immediately once this contract is active.
      </p>
      <div className="flex flex-wrap items-end gap-2">
        <div className="min-w-[12rem] flex-1">
          <Label>Calendar</Label>
          <Select value={calendarId || ""} onChange={(e) => setCalendarId(Number(e.target.value))}>
            <option value="">Select calendar…</option>
            {calendars.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name} {c.configured ? "" : "(setup needed)"}
              </option>
            ))}
          </Select>
        </div>
        <Button type="button" onClick={submit} disabled={save.isPending}>
          Save calendar
        </Button>
      </div>
    </div>
  );
}
