import { useState } from "react";
import {
  useBalance,
  useTransactions,
  useDisputes,
  useOpenDispute,
} from "@/features/admin/hooks";
import { BuyerStripeBilling } from "@/features/billing/BuyerStripeBilling";
import { BuyerInvoices } from "@/features/billing/BuyerInvoices";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Label, Textarea } from "@/components/ui/input";
import { Badge, Spinner, EmptyState, StatCard } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { cn, formatMoney, formatTxnType } from "@/lib/utils";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { Dispute, Transaction } from "@/types";

type BillingTab = "transactions" | "invoices" | "disputes" | "payments";

function txnBadgeVariant(t: Transaction): "overdue" | "distributed" | "pending" {
  if (t.type === "topup" || t.type === "credit" || t.type === "dispute_credit") return "distributed";
  if (t.type === "manual_invoice" || t.amount < 0) return "overdue";
  return "pending";
}

function disputeBadgeVariant(status: Dispute["status"]): "overdue" | "distributed" | "pending" {
  if (status === "accepted") return "distributed";
  if (status === "rejected") return "overdue";
  return "pending";
}

export function BuyerBillingPage() {
  const { data: balance } = useBalance();
  const [tab, setTab] = useState<BillingTab>("transactions");

  return (
    <PageBody>
      <div className="mb-5">
        <StatCard
          label="Current balance"
          value={
            <span className={cn((balance?.balance ?? 0) < 0 && "text-danger-fg")}>
              {formatMoney(balance?.balance)}
            </span>
          }
        />
      </div>

      <div className="mb-4 flex border-b border-gray-100">
        {(["transactions", "invoices", "disputes", "payments"] as const).map((t) => (
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
      {tab === "invoices" && <BuyerInvoices />}
      {tab === "disputes" && <Disputes />}
      {tab === "payments" && <BuyerStripeBilling />}
    </PageBody>
  );
}

function Transactions() {
  const { data: txns, isLoading } = useTransactions("buyer");
  const { data: disputes } = useDisputes("buyer");
  const [disputing, setDisputing] = useState<Transaction | null>(null);

  const disputedTxnIds = new Set((disputes ?? []).map((d) => d.transaction_id));

  if (isLoading) return <Spinner className="h-6 w-6" />;
  if ((txns ?? []).length === 0) return <EmptyState title="No transactions yet." />;

  return (
    <>
      <Table>
        <THead>
          <tr>
            <TH>Type</TH>
            <TH>Publisher</TH>
            <TH>Lead</TH>
            <TH>Amount</TH>
            <TH>Balance</TH>
            <TH>When</TH>
            <TH className="min-w-0 w-12" />
          </tr>
        </THead>
        <TBody>
          {(txns ?? []).map((t) => (
            <TR key={t.id}>
              <TD>
                <Badge variant={txnBadgeVariant(t)}>{formatTxnType(t.type)}</Badge>
              </TD>
              <TD className="font-medium text-gray-800">{t.publisher_name ?? "—"}</TD>
              <TD>{t.lead_name ?? "—"}</TD>
              <TD className={t.amount < 0 ? "font-medium text-danger-fg" : "text-jade-700"}>
                {formatMoney(t.amount)}
              </TD>
              <TD>{formatMoney(t.balance_after)}</TD>
              <TD>{format(new Date(t.created_at), "MMM d, h:mma")}</TD>
              <TD>
                {t.type === "debit" && !disputedTxnIds.has(t.id) && (
                  <Button size="sm" variant="outline" onClick={() => setDisputing(t)}>
                    Dispute
                  </Button>
                )}
                {disputedTxnIds.has(t.id) && <Badge variant="pending">disputed</Badge>}
              </TD>
            </TR>
          ))}
        </TBody>
      </Table>

      {disputing && (
        <DisputeDrawer key={disputing.id} txn={disputing} onClose={() => setDisputing(null)} />
      )}
    </>
  );
}

function Disputes() {
  const [statusFilter, setStatusFilter] = useState<"" | "open" | "accepted" | "rejected">("");
  const { data: disputes, isLoading } = useDisputes("buyer", statusFilter || undefined);

  return (
    <>
      <div className="mb-4 flex gap-2">
        {(["", "open", "accepted", "rejected"] as const).map((s) => (
          <Button
            key={s || "all"}
            size="sm"
            variant={statusFilter === s ? "primary" : "secondary"}
            onClick={() => setStatusFilter(s)}
          >
            {s === "" ? "All" : s.charAt(0).toUpperCase() + s.slice(1)}
          </Button>
        ))}
      </div>

      {isLoading ? (
        <Spinner className="h-6 w-6" />
      ) : (disputes ?? []).length === 0 ? (
        <EmptyState title="No disputes." />
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Publisher</TH>
              <TH>Reason</TH>
              <TH>Amount</TH>
              <TH>Status</TH>
              <TH>Opened</TH>
            </tr>
          </THead>
          <TBody>
            {(disputes ?? []).map((d) => (
              <TR key={d.id}>
                <TD className="font-medium text-gray-800">{d.counterparty_name ?? "—"}</TD>
                <TD className="text-gray-600">{d.reason}</TD>
                <TD className="font-medium text-danger-fg">{formatMoney(d.amount)}</TD>
                <TD>
                  <Badge variant={disputeBadgeVariant(d.status)}>{d.status}</Badge>
                </TD>
                <TD>{format(new Date(d.created_at), "MMM d, h:mma")}</TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}
    </>
  );
}

function DisputeDrawer({ txn, onClose }: { txn: Transaction; onClose: () => void }) {
  const open = useOpenDispute();
  const [reason, setReason] = useState("");
  return (
    <FormDrawer
      open
      onClose={onClose}
      title="Dispute Charge"
      subtitle={`Disputing ${formatMoney(txn.amount)} charged for ${txn.lead_name ?? "this lead"}.`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            disabled={!reason.trim() || open.isPending}
            onClick={() =>
              open.mutate(
                { transaction_id: txn.id, reason },
                {
                  onSuccess: () => {
                    toast.success("Dispute submitted");
                    onClose();
                  },
                  onError: (e) => toast.error(errorMessage(e)),
                }
              )
            }
          >
            Submit Dispute
          </Button>
        </>
      }
    >
      <div>
        <Label>Reason</Label>
        <Textarea value={reason} onChange={(e) => setReason(e.target.value)} />
      </div>
    </FormDrawer>
  );
}
