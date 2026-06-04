import { useState } from "react";
import {
  useBalance,
  useTransactions,
  useDisputes,
  useOpenDispute,
} from "@/features/admin/hooks";
import { BuyerStripeBilling } from "@/features/billing/BuyerStripeBilling";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Label, Textarea } from "@/components/ui/input";
import { Badge, Spinner, EmptyState, StatCard } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { formatMoney } from "@/lib/utils";
import { cn } from "@/lib/utils";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { Transaction } from "@/types";

function txnBadgeVariant(t: Transaction): "overdue" | "distributed" | "pending" {
  if (t.type === "topup" || t.type === "credit" || t.type === "dispute_credit") return "distributed";
  if (t.type === "manual_invoice" || t.amount < 0) return "overdue";
  return "pending";
}

export function BuyerBillingPage() {
  const { data: balance } = useBalance();
  const { data: txns, isLoading } = useTransactions("buyer");
  const { data: disputes } = useDisputes("buyer");
  const [disputing, setDisputing] = useState<Transaction | null>(null);

  const disputedTxnIds = new Set((disputes ?? []).map((d) => d.transaction_id));

  return (
    <>
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

        <BuyerStripeBilling />

        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : (txns ?? []).length === 0 ? (
          <EmptyState title="No transactions yet." />
        ) : (
          <Table>
            <THead>
              <tr>
                <TH>Type</TH>
                <TH>Lead</TH>
                <TH>Amount</TH>
                <TH>Balance</TH>
                <TH>When</TH>
                <TH />
              </tr>
            </THead>
            <TBody>
              {(txns ?? []).map((t) => (
                <TR key={t.id}>
                  <TD>
                    <Badge variant={txnBadgeVariant(t)}>{t.type}</Badge>
                  </TD>
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
        )}

        {disputing && (
          <DisputeDrawer key={disputing.id} txn={disputing} onClose={() => setDisputing(null)} />
        )}
      </PageBody>
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
