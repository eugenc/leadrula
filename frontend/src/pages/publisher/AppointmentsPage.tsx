import { useState } from "react";
import { Plus } from "lucide-react";
import { PageBody } from "@/components/layout/PageBody";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";
import { EmptyState, Spinner } from "@/components/ui/misc";
import { BookedAppointmentsTable } from "@/features/appointments/BookedAppointmentsTable";
import { PublisherBookAppointmentSheet } from "@/features/appointments/PublisherBookAppointmentSheet";
import { usePublisherAppointmentContracts, usePublisherBookings, usePublisherCalendars } from "@/features/appointments/hooks";

export function PublisherAppointmentsPage() {
  const { data: booked = [], isLoading: loadingBooked } = usePublisherBookings();
  const { data: contracts = [] } = usePublisherAppointmentContracts();
  const { data: calendars = [] } = usePublisherCalendars();
  const [bookOpen, setBookOpen] = useState(false);

  const canBook =
    calendars.some((c) => c.configured) || contracts.some((c) => c.configured);
  const canBookHint = canBook
    ? undefined
    : "Create a calendar under Calendars and add availability slots, or attach a calendar to a contract.";

  function handleBooked() {
    setBookOpen(false);
  }

  return (
    <>
      <PageHeader
        title="Appointments"
        action={
          <Button
            type="button"
            size="sm"
            disabled={!canBook}
            title={canBookHint}
            onClick={() => setBookOpen(true)}
          >
            <Plus className="h-4 w-4" />
            Add appointment
          </Button>
        }
      />
      <PageBody>
        {loadingBooked ? (
          <div className="flex justify-center py-16">
            <Spinner className="h-6 w-6" />
          </div>
        ) : booked.length === 0 ? (
          <EmptyState
            title="No appointments booked yet."
            subtitle={canBook ? "Book your first appointment from an available slot." : undefined}
            action={
              canBook ? (
                <Button type="button" size="sm" onClick={() => setBookOpen(true)}>
                  <Plus className="h-4 w-4" />
                  Add appointment
                </Button>
              ) : undefined
            }
          />
        ) : (
          <BookedAppointmentsTable items={booked} isLoading={false} showBuyer />
        )}
      </PageBody>

      <PublisherBookAppointmentSheet
        open={bookOpen}
        onClose={() => setBookOpen(false)}
        onBooked={handleBooked}
      />
    </>
  );
}
