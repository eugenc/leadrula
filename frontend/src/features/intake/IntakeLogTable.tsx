import { useEffect, useMemo, useState } from "react";
import { useRoutingLog } from "@/features/admin/hooks";
import { useAccountWebhookDeliveries, useWebhooks } from "@/features/webhooks/hooks";
import { useAuthStore } from "@/store/authStore";
import { Button } from "@/components/ui/button";
import { FilterInput, FilterSelect } from "@/components/ui/input";
import { Spinner, EmptyState, Badge } from "@/components/ui/misc";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { format } from "date-fns";
import type { QueueItem } from "@/types";
import {
  LOG_FILTERS,
  LOG_TYPE_FILTERS,
  LogPagination,
  WEBHOOK_DELIVERY_FILTERS,
  statusBadge,
  type LogFilter,
  type LogTypeFilter,
  type WebhookDeliveryStatusFilter,
} from "./logShared";
import { useInboundLog } from "./hooks";
import { UnifiedInboundLogTable } from "./UnifiedInboundLogTable";
import {
  inboundItemsToRows,
  mergeInboundRows,
  queueItemsToRows,
  webhookDeliveriesToRows,
} from "./inboundLog";
import { QueueItemDrawer, RouteDialog } from "@/pages/publisher/intakeShared";
import { useRejectQueue } from "@/features/admin/hooks";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";

type LogSource = "publisher" | "buyer";

interface IntakeLogTableProps {
  source: LogSource;
  readOnly?: boolean;
  emptyTitle?: string;
  sourceSlug?: string;
  initialLogType?: LogTypeFilter;
}

