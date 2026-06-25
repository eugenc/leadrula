import { useState } from "react";
import { format } from "date-fns";
import { PageBody } from "@/components/layout/PageBody";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";
import { DrawerBody, Sheet } from "@/components/ui/dialog";
import { EmptyState, Spinner } from "@/components/ui/misc";
import { Table, TBody, TD, TH, THead, TR } from "@/components/ui/table";
import { useAuthStore } from "@/store/authStore";
import {
  AVAILABILITY_DRAWER_WIDTH,
  BuyerAvailabilityEditor,
  BuyerSetupWizard,
} from "@/features/appointments/BuyerAvailabilityEditor";
import { useBuyerCalendars } from "@/features/appointments/hooks";
import type { BuyerBookingCalendar } from "@/types";

export function BuyerCalendarsPage() {
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";
  const { data: calendars = [], isLoading } = useBuyerCalendars();
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
        title="Calendars"
        action={
          isAdmin ? (
            <Button type="button" onClick={openWizard}>
              Add calendar
            </Button>
          ) : undefined
        }
      />
      <PageBody>
        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : (
          <CalendarsList
            calendars={calendars}
            isAdmin={isAdmin}
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
      />

      <Sheet open={drawerCalendarId !== null} onClose={() => setDrawerCalendarId(null)} width={AVAILABILITY_DRAWER_WIDTH}>
        {drawerCalendarId && (
          <DrawerBody>
            <h2 className="mb-4 text-lg font-bold text-gray-800">
              {calendars.find((c) => c.id === drawerCalendarId)?.name ?? "Calendar"}
            </h2>
            <BuyerAvailabilityEditor calendarId={drawerCalendarId} readOnly={!isAdmin} />
          </DrawerBody>
        )}
      </Sheet>
    </>
  );
}

function CalendarsList({
  calendars,
  isAdmin,
  onOpen,
  onAdd,
}: {
  calendars: BuyerBookingCalendar[];
  isAdmin: boolean;
  onOpen: (id: number) => void;
  onAdd: () => void;
}) {
  if (!calendars.length) {
    return (
      <EmptyState
        title="No calendars yet."
        subtitle={isAdmin ? "Create your first booking calendar." : undefined}
        action={
          isAdmin ? (
            <Button type="button" onClick={onAdd}>
              Add calendar
            </Button>
          ) : undefined
        }
      />
    );
  }

  return (
    <Table>
      <THead>
        <tr>
          <TH>Name</TH>
          <TH>Timezone</TH>
          <TH>Slots</TH>
          <TH>Status</TH>
          <TH>Updated</TH>
        </tr>
      </THead>
      <TBody>
        {calendars.map((c) => (
          <TR key={c.id} onClick={() => onOpen(c.id)}>
            <TD className="font-medium text-gray-800">{c.name}</TD>
            <TD>{c.timezone}</TD>
            <TD>{c.slot_count}</TD>
            <TD>{c.configured ? "Ready" : "Setup needed"}</TD>
            <TD className="text-gray-500">{format(new Date(c.updated_at), "MMM d, yyyy")}</TD>
          </TR>
        ))}
      </TBody>
    </Table>
  );
}
