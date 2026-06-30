import { useEffect, useMemo, useState } from "react";
import {
  addDays,
  addMonths,
  endOfMonth,
  endOfWeek,
  format,
  isAfter,
  isSameDay,
  isSameMonth,
  startOfDay,
  startOfMonth,
  startOfWeek,
} from "date-fns";
import { cn } from "@/lib/utils";
import { errorMessage } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { DrawerBody, DrawerHeader, Sheet } from "@/components/ui/dialog";
import { FilterSelect } from "@/components/ui/input";
import { EmptyState, Spinner } from "@/components/ui/misc";
import { BookAppointmentDrawer } from "@/features/appointments/BookAppointmentDrawer";
import {
  useBuyerAppointmentCalendarMarkers,
  useBuyerAppointmentContracts,
  useBuyerFreeSlots,
} from "@/features/appointments/hooks";
import type { AppointmentFreeSlot } from "@/types";

export function BuyerBookAppointmentSheet({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { data: contracts = [], isLoading: loadingContracts } = useBuyerAppointmentContracts();
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
    if (!open) {
      setBookSlot(null);
      setDrawerOpen(false);
      return;
    }
    if (bookableContracts.length === 0) {
      if (selectedContractId !== null) setSelectedContractId(null);
      return;
    }
    if (!bookableContracts.some((c) => c.contract_id === selectedContractId)) {
      setSelectedContractId(bookableContracts[0].contract_id);
    }
  }, [open, bookableContracts, selectedContractId]);

  const monthStart = format(startOfMonth(month), "yyyy-MM-dd");
  const monthEnd = format(endOfMonth(month), "yyyy-MM-dd");
  const { data: markers = [] } = useBuyerAppointmentCalendarMarkers(selectedContractId, monthStart, monthEnd);
  const { data: freeSlots = [], isLoading: loadingSlots, isError: slotsError, error: slotsErr } = useBuyerFreeSlots(selectedContractId, selectedDate);

  const calendarDays = useMemo(() => {
    const start = startOfWeek(startOfMonth(month));
    const end = endOfWeek(endOfMonth(month));
    const days: Date[] = [];
    for (let d = start; d <= end; d = addDays(d, 1)) days.push(d);
    return days;
  }, [month]);

  const today = startOfDay(new Date());

  function openBook(slot: AppointmentFreeSlot) {
    setBookSlot(slot);
    setDrawerOpen(true);
  }

  function closeAll() {
    setDrawerOpen(false);
    setBookSlot(null);
    onClose();
  }

  return (
    <>
      <Sheet open={open} onClose={closeAll} width={640}>
        <DrawerHeader title="Add appointment" onClose={closeAll} />
        <DrawerBody>
          {loadingContracts ? (
            <Spinner className="h-6 w-6" />
          ) : !bookableContracts.length ? (
            <EmptyState title="No bookable appointment contracts." subtitle="Configure a calendar and attach it to an appointment contract first." />
          ) : (
            <div className="space-y-4">
              <div>
                <FilterSelect
                  value={selectedContractId ?? ""}
                  onChange={(e) => setSelectedContractId(Number(e.target.value))}
                  className="w-full"
                >
                  {bookableContracts.map((c) => (
                    <option key={c.contract_id} value={c.contract_id}>
                      {c.contract_name} · {c.publisher_name}
                    </option>
                  ))}
                </FilterSelect>
              </div>

              {selectedContract && (
                <div className="flex gap-4">
                  <div className="min-w-0 flex-1">
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
                    <div className="overflow-hidden rounded border border-gray-100">
                      <div className="grid grid-cols-7 border-b border-gray-100 text-center text-xs font-semibold text-gray-400">
                        {["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"].map((d) => (
                          <div key={d} className="py-1">
                            {d}
                          </div>
                        ))}
                      </div>
                      <div className="grid grid-cols-7 divide-x divide-y divide-gray-100">
                        {calendarDays.map((d) => {
                          const key = format(d, "yyyy-MM-dd");
                          const m = markers.find((x) => x.date === key);
                          const selected = key === selectedDate;
                          const inMonth = isSameMonth(d, month);
                          const isFutureDay = inMonth && isAfter(startOfDay(d), today);
                          return (
                            <button
                              key={key}
                              type="button"
                              disabled={!inMonth}
                              onClick={() => setSelectedDate(key)}
                              className={cn(
                                "relative aspect-square text-sm",
                                !inMonth && "text-gray-300",
                                isFutureDay && !selected && "text-gray-400",
                                selected && "bg-jade-500 font-bold text-white",
                                !selected && isSameDay(d, new Date()) && "ring-1 ring-inset ring-jade-300"
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
                  </div>

                  <div className="w-44 shrink-0 border-l border-gray-100 pl-3">
                    <div className="mb-2 text-sm font-bold text-gray-800">
                      {format(new Date(selectedDate + "T12:00:00"), "EEE, MMM d")}
                    </div>
                    {loadingSlots ? (
                      <Spinner className="h-5 w-5" />
                    ) : slotsError ? (
                      <p className="text-sm text-red-600">{errorMessage(slotsErr)}</p>
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
          )}
        </DrawerBody>
      </Sheet>

      <BookAppointmentDrawer
        mode="buyer"
        open={drawerOpen}
        onClose={() => {
          setDrawerOpen(false);
          setBookSlot(null);
        }}
        onBooked={closeAll}
        contractId={selectedContractId ?? 0}
        slot={bookSlot}
      />
    </>
  );
}
