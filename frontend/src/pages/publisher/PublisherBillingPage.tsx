import { useState } from "react";
import { useTransactions, useDisputes } from "@/features/admin/hooks";
import { PublisherPayouts } from "@/features/billing/PublisherPayouts";
import { PublisherInvoices } from "@/features/billing/PublisherInvoices";
import { DisputeDetailDrawer } from "@/features/billing/DisputeDetailDrawer";
import { formatAccountTypeLabel, formatBuyerWithType } from "@/features/admin/contractType";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Badge, Spinner, EmptyState } from "@/components/ui/misc";
import { cn, formatMoney, resolveTxnCategory } from "@/lib/utils";
import { format } from "date-fns";
import { useUIStore } from "@/store/uiStore";
import { errorMessage } from "@/lib/api";
import { LogLeadLink } from "@/features/intake/logShared";
import type { Dispute, Transaction } from "@/types";

function formatAccountName(
  name: string | null | undefined,
  fallbackName?: string | null
): string {
  const n = name ?? fallbackName;
  return n?.trim() || "—";
}

function formatAccountType(accountType: string | null | undefined): string {
  if (!accountType) return "—";
  return formatAccountTypeLabel(accountType);
}

function formatAccount(
  name: string | null | undefined,
  accountType: string | null | undefined,
  fallbackName?: string | null
): string {
  const n = name ?? fallbackName;
  if (!n) return "—";
  return formatBuyerWithType(n, accountType ?? undefined) || n;
}

export function PublisherBillingPage() {
  const [tab, setTab] = useState<"transactions" | "invoices" | "disputes" | "payouts">("transactions");
  return (
    <PageBody>
        <div className="mb-4 flex border-b border-gray-100">
          {(["transactions", "invoices", "disputes", "payouts"] as const).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={cn(
                "-mb-px border-b-2 px-4 py-2 text-base font-semibold capitalize transition-colors",
                tab === t ? "border-jade-500 text-jade-700" : "border-transparent text-gray-400 hover:text-gray-600"
              )}
            >
              {t}
            </button>
          ))}
        </div>
        {tab === "transactions" && <Transactions />}
        {tab === "invoices" && <PublisherInvoices />}
        {tab === "disputes" && <Disputes />}
        {tab === "payouts" && <PublisherPayouts />}
    </PageBody>
  );
}

function disputeBadge(d: Dispute): { variant: "pending" | "disputed" | "closed"; label: string } {
  if (d.status === "open") {
    return { variant: "pending", label: d.awaiting_party === "publisher" ? "Your turn" : "Awaiting buyer" };
  }
  if (d.placement_party === "publisher" && !d.placement_completed_at) {
    return { variant: "disputed", label: "Placement needed" };
  }
  return { variant: "closed", label: "Resolved" };
}

function Disputes() {
  const { data: disputes, isLoading } = useDisputes("publisher");
  const [selected, setSelected] = useState<Dispute | null>(null);
  if (isLoading) return <Spinner className="h-6 w-6" />;
  if ((disputes ?? []).length === 0) return <EmptyState title="No disputes." />;
  return (
    <>
      <Table>
        <THead>
          <tr>
            <TH>Account</TH>
            <TH>Lead</TH>
            <TH>Reason</TH>
            <TH>Amount</TH>
            <TH>Status</TH>
            <TH>Opened</TH>
          </tr>
        </THead>
        <TBody>
          {(disputes ?? []).map((d) => {
            const badge = disputeBadge(d);
            return (
              <TR key={d.id} className="cursor-pointer" onClick={() => setSelected(d)}>
                <TD className="font-medium text-gray-800">
                  {formatAccount(d.counterparty_name, d.counterparty_account_type, d.buyer_name)}
                </TD>
                <TD className="text-gray-600">{d.lead_name ?? "—"}</TD>
                <TD className="text-gray-600">{d.reason}</TD>
                <TD className="font-medium text-danger-fg">{formatMoney(d.amount ?? 0)}</TD>
                <TD>
                  <Badge variant={badge.variant}>{badge.label}</Badge>
                </TD>
                <TD>{format(new Date(d.created_at), "MMM d")}</TD>
              </TR>
            );
          })}
        </TBody>
      </Table>
      {selected && (
        <DisputeDetailDrawer scope="publisher" dispute={selected} onClose={() => setSelected(null)} />
      )}
    </>
  );
}

function Transactions() {
  const { data: txns, isLoading, isError, error } = useTransactions("publisher");
  if (isLoading) return <Spinner className="h-6 w-6" />;
  if (isError) {
    return <EmptyState title="Could not load transactions." subtitle={errorMessage(error)} />;
  }
  if ((txns ?? []).length === 0) return <EmptyState title="No transactions." />;
  return (
    <Table>
      <THead>
        <tr>
          <TH>Transaction</TH>
          <TH>Account</TH>
          <TH>Type</TH>
          <TH>Lead</TH>
          <TH>Amount</TH>
          <TH>Balance</TH>
          <TH>When</TH>
        </tr>
      </THead>
      <TBody>
        {(txns ?? []).map((t) => (
          <TransactionRow key={`${t.ledger_source ?? "transaction"}-${t.id}`} t={t} />
        ))}
      </TBody>
    </Table>
  );
}

function TransactionRow({ t }: { t: Transaction }) {
  const openDetail = useUIStore((s) => s.openDetail);
  const typeLabel = resolveTxnCategory(t);
  return (
    <TR>
      <TD className="font-medium text-gray-800">{typeLabel}</TD>
      <TD className="font-medium text-gray-800">
        {formatAccountName(t.counterparty_name, t.buyer_name)}
      </TD>
      <TD className="text-gray-600">{formatAccountType(t.counterparty_account_type)}</TD>
      <TD>
        <LogLeadLink leadId={t.lead_viewable ? t.lead_id : null} fallback={t.lead_name} onClick={openDetail} />
      </TD>
      <TD className={t.amount < 0 ? "font-medium text-danger-fg" : "text-jade-700"}>
        {formatMoney(t.amount)}
      </TD>
      <TD>{t.balance_after != null ? formatMoney(t.balance_after) : "—"}</TD>
      <TD>{format(new Date(t.created_at), "MMM d, h:mma")}</TD>
    </TR>
  );
}
