import { formatStatus, formatBuyerStatus, buyerStatusBadgeVariant } from "@/features/leads/leadsListColumns";
import { Card, StatCard, Badge, Spinner } from "@/components/ui/misc";
import { formatMoney, cn } from "@/lib/utils";
import type { Lead } from "@/types";
import {
  type DashboardView,
  type WidgetId,
  type DashboardPeriodState,
} from "./dashboardViews";
import { useDashboardStats, type DashboardStats } from "./useDashboardStats";

function leadName(l: Lead): string {
  return `${l.first_name} ${l.last_name}`.trim() || l.public_id;
}

function formatPct(n: number): string {
  if (!Number.isFinite(n)) return "—";
  return `${n.toFixed(1)}%`;
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
                    <Badge plain variant={isBuyer ? buyerStatusBadgeVariant(l) : l.status}>
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
