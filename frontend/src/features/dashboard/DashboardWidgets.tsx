import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useLeads } from "@/features/leads/hooks";
import { useTransactions, usePayoutSummary } from "@/features/admin/hooks";
import { formatStatus, leadSourceLabel, formatBuyerStatus, buyerStatusBadgeVariant } from "@/features/leads/leadsListColumns";
import { get } from "@/lib/api";
import { Card, StatCard, Badge, Spinner } from "@/components/ui/misc";
import { formatMoney, cn } from "@/lib/utils";
import type { BuyerSummary, Lead, Transaction } from "@/types";
import {
  type DashboardView,
  type WidgetId,
  type DashboardPeriodState,
  inPeriod,
  periodBounds,
} from "./dashboardViews";

export type DashboardStats = ReturnType<typeof useDashboardStats>["stats"];

function sumAmounts(txns: Transaction[], types: Transaction["type"][]): number {
  return txns.filter((t) => types.includes(t.type)).reduce((s, t) => s + t.amount, 0);
}

function leadName(l: Lead): string {
  return `${l.first_name} ${l.last_name}`.trim() || l.public_id;
}

function formatPct(n: number): string {
  if (!Number.isFinite(n)) return "—";
  return `${n.toFixed(1)}%`;
}

function sourceLabel(l: Lead): string {
  const label = leadSourceLabel(l);
  return label === "—" ? "Unknown" : label;
}

function DailyTrendChart({ counts }: { counts: { label: string; value: number }[] }) {
  if (counts.length === 0) {
    return <p className="text-sm text-gray-400">No data for this period</p>;
  }
  const max = Math.max(1, ...counts.map((c) => c.value));
  const w = 320;
  const h = 80;
  const barW = w / counts.length - 2;

  return (
    <svg viewBox={`0 0 ${w} ${h + 20}`} className="h-28 w-full">
      {counts.map((c, i) => {
        const barH = (c.value / max) * h;
        const x = i * (barW + 2);
        return (
          <g key={c.label}>
            <rect
              x={x}
              y={h - barH}
              width={barW}
              height={barH}
              className="fill-jade-400"
              rx={2}
            />
            {i % 5 === 0 && (
              <text x={x + barW / 2} y={h + 14} textAnchor="middle" className="fill-gray-400 text-[8px]">
                {c.label}
              </text>
            )}
          </g>
        );
      })}
    </svg>
  );
}

function BarList({ rows }: { rows: { label: string; value: number; sub?: string }[] }) {
  const max = Math.max(1, ...rows.map((r) => r.value));
  return (
    <div className="space-y-2">
      {rows.length === 0 && <p className="text-sm text-gray-400">No data</p>}
      {rows.map((r) => (
        <div key={r.label}>
          <div className="mb-0.5 flex justify-between text-sm">
            <span className="truncate text-gray-700">{r.label}</span>
            <span className="shrink-0 font-semibold text-gray-800">
              {r.sub ?? r.value.toLocaleString()}
            </span>
          </div>
          <div className="h-2 rounded-full bg-gray-100">
            <div
              className="h-2 rounded-full bg-jade-400"
              style={{ width: `${(r.value / max) * 100}%` }}
            />
          </div>
        </div>
      ))}
    </div>
  );
}

function WidgetShell({
  title,
  wide,
  children,
}: {
  title: string;
  wide?: boolean;
  children: React.ReactNode;
}) {
  return (
    <Card className={cn("p-4", wide && "md:col-span-2")}>
      <div className="mb-3 text-xs font-semibold uppercase tracking-wide text-gray-400">{title}</div>
      {children}
    </Card>
  );
}

const STAT_WIDGET_IDS = new Set<WidgetId>([
  "leads_received",
  "leads_sold",
  "lead_costs",
  "lead_sales",
  "lead_profits",
  "per_lead_profit",
  "per_lead_cost",
  "conversion_rate",
  "return_rate",
  "new_buyers",
  "high_paying_buyer",
  "avg_lead_value",
  "payout_hold",
  "payout_cleared",
]);

