import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowDown, ArrowUp, ArrowUpDown, Plus } from "lucide-react";
import { PageBody } from "@/components/layout/PageBody";
import { Button } from "@/components/ui/button";
import { FilterSelect } from "@/components/ui/input";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { AppointmentListCard } from "@/features/appointments/AppointmentListCard";
import { BookedAppointmentsTable } from "@/features/appointments/BookedAppointmentsTable";
import { BuyerBookAppointmentSheet } from "@/features/appointments/BuyerBookAppointmentSheet";
import {
  APPOINTMENT_PRESET_LABELS,
  defaultAppointmentsUi,
  loadAppointmentsUi,
  PAGE_SIZES,
  saveAppointmentsUi,
  type AppointmentPreset,
  type AppointmentSort,
  type SortDir,
} from "@/features/appointments/appointmentsUiStorage";
import { useBuyerAppointmentContracts, useBuyerBookings, useBuyerCalendars } from "@/features/appointments/hooks";
import { useBuyerContracts } from "@/features/admin/hooks";
import { LeadSearchInput } from "@/features/leads/LeadSearchInput";
import { get } from "@/lib/api";
import { useAuthStore } from "@/store/authStore";
import { useUIStore } from "@/store/uiStore";
import type { Me } from "@/types";

export function BuyerAppointmentsPage() {
  const user = useAuthStore((s) => s.user);
  const openDetail = useUIStore((s) => s.openDetail);
  const uiHydrated = useRef(false);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [preset, setPreset] = useState<AppointmentPreset>("this_week");
  const [sort, setSort] = useState<AppointmentSort>("booked_at");
  const [sortDir, setSortDir] = useState<SortDir>("desc");
  const [contractId, setContractId] = useState(0);
  const [publisherId, setPublisherId] = useState(0);
  const [limit, setLimit] = useState(25);
  const [bookOpen, setBookOpen] = useState(false);

  const { data: me } = useQuery({
    queryKey: ["me"],
    queryFn: () => get<Me>("/auth/me"),
  });
  const timeZone = me?.account?.timezone || "UTC";

  const { data: contracts = [] } = useBuyerContracts();
  const { data: bookableContracts = [] } = useBuyerAppointmentContracts();
  const { data: calendars = [] } = useBuyerCalendars();
  const appointmentContracts = useMemo(
    () => contracts.filter((c) => c.lead_type === "Appointment"),
    [contracts]
  );
  const publishers = useMemo(() => {
    const seen = new Map<number, string>();
    for (const c of appointmentContracts) {
      if (c.publisher_id && c.publisher_name) {
        seen.set(c.publisher_id, c.publisher_name);
      }
    }
    return [...seen.entries()].map(([id, name]) => ({ id, name }));
  }, [appointmentContracts]);

  useEffect(() => {
    if (!user?.id || uiHydrated.current) return;
    const stored = loadAppointmentsUi(user.id);
    setPreset(stored.preset);
    setSort(stored.sort);
    setSortDir(stored.sort_dir);
    setContractId(stored.contract_id);
    setPublisherId(stored.publisher_id);
    setLimit(stored.limit);
    uiHydrated.current = true;
  }, [user?.id]);

  const persistUi = useCallback(
    (patch: Parameters<typeof saveAppointmentsUi>[1]) => {
      if (user?.id) saveAppointmentsUi(user.id, patch);
    },
    [user?.id]
  );

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search.trim()), 300);
    return () => clearTimeout(t);
  }, [search]);

  useEffect(() => {
    setPage(1);
  }, [debouncedSearch, preset, contractId, publisherId, limit, sort, sortDir]);

  const { data, isLoading } = useBuyerBookings({
    page,
    limit,
    sort,
    sort_dir: sortDir,
    q: debouncedSearch || undefined,
    contract_id: contractId || undefined,
    publisher_id: publisherId || undefined,
    appointment_preset: preset,
  });

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / limit));

  const filtersActive =
    preset !== "this_week" ||
    contractId !== 0 ||
    publisherId !== 0 ||
    debouncedSearch !== "";

  const datePresetOnly =
    preset !== "all" && contractId === 0 && publisherId === 0 && debouncedSearch === "";

  const canBook =
    calendars.some((c) => c.configured) || bookableContracts.some((c) => c.configured);
  const canBookHint = canBook
    ? undefined
    : "Create a calendar under Calendars and add availability slots, or attach a calendar to a contract.";

  const emptyTitle = datePresetOnly
    ? "No appointments in this date range."
    : filtersActive
      ? "No appointments match your filters."
      : "No distributed appointments yet.";

  const emptySubtitle = datePresetOnly
    ? "Try All dates, or book a slot for this week."
    : canBook && !filtersActive
      ? "Book your first appointment from an available slot."
      : undefined;

  const handleBooked = useCallback(() => {
    setPreset("all");
    setPage(1);
    setBookOpen(false);
    persistUi({ preset: "all" });
  }, [persistUi]);

  function clearFilters() {
    const d = defaultAppointmentsUi();
    setPreset(d.preset);
    setSort(d.sort);
    setSortDir(d.sort_dir);
    setContractId(d.contract_id);
    setPublisherId(d.publisher_id);
    setSearch("");
    setDebouncedSearch("");
    setPage(1);
    persistUi(d);
  }

  return (
    <>
      <PageBody>
        <div className="mb-4 flex flex-wrap items-center gap-2">
          <FilterSelect
            value={preset}
            onChange={(e) => {
              const v = e.target.value as AppointmentPreset;
              setPreset(v);
              persistUi({ preset: v });
            }}
            className="w-full min-w-0 sm:w-40"
          >
            {(Object.keys(APPOINTMENT_PRESET_LABELS) as AppointmentPreset[]).map((key) => (
              <option key={key} value={key}>
                {APPOINTMENT_PRESET_LABELS[key]}
              </option>
            ))}
          </FilterSelect>

          <FilterSelect
            value={contractId || ""}
            onChange={(e) => {
              const v = Number(e.target.value);
              setContractId(v);
              persistUi({ contract_id: v });
            }}
            className="w-full min-w-0 sm:w-48"
          >
            <option value="">All contracts</option>
            {appointmentContracts.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </FilterSelect>

          <FilterSelect
            value={publisherId || ""}
            onChange={(e) => {
              const v = Number(e.target.value);
              setPublisherId(v);
              persistUi({ publisher_id: v });
            }}
            className="w-full min-w-0 sm:w-48"
          >
            <option value="">All publishers</option>
            {publishers.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </FilterSelect>

          <LeadSearchInput
            value={search}
            onChange={setSearch}
            className="w-full min-w-0 sm:w-72"
            inputClassName="h-7 text-sm"
            placeholder="Search name, phone, or email…"
            suggestionsEnabled={false}
          />

          <div className="flex items-center gap-1">
            <FilterSelect
              value={sort}
              onChange={(e) => {
                const v = e.target.value as AppointmentSort;
                setSort(v);
                persistUi({ sort: v });
              }}
              className="w-40"
            >
              <option value="booked_at">Booked date</option>
              <option value="appointment_at">Appointment date</option>
            </FilterSelect>
            <Button
              variant="ghost"
              size="sm"
              className="px-2"
              title={sortDir === "asc" ? "Ascending" : "Descending"}
              onClick={() => {
                const v = sortDir === "asc" ? "desc" : "asc";
                setSortDir(v);
                persistUi({ sort_dir: v });
              }}
            >
              {sortDir === "asc" ? <ArrowUp className="h-4 w-4" /> : <ArrowDown className="h-4 w-4" />}
              <ArrowUpDown className="sr-only" />
            </Button>
          </div>

          {filtersActive && (
            <Button variant="outline" size="sm" onClick={clearFilters}>
              Clear filters
            </Button>
          )}

          <div className="flex w-full flex-wrap items-center gap-2 sm:ml-auto sm:w-auto">
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
          </div>
        </div>

        {isLoading ? (
          <div className="flex justify-center py-16">
            <Spinner className="h-6 w-6" />
          </div>
        ) : items.length === 0 ? (
          <EmptyState
            title={emptyTitle}
            subtitle={emptySubtitle}
            action={
              canBook && !filtersActive ? (
                <Button type="button" size="sm" onClick={() => setBookOpen(true)}>
                  <Plus className="h-4 w-4" />
                  Add appointment
                </Button>
              ) : undefined
            }
          />
        ) : (
          <>
            <div className="space-y-2 sm:hidden">
              {items.map((row) => (
                <AppointmentListCard
                  key={row.id}
                  row={row}
                  timeZone={timeZone}
                  onOpen={() => openDetail(row.lead_id)}
                />
              ))}
            </div>

            <div className="hidden sm:block">
              <BookedAppointmentsTable items={items} isLoading={false} timeZone={timeZone} />
            </div>

            <div className="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-gray-500">
              <span>
                {(page - 1) * limit + 1}–{Math.min(page * limit, total)} of {total}
              </span>
              <div className="flex items-center gap-3">
                <FilterSelect
                  value={limit}
                  onChange={(e) => {
                    const v = Number(e.target.value);
                    setLimit(v);
                    persistUi({ limit: v });
                  }}
                  className="w-24"
                >
                  {PAGE_SIZES.map((n) => (
                    <option key={n} value={n}>
                      {n} / page
                    </option>
                  ))}
                </FilterSelect>
                <Button variant="secondary" size="sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                  Previous
                </Button>
                <span>
                  Page {page} of {totalPages}
                </span>
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={page >= totalPages}
                  onClick={() => setPage((p) => p + 1)}
                >
                  Next
                </Button>
              </div>
            </div>
          </>
        )}
      </PageBody>

      <BuyerBookAppointmentSheet
        open={bookOpen}
        onClose={() => setBookOpen(false)}
        onBooked={handleBooked}
      />
    </>
  );
}
