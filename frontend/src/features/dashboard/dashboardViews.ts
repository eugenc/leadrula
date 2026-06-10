import { useMemo } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, patch, post, del, ns } from "@/lib/api";
import type { Me } from "@/types";

export type DashboardPeriod = "today" | "yesterday" | "7days" | "custom";

export interface DashboardPeriodState {
  period: DashboardPeriod;
  customFrom?: string;
  customTo?: string;
}

export type WidgetId =
  | "leads_received"
  | "leads_sold"
  | "lead_costs"
  | "lead_sales"
  | "lead_profits"
  | "per_lead_profit"
  | "per_lead_cost"
  | "conversion_rate"
  | "return_rate"
  | "new_buyers"
  | "high_paying_buyer"
  | "avg_lead_value"
  | "top_sources"
  | "buyer_leaderboard"
  | "pipeline_funnel"
  | "daily_trend"
  | "recent_activity"
  | "goals"
  | "payout_hold"
  | "payout_cleared";

export interface DashboardGoals {
  lead_target?: number;
  revenue_target?: number;
}

export interface DashboardView {
  id?: number;
  public_id: string;
  name: string;
  widgets: WidgetId[];
  period: string;
  goals: DashboardGoals;
  shared: boolean;
  position: number;
}

export const DASHBOARD_VIEW_PREF_KEY = "dashboard_view_id";
export const DASHBOARD_PERIOD_PREF_KEY = "dashboard_period";
export const DASHBOARD_PERIOD_FROM_KEY = "dashboard_period_from";
export const DASHBOARD_PERIOD_TO_KEY = "dashboard_period_to";

export const DEFAULT_PERIOD_STATE: DashboardPeriodState = { period: "7days" };

export const PERIOD_OPTIONS: { value: DashboardPeriod; label: string }[] = [
  { value: "today", label: "Today" },
  { value: "yesterday", label: "Yesterday" },
  { value: "7days", label: "7 days" },
  { value: "custom", label: "Custom date" },
];

export const PUBLISHER_WIDGETS: { id: WidgetId; label: string }[] = [
  { id: "leads_received", label: "Leads Received" },
  { id: "leads_sold", label: "Leads Sold" },
  { id: "lead_costs", label: "Lead Costs" },
  { id: "lead_sales", label: "Lead Sales" },
  { id: "lead_profits", label: "Lead Profits" },
  { id: "per_lead_profit", label: "Per Lead Profit" },
  { id: "conversion_rate", label: "Conversion Rate" },
  { id: "return_rate", label: "Return Rate" },
  { id: "new_buyers", label: "New Buyers" },
  { id: "high_paying_buyer", label: "High Paying Buyer" },
  { id: "avg_lead_value", label: "Avg Lead Value" },
  { id: "top_sources", label: "Top Sources" },
  { id: "buyer_leaderboard", label: "Buyer Leaderboard" },
  { id: "pipeline_funnel", label: "Pipeline Funnel" },
  { id: "daily_trend", label: "Daily Trend" },
  { id: "recent_activity", label: "Recent Activity" },
  { id: "goals", label: "Goals" },
  { id: "payout_hold", label: "Payout Hold" },
  { id: "payout_cleared", label: "Payout Cleared" },
];

export const BUYER_WIDGETS: { id: WidgetId; label: string }[] = [
  { id: "leads_received", label: "Leads Received" },
  { id: "leads_sold", label: "Leads Sold" },
  { id: "lead_costs", label: "Lead Costs" },
  { id: "per_lead_cost", label: "Per Lead Cost" },
  { id: "conversion_rate", label: "Conversion Rate" },
  { id: "return_rate", label: "Return Rate" },
  { id: "pipeline_funnel", label: "Pipeline Funnel" },
  { id: "daily_trend", label: "Daily Trend" },
  { id: "recent_activity", label: "Recent Activity" },
  { id: "goals", label: "Goals" },
];

export const DEFAULT_PUBLISHER_VIEW: DashboardView = {
  public_id: "default",
  name: "Default",
  widgets: [],
  period: "all",
  goals: {},
  shared: true,
  position: 0,
};

export const DEFAULT_BUYER_VIEW: DashboardView = {
  public_id: "default",
  name: "Default",
  widgets: [],
  period: "all",
  goals: {},
  shared: true,
  position: 0,
};

export function widgetCatalog(isBuyer: boolean) {
  return isBuyer ? BUYER_WIDGETS : PUBLISHER_WIDGETS;
}

export function defaultDashboardView(isBuyer: boolean): DashboardView {
  return isBuyer ? DEFAULT_BUYER_VIEW : DEFAULT_PUBLISHER_VIEW;
}

export function useDashboardViews() {
  return useQuery({
    queryKey: ["dashboard-views"],
    queryFn: () => get<DashboardView[]>(`${ns()}/dashboard/views`),
  });
}