function buildDayCounts(allLeads: Lead[], periodState: DashboardPeriodState) {
  const { start, end } = periodBounds(periodState);
  const days: { label: string; value: number }[] = [];
  const cur = new Date(start.getFullYear(), start.getMonth(), start.getDate());
  const last = new Date(end.getFullYear(), end.getMonth(), end.getDate());
  while (cur <= last) {
    const key = cur.toISOString().slice(0, 10);
    days.push({
      label: cur.toLocaleDateString("en-US", { month: "numeric", day: "numeric" }),
      value: allLeads.filter((l) => l.created_at.slice(0, 10) === key).length,
    });
    cur.setDate(cur.getDate() + 1);
  }
  return days.length > 30 ? days.slice(-30) : days;
}

export function useDashboardStats(
  view: DashboardView,
  isBuyer: boolean,
  periodState: DashboardPeriodState
) {
  const { data: leadsData, isLoading: leadsLoading } = useLeads({ all: true });
  const scope = isBuyer ? "buyer" : "publisher";
  const { data: txns, isLoading: txnsLoading } = useTransactions(scope);
  const { data: buyers } = useQuery({
    queryKey: ["buyers"],
    queryFn: () => get<BuyerSummary[]>(`/publisher/buyers`),
    enabled: !isBuyer,
  });
  const { data: payoutSummary } = usePayoutSummary();

  const stats = useMemo(() => {
    const allLeads = leadsData?.items ?? [];
    const leads = allLeads.filter((l) => inPeriod(l.created_at, periodState));
    const allTxns = (txns ?? []).filter((t) => inPeriod(t.created_at, periodState));

    const received = leads.length;
    const sold = leads.filter((l) => l.status === "distributed" || l.status === "closed").length;
    const returned = leads.filter((l) => l.status === "returned").length;

    const sales = isBuyer ? 0 : sumAmounts(allTxns, ["credit"]);
    const costs = sumAmounts(allTxns, ["debit"]);
    const profits = sales - costs;
    const perLeadProfit = sold > 0 ? profits / sold : 0;
    const perLeadCost = sold > 0 ? costs / sold : received > 0 ? costs / received : 0;
    const conversionRate = received > 0 ? (sold / received) * 100 : 0;
    const returnRate = received > 0 ? (returned / received) * 100 : 0;
    const avgLeadValue = sold > 0 ? sales / sold : 0;

    const buyerSpend = new Map<string, number>();
    for (const t of allTxns) {
      if (t.type !== "credit") continue;
      const name =
        buyers?.find((b) => b.id === t.buyer_id)?.name ??
        leads.find((l) => l.id === t.lead_id)?.buyer_name ??
        `Buyer #${t.buyer_id}`;
      buyerSpend.set(name, (buyerSpend.get(name) ?? 0) + t.amount);
    }
    let highPayingBuyer = "—";
    let highPayingAmount = 0;
    for (const [name, amt] of buyerSpend) {
      if (amt > highPayingAmount) {
        highPayingAmount = amt;
        highPayingBuyer = name;
      }
    }

    const buyersInPeriod = new Set(
      leads.filter((l) => l.buyer_name && l.status === "distributed").map((l) => l.buyer_name!)
    );

    const sourceCounts = new Map<string, number>();
    for (const l of leads) {
      const src = sourceLabel(l);
      sourceCounts.set(src, (sourceCounts.get(src) ?? 0) + 1);
    }
    const topSources = [...sourceCounts.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, 5)
      .map(([label, value]) => ({ label, value }));

    const buyerVolume = new Map<string, { count: number; spend: number }>();
    for (const l of leads) {
      if (!l.buyer_name) continue;
      const cur = buyerVolume.get(l.buyer_name) ?? { count: 0, spend: 0 };
      cur.count += 1;
      buyerVolume.set(l.buyer_name, cur);
    }
    for (const [name, spend] of buyerSpend) {
      const cur = buyerVolume.get(name) ?? { count: 0, spend: 0 };
      cur.spend = spend;
      buyerVolume.set(name, cur);
    }
    const buyerLeaderboard = [...buyerVolume.entries()]
      .sort((a, b) => b[1].spend - a[1].spend || b[1].count - a[1].count)
      .slice(0, 8)
      .map(([label, v]) => ({
        label,
        value: v.count,
        sub: `${v.count} leads · ${formatMoney(v.spend)}`,
      }));

    const stageCounts = new Map<string, number>();
    for (const l of leads) {
      const stage = l.stage_name?.trim() || "Unassigned";
      stageCounts.set(stage, (stageCounts.get(stage) ?? 0) + 1);
    }
    const pipelineFunnel = [...stageCounts.entries()]
      .sort((a, b) => b[1] - a[1])
      .map(([label, value]) => ({ label, value }));

    const dayCounts = buildDayCounts(allLeads, periodState);

    const recent = [...leads]
      .sort((a, b) => b.created_at.localeCompare(a.created_at))
      .slice(0, 10);

    return {
      received,
      sold,
      returned,
      sales,
      costs,
      profits,
      perLeadProfit,
      perLeadCost,
      conversionRate,
      returnRate,
      avgLeadValue,
      newBuyers: buyersInPeriod.size,
      highPayingBuyer,
      highPayingAmount,
      topSources,
      buyerLeaderboard,
      pipelineFunnel,
      dayCounts,
      recent,
      payoutHold: payoutSummary?.hold ?? 0,
      payoutCleared: payoutSummary?.cleared ?? 0,
    };
  }, [leadsData, txns, buyers, periodState, isBuyer, payoutSummary]);

  return { stats, loading: leadsLoading || txnsLoading };
}

