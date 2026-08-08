import { useEffect, useState } from "react";
import { Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { Button } from "@/components/ui/button";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  usePublisherCalendars,
  useSetContractPublisherAppointmentCalendar,
} from "@/features/appointments/hooks";

export function PublisherContractCalendarSection({
  contractId,
  publisherAppointmentCalendarId,
  standalone = false,
}: {
  contractId: number;
  publisherAppointmentCalendarId?: number | null;
  standalone?: boolean;
}) {
  const { data: calendars = [], isLoading } = usePublisherCalendars();
  const save = useSetContractPublisherAppointmentCalendar();
  const [calendarId, setCalendarId] = useState(publisherAppointmentCalendarId ?? 0);

  useEffect(() => {
    setCalendarId(publisherAppointmentCalendarId ?? 0);
  }, [publisherAppointmentCalendarId]);

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
      { contractId, publisher_appointment_calendar_id: calendarId },
      {
        onSuccess: () => toast.success("Calendar attached"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  return (
    <div className={standalone ? "space-y-3" : "mt-4 space-y-3 border-t border-gray-100 pt-4"}>
      <SectionLabel>Booking Calendar</SectionLabel>
      <p className="text-xs text-gray-400">
        Your calendar — you can book immediately; the buyer sees it after they accept the contract.
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
        <Button type="button" size="sm" onClick={submit} disabled={save.isPending}>
          Attach calendar
        </Button>
      </div>
    </div>
  );
}
