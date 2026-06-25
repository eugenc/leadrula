import { useMemo } from "react";
import { PageBody } from "@/components/layout/PageBody";
import { PageHeader } from "@/components/layout/PageHeader";
import { BookedAppointmentsTable } from "@/features/appointments/BookedAppointmentsTable";
import { useBuyerBookings } from "@/features/appointments/hooks";

export function BuyerAppointmentsPage() {
  const { data: booked = [], isLoading: loadingBooked } = useBuyerBookings();

  const delivered = useMemo(
    () => booked.filter((b) => b.delivery_status === "delivered"),
    [booked]
  );

  return (
    <>
      <PageHeader title="Appointments" />
      <PageBody>
        <BookedAppointmentsTable
          items={delivered}
          isLoading={loadingBooked}
          emptyTitle="No distributed appointments yet."
        />
      </PageBody>
    </>
  );
}
