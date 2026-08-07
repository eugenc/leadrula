import { useState } from "react";
import { PageBody } from "@/components/layout/PageBody";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";
import { DrawerBody, Sheet } from "@/components/ui/dialog";
import { EmptyState, Spinner } from "@/components/ui/misc";
import { ActionAppointmentsManage, canAction } from "@/lib/permissions";
import { useAuthStore } from "@/store/authStore";
import {
  AVAILABILITY_DRAWER_WIDTH,
  BuyerAvailabilityEditor,
  BuyerSetupWizard,
} from "@/features/appointments/BuyerAvailabilityEditor";
import { BookingCalendarsTable } from "@/features/appointments/BookingCalendarsTable";
import { usePublisherCalendars } from "@/features/appointments/hooks";
import type { PublisherBookingCalendar } from "@/types";

export function PublisherCalendarsPage() {
  const user = useAuthStore((s) => s.user);
  const canManageCalendars = canAction(user, ActionAppointmentsManage);
  const { data: calendars = [], isLoading } = usePublisherCalendars();
  const [drawerCalendarId, setDrawerCalendarId] = useState<number | null>(null);
  const [showWizard, setShowWizard] = useState(false);
  const [wizardKey, setWizardKey] = useState(0);

  function openWizard() {
    setWizardKey((k) => k + 1);
    setShowWizard(true);
  }

  return (
    <>
      <PageHeader
        action={
          canManageCalendars ? (
            <Button type="button" onClick={openWizard}>
              Add calendar
            </Button>
          ) : undefined
        }
      />
      <PageBody>
        {!canManageCalendars ? (
          <EmptyState
            title="Calendar management requires permission"
            subtitle='Ask an admin to grant "Manage calendars & booking slots".'
          />
        ) : isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : (
          <PublisherCalendarsList
            calendars={calendars}
            ownerName={user?.account_name ?? ""}
            onOpen={(id) => setDrawerCalendarId(id)}
            onAdd={openWizard}
          />
        )}
      </PageBody>

      <BuyerSetupWizard
        key={wizardKey}
        open={showWizard}
        onOpenChange={setShowWizard}
        onComplete={(id) => setDrawerCalendarId(id)}
        owner="publisher"
      />

      <Sheet open={drawerCalendarId !== null} onClose={() => setDrawerCalendarId(null)} width={AVAILABILITY_DRAWER_WIDTH}>
        {drawerCalendarId && (
          <DrawerBody>
            <BuyerAvailabilityEditor calendarId={drawerCalendarId} owner="publisher" />
          </DrawerBody>
        )}
      </Sheet>
    </>
  );
}

function PublisherCalendarsList({
  calendars,
  ownerName,
  onOpen,
  onAdd,
}: {
  calendars: PublisherBookingCalendar[];
  ownerName: string;
  onOpen: (id: number) => void;
  onAdd: () => void;
}) {
  if (!calendars.length) {
    return (
      <EmptyState
        title="No calendars yet"
        action={
          <Button type="button" onClick={onAdd}>
            Add calendar
          </Button>
        }
      />
    );
  }

  return <BookingCalendarsTable calendars={calendars} ownerName={ownerName} onOpen={onOpen} />;
}
