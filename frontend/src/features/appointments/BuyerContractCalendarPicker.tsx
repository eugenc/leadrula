import { useEffect, useState } from "react";
import { Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { Button } from "@/components/ui/button";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  useBuyerCalendars,
  useSetContractAppointmentCalendarSource,
} from "@/features/appointments/hooks";
import { BuyerPublisherSlotSection } from "@/features/appointments/BuyerPublisherSlotSection";
import type { Contract } from "@/types";

export function BuyerContractCalendarPicker({ contract }: { contract: Contract }) {
  const { data: calendars = [], isLoading } = useBuyerCalendars();
  const saveSource = useSetContractAppointmentCalendarSource();
  const hasPublisherCalendar = (contract.publisher_appointment_calendar_id ?? 0) > 0;
  const currentSource = contract.appointment_calendar_source ?? "";
  const [source, setSource] = useState<"" | "buyer" | "publisher">(
    currentSource === "buyer" || currentSource === "publisher" ? currentSource : ""
  );
  const [calendarId, setCalendarId] = useState(contract.appointment_calendar_id ?? 0);

  useEffect(() => {
    setSource(
      contract.appointment_calendar_source === "buyer" || contract.appointment_calendar_source === "publisher"
        ? contract.appointment_calendar_source
        : ""
    );
    setCalendarId(contract.appointment_calendar_id ?? 0);
  }, [contract.id, contract.appointment_calendar_source, contract.appointment_calendar_id]);

  if (isLoading) return <p className="text-sm text-gray-400">Loading calendars…</p>;

  function submitSource() {
    if (!source) {
      toast.error("Select which calendar to use");
      return;
    }
    if (source === "buyer" && !calendarId) {
      toast.error("Select your calendar");
      return;
    }
    saveSource.mutate(
      {
        contractId: contract.id,
        source,
        appointment_calendar_id: source === "buyer" ? calendarId : undefined,
      },
      {
        onSuccess: () => toast.success("Calendar preference saved"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  return (
    <div className="mt-4 space-y-3 border-t border-gray-100 pt-4">
      <SectionLabel>Booking calendar</SectionLabel>
      <p className="text-xs text-gray-400">
        Choose whether appointments use your calendar or the publisher&apos;s calendar attached to this contract.
      </p>

      <div className="space-y-2">
        <label className="flex items-center gap-2 text-sm">
          <input
            type="radio"
            name={`calendar-source-${contract.id}`}
            checked={source === "publisher"}
            disabled={!hasPublisherCalendar}
            onChange={() => setSource("publisher")}
          />
          Use publisher calendar
          {!hasPublisherCalendar && (
            <span className="text-xs text-amber-700">(publisher has not attached one)</span>
          )}
        </label>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="radio"
            name={`calendar-source-${contract.id}`}
            checked={source === "buyer"}
            onChange={() => setSource("buyer")}
          />
          Use my calendar
        </label>
      </div>

      {source === "buyer" && (
        <div className="min-w-[12rem]">
          <Label>My calendar</Label>
          {!calendars.length ? (
            <p className="text-sm text-amber-700">Create a calendar under Calendars first.</p>
          ) : (
            <Select value={calendarId || ""} onChange={(e) => setCalendarId(Number(e.target.value))}>
              <option value="">Select calendar…</option>
              {calendars.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name} {c.configured ? "" : "(setup needed)"}
                </option>
              ))}
            </Select>
          )}
        </div>
      )}

      <Button type="button" size="sm" onClick={submitSource} disabled={saveSource.isPending}>
        Save calendar choice
      </Button>

      {contract.appointment_calendar_source === "publisher" && hasPublisherCalendar && (
        <BuyerPublisherSlotSection contractId={contract.id} />
      )}
    </div>
  );
}
