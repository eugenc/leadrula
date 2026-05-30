import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  useLeads,
  useCustomFields,
  useBulkLeads,
  useUsers,
  type BulkLeadAction,
} from "@/features/leads/hooks";
import { LeadTagBadges } from "@/features/leads/LeadTagsEditor";
import { LeadsColumnPicker } from "@/features/leads/LeadsColumnPicker";
import { LeadFilterBuilder } from "@/features/leads/LeadFilterBuilder";
import { LeadViewsMenu } from "@/features/leads/LeadViewsMenu";
import { NewLeadDrawer } from "@/features/leads/NewLeadDrawer";
import { ImportLeadsModal } from "@/features/leads/ImportLeadsModal";
import {
  useSavedLeadViews,
  useActiveViewId,
  mergeViews,
  getViewById,
  viewStateEqual,
  type FilterCondition,
  type SavedLeadView,
} from "@/features/leads/leadsViews";
import { useContracts } from "@/features/admin/hooks";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Badge, Spinner, EmptyState } from "@/components/ui/misc";
import { FilterSelect } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Dropdown, DropdownItem } from "@/components/ui/dropdown";
import { PageBody } from "@/components/layout/PageBody";
import { useUIStore } from "@/store/uiStore";
import { useAuthStore } from "@/store/authStore";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";
import { cn } from "@/lib/utils";
import {
  ChevronDown,
  ChevronUp,
  ChevronsUpDown,
  Trash2,
  UserPlus,
  Users,
  Building2,
  Plus,
  Upload,
} from "lucide-react";
import {
  DEFAULT_VISIBLE_COLUMNS,
  SYSTEM_COLUMNS,
  cellValue,
  columnLabel,
  columnSortKey,
  formatStatus,
} from "@/features/leads/leadsListColumns";

const PAGE_SIZES = [25, 50, 100] as const;

const statusVariant: Record<
  string,
  "distributed" | "returned" | "review" | "closed" | "default"
> = {
  distributed: "distributed",
  returned: "returned",
  review: "review",
  closed: "closed",
};

