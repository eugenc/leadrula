import { useMemo } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, patch, post, del, ns } from "@/lib/api";
import type { Me } from "@/types";
import { DEFAULT_VISIBLE_COLUMNS } from "./leadsListColumns";

export interface FilterCondition {
  field: string;
  op: string;
  value?: string | number;
}

export interface SavedLeadView {
  id?: number;
  public_id: string;
  name: string;
  placement: "list" | "board" | "both";
  filters: FilterCondition[];
  columns?: string[];
  sort?: string;
  sort_dir?: "asc" | "desc";
  is_builtin: boolean;
  builtin_key?: string;
  shared: boolean;
}

export type ViewPlacement = "list" | "board";

export const FILTER_FIELDS: {
  field: string;
  label: string;
  ops: { op: string; label: string; needsValue?: boolean; valueType?: "text" | "user" | "date" | "status" | "pipeline" | "stage" }[];
}[] = [
  {
    field: "assigned_user_id",
    label: "Assignee",
    ops: [
      { op: "equals", label: "is", needsValue: true, valueType: "user" },
      { op: "not_equals", label: "is not", needsValue: true, valueType: "user" },
      { op: "is_empty", label: "is empty" },
      { op: "is_not_empty", label: "is not empty" },
    ],
  },
  {
    field: "action_at",
    label: "Action date",
    ops: [
      { op: "on", label: "is on", needsValue: true, valueType: "date" },
      { op: "before", label: "is before", needsValue: true, valueType: "date" },
      { op: "after", label: "is after", needsValue: true, valueType: "date" },
      { op: "overdue", label: "is overdue" },
      { op: "is_empty", label: "is empty" },
      { op: "is_not_empty", label: "is not empty" },
    ],
  },
  {
    field: "status",
    label: "Status",
    ops: [
      { op: "equals", label: "is", needsValue: true, valueType: "status" },
      { op: "not_equals", label: "is not", needsValue: true, valueType: "status" },
    ],
  },
  {
    field: "pipeline_id",
    label: "Pipeline",
    ops: [{ op: "equals", label: "is", needsValue: true, valueType: "pipeline" }],
  },
  {
    field: "stage_id",
    label: "Stage",
    ops: [{ op: "equals", label: "is", needsValue: true, valueType: "stage" }],
  },
  {
    field: "tags",
    label: "Tags",
    ops: [{ op: "contains", label: "contains", needsValue: true, valueType: "text" }],
  },
  {
    field: "source",
    label: "Source",
    ops: [
      { op: "equals", label: "is", needsValue: true, valueType: "text" },
      { op: "contains", label: "contains", needsValue: true, valueType: "text" },
    ],
  },
  {
    field: "buyer_name",
    label: "Buyer",
    ops: [
      { op: "equals", label: "is", needsValue: true, valueType: "text" },
      { op: "contains", label: "contains", needsValue: true, valueType: "text" },
    ],
  },
];

export const BUILTIN_VIEWS: SavedLeadView[] = [
  {
    public_id: "all",
    name: "All leads",
    placement: "both",
    filters: [],
    columns: [...DEFAULT_VISIBLE_COLUMNS],
    sort: "created_at",
    sort_dir: "desc",
    is_builtin: true,
    builtin_key: "all",
    shared: false,
  },
  {
    public_id: "action_today",
    name: "Action date today",
    placement: "both",
    filters: [{ field: "action_at", op: "on", value: "today" }],
    columns: ["name", "assignee", "action_at", "status"],
    sort: "action_at",
    sort_dir: "asc",
    is_builtin: true,
    builtin_key: "action_today",
    shared: false,
  },
  {
    public_id: "overdue",
    name: "Overdue actions",
    placement: "both",
    filters: [{ field: "action_at", op: "overdue" }],
    columns: ["name", "assignee", "action_at", "status"],
    sort: "action_at",
    sort_dir: "asc",
    is_builtin: true,
    builtin_key: "overdue",
    shared: false,
  },
];

export function filtersEqual(a: FilterCondition[], b: FilterCondition[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((c, i) => {
    const d = b[i]!;
    return c.field === d.field && c.op === d.op && String(c.value ?? "") === String(d.value ?? "");
  });
}

export function viewStateEqual(
  view: SavedLeadView,
  state: { filters: FilterCondition[]; columns: string[]; sort: string; sort_dir: "asc" | "desc" }
): boolean {
  if (view.sort !== state.sort || view.sort_dir !== state.sort_dir) return false;
  if (view.columns?.length !== state.columns.length) return false;
  if (view.columns?.some((c, i) => c !== state.columns[i])) return false;
  return filtersEqual(view.filters, state.filters);
}

export function useSavedLeadViews(placement: ViewPlacement) {
  return useQuery({
    queryKey: ["lead-views", placement],
    queryFn: () => get<SavedLeadView[]>(`${ns()}/leads/views?placement=${placement}`),
  });
}

export function useCreateLeadView() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      name: string;
      placement: string;
      shared?: boolean;
      filters: FilterCondition[];
      columns?: string[];
      sort?: string;
      sort_dir?: string;
    }) => post<SavedLeadView>(`${ns()}/leads/views`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["lead-views"] });
    },
  });
}

export function useUpdateLeadView() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      body,
    }: {
      id: string;
      body: Partial<{
        name: string;
        placement: string;
        filters: FilterCondition[];
        columns: string[];
        sort: string;
        sort_dir: string;
      }>;
    }) => patch<SavedLeadView>(`${ns()}/leads/views/${id}`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["lead-views"] });
    },
  });
}

export function useDeleteLeadView() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => del(`${ns()}/leads/views/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["lead-views"] });
    },
  });
}

export function useActiveViewId(placement: ViewPlacement) {
  const qc = useQueryClient();
  const prefKey = placement === "list" ? "active_list_view_id" : "active_board_view_id";

  const { data: me, isLoading } = useQuery({
    queryKey: ["me"],
    queryFn: () => get<Me>("/auth/me"),
  });

  const activeId = useMemo(() => {
    const id = me?.user.prefs?.[prefKey];
    return typeof id === "string" ? id : "all";
  }, [me, prefKey]);

  const setActiveId = useMutation({
    mutationFn: (viewId: string) => patch<Record<string, unknown>>("/auth/me/prefs", { [prefKey]: viewId }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["me"] }),
  });

  return { activeId, setActiveId: setActiveId.mutateAsync, isLoading };
}

export function mergeViews(apiViews: SavedLeadView[] | undefined, placement: ViewPlacement): SavedLeadView[] {
  const byId = new Map<string, SavedLeadView>();
  for (const v of BUILTIN_VIEWS) {
    if (v.placement === "both" || v.placement === placement) {
      byId.set(v.public_id, v);
    }
  }
  for (const v of apiViews ?? []) {
    byId.set(v.public_id, v);
  }
  return [...byId.values()];
}

export function getViewById(views: SavedLeadView[], id: string): SavedLeadView {
  return views.find((v) => v.public_id === id) ?? views[0]!;
}