/** Compact source-only log for SourcesPage drawer — unchanged. */
export function IntakeLogSection({
  source,
  readOnly,
  emptyTitle,
  sectionTitle,
  sourceSlug,
  compact = false,
}: {
  source: LogSource;
  readOnly: boolean;
  emptyTitle: string;
  sectionTitle?: string;
  sourceSlug?: string;
  compact?: boolean;
}) {
  const reject = useRejectQueue();
  const [drawerItem, setDrawerItem] = useState<QueueItem | null>(null);
  const [routing, setRouting] = useState<QueueItem | null>(null);
  const [logFilter, setLogFilter] = useState<LogFilter>("all");
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState<number>(compact ? 10 : 25);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search.trim()), 300);
    return () => clearTimeout(t);
  }, [search]);

  useEffect(() => {
    setPage(1);
  }, [logFilter, limit, debouncedSearch, sourceSlug]);

  const filters = {
    status: logFilter,
    page,
    limit,
    q: debouncedSearch || undefined,
    source: sourceSlug,
  };

  const { data, isLoading } = useRoutingLog(source, filters);

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const hasFilters = logFilter !== "all" || debouncedSearch !== "" || !!sourceSlug;

  if (isLoading) return <Spinner className="h-6 w-6" />;

  return (
    <div className={sectionTitle ? "space-y-3" : undefined}>
      {sectionTitle && (
        <p className="text-xs font-semibold uppercase tracking-wide text-gray-600">{sectionTitle}</p>
      )}

      {!compact && (
        <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <FilterInput
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search name, phone, source…"
            className="max-w-sm w-auto"
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
      )}

      {items.length === 0 ? (
        <EmptyState title={hasFilters ? "No results." : emptyTitle} />
      ) : (
        <>
          <Table>
            <THead>
              <tr>
                <TH>Lead</TH>
                {!compact && <TH>Source</TH>}
                <TH>Received</TH>
                <TH>Status</TH>
                <TH>Unmapped</TH>
                {!readOnly && <TH className="min-w-0 w-12" />}
              </tr>
            </THead>
            <TBody>
              {items.map((q) => (
                <TR key={q.id} onClick={() => setDrawerItem(q)} className="cursor-pointer">
                  <TD className="font-medium text-gray-800">
                    {q.first_name} {q.last_name}
                  </TD>
                  {!compact && <TD>{q.source ?? "—"}</TD>}
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

          {!compact && (
            <LogPagination
              page={page}
              limit={limit}
              total={total}
              onPageChange={setPage}
              onLimitChange={setLimit}
            />
          )}
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
    </div>
  );
}

export function IntakeLogTable({
  source,
  readOnly = false,
  emptyTitle = source === "buyer" ? "No contract leads yet." : "No intake history yet.",
  sourceSlug,
  initialLogType = "all",
}: IntakeLogTableProps) {
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";
  const canReplayWebhooks = isAdmin;

  const [logType, setLogType] = useState<LogTypeFilter>(initialLogType);
  const [logFilter, setLogFilter] = useState<LogFilter>("all");
  const [webhookStatus, setWebhookStatus] = useState<WebhookDeliveryStatusFilter>("");
  const [webhookId, setWebhookId] = useState<number | "">("");
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search.trim()), 300);
    return () => clearTimeout(t);
  }, [search]);

  useEffect(() => {
    setPage(1);
  }, [logType, logFilter, webhookStatus, webhookId, limit, debouncedSearch, sourceSlug]);

  const { data: webhooks } = useWebhooks();

  const intakeFilters = {
    status: logFilter,
    page,
    limit,
    q: debouncedSearch || undefined,
    source: sourceSlug,
  };

  const webhookFilters = {
    status: webhookStatus || undefined,
    webhookId: webhookId === "" ? undefined : webhookId,
    page,
    limit,
  };

  const inboundFilters = {
    type: (logType === "integrations" ? "integration" : "all") as "all" | "integration",
    page,
    limit,
  };

  const showIntakeData = logType === "intake" || (logType === "all" && source === "buyer");
  const showWebhookData = logType === "webhooks" || (logType === "all" && source === "buyer");
  const showInboundData =
    (logType === "all" || logType === "integrations") && source === "publisher";

  const intakeQuery = useRoutingLog(source, intakeFilters);
  const webhookQuery = useAccountWebhookDeliveries(webhookFilters);
  const inboundQuery = useInboundLog(inboundFilters, showInboundData);

  const buyerAllIntakeQuery = useRoutingLog("buyer", { status: "all", page, limit });
  const buyerAllWebhookQuery = useAccountWebhookDeliveries({ page, limit });

  const { rows, total, isLoading, hasFilters, refetchWebhooks } = useMemo(() => {
    if (logType === "intake") {
      const items = showIntakeData ? (intakeQuery.data?.items ?? []) : [];
      return {
        rows: queueItemsToRows(items),
        total: intakeQuery.data?.total ?? 0,
        isLoading: showIntakeData && intakeQuery.isLoading,
        hasFilters: logFilter !== "all" || debouncedSearch !== "" || !!sourceSlug,
        refetchWebhooks: () => intakeQuery.refetch(),
      };
    }
    if (logType === "webhooks") {
      const items = showWebhookData ? (webhookQuery.data?.items ?? []) : [];
      return {
        rows: webhookDeliveriesToRows(items),
        total: webhookQuery.data?.total ?? 0,
        isLoading: showWebhookData && webhookQuery.isLoading,
        hasFilters: webhookStatus !== "" || webhookId !== "",
        refetchWebhooks: () => webhookQuery.refetch(),
      };
    }

    if (logType === "integrations") {
      const items = inboundQuery.data?.items ?? [];
      return {
        rows: inboundItemsToRows(items),
        total: inboundQuery.data?.total ?? 0,
        isLoading: inboundQuery.isLoading,
        hasFilters: false,
        refetchWebhooks: () => inboundQuery.refetch(),
      };
    }

    // All
    if (source === "publisher") {
      const items = inboundQuery.data?.items ?? [];
      return {
        rows: inboundItemsToRows(items),
        total: inboundQuery.data?.total ?? 0,
        isLoading: inboundQuery.isLoading,
        hasFilters: false,
        refetchWebhooks: () => inboundQuery.refetch(),
      };
    }

    // Buyer "All" — approximate client merge
    const intakeRows = queueItemsToRows(buyerAllIntakeQuery.data?.items ?? []);
    const webhookRows = webhookDeliveriesToRows(buyerAllWebhookQuery.data?.items ?? []);
    const merged = mergeInboundRows(intakeRows, webhookRows, limit);
    const intakeTotal = buyerAllIntakeQuery.data?.total ?? 0;
    const webhookTotal = buyerAllWebhookQuery.data?.total ?? 0;
    return {
      rows: merged,
      total: intakeTotal + webhookTotal,
      isLoading: buyerAllIntakeQuery.isLoading || buyerAllWebhookQuery.isLoading,
      hasFilters: false,
      refetchWebhooks: () => {
        buyerAllIntakeQuery.refetch();
        buyerAllWebhookQuery.refetch();
      },
    };
  }, [
    logType,
    source,
    intakeQuery.data,
    intakeQuery.isLoading,
    webhookQuery.data,
    webhookQuery.isLoading,
    inboundQuery.data,
    inboundQuery.isLoading,
    buyerAllIntakeQuery.data,
    buyerAllIntakeQuery.isLoading,
    buyerAllWebhookQuery.data,
    buyerAllWebhookQuery.isLoading,
    logFilter,
    debouncedSearch,
    sourceSlug,
    webhookStatus,
    webhookId,
    limit,
    webhookQuery,
    inboundQuery,
    buyerAllIntakeQuery,
    buyerAllWebhookQuery,
  ]);

  const emptyMessage =
    logType === "webhooks"
      ? "No webhook deliveries yet."
      : logType === "integrations"
        ? "No integration deliveries yet."
        : logType === "all"
          ? "No intake history yet."
          : emptyTitle;

  const typeFilters =
    source === "publisher"
      ? LOG_TYPE_FILTERS
      : LOG_TYPE_FILTERS.filter((f) => f.value !== "integrations");

  return (
    <>
      <div className="mb-4 flex flex-wrap gap-2">
        {typeFilters.map((f) => (
          <Button
            key={f.value}
            size="sm"
            variant={logType === f.value ? "primary" : "secondary"}
            onClick={() => setLogType(f.value)}
          >
            {f.label}
          </Button>
        ))}
      </div>

      {logType === "intake" && (
        <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <FilterInput
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search name, phone, source…"
            className="max-w-sm w-auto"
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
      )}

      {logType === "webhooks" && (
        <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <FilterSelect
            className="h-7 min-w-[10rem] w-auto"
            value={webhookId === "" ? "" : String(webhookId)}
            onChange={(e) => setWebhookId(e.target.value === "" ? "" : Number(e.target.value))}
          >
            <option value="">All webhooks</option>
            {(webhooks ?? []).map((w) => (
              <option key={w.id} value={w.id}>
                {w.name}
              </option>
            ))}
          </FilterSelect>
          <div className="flex flex-wrap gap-2">
            {WEBHOOK_DELIVERY_FILTERS.map((f) => (
              <Button
                key={f.value}
                size="sm"
                variant={webhookStatus === f.value ? "primary" : "secondary"}
                onClick={() => setWebhookStatus(f.value)}
              >
                {f.label}
              </Button>
            ))}
          </div>
        </div>
      )}

      <UnifiedInboundLogTable
        rows={rows}
        total={total}
        page={page}
        limit={limit}
        isLoading={isLoading}
        emptyTitle={emptyMessage}
        hasFilters={hasFilters}
        readOnly={readOnly}
        canReplayWebhooks={canReplayWebhooks}
        onPageChange={setPage}
        onLimitChange={setLimit}
        onWebhookReplayed={refetchWebhooks}
      />
    </>
  );
}
