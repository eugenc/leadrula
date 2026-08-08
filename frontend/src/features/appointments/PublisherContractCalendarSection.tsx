import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  usePublisherCalendars,
  useSetContractPublisherAppointmentCalendar,
} from "@/features/appointments/hooks";

export type BookingSectionSave = {
  isDirty: () => boolean;
  flush: () => Promise<boolean>;
};

export function PublisherContractCalendarSection({
  contractId,
  publisherAppointmentCalendarId,
  standalone = false,
  registerSave,
  onDirtyChange,
}: {
  contractId: number;
  publisherAppointmentCalendarId?: number | null;
  standalone?: boolean;
  registerSave?: (api: BookingSectionSave | null) => void;
  onDirtyChange?: (dirty: boolean) => void;
}) {
  const { data: calendars = [], isLoading } = usePublisherCalendars();
  const save = useSetContractPublisherAppointmentCalendar();
  const [calendarId, setCalendarId] = useState(publisherAppointmentCalendarId ?? 0);
  const savedId = publisherAppointmentCalendarId ?? 0;

  const calendarIdRef = useRef(calendarId);
  calendarIdRef.current = calendarId;
  const savedIdRef = useRef(savedId);
  savedIdRef.current = savedId;

  useEffect(() => {
    setCalendarId(publisherAppointmentCalendarId ?? 0);
  }, [publisherAppointmentCalendarId]);

  const isDirty = useCallback(
    () => calendarIdRef.current !== savedIdRef.current,
    []
  );

  const flush = useCallback(async (): Promise<boolean> => {
    if (!isDirty()) return true;
    const id = calendarIdRef.current;
    if (!id) {
      toast.error("Select a calendar");
      return false;
    }
    try {
      await save.mutateAsync({ contractId, publisher_appointment_calendar_id: id });
      return true;
    } catch (e) {
      toast.error(errorMessage(e));
      return false;
    }
  }, [contractId, isDirty, save]);

  useLayoutEffect(() => {
    registerSave?.({ isDirty, flush });
    return () => registerSave?.(null);
  }, [registerSave, isDirty, flush]);

  useEffect(() => {
    onDirtyChange?.(calendarId !== savedId);
  }, [calendarId, savedId, onDirtyChange]);

  if (isLoading) return <p className="text-sm text-gray-400">Loading calendars…</p>;
  if (!calendars.length) {
    return (
      <p className="text-sm text-amber-700">
        Create a booking calendar under Calendars before attaching one to this contract.
      </p>
    );
  }

  return (
    <div className={standalone ? "space-y-3" : "mt-4 space-y-3 border-t border-gray-100 pt-4"}>
      <SectionLabel>Booking Calendar</SectionLabel>
      <p className="text-xs text-gray-400">
        Your calendar — you can book immediately; the buyer sees it after they accept the contract.
      </p>
      <div>
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
    </div>
  );
}