function buildStatWidgets(stats: NonNullable<DashboardStats>): Partial<Record<WidgetId, React.ReactNode>> {
  return {
    leads_received: (
      <StatCard key="leads_received" label="Leads Received" value={stats.received.toLocaleString()} />
    ),
    leads_sold: (
      <StatCard key="leads_sold" label="Leads Sold" value={stats.sold.toLocaleString()} />
    ),
    lead_costs: (
      <StatCard key="lead_costs" label="Lead Costs" value={formatMoney(stats.costs)} />
    ),
    lead_sales: (
      <StatCard key="lead_sales" label="Lead Sales" value={formatMoney(stats.sales)} />
    ),
    lead_profits: (
      <StatCard key="lead_profits" label="Lead Profits" value={formatMoney(stats.profits)} />
    ),
    per_lead_profit: (
      <StatCard key="per_lead_profit" label="Per Lead Profit" value={formatMoney(stats.perLeadProfit)} />
    ),
    per_lead_cost: (
      <StatCard key="per_lead_cost" label="Per Lead Cost" value={formatMoney(stats.perLeadCost)} />
    ),
    conversion_rate: (
      <StatCard key="conversion_rate" label="Conversion Rate" value={formatPct(stats.conversionRate)} />
    ),
    return_rate: (
      <StatCard key="return_rate" label="Return Rate" value={formatPct(stats.returnRate)} />
    ),
    new_buyers: (
      <StatCard key="new_buyers" label="New Buyers" value={stats.newBuyers.toLocaleString()} />
    ),
    high_paying_buyer: (
      <StatCard
        key="high_paying_buyer"
        label="High Paying Buyer"
        value={
          stats.highPayingAmount > 0 ? (
            <span>
              {stats.highPayingBuyer}
              <span className="mt-1 block text-sm font-normal text-gray-500">
                {formatMoney(stats.highPayingAmount)}
              </span>
            </span>
          ) : (
            "—"
          )
        }
      />
    ),
    avg_lead_value: (
      <StatCard key="avg_lead_value" label="Avg Lead Value" value={formatMoney(stats.avgLeadValue)} />
    ),
    payout_hold: (
      <StatCard key="payout_hold" label="Payout Hold" value={formatMoney(stats.payoutHold)} />
    ),
    payout_cleared: (
      <StatCard key="payout_cleared" label="Payout Cleared" value={formatMoney(stats.payoutCleared)} />
    ),
  };
}

export function DashboardStatCards({
  view,
  stats,
}: {
  view: DashboardView;
  stats: NonNullable<DashboardStats>;
}) {
  const statWidgets = buildStatWidgets(stats);
  const statRow = view.widgets.filter((id) => STAT_WIDGET_IDS.has(id));
  return <>{statRow.map((id) => statWidgets[id]).filter(Boolean)}</>;
}

