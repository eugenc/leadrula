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
import { Button } from "@/components/ui/button";
import { DrawerBody, DrawerHeader, Sheet } from "@/components/ui/dialog";
import { FilterSelect } from "@/components/ui/input";
import { EmptyState, Spinner } from "@/components/ui/misc";
import { BookDaySlotsColumn } from "@/features/appointments/BookDaySlotsColumn";
import { BookAppointmentDrawer } from "@/features/appointments/BookAppointmentDrawer";
import {
  useBuyerAppointmentCalendarMarkers,
  useBuyerAppointmentContracts,
  useBuyerCalendars,
  useBuyerFreeSlots,
  useCalendarAppointmentFreeSlots,
  useCalendarAppointmentMarkers,
  workingHoursForDate,
} from "@/features/appointments/hooks";
import type { AppointmentFreeSlot } from "@/types";

type BookTarget =
  | { kind: "calendar"; id: number }
  | { kind: "contract"; id: number };

function targetKey(t: BookTarget) {
  return `${t.kind}:${t.id}`;
}

function parseTargetKey(key: string): BookTarget | null {
  const [kind, idStr] = key.split(":");
  const id = Number(idStr);
  if (!id || (kind !== "calendar" && kind !== "contract")) return null;
  return { kind, id } as BookTarget;
}

