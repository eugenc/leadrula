import { useEffect, useMemo, useState } from "react";
import {
  addDays,
  addMonths,
  endOfMonth,
  endOfWeek,
  format,
  isSameDay,
  isSameMonth,
  startOfMonth,
  startOfWeek,
} from "date-fns";
import { PageBody } from "@/components/layout/PageBody";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";
import { DrawerBody, Sheet } from "@/components/ui/dialog";
import { EmptyState, Spinner } from "@/components/ui/misc";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/store/authStore";
import {
  BuyerAvailabilityEditor,
  BuyerSetupWizard,
} from "@/features/appointments/BuyerAvailabilityEditor";
import {
  useBuyerCalendarBookings,
  useBuyerCalendarMarkers,
  useBuyerCalendars,
} from "@/features/appointments/hooks";
import type { AppointmentBooking, BuyerBookingCalendar } from "@/types";

export function BuyerCalendarsPage() {
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";
  const [tab, setTab] = useState<"overview" | "calendars">("overview");
  const { data: calendars = [], isLoading } = useBuyerCalendars();
  const [selectedCalendarId, setSelectedCalendarId] = useState<number | null>(null);
  const [drawerCalendarId, setDrawerCalendarId] = useState<number | null>(null);
  const [showWizard, setShowWizard] = useState(false);

  useEffect(() => {
    if (!calendars.length) {
      if (selectedCalendarId !== null) setSelectedCalendarId(null);
      return;
    }
    if (!calendars.some((c) => c.id === selectedCalendarId)) {
      setSelectedCalendarId(calendars[0].id);
    }
  }, [calendars, selectedCalendarId]);

  const selectedCalendar = useMemo(
    () => calendars.find((c) => c.id === selectedCalendarId) ?? null,
    [calendars, selectedCalendarId]
  );

  return (
    <>
      <PageHeader
        title="Calendars"
        action={
          <div className="flex items-center gap-2">
            {isAdmin && tab === "calendars" && (
              <Button type="button" onClick={() => setShowWizard(true)}>
                Add calendar
              </Button>
            )}
            <div className="flex overflow-hidden rounded-md border border-gray-200">
              {(["overview", "calendars"] as const).map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setTab(t)}
                  className={cn(
                    "px-3 py-1.5 text-sm font-semibold capitalize",
                    tab === t ? "bg-jade-500 text-white" : "text-gray-700 hover:bg-surface-card"
                  )}
                >
                  {t}
                </button>
              ))}
            </div>
          </div>
        }
      />
      <PageBody>
        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : showWizard ? (
          <BuyerSetupWizard
            onComplete={(id) => {
              setShowWizard(false);
              setSelectedCalendarId(id);
              setDrawerCalendarId(id);
              setTab("calendars");
            }}
          />
        ) : tab === "overview" ? (
          <OverviewTab
            calendars={calendars}
            selectedCalendarId={selectedCalendarId}
            onSelectCalendar={setSelectedCalendarId}
            selectedCalendar={selectedCalendar}
          />
        ) : (
          <CalendarsTab
            calendars={calendars}
            isAdmin={isAdmin}
            onOpen={(id) => setDrawerCalendarId(id)}
            onAdd={() => setShowWizard(true)}
          />
        )}
      </PageBody>

      <Sheet open={drawerCalendarId !== null} onClose={() => setDrawerCalendarId(null)} width={640}>
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

function CalendarSelect({
  calendars,
  value,
  onChange,
}: {
  calendars: BuyerBookingCalendar[];
  value: number | null;
  onChange: (id: number) => void;
}) {
  return (
    <select
      className="rounded border border-gray-200 px-2 py-1.5 text-sm"
      value={value ?? ""}
      onChange={(e) => onChange(Number(e.target.value))}
    >
      {calendars.map((c) => (
        <option key={c.id} value={c.id}>
          {c.name}
        </option>
      ))}
    </select>
  );
}