export function useCreateDashboardView() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      name: string;
      widgets: WidgetId[];
      period?: string;
      goals?: DashboardGoals;
      position?: number;
    }) => post<DashboardView>(`${ns()}/dashboard/views`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["dashboard-views"] }),
  });
}

export function useUpdateDashboardView() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      body,
    }: {
      id: string;
      body: Partial<{
        name: string;
        widgets: WidgetId[];
        period: string;
        goals: DashboardGoals;
        position: number;
      }>;
    }) => patch<DashboardView>(`${ns()}/dashboard/views/${id}`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["dashboard-views"] }),
  });
}

export function useDeleteDashboardView() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => del(`${ns()}/dashboard/views/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["dashboard-views"] }),
  });
}

export function useActiveDashboardViewId() {
  const qc = useQueryClient();

  const { data: me, isLoading } = useQuery({
    queryKey: ["me"],
    queryFn: () => get<Me>("/auth/me"),
  });

  const activeId = useMemo(() => {
    const id = me?.user.prefs?.[DASHBOARD_VIEW_PREF_KEY];
    return typeof id === "string" ? id : "";
  }, [me]);

  const setActiveId = useMutation({
    mutationFn: (viewId: string) =>
      patch<Record<string, unknown>>("/auth/me/prefs", { [DASHBOARD_VIEW_PREF_KEY]: viewId }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["me"] }),
  });

  return { activeId, setActiveId: setActiveId.mutateAsync, isLoading };
}

export function resolveDashboardView(
  views: DashboardView[] | undefined,
  activeId: string,
  isBuyer: boolean
): DashboardView {
  const saved = views ?? [];
  if (saved.length === 0) {
    return defaultDashboardView(isBuyer);
  }
  if (activeId) {
    const match = saved.find((v) => v.public_id === activeId);
    if (match) return match;
  }
  return saved[0]!;
}

const VALID_PERIODS = new Set<DashboardPeriod>(["today", "yesterday", "7days", "custom"]);

export function parsePeriodState(prefs: Record<string, unknown> | undefined): DashboardPeriodState {
  const period = prefs?.[DASHBOARD_PERIOD_PREF_KEY];
  const customFrom = prefs?.[DASHBOARD_PERIOD_FROM_KEY];
  const customTo = prefs?.[DASHBOARD_PERIOD_TO_KEY];
  return {
    period:
      typeof period === "string" && VALID_PERIODS.has(period as DashboardPeriod)
        ? (period as DashboardPeriod)
        : DEFAULT_PERIOD_STATE.period,
    customFrom: typeof customFrom === "string" ? customFrom : undefined,
    customTo: typeof customTo === "string" ? customTo : undefined,
  };
}

function startOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}

function endOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate(), 23, 59, 59, 999);
}

export function periodBounds(state: DashboardPeriodState): { start: Date; end: Date } {
  const now = new Date();
  const todayStart = startOfDay(now);
  const todayEnd = endOfDay(now);

  if (state.period === "today") {
    return { start: todayStart, end: todayEnd };
  }
  if (state.period === "yesterday") {
    const y = new Date(todayStart);
    y.setDate(y.getDate() - 1);
    return { start: y, end: endOfDay(y) };
  }
  if (state.period === "7days") {
    const start = new Date(todayStart);
    start.setDate(start.getDate() - 7);
    return { start, end: todayEnd };
  }
  if (state.customFrom && state.customTo) {
    const start = startOfDay(new Date(`${state.customFrom}T00:00:00`));
    const end = endOfDay(new Date(`${state.customTo}T00:00:00`));
    if (!Number.isNaN(start.getTime()) && !Number.isNaN(end.getTime()) && start <= end) {
      return { start, end };
    }
  }
  const start = new Date(todayStart);
  start.setDate(start.getDate() - 7);
  return { start, end: todayEnd };
}

export function inPeriod(iso: string, state: DashboardPeriodState): boolean {
  const { start, end } = periodBounds(state);
  const d = new Date(iso);
  return d >= start && d <= end;
}

export function useDashboardPeriod() {
  const qc = useQueryClient();

  const { data: me, isLoading } = useQuery({
    queryKey: ["me"],
    queryFn: () => get<Me>("/auth/me"),
  });

  const periodState = useMemo(
    () => parsePeriodState(me?.user.prefs),
    [me]
  );

  const setPeriod = useMutation({
    mutationFn: (next: DashboardPeriodState) =>
      patch<Record<string, unknown>>("/auth/me/prefs", {
        [DASHBOARD_PERIOD_PREF_KEY]: next.period,
        [DASHBOARD_PERIOD_FROM_KEY]: next.customFrom ?? null,
        [DASHBOARD_PERIOD_TO_KEY]: next.customTo ?? null,
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["me"] }),
  });

  return {
    periodState,
    setPeriod: setPeriod.mutateAsync,
    isLoading,
  };
}
