import { format } from "date-fns";
import { Table, TBody, TD, TH, THead, TR } from "@/components/ui/table";
import type { BuyerBookingCalendar } from "@/types";

type Props = {
  calendars: BuyerBookingCalendar[];
  ownerName: string;
  showUpdated?: boolean;
  onOpen: (id: number) => void;
};

export function BookingCalendarsTable({ calendars, ownerName, showUpdated, onOpen }: Props) {
  const owner = ownerName.trim() || "—";

  return (
    <Table>
      <THead>
        <TR>
          <TH>Name</TH>
          <TH>Owner</TH>
          <TH>Timezone</TH>
          <TH>Slots</TH>
          <TH>Status</TH>
          {showUpdated && <TH>Updated</TH>}
        </TR>
      </THead>
      <TBody>
        {calendars.map((c) => (
          <TR key={c.id} className="cursor-pointer hover:bg-gray-50" onClick={() => onOpen(c.id)}>
            <TD className="font-medium text-gray-800">{c.name}</TD>
            <TD>{owner}</TD>
            <TD>{c.timezone}</TD>
            <TD>{c.slot_count}</TD>
            <TD>{c.configured ? "Ready" : "Setup needed"}</TD>
            {showUpdated && (
              <TD className="text-gray-500">{format(new Date(c.updated_at), "MMM d, yyyy")}</TD>
            )}
          </TR>
        ))}
      </TBody>
    </Table>
  );
}
