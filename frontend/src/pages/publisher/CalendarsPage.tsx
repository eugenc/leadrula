import { useState } from "react";
import { PageBody } from "@/components/layout/PageBody";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";
import { DrawerBody, Sheet } from "@/components/ui/dialog";
import { EmptyState, Spinner } from "@/components/ui/misc";
import { ActionAppointmentsManage, canAction } from "@/lib/permissions";
import { errorMessage } from "@/lib/api";
import { useAuthStore } from "@/store/authStore";
import { toast } from "@/store/toastStore";
import {
  AVAILABILITY_DRAWER_WIDTH,
  BuyerAvailabilityEditor,
  BuyerSetupWizard,
} from "@/features/appointments/BuyerAvailabilityEditor";
import { BookingCalendarsTable } from "@/features/appointments/BookingCalendarsTable";
import { useDeletePublisherBookingCalendar, usePublisherCalendars } from "@/features/appointments/hooks";
import { DeletePipelineResourceConfirmDialog } from "@/features/pipelines/DeletePipelineResourceConfirmDialog";
import type { PublisherBookingCalendar } from "@/types";

export function PublisherCalendarsPage() {
  const user = useAuthStore((s) => s.user);
  const canManageCalendars = canAction(user, ActionAppointmentsManage);
  const { data: calendars = [], isLoading } = usePublisherCalendars();
  const remove = useDeletePublisherBookingCalendar();
  const [drawerCalendarId, setDrawerCalendarId] = useState<number | null>(null);
  const [calendarToDelete, setCalendarToDelete] = useState<PublisherBookingCalendar | null>(null);
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
            onDelete={setCalendarToDelete}
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

      <DeletePipelineResourceConfirmDialog
        open={calendarToDelete != null}
        onClose={() => setCalendarToDelete(null)}
        title="Delete calendar?"
        subtitle={
          calendarToDelete
            ? `"${calendarToDelete.name}" will be permanently removed. This cannot be undone.`
            : ""
        }
        loading={remove.isPending}
        onConfirm={() => {
          if (!calendarToDelete) return;
          remove.mutate(calendarToDelete.id, {
            onSuccess: () => {
              toast.success("Calendar deleted");
              setCalendarToDelete(null);
              if (drawerCalendarId === calendarToDelete.id) setDrawerCalendarId(null);
            },
            onError: (err) => toast.error(errorMessage(err)),
          });
        }}
      />
    </>
  );
}

function PublisherCalendarsList({
  calendars,
  ownerName,
  onOpen,
  onAdd,
  onDelete,
}: {
  calendars: PublisherBookingCalendar[];
  ownerName: string;
  onOpen: (id: number) => void;
  onAdd: () => void;
  onDelete: (calendar: PublisherBookingCalendar) => void;
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

  return (
    <BookingCalendarsTable
      calendars={calendars}
      ownerName={ownerName}
      canManage
      onOpen={onOpen}
      onDelete={onDelete}
    />
  );
}
