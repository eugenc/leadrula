import { useState } from "react";
import {
  useBalance,
  useTopup,
  useTransactions,
  useDisputes,
  useOpenDispute,
} from "@/features/admin/hooks";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Textarea } from "@/components/ui/input";
import { Card, Badge, Spinner, EmptyState, StatCard } from "@/components/ui/misc";
import { Dialog } from "@/components/ui/dialog";
import { formatMoney } from "@/lib/utils";
import { cn } from "@/lib/utils";
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
    <>
      <PageBody>
        <div className="mb-5 flex flex-wrap items-end gap-4">
          <StatCard
            label="Current balance"
            value={
              <span className={cn((balance?.balance ?? 0) < 0 && "text-danger")}>
                {formatMoney(balance?.balance)}
              </span>
            }
          />
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
                    <Badge variant={t.amount < 0 ? "overdue" : "distributed"}>{t.type}</Badge>
                  </TD>
                  <TD>{t.lead_name ?? "—"}</TD>
                  <TD className={t.amount < 0 ? "text-danger" : "text-jade-700"}>{formatMoney(t.amount)}</TD>
                  <TD>{formatMoney(t.balance_after)}</TD>
                  <TD>{format(new Date(t.created_at), "MMM d, h:mma")}</TD>
                  <TD>
                    {t.type === "debit" && !disputedTxnIds.has(t.id) && (
                      <Button size="sm" variant="ghost" onClick={() => setDisputing(t)}>
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

        {disputing && <DisputeDialog txn={disputing} onClose={() => setDisputing(null)} />}
      </PageBody>
    </>
  );
}

function DisputeDialog({ txn, onClose }: { txn: Transaction; onClose: () => void }) {
  const open = useOpenDispute();
  const [reason, setReason] = useState("");
  return (
    <Dialog open onClose={onClose} title="Dispute Charge">
      <div className="space-y-3">
        <p className="text-sm text-gray-400">
          Disputing {formatMoney(txn.amount)} charged for {txn.lead_name ?? "this lead"}.
        </p>
        <div>
          <Label>Reason</Label>
          <Textarea value={reason} onChange={(e) => setReason(e.target.value)} />
        </div>
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="secondary" onClick={onClose}>
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
