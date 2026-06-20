import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  useLeads,
  useCustomFields,
  useBulkLeads,
  useUsers,
  fetchAllLeadIds,
  deleteLeadsWithProgress,
  type BulkLeadAction,
} from "@/features/leads/hooks";
import { DeleteLeadConfirmDialog } from "@/features/leads/DeleteLeadConfirmDialog";
import { LeadTagBadges } from "@/features/leads/LeadTagsEditor";
import { LeadsColumnPicker } from "@/features/leads/LeadsColumnPicker";
import { LeadFilterBuilder } from "@/features/leads/LeadFilterBuilder";
import { LeadSearchInput } from "@/features/leads/LeadSearchInput";
import { LeadViewsMenu } from "@/features/leads/LeadViewsMenu";
import { NewLeadDrawer } from "@/features/leads/NewLeadDrawer";
import { ImportLeadsModal } from "@/features/leads/ImportLeadsModal";
import {
  useSavedLeadViews,
  useActiveViewId,
  mergeViews,
  getViewById,
  filtersViewChanged,
  type FilterCondition,
  type SavedLeadView,
} from "@/features/leads/leadsViews";
import { loadListUi, saveListUi, normalizeListColumns } from "@/features/leads/leadsUiStorage";
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
import { errorMessage } from "@/lib/api";
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
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [selectAllMatching, setSelectAllMatching] = useState(false);
  const [fetchingIds, setFetchingIds] = useState(false);
  const [bulkOpen, setBulkOpen] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [assignOpen, setAssignOpen] = useState<"user" | "follower" | "buyer" | null>(null);
  const [assignTarget, setAssignTarget] = useState(0);

  const { data: users } = useUsers();
  const { data: customFields, isLoading: customFieldsLoading } = useCustomFields();
  const { data: contracts } = useContracts(isPublisher);

  const applyViewFilters = useCallback((view: SavedLeadView) => {
    setConditions([...view.filters]);
    setSearch("");
    setDebouncedSearch("");
    setPage(1);
  }, []);

  const filtersChanged = filtersViewChanged(activeView, conditions);

  const filters = useMemo(
    () => ({
      view_id: filtersChanged ? undefined : activeId,
      filters: filtersChanged ? JSON.stringify(conditions) : undefined,
      q: debouncedSearch || undefined,
      page,
      limit,
      sort,
      sort_dir: sortDir,
    }),
    [filtersChanged, activeId, conditions, debouncedSearch, page, limit, sort, sortDir]
  );

  const bulkListFilters = useMemo(
    () => ({
      view_id: filtersChanged ? undefined : activeId,
      filters: filtersChanged ? JSON.stringify(conditions) : undefined,
      q: debouncedSearch || undefined,
      sort,
      sort_dir: sortDir,
    }),
    [filtersChanged, activeId, conditions, debouncedSearch, sort, sortDir]
  );

  const { data, isLoading, isError, error } = useLeads(filters);
  const bulk = useBulkLeads();
  const openDetail = useUIStore((s) => s.openDetail);

  const leads = data?.items ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / limit));
  const selectedCount = selectAllMatching ? total : selected.size;
  const hasSelection = selectAllMatching || selected.size > 0;
  const showSelectAllBanner =
    isAdmin &&
    !selectAllMatching &&
    selected.size === leads.length &&
    leads.length > 0 &&
    total > leads.length;
  const showAllSelectedBanner = isAdmin && selectAllMatching && total > 0;

  function clearSelection() {
    setSelected(new Set());
    setSelectAllMatching(false);
  }

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search.trim()), 300);
    return () => clearTimeout(t);
  }, [search]);

  useEffect(() => {
    setPage(1);
  }, [conditions, limit, debouncedSearch]);

  useEffect(() => {
    setSelected(new Set());
    setSelectAllMatching(false);
  }, [conditions, limit, sort, sortDir, activeId, filtersChanged, debouncedSearch]);

  const allColumnIds = useMemo(() => {
    const custom = (customFields ?? [])
      .filter((f) => f.is_active)
      .map((f) => `custom_${f.id}`);
    return [...SYSTEM_COLUMNS.map((c) => c.id), ...custom];
  }, [customFields]);

  useEffect(() => {
    if (viewsLoading || activeLoading || customFieldsLoading || viewApplied.current || !user?.id) return;
    applyViewFilters(activeView);
    const stored = loadListUi(user.id, allColumnIds);
    if (stored) {
      setSort(stored.sort);
      setSortDir(stored.sort_dir);
      setVisibleCols(stored.columns);
    } else {
      setSort(activeView.sort ?? "created_at");
      setSortDir(activeView.sort_dir ?? "desc");
      setVisibleCols(
        activeView.columns?.length ? [...activeView.columns] : [...DEFAULT_VISIBLE_COLUMNS]
      );
    }
    viewApplied.current = true;
  }, [viewsLoading, activeLoading, customFieldsLoading, activeView, applyViewFilters, user?.id, allColumnIds]);

  const activeCols = visibleCols.filter((id) => allColumnIds.includes(id));

  const updateVisibleCols = useCallback(
    (cols: string[]) => {
      const normalized = normalizeListColumns(cols, allColumnIds);
      setVisibleCols(normalized);
      if (user?.id) saveListUi(user.id, { columns: normalized });
    },
    [allColumnIds, user?.id]
  );

  function toggleSort(colId: string) {
    const key = columnSortKey(colId);
    if (!key) return;
    if (sort === key) {
      const nextDir = sortDir === "asc" ? "desc" : "asc";
      setSortDir(nextDir);
      if (user?.id) saveListUi(user.id, { sort_dir: nextDir });
    } else {
      setSort(key);
      setSortDir("asc");
      if (user?.id) saveListUi(user.id, { sort: key, sort_dir: "asc" });
    }
  }

  function toggleRow(id: number) {
    if (selectAllMatching) {
      setSelectAllMatching(false);
      setSelected(new Set(leads.map((l) => l.id).filter((x) => x !== id)));
      return;
    }
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleAll() {
    if (selectAllMatching || (selected.size === leads.length && leads.length > 0)) {
      clearSelection();
    } else {
      setSelected(new Set(leads.map((l) => l.id)));
      setSelectAllMatching(false);
    }
  }

  async function runBulk(action: BulkLeadAction, extra?: { user_id?: number; contract_id?: number }) {
    try {
      setFetchingIds(true);
      const ids = selectAllMatching ? await fetchAllLeadIds(bulkListFilters) : [...selected];
      if (!ids.length) return;

      if (action === "delete") {
        const affected = await deleteLeadsWithProgress(ids, (body) => bulk.mutateAsync(body));
        toast.success(`Deleted ${affected.toLocaleString()} lead${affected === 1 ? "" : "s"}`);
      } else {
        const result = await bulk.mutateAsync({ action, ids, ...extra });
        toast.success(`${result.affected} lead${result.affected === 1 ? "" : "s"} updated`);
      }

      clearSelection();
      setConfirmDelete(false);
      setAssignOpen(null);
      setBulkOpen(false);
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setFetchingIds(false);
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
            onViewApply={applyViewFilters}
          />
          <Button variant="outline" size="sm" onClick={() => setFiltersExpanded((e) => !e)}>
            {filtersExpanded ? "Hide filters" : "Edit filters"}
          </Button>
          <LeadSearchInput
            value={search}
            onChange={setSearch}
            className="w-72"
            inputClassName="h-7 text-sm"
            leadFilters={bulkListFilters}
            onSelectLead={(lead) => openDetail(lead.id)}
          />
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
            {isAdmin && hasSelection && (
              <Dropdown
                open={bulkOpen}
                onClose={() => setBulkOpen(false)}
                trigger={
                  <Button variant="secondary" size="sm" onClick={() => setBulkOpen((o) => !o)}>
                    Bulk actions ({selectedCount})
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
              defaultCols={DEFAULT_VISIBLE_COLUMNS}
              lockedCols={DEFAULT_VISIBLE_COLUMNS}
              onChange={updateVisibleCols}
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
          <EmptyState title="Could not load leads." subtitle={errorMessage(error)} />
        ) : leads.length === 0 ? (
          <EmptyState title="No leads match these filters." />
        ) : (
          <>
            {(showSelectAllBanner || showAllSelectedBanner) && (
              <div className="mb-2 rounded-lg border border-blue-100 bg-blue-50 px-4 py-2 text-sm text-gray-700">
                {showSelectAllBanner ? (
                  <>
                    All {leads.length} leads on this page are selected.{" "}
                    <button
                      type="button"
                      className="font-medium text-blue-600 hover:underline"
                      onClick={() => setSelectAllMatching(true)}
                    >
                      Select all {total} leads in {activeView.name}
                    </button>
                  </>
                ) : (
                  <>
                    All {total} leads selected.{" "}
                    <button
                      type="button"
                      className="font-medium text-blue-600 hover:underline"
                      onClick={clearSelection}
                    >
                      Clear selection
                    </button>
                  </>
                )}
              </div>
            )}
            <Table>
              <THead>
                <tr>
                  {isAdmin && (
                    <TH className="w-10 min-w-10">
                      <input
                        type="checkbox"
                        checked={
                          selectAllMatching ||
                          (selected.size === leads.length && leads.length > 0)
                        }
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
                      <TD className="w-10 min-w-10">
                        <div onClick={(e) => e.stopPropagation()}>
                          <input
                            type="checkbox"
                            checked={selectAllMatching || selected.has(l.id)}
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

      <DeleteLeadConfirmDialog
        open={confirmDelete}
        onClose={() => setConfirmDelete(false)}
        count={selectedCount}
        loading={bulk.isPending || fetchingIds}
        onConfirm={() => runBulk("delete")}
      />

      <Dialog
        open={assignOpen === "user"}
        onClose={() => setAssignOpen(null)}
        title="Assign to user"
        subtitle={`Assign ${selectedCount} lead${selectedCount === 1 ? "" : "s"} to a team member.`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setAssignOpen(null)}>
              Cancel
            </Button>
            <Button
              disabled={!assignTarget || bulk.isPending || fetchingIds}
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
        subtitle={`Add a follower to ${selectedCount} selected lead${selectedCount === 1 ? "" : "s"}.`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setAssignOpen(null)}>
              Cancel
            </Button>
            <Button
              disabled={!assignTarget || bulk.isPending || fetchingIds}
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
        subtitle={`Re-distribute ${selectedCount} returned lead${selectedCount === 1 ? "" : "s"} via contract.`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setAssignOpen(null)}>
              Cancel
            </Button>
            <Button
              disabled={!assignTarget || bulk.isPending || fetchingIds}
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