export function DashboardWideWidgets({
  view,
  isBuyer,
  stats,
}: {
  view: DashboardView;
  isBuyer: boolean;
  stats: NonNullable<DashboardStats>;
}) {
  const wideRow = view.widgets.filter((id) => !STAT_WIDGET_IDS.has(id));

  if (wideRow.length === 0) {
    return null;
  }

  const wideWidgets: Partial<Record<WidgetId, React.ReactNode>> = {
    top_sources: (
      <WidgetShell key="top_sources" title="Top Sources" wide>
        <BarList rows={stats.topSources} />
      </WidgetShell>
    ),
    buyer_leaderboard: (
      <WidgetShell key="buyer_leaderboard" title="Buyer Leaderboard" wide>
        <BarList rows={stats.buyerLeaderboard} />
      </WidgetShell>
    ),
    pipeline_funnel: (
      <WidgetShell key="pipeline_funnel" title="Pipeline Funnel" wide>
        <BarList rows={stats.pipelineFunnel} />
      </WidgetShell>
    ),
    daily_trend: (
      <WidgetShell key="daily_trend" title="Daily Lead Volume" wide>
        <DailyTrendChart counts={stats.dayCounts} />
      </WidgetShell>
    ),
    recent_activity: (
      <WidgetShell key="recent_activity" title="Recent Activity" wide>
        {stats.recent.length === 0 ? (
          <p className="text-sm text-gray-400">No recent leads</p>
        ) : (
          <div>
            <div className="mb-2 grid grid-cols-[1fr_7rem_5rem] gap-2 border-b border-gray-100 pb-2 text-xs font-semibold uppercase tracking-wide text-gray-400">
              <span>Name</span>
              <span className="text-center">Status</span>
              <span className="text-right">Date</span>
            </div>
            <div className="space-y-2">
              {stats.recent.map((l) => (
                <div key={l.id} className="grid grid-cols-[1fr_7rem_5rem] items-center gap-2 text-sm">
                  <span className="truncate font-medium text-gray-800">{leadName(l)}</span>
                  <div className="flex justify-center">
                    <Badge variant={isBuyer ? buyerStatusBadgeVariant(l) : l.status}>
                      {isBuyer ? formatBuyerStatus(l) : formatStatus(l.status, "publisher")}
                    </Badge>
                  </div>
                  <span className="text-right text-xs text-gray-400">
                    {new Date(l.created_at).toLocaleDateString()}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </WidgetShell>
    ),
    goals: (
      <WidgetShell key="goals" title="Goals" wide>
        <div className="space-y-4">
          {view.goals.lead_target != null && view.goals.lead_target > 0 && (
            <div>
              <div className="mb-1 flex justify-between text-sm">
                <span className="text-gray-600">Lead target</span>
                <span className="font-semibold">
                  {stats.received.toLocaleString()} / {view.goals.lead_target.toLocaleString()}
                </span>
              </div>
              <div className="h-2 rounded-full bg-gray-100">
                <div
                  className="h-2 rounded-full bg-jade-500"
                  style={{
                    width: `${Math.min(100, (stats.received / view.goals.lead_target) * 100)}%`,
                  }}
                />
              </div>
            </div>
          )}
          {view.goals.revenue_target != null && view.goals.revenue_target > 0 && (
            <div>
              <div className="mb-1 flex justify-between text-sm">
                <span className="text-gray-600">Revenue target</span>
                <span className="font-semibold">
                  {formatMoney(isBuyer ? stats.costs : stats.sales)} /{" "}
                  {formatMoney(view.goals.revenue_target)}
                </span>
              </div>
              <div className="h-2 rounded-full bg-gray-100">
                <div
                  className="h-2 rounded-full bg-jade-500"
                  style={{
                    width: `${Math.min(
                      100,
                      ((isBuyer ? stats.costs : stats.sales) / view.goals.revenue_target) * 100
                    )}%`,
                  }}
                />
              </div>
            </div>
          )}
          {(!view.goals.lead_target || view.goals.lead_target <= 0) &&
            (!view.goals.revenue_target || view.goals.revenue_target <= 0) && (
              <p className="text-sm text-gray-400">Set targets in dashboard view settings.</p>
            )}
        </div>
      </WidgetShell>
    ),
  };

  return (
    <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
      {wideRow.map((id) => wideWidgets[id]).filter(Boolean)}
    </div>
  );
}

export function DashboardWidgets({
  view,
  isBuyer,
  periodState,
}: {
  view: DashboardView;
  isBuyer: boolean;
  periodState: DashboardPeriodState;
}) {
  const { stats, loading } = useDashboardStats(view, isBuyer, periodState);

  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <Spinner className="h-6 w-6" />
      </div>
    );
  }

  if (view.widgets.length === 0) {
    return <p className="text-sm text-gray-400">No widgets in this view.</p>;
  }

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <DashboardStatCards view={view} stats={stats} />
      </div>
      <DashboardWideWidgets view={view} isBuyer={isBuyer} stats={stats} />
    </div>
  );
}
