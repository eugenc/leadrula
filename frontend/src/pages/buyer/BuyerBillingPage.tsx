import { useState } from "react";
import {
  useBalance,
  useTopup,
  useTransactions,
  useDisputes,
  useOpenDispute,
} from "@/features/admin/hooks";
import { PageHeader } from "@/components/layout/PageHeader";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Textarea } from "@/components/ui/input";
import { Card, Badge, Spinner, EmptyState } from "@/components/ui/misc";
import { Dialog } from "@/components/ui/dialog";
import { formatMoney } from "@/lib/utils";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";
import type { Transaction } from "@/types";

export function BuyerBillingPage() {
  const { data: balance } = useBalance();
  const { data: txns, isLoading } = useTransactions("buyer");
  const { data: disputes } = useDisputes("buyer");
  const topup = useTopup();
  const [amount, setAmount] = useState(100);
  const [disputing, setDisputing] = useState<Transaction | null>(null);

  const disputedTxnIds = new Set((disputes ?? []).map((d) => d.transaction_id));

  return (
    <div>
      <PageHeader title="Billing" subtitle="Your balance and lead charges." />
      <div className="mb-5 flex flex-wrap items-end gap-4">
        <Card className="p-4">
          <div className="text-xs uppercase text-pd-muted">Current balance</div>
          <div className={`text-3xl font-bold ${(balance?.balance ?? 0) < 0 ? "text-pd-red" : ""}`}>
            {formatMoney(balance?.balance)}
          </div>
        </Card>
        <Card className="flex items-end gap-2 p-4">
          <div>
            <Label>Add funds</Label>
            <Input type="number" value={amount} onChange={(e) => setAmount(Number(e.target.value))} className="w-32" />
          </div>
          <Button
            onClick={() =>
              topup.mutate(amount, {
                onSuccess: () => toast.success("Balance topped up"),
                onError: (e) => toast.error(apiError(e).message),
              })
            }
          >
            Top up
          </Button>
        </Card>
      </div>

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
                  <Badge variant={t.amount < 0 ? "red" : "green"}>{t.type}</Badge>
                </TD>
                <TD>{t.lead_name ?? "—"}</TD>
                <TD className={t.amount < 0 ? "text-pd-red" : "text-pd-green"}>{formatMoney(t.amount)}</TD>
                <TD>{formatMoney(t.balance_after)}</TD>
                <TD>{format(new Date(t.created_at), "MMM d, h:mma")}</TD>
                <TD>
                  {t.type === "debit" && !disputedTxnIds.has(t.id) && (
                    <Button size="sm" variant="ghost" onClick={() => setDisputing(t)}>
                      Dispute
                    </Button>
                  )}
                  {disputedTxnIds.has(t.id) && <Badge variant="amber">disputed</Badge>}
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}

      {disputing && <DisputeDialog txn={disputing} onClose={() => setDisputing(null)} />}
    </div>
  );
}

function DisputeDialog({ txn, onClose }: { txn: Transaction; onClose: () => void }) {
  const open = useOpenDispute();
  const [reason, setReason] = useState("");
  return (
    <Dialog open onClose={onClose} title="Dispute Charge">
      <div className="space-y-3">
        <p className="text-sm text-pd-muted">
          Disputing {formatMoney(txn.amount)} charged for {txn.lead_name ?? "this lead"}.
        </p>
        <div>
          <Label>Reason</Label>
          <Textarea value={reason} onChange={(e) => setReason(e.target.value)} />
        </div>
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            disabled={!reason.trim()}
            onClick={() =>
              open.mutate(
                { transaction_id: txn.id, reason },
                {
                  onSuccess: () => {
                    toast.success("Dispute submitted");
                    onClose();
                  },
                  onError: (e) => toast.error(apiError(e).message),
                }
              )
            }
          >
            Submit Dispute
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
