import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, patch, post, del, ns } from "@/lib/api";
import { useAuthStore } from "@/store/authStore";
import { loadActiveViewId, saveActiveViewId } from "./leadsUiStorage";
import {
  DEFAULT_VISIBLE_COLUMNS,
} from "./leadsListColumns";

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
    label: "Action Date & Time",
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
    name: "Action Date & Time today",
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

export function filtersViewChanged(view: SavedLeadView, conditions: FilterCondition[]): boolean {
  return !filtersEqual(view.filters, conditions);
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
    onSuccess: (created, vars) => {
      const placement = vars.placement === "list" ? "list" : "board";
      qc.setQueryData<SavedLeadView[]>(["lead-views", placement], (old) => [
        ...(old ?? []),
        created,
      ]);
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
  const userId = useAuthStore((s) => s.user?.id);
  const { data: apiViews, isLoading: viewsLoading } = useSavedLeadViews(placement);
  const views = useMemo(() => mergeViews(apiViews, placement), [apiViews, placement]);
  const [activeId, setActiveIdState] = useState("all");
  const hydrated = useRef(false);

  useEffect(() => {
    hydrated.current = false;
  }, [userId, placement]);

  useEffect(() => {
    if (!userId || viewsLoading || hydrated.current) return;
    const stored = loadActiveViewId(userId, placement) ?? "all";
    const valid = views.some((v) => v.public_id === stored) ? stored : "all";
    setActiveIdState(valid);
    if (valid !== stored) saveActiveViewId(userId, placement, valid);
    hydrated.current = true;
  }, [userId, placement, viewsLoading, views]);

  const setActiveId = useCallback(
    (viewId: string) => {
      setActiveIdState(viewId);
      if (userId) saveActiveViewId(userId, placement, viewId);
    },
    [userId, placement]
  );

  return {
    activeId,
    setActiveId,
    isLoading: !userId || viewsLoading || !hydrated.current,
  };
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
