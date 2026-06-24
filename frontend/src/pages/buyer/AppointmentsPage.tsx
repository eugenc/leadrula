import { useState } from "react";
import { PageBody } from "@/components/layout/PageBody";
import { PageHeader } from "@/components/layout/PageHeader";
import { Spinner } from "@/components/ui/misc";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/store/authStore";
import {
  BuyerAvailabilityEditor,
  BuyerSetupWizard,
} from "@/features/appointments/BuyerAvailabilityEditor";
import { BookedAppointmentsTable } from "@/features/appointments/BookedAppointmentsTable";
import { useBuyerAvailability, useBuyerBookings } from "@/features/appointments/hooks";

export function BuyerAppointmentsPage() {
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";
  const [tab, setTab] = useState<"booked" | "calendar">(isAdmin ? "calendar" : "booked");
  const { data: availability, isLoading } = useBuyerAvailability();
  const { data: booked = [], isLoading: loadingBooked } = useBuyerBookings();
  const [wizardDone, setWizardDone] = useState(false);

  const showWizard = isAdmin && availability && !availability.configured && !wizardDone;

  return (
    <>
      <PageHeader
        title="Appointments"
        action={
          <div className="flex overflow-hidden rounded-md border border-gray-200">
            {(isAdmin ? (["calendar", "booked"] as const) : (["booked"] as const)).map((t) => (
              <button
                key={t}
                type="button"
                onClick={() => setTab(t)}
                className={cn(
                  "px-3 py-1.5 text-sm font-semibold capitalize",
                  tab === t ? "bg-jade-500 text-white" : "text-gray-700 hover:bg-gray-100"
                )}
              >
                {t === "calendar" ? "Calendar" : "Booked"}
              </button>
            ))}
          </div>
        }
      />
      <PageBody>
        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : showWizard ? (
          <BuyerSetupWizard onComplete={() => setWizardDone(true)} />
        ) : tab === "booked" ? (
          <BookedAppointmentsTable items={booked} isLoading={loadingBooked} />
        ) : availability ? (
          <BuyerAvailabilityEditor availability={availability} readOnly={!isAdmin} />
        ) : null}
      </PageBody>
    </>
  );
}
