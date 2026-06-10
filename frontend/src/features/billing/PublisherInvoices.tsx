import { useState } from "react";
import {
  useInvoices,
  useCreateInvoice,
  useMarkInvoicePaid,
  useVoidInvoice,
  useBuyers,
} from "@/features/admin/hooks";
import {
  INVOICE_KIND_LABELS,
  MANUAL_PAYMENT_METHODS,
  formatInvoiceStatus,
  formatInvoicePaymentMethod,
} from "@/features/billing/invoices";
import { formatBuyerWithType } from "@/features/admin/contractType";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Select, Textarea } from "@/components/ui/input";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { formatMoney } from "@/lib/utils";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { Invoice } from "@/types";

export function PublisherInvoices() {
  const [statusFilter, setStatusFilter] = useState<"" | "open" | "paid">("");
  const [issueOpen, setIssueOpen] = useState(false);
  const [marking, setMarking] = useState<Invoice | null>(null);
  const voidInvoice = useVoidInvoice();
  const { data: invoices, isLoading } = useInvoices("publisher", statusFilter || undefined);

  return (
    <>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex gap-2">
          {(["", "open", "paid"] as const).map((s) => (
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
        <Button onClick={() => setIssueOpen(true)}>Issue invoice</Button>
      </div>

      {isLoading ? (
        <Spinner className="h-6 w-6" />
      ) : (invoices ?? []).length === 0 ? (
        <EmptyState title="No invoices." />
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Buyer</TH>
              <TH>Type</TH>
              <TH>Description</TH>
              <TH>Amount</TH>
              <TH>Status</TH>
              <TH>Payment</TH>
              <TH>Created</TH>
              <TH className="whitespace-nowrap" />
            </tr>
          </THead>
          <TBody>
            {(invoices ?? []).map((inv) => (
              <TR key={inv.id}>
                <TD className="font-medium text-gray-800">{inv.buyer_name ?? "—"}</TD>
                <TD className="text-gray-600">{INVOICE_KIND_LABELS[inv.kind] ?? inv.kind}</TD>
                <TD className="text-gray-600">{inv.description}</TD>
                <TD className="font-medium text-jade-700">{formatMoney(inv.amount)}</TD>
                <TD className="text-gray-700">{formatInvoiceStatus(inv.status)}</TD>
                <TD className="text-gray-600">
                  {inv.payment_method ? formatInvoicePaymentMethod(inv.payment_method) : "—"}
                </TD>
                <TD>{format(new Date(inv.created_at), "MMM d, h:mma")}</TD>
                <TD className="whitespace-nowrap">
                  {inv.status === "open" && (
                    <div className="flex shrink-0 justify-end gap-2">
                      <Button
                        size="sm"
                        variant="outline"
                        className="whitespace-nowrap"
                        onClick={() => setMarking(inv)}
                      >
                        Mark paid
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        className="whitespace-nowrap"
                        disabled={voidInvoice.isPending}
                        onClick={() => {
                          if (!window.confirm("Cancel this invoice?")) return;
                          voidInvoice.mutate(inv.id, {
                            onSuccess: () => toast.success("Invoice canceled"),
                            onError: (e) => toast.error(errorMessage(e)),
                          });
                        }}
                      >
                        Cancel
                      </Button>
                    </div>
                  )}
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}

      {issueOpen && <IssueInvoiceDrawer onClose={() => setIssueOpen(false)} />}
      {marking && <MarkPaidDrawer invoice={marking} onClose={() => setMarking(null)} />}
    </>
  );
}

function IssueInvoiceDrawer({ onClose }: { onClose: () => void }) {
  const { data: buyers } = useBuyers();
  const create = useCreateInvoice();
  const [buyerId, setBuyerId] = useState("");
  const [amount, setAmount] = useState("");
  const [description, setDescription] = useState("");

  const amountNum = parseFloat(amount);
  const canSubmit =
    buyerId !== "" && !Number.isNaN(amountNum) && amountNum > 0 && description.trim().length > 0;

  return (
    <FormDrawer
      open
      onClose={onClose}
      title="Issue prepay invoice"
      subtitle="The buyer receives spendable balance only after this invoice is paid."
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            disabled={!canSubmit || create.isPending}
            onClick={() =>
              create.mutate(
                {
                  buyer_id: Number(buyerId),
                  amount: amountNum,
                  description: description.trim(),
                },
                {
                  onSuccess: () => {
                    toast.success("Invoice issued");
                    onClose();
                  },
                  onError: (e) => toast.error(errorMessage(e)),
                }
              )
            }
          >
            Issue invoice
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <div>
          <Label>Buyer</Label>
          <Select value={buyerId} onChange={(e) => setBuyerId(e.target.value)}>
            <option value="">Select buyer…</option>
            {(buyers ?? []).map((b) => (
              <option key={b.id} value={b.id}>
                {formatBuyerWithType(b.name, b.buyer_kind) || b.name}
              </option>
            ))}
          </Select>
        </div>
        <div>
          <Label>Amount (USD)</Label>
          <Input
            type="number"
            min={0.01}
            step={0.01}
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
          />
        </div>
        <div>
          <Label>Description</Label>
          <Input value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
      </div>
    </FormDrawer>
  );
}

function MarkPaidDrawer({ invoice, onClose }: { invoice: Invoice; onClose: () => void }) {
  const markPaid = useMarkInvoicePaid();
  const [paymentMethod, setPaymentMethod] = useState("");
  const [note, setNote] = useState("");

  const needsNote = paymentMethod === "other";
  const canSubmit = paymentMethod !== "" && (!needsNote || note.trim().length > 0);

  return (
    <FormDrawer
      open
      onClose={onClose}
      title="Mark invoice paid"
      subtitle={`${invoice.buyer_name ?? "Buyer"} — ${formatMoney(invoice.amount)}`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            disabled={!canSubmit || markPaid.isPending}
            onClick={() =>
              markPaid.mutate(
                { id: invoice.id, payment_method: paymentMethod, note: note.trim() || undefined },
                {
                  onSuccess: () => {
                    toast.success("Invoice marked paid");
                    onClose();
                  },
                  onError: (e) => toast.error(errorMessage(e)),
                }
              )
            }
          >
            Confirm paid
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <div>
          <Label>How was this paid?</Label>
          <Select value={paymentMethod} onChange={(e) => setPaymentMethod(e.target.value)}>
            <option value="">Select…</option>
            {MANUAL_PAYMENT_METHODS.map((m) => (
              <option key={m.value} value={m.value}>
                {m.label}
              </option>
            ))}
          </Select>
        </div>
        <div>
          <Label>{needsNote ? "Note (required)" : "Note (optional)"}</Label>
          <Textarea
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder={paymentMethod === "check" ? "Check number, etc." : undefined}
          />
        </div>
      </div>
    </FormDrawer>
  );
}
