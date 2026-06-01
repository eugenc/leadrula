import { useEffect, useState } from "react";
import { useRoutingLog, useRejectQueue } from "@/features/admin/hooks";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, FilterSelect } from "@/components/ui/input";
import { Spinner, EmptyState, Badge } from "@/components/ui/misc";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { QueueItem } from "@/types";
import { LOG_FILTERS, PAGE_SIZES, statusBadge, type LogFilter } from "./logShared";
import { QueueItemDrawer, RouteDialog } from "@/pages/publisher/intakeShared";

type LogSource = "publisher" | "buyer";

interface IntakeLogTableProps {
  source: LogSource;
  readOnly?: boolean;
  emptyTitle?: string;
}

export function IntakeLogTable({
  source,
  readOnly = false,
  emptyTitle = source === "buyer" ? "No contract leads yet." : "No intake history yet.",
}: IntakeLogTableProps) {
  const reject = useRejectQueue();
  const [drawerItem, setDrawerItem] = useState<QueueItem | null>(null);
  const [routing, setRouting] = useState<QueueItem | null>(null);
  const [logFilter, setLogFilter] = useState<LogFilter>("all");
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState<number>(25);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search.trim()), 300);
    return () => clearTimeout(t);
  }, [search]);

  useEffect(() => {
    setPage(1);
  }, [logFilter, limit, debouncedSearch]);

  const filters = {
    status: logFilter,
    page,
    limit,
    q: debouncedSearch || undefined,
  };

  const { data, isLoading } = useRoutingLog(source, filters);

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / limit));
  const hasFilters = logFilter !== "all" || debouncedSearch !== "";

  if (isLoading) return <Spinner className="h-6 w-6" />;

  return (
    <>
      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <Input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search name, phone, source…"
          className="max-w-sm"
        />
        <div className="flex flex-wrap gap-2">
          {LOG_FILTERS.map((f) => (
            <Button
              key={f.value}
              size="sm"
              variant={logFilter === f.value ? "primary" : "secondary"}
              onClick={() => setLogFilter(f.value)}
            >
              {f.label}
            </Button>
          ))}
        </div>
      </div>

      {items.length === 0 ? (
        <EmptyState title={hasFilters ? "No results." : emptyTitle} />
      ) : (
        <>
          <Table>
            <THead>
              <tr>
                <TH>Lead</TH>
                <TH>Source</TH>
                <TH>Received</TH>
                <TH>Status</TH>
                <TH>Unmapped</TH>
                {!readOnly && <TH />}
              </tr>
            </THead>
            <TBody>
              {items.map((q) => (
                <TR key={q.id} onClick={() => setDrawerItem(q)} className="cursor-pointer">
                  <TD className="font-medium text-gray-800">
                    {q.first_name} {q.last_name}
                  </TD>
                  <TD>{q.source ?? "—"}</TD>
                  <TD>{format(new Date(q.created_at), "MMM d, h:mma")}</TD>
                  <TD>{statusBadge(q.status)}</TD>
                  <TD>
                    {(q.unmapped_keys?.length ?? 0) > 0 ? (
                      <Badge variant="pending">{q.unmapped_keys!.length}</Badge>
                    ) : (
                      <span className="text-sm text-gray-300">0</span>
                    )}
                  </TD>
                  {!readOnly && (
                    <TD>
                      <div className="flex justify-end gap-2" onClick={(e) => e.stopPropagation()}>
                        {(q.unmapped_keys?.length ?? 0) > 0 && (
                          <Button size="sm" variant="secondary" onClick={() => setDrawerItem(q)}>
                            Map
                          </Button>
                        )}
                      </div>
                    </TD>
                  )}
                </TR>
              ))}
            </TBody>
          </Table>

          <div className="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-gray-500">
            <span>
              {total === 0
                ? "No results"
                : `${(page - 1) * limit + 1}–${Math.min(page * limit, total)} of ${total}`}
            </span>
            <div className="flex items-center gap-3">
              <FilterSelect
                value={limit}
                onChange={(e) => setLimit(Number(e.target.value))}
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

      {!readOnly && routing && <RouteDialog item={routing} onClose={() => setRouting(null)} />}
      {drawerItem && (
        <QueueItemDrawer
          item={drawerItem}
          readOnly={readOnly}
          onClose={() => setDrawerItem(null)}
          onUpdated={readOnly ? undefined : setDrawerItem}
          onRoute={
            readOnly
              ? undefined
              : () => {
                  setRouting(drawerItem);
                  setDrawerItem(null);
                }
          }
          onReject={
            readOnly
              ? undefined
              : () => {
                  reject.mutate(drawerItem.id, {
                    onSuccess: () => {
                      toast.success("Lead rejected");
                      setDrawerItem(null);
                    },
                    onError: (e) => toast.error(errorMessage(e)),
                  });
                }
          }
        />
      )}
    </>
  );
}