function OverviewTab({
  calendars,
  selectedCalendarId,
  onSelectCalendar,
  selectedCalendar,
}: {
  calendars: BuyerBookingCalendar[];
  selectedCalendarId: number | null;
  onSelectCalendar: (id: number) => void;
  selectedCalendar: BuyerBookingCalendar | null;
}) {
  const [month, setMonth] = useState(() => startOfMonth(new Date()));
  const [selectedDate, setSelectedDate] = useState(() => format(new Date(), "yyyy-MM-dd"));
  const [view, setView] = useState<"grid" | "list">("grid");

  const monthStart = format(startOfMonth(month), "yyyy-MM-dd");
  const monthEnd = format(endOfMonth(month), "yyyy-MM-dd");
  const { data: markers = [] } = useBuyerCalendarMarkers(selectedCalendarId, monthStart, monthEnd);
  const { data: bookings = [], isLoading } = useBuyerCalendarBookings(selectedCalendarId);

  const calendarDays = useMemo(() => {
    const start = startOfWeek(startOfMonth(month));
    const end = endOfWeek(endOfMonth(month));
    const days: Date[] = [];
    for (let d = start; d <= end; d = addDays(d, 1)) days.push(d);
    return days;
  }, [month]);

  const dayBookings = useMemo(() => {
    return bookings.filter((b) => format(new Date(b.slot_start), "yyyy-MM-dd") === selectedDate);
  }, [bookings, selectedDate]);

  const listDays = useMemo(() => groupBookingsByDate(bookings), [bookings]);

  if (!calendars.length) {
    return <EmptyState title="No booking calendars yet." subtitle="Add a calendar to get started." />;
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <CalendarSelect calendars={calendars} value={selectedCalendarId} onChange={onSelectCalendar} />
        <div className="flex overflow-hidden rounded-md border border-gray-200">
          {(["grid", "list"] as const).map((v) => (
            <button
              key={v}
              type="button"
              onClick={() => setView(v)}
              className={cn(
                "px-3 py-1 text-sm font-semibold capitalize",
                view === v ? "bg-jade-500 text-white" : "text-gray-700 hover:bg-gray-50"
              )}
            >
              {v}
            </button>
          ))}
        </div>
      </div>

      {view === "list" ? (
        isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : !listDays.length ? (
          <EmptyState title="No appointments on this calendar yet." />
        ) : (
          <div className="space-y-4">
            {listDays.map(({ date, items }) => (
              <div key={date} className="rounded-lg border border-gray-100 bg-surface-card p-4">
                <div className="mb-2 text-sm font-bold text-gray-800">
                  {format(new Date(date + "T12:00:00"), "EEEE, MMM d")}
                </div>
                <div className="space-y-1">
                  {items.map((b) => (
                    <div key={b.id} className="flex justify-between text-sm">
                      <span>{b.lead_name}</span>
                      <span className="text-gray-500">{format(new Date(b.slot_start), "h:mm a")}</span>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )
      ) : !selectedCalendar ? (
        <EmptyState title="Select a calendar." />
      ) : (
        <div className="flex gap-4">
          <div className="flex-1">
            <div className="mb-2 flex items-center justify-between">
              <div>
                <div className="font-semibold text-gray-800">{format(month, "MMMM yyyy")}</div>
                <div className="text-xs text-gray-400">Times in {selectedCalendar.timezone}</div>
              </div>
              <div className="flex gap-1">
                <Button variant="secondary" className="h-8" onClick={() => setMonth(addMonths(month, -1))}>
                  Prev
                </Button>
                <Button variant="secondary" className="h-8" onClick={() => setMonth(addMonths(month, 1))}>
                  Next
                </Button>
              </div>
            </div>
            <div className="grid grid-cols-7 gap-1 text-center text-xs font-semibold text-gray-400">
              {["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"].map((d) => (
                <div key={d}>{d}</div>
              ))}
            </div>
            <div className="mt-1 grid grid-cols-7 gap-1">
              {calendarDays.map((d) => {
                const key = format(d, "yyyy-MM-dd");
                const m = markers.find((x) => x.date === key);
                const selected = key === selectedDate;
                return (
                  <button
                    key={key}
                    type="button"
                    disabled={!isSameMonth(d, month)}
                    onClick={() => setSelectedDate(key)}
                    className={cn(
                      "relative aspect-square rounded text-sm",
                      !isSameMonth(d, month) && "text-gray-300",
                      selected && "bg-jade-500 font-bold text-white",
                      !selected && isSameDay(d, new Date()) && "ring-1 ring-jade-300"
                    )}
                  >
                    {format(d, "d")}
                    {m?.has_bookable && (
                      <span
                        className={cn(
                          "absolute bottom-1 left-1/2 h-1.5 w-1.5 -translate-x-1/2 rounded-full",
                          selected ? "bg-white" : "bg-jade-500"
                        )}
                      />
                    )}
                    {m?.has_bookings && (
                      <span
                        className={cn(
                          "absolute bottom-1 right-1 h-1.5 w-1.5 rounded-full",
                          selected ? "bg-jade-200" : "bg-amber-500"
                        )}
                      />
                    )}
                  </button>
                );
              })}
            </div>
          </div>
          <div className="w-64 shrink-0 border-l border-gray-100 pl-3">
            <div className="mb-2 text-sm font-bold text-gray-800">
              {format(new Date(selectedDate + "T12:00:00"), "EEE, MMM d")}
            </div>
            {isLoading ? (
              <Spinner className="h-5 w-5" />
            ) : !dayBookings.length ? (
              <p className="text-sm text-gray-400">No appointments.</p>
            ) : (
              <div className="space-y-2">
                {dayBookings.map((b) => (
                  <div key={b.id} className="rounded border border-gray-100 px-2 py-2 text-sm">
                    <div className="font-medium">{format(new Date(b.slot_start), "h:mm a")}</div>
                    <div className="text-gray-600">{b.lead_name}</div>
                    <div className="text-xs text-gray-400">{b.publisher_name}</div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function CalendarsTab({
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
    <div className="overflow-x-auto rounded-md border border-gray-100">
      <table className="w-full text-sm">
        <thead className="bg-gray-50 text-left text-xs font-semibold uppercase text-gray-400">
          <tr>
            <th className="px-3 py-2">Name</th>
            <th className="px-3 py-2">Timezone</th>
            <th className="px-3 py-2">Slots</th>
            <th className="px-3 py-2">Status</th>
            <th className="px-3 py-2">Updated</th>
          </tr>
        </thead>
        <tbody>
          {calendars.map((c) => (
            <tr
              key={c.id}
              className="cursor-pointer border-t border-gray-50 hover:bg-jade-50/50"
              onClick={() => onOpen(c.id)}
            >
              <td className="px-3 py-2 font-medium">{c.name}</td>
              <td className="px-3 py-2">{c.timezone}</td>
              <td className="px-3 py-2">{c.slot_count}</td>
              <td className="px-3 py-2">{c.configured ? "Ready" : "Setup needed"}</td>
              <td className="px-3 py-2 text-gray-500">
                {format(new Date(c.updated_at), "MMM d, yyyy")}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function groupBookingsByDate(bookings: AppointmentBooking[]) {
  const map = new Map<string, AppointmentBooking[]>();
  for (const b of [...bookings].sort(
    (a, c) => new Date(a.slot_start).getTime() - new Date(c.slot_start).getTime()
  )) {
    const key = format(new Date(b.slot_start), "yyyy-MM-dd");
    const list = map.get(key) ?? [];
    list.push(b);
    map.set(key, list);
  }
  return [...map.entries()].map(([date, items]) => ({ date, items }));
}