export function LeadsListPage() {
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";
  const isPublisher = user?.account_type === "publisher";
  const canCreate = user?.role === "admin" || user?.role === "user";

  const [newLeadOpen, setNewLeadOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);

  const { data: apiViews, isLoading: viewsLoading } = useSavedLeadViews("list");
  const views = useMemo(() => mergeViews(apiViews, "list"), [apiViews]);
  const { activeId, isLoading: activeLoading } = useActiveViewId("list");
  const activeView = getViewById(views, activeId);
  const viewApplied = useRef(false);

  const [conditions, setConditions] = useState<FilterCondition[]>([]);
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState<number>(25);
  const [sort, setSort] = useState("created_at");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [visibleCols, setVisibleCols] = useState<string[]>(DEFAULT_VISIBLE_COLUMNS);
  const [colsOpen, setColsOpen] = useState(false);
  const [filtersExpanded, setFiltersExpanded] = useState(false);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [bulkOpen, setBulkOpen] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [assignOpen, setAssignOpen] = useState<"user" | "follower" | "buyer" | null>(null);
  const [assignTarget, setAssignTarget] = useState(0);

  const { data: users } = useUsers();
  const { data: customFields } = useCustomFields();
  const { data: contracts } = useContracts(isPublisher);

  const applyView = useCallback((view: SavedLeadView) => {
    setConditions([...view.filters]);
    setSort(view.sort ?? "created_at");
    setSortDir(view.sort_dir ?? "desc");
    setVisibleCols(view.columns?.length ? [...view.columns] : [...DEFAULT_VISIBLE_COLUMNS]);
    setPage(1);
  }, []);

  useEffect(() => {
    if (viewsLoading || activeLoading || viewApplied.current) return;
    applyView(activeView);
    viewApplied.current = true;
  }, [viewsLoading, activeLoading, activeView, applyView]);

  const viewChanged = !viewStateEqual(activeView, {
    filters: conditions,
    columns: visibleCols,
    sort,
    sort_dir: sortDir,
  });

  const filters = useMemo(
    () => ({
      view_id: viewChanged ? undefined : activeId,
      filters: viewChanged ? JSON.stringify(conditions) : undefined,
      page,
      limit,
      sort,
      sort_dir: sortDir,
    }),
    [viewChanged, activeId, conditions, page, limit, sort, sortDir]
  );

  const { data, isLoading, isError, error } = useLeads(filters);
  const bulk = useBulkLeads();
  const openDetail = useUIStore((s) => s.openDetail);

  const leads = data?.items ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / limit));

  useEffect(() => {
    setPage(1);
  }, [conditions, limit]);

  useEffect(() => {
    setSelected(new Set());
  }, [filters]);

  const allColumnIds = useMemo(() => {
    const custom = (customFields ?? [])
      .filter((f) => f.is_active)
      .map((f) => `custom_${f.id}`);
    return [...SYSTEM_COLUMNS.map((c) => c.id), ...custom];
  }, [customFields]);

  const activeCols = visibleCols.filter((id) => allColumnIds.includes(id));

  function toggleSort(colId: string) {
    const key = columnSortKey(colId);
    if (!key) return;
    if (sort === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSort(key);
      setSortDir("asc");
    }
  }

  function toggleRow(id: number) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleAll() {
    if (selected.size === leads.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(leads.map((l) => l.id)));
    }
  }

  async function runBulk(action: BulkLeadAction, extra?: { user_id?: number; contract_id?: number }) {
    const ids = [...selected];
    if (!ids.length) return;
    try {
      const result = await bulk.mutateAsync({ action, ids, ...extra });
      toast.success(`${result.affected} lead${result.affected === 1 ? "" : "s"} updated`);
      setSelected(new Set());
      setConfirmDelete(false);
      setAssignOpen(null);
      setBulkOpen(false);
    } catch (err) {
      toast.error(apiError(err).message);
    }
  }

  const activeUsers = (users ?? []).filter((u) => u.status === "active");

  return (
    <>
      <PageBody>
        <div className="mb-4 flex flex-wrap items-center gap-2">
          <LeadViewsMenu
            placement="list"
            filters={conditions}
            onFiltersChange={setConditions}
            columns={visibleCols}
            sort={sort}
            sortDir={sortDir}
            onViewApply={applyView}
          />
          <Button variant="ghost" size="sm" onClick={() => setFiltersExpanded((e) => !e)}>
            {filtersExpanded ? "Hide filters" : "Edit filters"}
          </Button>
          <div className="ml-auto flex items-center gap-2">
            {canCreate && (
              <>
                <Button variant="outline" size="sm" onClick={() => setImportOpen(true)}>
                  <Upload className="h-4 w-4" />
                  Import CSV
                </Button>
                <Button size="sm" onClick={() => setNewLeadOpen(true)}>
                  <Plus className="h-4 w-4" />
                  New Lead
                </Button>
              </>
            )}
            {isAdmin && selected.size > 0 && (
              <Dropdown
                open={bulkOpen}
                onClose={() => setBulkOpen(false)}
                trigger={
                  <Button variant="secondary" size="sm" onClick={() => setBulkOpen((o) => !o)}>
                    Bulk actions ({selected.size})
                    <ChevronDown className="h-4 w-4" />
                  </Button>
                }
              >
                <DropdownItem onClick={() => { setAssignTarget(0); setAssignOpen("user"); setBulkOpen(false); }}>
                  <UserPlus className="mr-2 inline h-4 w-4" />
                  Assign to user
                </DropdownItem>
                <DropdownItem onClick={() => { setAssignTarget(0); setAssignOpen("follower"); setBulkOpen(false); }}>
                  <Users className="mr-2 inline h-4 w-4" />
                  Add follower
                </DropdownItem>
                {isPublisher && (
                  <DropdownItem onClick={() => { setAssignTarget(0); setAssignOpen("buyer"); setBulkOpen(false); }}>
                    <Building2 className="mr-2 inline h-4 w-4" />
                    Assign to buyer
                  </DropdownItem>
                )}
                <DropdownItem
                  className="text-danger hover:bg-danger-bg"
                  onClick={() => {
                    setBulkOpen(false);
                    setConfirmDelete(true);
                  }}
                >
                  <Trash2 className="mr-2 inline h-4 w-4" />
                  Delete
                </DropdownItem>
              </Dropdown>
            )}
            <LeadsColumnPicker
              open={colsOpen}
              onOpenChange={setColsOpen}
              visibleCols={visibleCols}
              allColumnIds={allColumnIds}
              customFields={customFields ?? []}
              onChange={setVisibleCols}
            />
          </div>
        </div>

        {filtersExpanded && (
          <div className="mb-4 rounded-lg border border-gray-100 bg-gray-50/50 p-4">
            <LeadFilterBuilder conditions={conditions} onChange={setConditions} />
          </div>
        )}

        {isLoading || viewsLoading || activeLoading ? (
          <div className="flex justify-center py-16">
            <Spinner className="h-6 w-6" />
          </div>
        ) : isError ? (
          <EmptyState title="Could not load leads." subtitle={apiError(error).message} />
        ) : leads.length === 0 ? (
          <EmptyState title="No leads match these filters." />
        ) : (
          <>
            <Table>
              <THead>
                <tr>
                  {isAdmin && (
                    <TH className="w-10">
                      <input
                        type="checkbox"
                        checked={selected.size === leads.length && leads.length > 0}
                        onChange={toggleAll}
                        aria-label="Select all"
                      />
                    </TH>
                  )}
                  {activeCols.map((colId) => {
                    const sortKey = columnSortKey(colId);
                    const sorted = sortKey && sort === sortKey;
                    return (
                      <TH key={colId}>
                        <button
                          type="button"
                          disabled={!sortKey}
                          onClick={() => toggleSort(colId)}
                          className={cn(
                            "inline-flex items-center gap-1",
                            sortKey && "cursor-pointer hover:text-gray-800"
                          )}
                        >
                          {columnLabel(colId, customFields ?? [])}
                          {sortKey &&
                            (sorted ? (
                              sortDir === "asc" ? (
                                <ChevronUp className="h-3.5 w-3.5" />
                              ) : (
                                <ChevronDown className="h-3.5 w-3.5" />
                              )
                            ) : (
                              <ChevronsUpDown className="h-3.5 w-3.5 opacity-40" />
                            ))}
                        </button>
                      </TH>
                    );
                  })}
                </tr>
              </THead>
              <TBody>
                {leads.map((l) => (
                  <TR key={l.id} onClick={() => openDetail(l.id)}>
                    {isAdmin && (
                      <TD className="w-10">
                        <div onClick={(e) => e.stopPropagation()}>
                          <input
                            type="checkbox"
                            checked={selected.has(l.id)}
                            onChange={() => toggleRow(l.id)}
                            aria-label={`Select lead ${l.id}`}
                          />
                        </div>
                      </TD>
                    )}
                    {activeCols.map((colId) => (
                      <TD
                        key={colId}
                        className={cn(colId === "name" && "font-medium text-gray-800")}
                      >
                        {colId === "status" ? (
                          <Badge variant={statusVariant[l.status] ?? "default"}>
                            {formatStatus(l.status)}
                          </Badge>
                        ) : colId === "tags" ? (
                          (l.tags ?? []).length ? (
                            <LeadTagBadges tags={l.tags ?? []} />
                          ) : (
                            "—"
                          )
                        ) : (
                          cellValue(l, colId, customFields ?? [])
                        )}
                      </TD>
                    ))}
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
      </PageBody>

      <Dialog
        open={confirmDelete}
        onClose={() => setConfirmDelete(false)}
        title="Delete leads?"
        subtitle={`Permanently delete ${selected.size} selected lead${selected.size === 1 ? "" : "s"}?`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirmDelete(false)}>
              Cancel
            </Button>
            <Button variant="danger" disabled={bulk.isPending} onClick={() => runBulk("delete")}>
              Delete
            </Button>
          </>
        }
      >
        <span />
      </Dialog>

      <Dialog
        open={assignOpen === "user"}
        onClose={() => setAssignOpen(null)}
        title="Assign to user"
        subtitle={`Assign ${selected.size} lead${selected.size === 1 ? "" : "s"} to a team member.`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setAssignOpen(null)}>
              Cancel
            </Button>
            <Button
              disabled={!assignTarget || bulk.isPending}
              onClick={() => runBulk("assign_user", { user_id: assignTarget })}
            >
              Assign
            </Button>
          </>
        }
      >
        <FilterSelect
          value={assignTarget}
          onChange={(e) => setAssignTarget(Number(e.target.value))}
          className="w-full"
        >
          <option value={0}>Select user…</option>
          {activeUsers.map((u) => (
            <option key={u.id} value={u.id}>
              {u.full_name} ({u.role})
            </option>
          ))}
        </FilterSelect>
      </Dialog>

      <Dialog
        open={assignOpen === "follower"}
        onClose={() => setAssignOpen(null)}
        title="Add follower"
        subtitle={`Add a follower to ${selected.size} selected lead${selected.size === 1 ? "" : "s"}.`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setAssignOpen(null)}>
              Cancel
            </Button>
            <Button
              disabled={!assignTarget || bulk.isPending}
              onClick={() => runBulk("add_follower", { user_id: assignTarget })}
            >
              Add follower
            </Button>
          </>
        }
      >
        <FilterSelect
          value={assignTarget}
          onChange={(e) => setAssignTarget(Number(e.target.value))}
          className="w-full"
        >
          <option value={0}>Select user…</option>
          {activeUsers.map((u) => (
            <option key={u.id} value={u.id}>
              {u.full_name} ({u.role})
            </option>
          ))}
        </FilterSelect>
      </Dialog>

      <Dialog
        open={assignOpen === "buyer"}
        onClose={() => setAssignOpen(null)}
        title="Assign to buyer"
        subtitle={`Re-distribute ${selected.size} returned lead${selected.size === 1 ? "" : "s"} via contract.`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setAssignOpen(null)}>
              Cancel
            </Button>
            <Button
              disabled={!assignTarget || bulk.isPending}
              onClick={() => runBulk("assign_buyer", { contract_id: assignTarget })}
            >
              Assign
            </Button>
          </>
        }
      >
        <FilterSelect
          value={assignTarget}
          onChange={(e) => setAssignTarget(Number(e.target.value))}
          className="w-full"
        >
          <option value={0}>Select contract…</option>
          {(contracts ?? []).map((c) => (
            <option key={c.id} value={c.id}>
              {c.name} — {c.buyer_name ?? `Buyer #${c.buyer_id}`}
            </option>
          ))}
        </FilterSelect>
      </Dialog>

      <NewLeadDrawer open={newLeadOpen} onClose={() => setNewLeadOpen(false)} />
      <ImportLeadsModal open={importOpen} onClose={() => setImportOpen(false)} />
    </>
  );
}
