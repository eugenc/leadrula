import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useLeads } from "@/features/leads/hooks";
import { useTransactions, usePayoutSummary } from "@/features/admin/hooks";
import { leadSourceLabel } from "@/features/leads/leadsListColumns";
import { get } from "@/lib/api";
import { formatMoney } from "@/lib/utils";
import type { BuyerSummary, Lead, Transaction } from "@/types";
import {
  type DashboardView,
  type DashboardPeriodState,
  inPeriod,
  periodBounds,
} from "./dashboardViews";

export type DashboardStats = ReturnType<typeof useDashboardStats>["stats"];

function sumAmounts(txns: Transaction[], types: Transaction["type"][]): number {
  return txns.filter((t) => types.includes(t.type)).reduce((s, t) => s + t.amount, 0);
}

function sourceLabel(l: Lead): string {
  const label = leadSourceLabel(l);
  return label === "—" ? "Unknown" : label;
}

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
