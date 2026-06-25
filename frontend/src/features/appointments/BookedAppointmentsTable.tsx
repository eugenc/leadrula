import { format } from "date-fns";
import { Badge, EmptyState, Spinner } from "@/components/ui/misc";
import { useUIStore } from "@/store/uiStore";
import type { AppointmentBooking } from "@/types";

export function BookedAppointmentsTable({
  items,
  isLoading,
  showBuyer,
  emptyTitle = "No appointments booked yet.",
}: {
  items: AppointmentBooking[];
  isLoading: boolean;
  showBuyer?: boolean;
  emptyTitle?: string;
}) {
  const openDetail = useUIStore((s) => s.openDetail);

  if (isLoading) return <Spinner className="h-6 w-6" />;
  if (!items.length) return <EmptyState title={emptyTitle} />;

  return (
    <div className="overflow-x-auto rounded-md border border-gray-100">
      <table className="w-full text-sm">
        <thead className="bg-gray-50 text-left text-xs font-semibold uppercase text-gray-400">
          <tr>
            <th className="px-3 py-2">Booked</th>
            <th className="px-3 py-2">Appointment</th>
            <th className="px-3 py-2">Lead</th>
            <th className="px-3 py-2">Phone</th>
            {showBuyer ? <th className="px-3 py-2">Buyer</th> : <th className="px-3 py-2">Publisher</th>}
            <th className="px-3 py-2">Contract</th>
            <th className="px-3 py-2">Delivery</th>
          </tr>
        </thead>
        <tbody>
          {items.map((row) => (
            <tr
              key={row.id}
              className="cursor-pointer border-t border-gray-50 hover:bg-jade-50/50"
              onClick={() => openDetail(row.lead_id)}
            >
              <td className="px-3 py-2 text-gray-500">{format(new Date(row.created_at), "MMM d, h:mm a")}</td>
              <td className="px-3 py-2 font-medium">{format(new Date(row.slot_start), "MMM d, h:mm a")}</td>
              <td className="px-3 py-2">{row.lead_name || "—"}</td>
              <td className="px-3 py-2">{row.phone || row.email || "—"}</td>
              <td className="px-3 py-2">{showBuyer ? row.buyer_name : row.publisher_name}</td>
              <td className="px-3 py-2">{row.contract_name}</td>
              <td className="px-3 py-2">
                <Badge variant={row.delivery_status === "delivered" ? "distributed" : "pending"}>
                  {row.delivery_status}
                </Badge>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
