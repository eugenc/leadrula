import { format } from "date-fns";
import { EmptyState, Spinner } from "@/components/ui/misc";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { useUIStore } from "@/store/uiStore";
import type { AppointmentBooking } from "@/types";

export function formatAppointmentTime(iso: string | null | undefined, timeZone: string): string {
  if (!iso) return "—";
  try {
    return new Intl.DateTimeFormat("en-US", {
      timeZone,
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit",
    }).format(new Date(iso));
  } catch {
    return format(new Date(iso), "MMM d, h:mm a");
  }
}

export function BookedAppointmentsTable({
  items,
  isLoading,
  showBuyer,
  timeZone = "UTC",
  emptyTitle = "No appointments booked yet.",
}: {
  items: AppointmentBooking[];
  isLoading: boolean;
  showBuyer?: boolean;
  timeZone?: string;
  emptyTitle?: string;
}) {
  const openDetail = useUIStore((s) => s.openDetail);

  if (isLoading) {
    return (
      <div className="flex justify-center py-16">
        <Spinner className="h-6 w-6" />
      </div>
    );
  }
  if (!items.length) return <EmptyState title={emptyTitle} />;

  return (
    <Table>
      <THead>
        <tr>
          <TH>Booked</TH>
          <TH>Appointment</TH>
          <TH>Lead</TH>
          <TH>Phone</TH>
          <TH>{showBuyer ? "Buyer" : "Publisher"}</TH>
          <TH>Contract</TH>
        </tr>
      </THead>
      <TBody>
        {items.map((row) => (
          <TR key={row.id} onClick={() => openDetail(row.lead_id)}>
            <TD className="text-gray-500">{formatAppointmentTime(row.booked_at, timeZone)}</TD>
            <TD className="font-medium text-gray-800">
              {formatAppointmentTime(row.appointment_at, timeZone)}
            </TD>
            <TD>{row.lead_name || "—"}</TD>
            <TD>{row.phone || row.email || "—"}</TD>
            <TD>{showBuyer ? row.buyer_name : row.publisher_name}</TD>
            <TD>{row.contract_name}</TD>
          </TR>
        ))}
      </TBody>
    </Table>
  );
}
