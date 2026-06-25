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
import { cn } from "@/lib/utils";
import { PageBody } from "@/components/layout/PageBody";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";
import { EmptyState, Spinner } from "@/components/ui/misc";
import { BookAppointmentDrawer } from "@/features/appointments/BookAppointmentDrawer";
import {
  useCalendarMarkers,
  useFreeSlots,
  usePublisherAppointmentContracts,
} from "@/features/appointments/hooks";
import type { AppointmentContractOption, AppointmentFreeSlot } from "@/types";

export function PublisherCalendarsPage() {
  const { data: contracts = [], isLoading: loadingContracts } = usePublisherAppointmentContracts();
  const [selectedContractId, setSelectedContractId] = useState<number | null>(null);
  const [selectedDate, setSelectedDate] = useState(() => format(new Date(), "yyyy-MM-dd"));
  const [month, setMonth] = useState(() => startOfMonth(new Date()));
  const [bookSlot, setBookSlot] = useState<AppointmentFreeSlot | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const bookableContracts = useMemo(
    () => contracts.filter((c) => c.configured),
    [contracts]
  );

  const selectedContract = useMemo(
    () => bookableContracts.find((c) => c.contract_id === selectedContractId) ?? null,
    [bookableContracts, selectedContractId]
  );

  useEffect(() => {
    if (bookableContracts.length === 0) {
      if (selectedContractId !== null) setSelectedContractId(null);
      return;
    }
    if (!bookableContracts.some((c) => c.contract_id === selectedContractId)) {
      setSelectedContractId(bookableContracts[0].contract_id);
    }
  }, [bookableContracts, selectedContractId]);

  const monthStart = format(startOfMonth(month), "yyyy-MM-dd");
  const monthEnd = format(endOfMonth(month), "yyyy-MM-dd");
  const { data: markers = [] } = useCalendarMarkers(selectedContractId, monthStart, monthEnd);
  const { data: freeSlots = [], isLoading: loadingSlots } = useFreeSlots(selectedContractId, selectedDate);

  const buyers = useMemo(() => {
    const map = new Map<number, { buyer_id: number; buyer_name: string; contracts: AppointmentContractOption[] }>();
    for (const c of bookableContracts) {
      const g = map.get(c.buyer_id) ?? { buyer_id: c.buyer_id, buyer_name: c.buyer_name, contracts: [] };
      g.contracts.push(c);
      map.set(c.buyer_id, g);
    }
    return [...map.values()];
  }, [bookableContracts]);

  const calendarDays = useMemo(() => {
    const start = startOfWeek(startOfMonth(month));
    const end = endOfWeek(endOfMonth(month));
    const days: Date[] = [];
    for (let d = start; d <= end; d = addDays(d, 1)) days.push(d);
    return days;
  }, [month]);

  function openBook(slot: AppointmentFreeSlot) {
    setBookSlot(slot);
    setDrawerOpen(true);
  }

  return (
    <>
      <PageHeader title="Calendars" />
      <PageBody>
        {loadingContracts ? (
          <Spinner className="h-6 w-6" />
        ) : !bookableContracts.length ? (
          <EmptyState title="No buyers have configured availability yet." />
        ) : (
          <div className="flex min-h-[32rem] gap-4">
            <aside className="w-56 shrink-0 space-y-3 overflow-y-auto border-r border-gray-100 pr-3">
              {buyers.map((b) => (
                <div key={b.buyer_id}>
                  <div className="text-xs font-bold uppercase text-gray-400">{b.buyer_name}</div>
                  {b.contracts.map((c) => (
                    <button
                      key={c.contract_id}
                      type="button"
                      onClick={() => setSelectedContractId(c.contract_id)}
                      className={cn(
                        "mt-1 block w-full rounded px-2 py-1.5 text-left text-sm",
                        selectedContractId === c.contract_id
                          ? "bg-jade-100 font-semibold text-jade-800"
                          : "text-gray-700 hover:bg-gray-50"
                      )}
                    >
                      {c.contract_name}
                    </button>
                  ))}
                </div>
              ))}
            </aside>

            <div className="min-w-0 flex-1">
              {!selectedContract ? (
                <EmptyState title="Select a contract to view availability." />
              ) : (
                <div className="flex gap-4">
                  <div className="flex-1">
                    <div className="mb-2 flex items-center justify-between">
                      <div>
                        <div className="font-semibold text-gray-800">{format(month, "MMMM yyyy")}</div>
                        <div className="text-xs text-gray-400">Times shown in {selectedContract.timezone}</div>
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

                  <div className="w-52 shrink-0 border-l border-gray-100 pl-3">
                    <div className="mb-2 text-sm font-bold text-gray-800">
                      {format(new Date(selectedDate + "T12:00:00"), "EEE, MMM d")}
                    </div>
                    {loadingSlots ? (
                      <Spinner className="h-5 w-5" />
                    ) : !freeSlots.length ? (
                      <p className="text-sm text-gray-400">No free slots.</p>
                    ) : (
                      <div className="space-y-1">
                        {freeSlots.map((s) => (
                          <button
                            key={s.slot_start}
                            type="button"
                            onClick={() => openBook(s)}
                            className="flex w-full items-center justify-between rounded border border-gray-100 px-2 py-2 text-left text-sm hover:bg-jade-50"
                          >
                            <span>{format(new Date(s.slot_start), "h:mm a")}</span>
                            <span className="text-xs font-semibold text-jade-700">{s.remaining_capacity} free</span>
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>
          </div>
        )}
      </PageBody>

      <BookAppointmentDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        contractId={selectedContractId ?? 0}
        slot={bookSlot}
      />
    </>
  );
}
