import { type ReactNode } from "react";
import { CalendarClock, Phone, Building2, FileText, CalendarDays } from "lucide-react";
import { cn } from "@/lib/utils";
import type { AppointmentBooking } from "@/types";
import { formatAppointmentTime } from "./BookedAppointmentsTable";

function FieldLine({
  icon: Icon,
  children,
  className,
}: {
  icon: typeof Phone;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex items-center gap-2 leading-tight", className)}>
      <span className="flex w-4 shrink-0 justify-center">
        <Icon className="h-3.5 w-3.5 text-gray-300" aria-hidden />
      </span>
      <span className="min-w-0 truncate">{children}</span>
    </div>
  );
}

export function AppointmentListCard({
  row,
  timeZone,
  onOpen,
}: {
  row: AppointmentBooking;
  timeZone: string;
  onOpen: () => void;
}) {
  const contact = row.phone || row.email || "—";

  return (
    <button
      type="button"
      className="w-full rounded-lg border border-gray-100 bg-surface-card px-4 py-3 text-left"
      onClick={onOpen}
    >
      <div className="flex items-start justify-between gap-2">
        <span className="font-medium text-gray-800">{row.lead_name || "—"}</span>
        <span className="shrink-0 text-sm font-medium text-gray-800">
          {formatAppointmentTime(row.appointment_at, timeZone)}
        </span>
      </div>
      <div className="mt-1.5 space-y-0.5">
        <FieldLine icon={CalendarClock} className="text-xs text-gray-400">
          Booked {formatAppointmentTime(row.booked_at, timeZone)}
        </FieldLine>
        {contact !== "—" && (
          <FieldLine icon={Phone} className="text-sm text-gray-500">
            {contact}
          </FieldLine>
        )}
        {row.publisher_name && (
          <FieldLine icon={Building2} className="text-sm text-gray-500">
            {row.publisher_name}
          </FieldLine>
        )}
        {row.calendar_name && (
          <FieldLine icon={CalendarDays} className="text-sm text-gray-500">
            {row.calendar_name}
          </FieldLine>
        )}
        {row.contract_name && (
          <FieldLine icon={FileText} className="text-xs text-gray-400">
            {row.contract_name}
          </FieldLine>
        )}
      </div>
    </button>
  );
}