export function BuyerBookAppointmentSheet({
  open,
  onClose,
  onBooked,
}: {
  open: boolean;
  onClose: () => void;
  onBooked?: () => void;
}) {
  const { data: contracts = [], isLoading: loadingContracts } = useBuyerAppointmentContracts();
  const { data: calendars = [], isLoading: loadingCalendars } = useBuyerCalendars();
  const [selectedTarget, setSelectedTarget] = useState<BookTarget | null>(null);
  const [selectedDate, setSelectedDate] = useState(() => format(new Date(), "yyyy-MM-dd"));
  const [month, setMonth] = useState(() => startOfMonth(new Date()));
  const [bookSlot, setBookSlot] = useState<AppointmentFreeSlot | null>(null);
  const [customSchedule, setCustomSchedule] = useState<{ date: string; timezone: string } | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const bookableCalendars = useMemo(() => calendars.filter((c) => c.configured), [calendars]);
  const bookableContracts = useMemo(() => contracts.filter((c) => c.configured), [contracts]);

  const selectedContractId = selectedTarget?.kind === "contract" ? selectedTarget.id : null;
  const selectedCalendarId = selectedTarget?.kind === "calendar" ? selectedTarget.id : null;

  const selectedContract = useMemo(
    () => bookableContracts.find((c) => c.contract_id === selectedContractId) ?? null,
    [bookableContracts, selectedContractId]
  );
  const selectedCalendar = useMemo(
    () => bookableCalendars.find((c) => c.id === selectedCalendarId) ?? null,
    [bookableCalendars, selectedCalendarId]
  );

  useEffect(() => {
    if (!open) {
      setBookSlot(null);
      setCustomSchedule(null);
      setDrawerOpen(false);
      return;
    }
    if (bookableCalendars.length === 0 && bookableContracts.length === 0) {
      if (selectedTarget !== null) setSelectedTarget(null);
      return;
    }
    if (selectedTarget?.kind === "calendar" && bookableCalendars.some((c) => c.id === selectedTarget.id)) return;
    if (selectedTarget?.kind === "contract" && bookableContracts.some((c) => c.contract_id === selectedTarget.id)) return;
    if (bookableCalendars.length > 0) {
      setSelectedTarget({ kind: "calendar", id: bookableCalendars[0].id });
    } else {
      setSelectedTarget({ kind: "contract", id: bookableContracts[0].contract_id });
    }
  }, [open, bookableCalendars, bookableContracts, selectedTarget]);

  const monthStart = format(startOfMonth(month), "yyyy-MM-dd");
  const monthEnd = format(endOfMonth(month), "yyyy-MM-dd");
  const { data: contractMarkers = [] } = useBuyerAppointmentCalendarMarkers(selectedContractId, monthStart, monthEnd);
  const { data: calendarMarkers = [] } = useCalendarAppointmentMarkers(
    selectedCalendarId,
    monthStart,
    monthEnd,
    "buyer"
  );
  const { data: contractDaySlots, isLoading: loadingContractSlots, isError: contractSlotsError, error: contractSlotsErr } =
    useBuyerFreeSlots(selectedContractId, selectedDate);
  const { data: calendarDaySlots, isLoading: loadingCalendarSlots, isError: calendarSlotsError, error: calendarSlotsErr } =
    useCalendarAppointmentFreeSlots(selectedCalendarId, selectedDate, "buyer");

  const daySlots = selectedCalendarId ? calendarDaySlots : contractDaySlots;
  const freeSlots = daySlots?.items ?? [];
  const bookedSlots = daySlots?.booked ?? [];
  const loadingSlots = selectedCalendarId ? loadingCalendarSlots : loadingContractSlots;
  const slotsError = selectedCalendarId ? calendarSlotsError : contractSlotsError;
  const slotsErr = selectedCalendarId ? calendarSlotsErr : contractSlotsErr;
  const markers = selectedCalendarId ? calendarMarkers : contractMarkers;
  const timezone = selectedCalendar?.timezone ?? selectedContract?.timezone ?? "UTC";

  const workingHours = useMemo(
    () => workingHoursForDate(daySlots?.working_hours, selectedCalendar?.schedule, selectedDate),
    [daySlots?.working_hours, selectedCalendar?.schedule, selectedDate]
  );

  const calendarDays = useMemo(() => {
    const start = startOfWeek(startOfMonth(month));
    const end = endOfWeek(endOfMonth(month));
    const days: Date[] = [];
    for (let d = start; d <= end; d = addDays(d, 1)) days.push(d);
    return days;
  }, [month]);

  const today = startOfDay(new Date());

  function openBook(slot: AppointmentFreeSlot) {
    setCustomSchedule(null);
    setBookSlot(slot);
    setDrawerOpen(true);
  }

  function openCustomTime() {
    setBookSlot(null);
    setCustomSchedule({ date: selectedDate, timezone });
    setDrawerOpen(true);
  }

  function closeAll() {
    setDrawerOpen(false);
    setBookSlot(null);
    setCustomSchedule(null);
    onClose();
  }

  function handleBooked() {
    setDrawerOpen(false);
    setBookSlot(null);
    setCustomSchedule(null);
    if (onBooked) {
      onBooked();
    } else {
      onClose();
    }
  }

  const hasBookable = bookableCalendars.length > 0 || bookableContracts.length > 0;

  return (
    <>
      <Sheet open={open} onClose={closeAll} width={640}>
        <DrawerHeader title="Add appointment" onClose={closeAll} />
        <DrawerBody>
          {loadingContracts || loadingCalendars ? (
            <Spinner className="h-6 w-6" />
          ) : !hasBookable ? (
            <EmptyState
              title="No bookable calendars yet."
              subtitle="Create a calendar under Calendars and add availability slots, or attach one to an appointment contract."
            />
          ) : (
            <div className="space-y-4">
              <FilterSelect
                value={selectedTarget ? targetKey(selectedTarget) : ""}
                onChange={(e) => setSelectedTarget(parseTargetKey(e.target.value))}
                className="w-full"
              >
                {bookableCalendars.length > 0 && (
                  <optgroup label="My calendars">
                    {bookableCalendars.map((c) => (
                      <option key={c.id} value={targetKey({ kind: "calendar", id: c.id })}>
                        {c.name}
                      </option>
                    ))}
                  </optgroup>
                )}
                {bookableContracts.length > 0 && (
                  <optgroup label="Contracts">
                    {bookableContracts.map((c) => (
                      <option key={c.contract_id} value={targetKey({ kind: "contract", id: c.contract_id })}>
                        {c.contract_name} · {c.publisher_name}
                      </option>
                    ))}
                  </optgroup>
                )}
              </FilterSelect>

              {selectedTarget && (
                <div className="flex gap-4">
                  <div className="min-w-0 flex-1">
                    <div className="mb-2 flex items-center justify-between">
                      <div>
                        <div className="font-semibold text-gray-800">{format(month, "MMMM yyyy")}</div>
                        <div className="text-xs text-gray-400">Times shown in {timezone}</div>
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

                  <BookDaySlotsColumn
                    selectedDate={selectedDate}
                    loading={loadingSlots}
                    error={slotsError ? slotsErr : null}
                    freeSlots={freeSlots}
                    booked={bookedSlots}
                    workingHours={workingHours}
                    onBookSlot={openBook}
                    onCustomTime={openCustomTime}
                  />
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
          setCustomSchedule(null);
        }}
        onBooked={handleBooked}
        contractId={selectedContractId ?? undefined}
        calendarId={selectedCalendarId ?? undefined}
        slot={bookSlot}
        customSchedule={customSchedule}
      />
    </>
  );
}
