import { PageBody } from "@/components/layout/PageBody";
import { PageHeader } from "@/components/layout/PageHeader";
import { BookedAppointmentsTable } from "@/features/appointments/BookedAppointmentsTable";
import { usePublisherBookings } from "@/features/appointments/hooks";

export function PublisherAppointmentsPage() {
  const { data: booked = [], isLoading: loadingBooked } = usePublisherBookings();

  return (
    <>
      <PageHeader title="Appointments" />
      <PageBody>
        <BookedAppointmentsTable
          items={booked}
          isLoading={loadingBooked}
          showBuyer
          emptyTitle="No appointments booked yet."
        />
      </PageBody>
    </>
  );
}
