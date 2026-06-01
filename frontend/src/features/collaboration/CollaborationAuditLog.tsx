import { useState } from "react";
import { useBuyerLogs, useBuyerLogActors } from "@/features/admin/hooks";
import { Card, Spinner, EmptyState } from "@/components/ui/misc";
import { Button } from "@/components/ui/button";
import { Input, Label, FilterSelect } from "@/components/ui/input";
import { formatCollaborationAuditEntry } from "@/lib/collaborationAudit";

const PAGE_SIZES = [25, 50, 100] as const;

function toISO(value: string): string | undefined {
  if (!value.trim()) return undefined;
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return undefined;
  return d.toISOString();
}

type AppliedFilters = {
  from?: string;
  to?: string;
  actor_user_id?: number;
};

export function CollaborationAuditLog() {
  const { data: actors } = useBuyerLogActors();
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState<number>(25);
  const [draftFrom, setDraftFrom] = useState("");
  const [draftTo, setDraftTo] = useState("");
  const [draftActorId, setDraftActorId] = useState("");
  const [applied, setApplied] = useState<AppliedFilters>({});

  const { data, isLoading } = useBuyerLogs({
    page,
    limit,
    from: applied.from,
    to: applied.to,
    actor_user_id: applied.actor_user_id,
  });

  const logs = data?.items ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / limit));
  const hasFilters = !!(applied.from || applied.to || applied.actor_user_id);

  function applyFilters() {
    setApplied({
      from: toISO(draftFrom),
      to: toISO(draftTo),
      actor_user_id: draftActorId ? Number(draftActorId) : undefined,
    });
    setPage(1);
  }

  function clearFilters() {
    setDraftFrom("");
    setDraftTo("");
    setDraftActorId("");
    setApplied({});
    setPage(1);
  }

  return (
    <Card className="p-5">
      <h2 className="mb-1 text-sm font-semibold text-gray-800">Activity log</h2>
      <p className="mb-4 text-sm text-gray-500">
        Collaboration invitations, access changes, and actions taken while a publisher is impersonating this account.
      </p>

      <div className="mb-4 flex flex-col gap-3 rounded-lg border border-gray-100 bg-gray-50 p-4">
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <Label>From</Label>
            <Input
              type="datetime-local"
              value={draftFrom}
              onChange={(e) => setDraftFrom(e.target.value)}
            />
          </div>
          <div>
            <Label>To</Label>
            <Input type="datetime-local" value={draftTo} onChange={(e) => setDraftTo(e.target.value)} />
          </div>
          <div>
            <Label>User</Label>
            <FilterSelect value={draftActorId} onChange={(e) => setDraftActorId(e.target.value)}>
              <option value="">All users</option>
              {(actors ?? []).map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </FilterSelect>
          </div>
          <div className="flex items-end gap-2">
            <Button onClick={applyFilters}>Apply filters</Button>
            {hasFilters && (
              <Button variant="secondary" onClick={clearFilters}>
                Clear
              </Button>
            )}
          </div>
        </div>
      </div>

      {isLoading ? (
        <Spinner className="h-5 w-5" />
      ) : logs.length === 0 ? (
        <EmptyState title={hasFilters ? "No results." : "No activity logged yet."} />
      ) : (
        <>
          <ul className="space-y-2">
            {logs.map((e) => (
              <li key={e.id} className="border-b border-gray-100 pb-2 text-sm last:border-0">
                <span className="font-medium text-gray-700">{formatCollaborationAuditEntry(e)}</span>
                {e.actor_name ? <span className="text-gray-500"> — {e.actor_name}</span> : null}
                <span className="block text-xs text-gray-400">
                  {new Date(e.created_at).toLocaleString()}
                </span>
              </li>
            ))}
          </ul>

          <div className="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-gray-500">
            <span>
              {total === 0
                ? "No results"
                : `${(page - 1) * limit + 1}–${Math.min(page * limit, total)} of ${total}`}
            </span>
            <div className="flex items-center gap-3">
              <FilterSelect
                value={limit}
                onChange={(e) => {
                  setLimit(Number(e.target.value));
                  setPage(1);
                }}
                className="w-24"
              >
                {PAGE_SIZES.map((n) => (
                  <option key={n} value={n}>
                    {n} / page
                  </option>
                ))}
              </FilterSelect>
              <Button
                variant="secondary"
                size="sm"
                disabled={page <= 1}
                onClick={() => setPage((p) => p - 1)}
              >
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
    </Card>
  );
}
