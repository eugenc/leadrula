import { useEffect, useState } from "react";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Select } from "@/components/ui/input";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { PageBody } from "@/components/layout/PageBody";
import { formatMoney } from "@/lib/utils";
import { useUIStore } from "@/store/uiStore";
import { useBuyerCalls, type CallLogFilters } from "@/features/calls/hooks";
import { formatCallStatus } from "@/features/calls/format";
import { CallSearchInput } from "@/features/calls/CallSearchInput";
import { CallDetailDrawer } from "@/features/calls/CallDetailDrawer";
import type { Call } from "@/types";

function fmtDuration(sec: number) {
  if (!sec) return "—";
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return m ? `${m}m ${s}s` : `${s}s`;
}

function fmtTime(iso: string) {
  return new Date(iso).toLocaleString();
}

const STATUS_OPTIONS = ["", "ringing", "connected", "completed", "no_answer", "blocked"];

export function BuyerCallsLogTab() {
  const [filters, setFilters] = useState<CallLogFilters>({});
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const { data: calls, isLoading } = useBuyerCalls({ ...filters, q: debouncedSearch || undefined });
  const [openId, setOpenId] = useState<number | null>(null);
  const openDetail = useUIStore((s) => s.openDetail);

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search.trim()), 300);
    return () => clearTimeout(t);
  }, [search]);

  return (
    <PageBody>
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <CallSearchInput
          value={search}
          onChange={setSearch}
          role="buyer"
          callFilters={{ status: filters.status }}
          onSelectCall={(c) => setOpenId(c.id)}
          className="w-72"
          inputClassName="h-7 text-sm"
        />
        <Select
          className="h-7 w-40 text-sm"
          value={filters.status ?? ""}
          onChange={(e) => setFilters((f) => ({ ...f, status: e.target.value || undefined }))}
        >
          {STATUS_OPTIONS.map((s) => (
            <option key={s} value={s}>
              {s === "" ? "All statuses" : formatCallStatus(s)}
            </option>
          ))}
        </Select>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-10">
          <Spinner />
        </div>
      ) : (calls ?? []).length === 0 ? (
        debouncedSearch ? (
          <EmptyState title="No matching calls" subtitle="Try a different search term." />
        ) : (
          <EmptyState title="No calls yet" subtitle="Billable calls assigned to you will appear here." />
        )
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Time</TH>
              <TH>Phone</TH>
              <TH>Publisher</TH>
              <TH>Contract</TH>
              <TH>Status</TH>
              <TH>Duration</TH>
              <TH>Price</TH>
              <TH>Lead</TH>
            </tr>
          </THead>
          <TBody>
            {(calls ?? []).map((c: Call) => (
              <TR key={c.id} onClick={() => setOpenId(c.id)} className="cursor-pointer">
                <TD className="whitespace-nowrap text-xs text-gray-500">{fmtTime(c.created_at)}</TD>
                <TD>{c.lead_phone ?? "—"}</TD>
                <TD>{c.publisher_name ?? "—"}</TD>
                <TD>{c.contract_name ?? "—"}</TD>
                <TD>{formatCallStatus(c.status)}</TD>
                <TD>{fmtDuration(c.duration_sec)}</TD>
                <TD>{c.price_cents ? formatMoney(c.price_cents / 100) : "—"}</TD>
                <TD>
                  {c.lead_id ? (
                    <button
                      className="font-medium text-jade-600 hover:underline"
                      onClick={(e) => {
                        e.stopPropagation();
                        openDetail(c.lead_id!);
                      }}
                    >
                      {c.lead_name ?? `#${c.lead_id}`}
                    </button>
                  ) : (
                    "—"
                  )}
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}

      <CallDetailDrawer callId={openId} onClose={() => setOpenId(null)} role="buyer" />
    </PageBody>
  );
}
