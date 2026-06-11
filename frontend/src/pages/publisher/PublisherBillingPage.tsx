import { useState } from "react";
import { useTransactions, useDisputes, useResolveDispute } from "@/features/admin/hooks";
import { PublisherPayouts } from "@/features/billing/PublisherPayouts";
import { PublisherInvoices } from "@/features/billing/PublisherInvoices";
import { formatAccountTypeLabel, formatBuyerWithType } from "@/features/admin/contractType";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { cn, formatMoney, resolveTxnCategory } from "@/lib/utils";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { Transaction } from "@/types";

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

function Disputes() {
  const { data: disputes, isLoading } = useDisputes("publisher", "open");
  const resolve = useResolveDispute();
  if (isLoading) return <Spinner className="h-6 w-6" />;
  if ((disputes ?? []).length === 0) return <EmptyState title="No open disputes." />;
  return (
    <Table>
      <THead>
        <tr>
          <TH>Account</TH>
          <TH>Reason</TH>
          <TH>Amount</TH>
          <TH>Opened</TH>
          <TH className="min-w-0 w-12" />
        </tr>
      </THead>
      <TBody>
        {(disputes ?? []).map((d) => (
          <TR key={d.id}>
            <TD className="font-medium text-gray-800">
              {formatAccount(d.counterparty_name, d.counterparty_account_type, d.buyer_name)}
            </TD>
            <TD className="text-gray-600">{d.reason}</TD>
            <TD className="font-medium text-danger-fg">{formatMoney(d.amount)}</TD>
            <TD>{format(new Date(d.created_at), "MMM d")}</TD>
            <TD>
              <div className="flex justify-end gap-2">
                <Button
                  size="sm"
                  onClick={() =>
                    resolve.mutate(
                      { id: d.id, accept: true },
                      { onSuccess: () => toast.success("Refunded"), onError: (e) => toast.error(errorMessage(e)) }
                    )
                  }
                >
                  Accept
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() =>
                    resolve.mutate(
                      { id: d.id, accept: false },
                      { onSuccess: () => toast.info("Rejected"), onError: (e) => toast.error(errorMessage(e)) }
                    )
                  }
                >
                  Reject
                </Button>
              </div>
            </TD>
          </TR>
        ))}
      </TBody>
    </Table>
  );
}

function Transactions() {
  const { data: txns, isLoading } = useTransactions("publisher");
  if (isLoading) return <Spinner className="h-6 w-6" />;
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
          <TransactionRow key={t.id} t={t} />
        ))}
      </TBody>
    </Table>
  );
}

function TransactionRow({ t }: { t: Transaction }) {
  const typeLabel = resolveTxnCategory(t);
  return (
    <TR>
      <TD className="font-medium text-gray-800">{typeLabel}</TD>
      <TD className="font-medium text-gray-800">
        {formatAccountName(t.counterparty_name, t.buyer_name)}
      </TD>
      <TD className="text-gray-600">{formatAccountType(t.counterparty_account_type)}</TD>
      <TD>{t.lead_name ?? "—"}</TD>
      <TD className={t.amount < 0 ? "font-medium text-danger-fg" : "text-jade-700"}>
        {formatMoney(t.amount)}
      </TD>
      <TD>{formatMoney(t.balance_after)}</TD>
      <TD>{format(new Date(t.created_at), "MMM d, h:mma")}</TD>
    </TR>
  );
}
