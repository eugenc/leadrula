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
import { useBuyerCalendars, useDeleteBookingCalendar } from "@/features/appointments/hooks";
import { DeletePipelineResourceConfirmDialog } from "@/features/pipelines/DeletePipelineResourceConfirmDialog";
import type { BuyerBookingCalendar } from "@/types";

export function BuyerCalendarsPage() {
  const user = useAuthStore((s) => s.user);
  const canManageCalendars = canAction(user, ActionAppointmentsManage);
  const { data: calendars = [], isLoading } = useBuyerCalendars();
  const remove = useDeleteBookingCalendar();
  const [drawerCalendarId, setDrawerCalendarId] = useState<number | null>(null);
  const [calendarToDelete, setCalendarToDelete] = useState<BuyerBookingCalendar | null>(null);
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
        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : (
          <CalendarsList
            calendars={calendars}
            ownerName={user?.account_name ?? ""}
            canManageCalendars={canManageCalendars}
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
      />

      <Sheet open={drawerCalendarId !== null} onClose={() => setDrawerCalendarId(null)} width={AVAILABILITY_DRAWER_WIDTH}>
        {drawerCalendarId && (
          <DrawerBody>
            <BuyerAvailabilityEditor calendarId={drawerCalendarId} readOnly={!canManageCalendars} />
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

function CalendarsList({
  calendars,
  ownerName,
  canManageCalendars,
  onOpen,
  onAdd,
  onDelete,
}: {
  calendars: BuyerBookingCalendar[];
  ownerName: string;
  canManageCalendars: boolean;
  onOpen: (id: number) => void;
  onAdd: () => void;
  onDelete: (calendar: BuyerBookingCalendar) => void;
}) {
  if (!calendars.length) {
    return (
      <EmptyState
        title="No calendars yet."
        subtitle={canManageCalendars ? "Create your first booking calendar." : undefined}
        action={
          canManageCalendars ? (
            <Button type="button" onClick={onAdd}>
              Add calendar
            </Button>
          ) : undefined
        }
      />
    );
  }

  return (
    <BookingCalendarsTable
      calendars={calendars}
      ownerName={ownerName}
      showUpdated
      canManage={canManageCalendars}
      onOpen={onOpen}
      onDelete={onDelete}
    />
  );
}
