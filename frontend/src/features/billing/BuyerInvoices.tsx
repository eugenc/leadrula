import { useState } from "react";
import { Elements, PaymentElement, useElements, useStripe } from "@stripe/react-stripe-js";
import { useQueryClient } from "@tanstack/react-query";
import {
  useInvoices,
  usePayInvoiceIntent,
  useConfirmInvoicePayment,
  useBalance,
  useBuyerStripeConfig,
} from "@/features/admin/hooks";
import { isStripeAvailable, stripePromiseForIntent } from "@/features/billing/stripe";
import {
  INVOICE_KIND_LABELS,
  formatInvoiceStatus,
  formatInvoicePaymentMethod,
} from "@/features/billing/invoices";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { formatMoney } from "@/lib/utils";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { Invoice } from "@/types";

export function BuyerInvoices() {
  const [statusFilter, setStatusFilter] = useState<"" | "open" | "paid">("");
  const [paying, setPaying] = useState<Invoice | null>(null);
  const { data: invoices, isLoading } = useInvoices("buyer", statusFilter || undefined);
  const { data: stripeConfig } = useBuyerStripeConfig();
  const isDirect = stripeConfig?.buyer_kind === "direct";

  return (
    <>
      <div className="mb-4 flex gap-2">
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

      {isLoading ? (
        <Spinner className="h-6 w-6" />
      ) : (invoices ?? []).length === 0 ? (
        <EmptyState title="No invoices." />
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Publisher</TH>
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
                <TD className="font-medium text-gray-800">{inv.publisher_name ?? "—"}</TD>
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
                    <div className="flex shrink-0 justify-end">
                      <Button size="sm" variant="outline" onClick={() => setPaying(inv)}>
                        Pay
                      </Button>
                    </div>
                  )}
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}

      {paying && (
        <InvoicePayDrawer
          key={paying.id}
          invoice={paying}
          isDirect={isDirect}
          onClose={() => setPaying(null)}
        />
      )}
    </>
  );
}

function InvoicePayDrawer({
  invoice,
  isDirect,
  onClose,
}: {
  invoice: Invoice;
  isDirect: boolean;
  onClose: () => void;
}) {
  const { data: stripeConfig } = useBuyerStripeConfig();
  const payIntent = usePayInvoiceIntent();
  const [clientSecret, setClientSecret] = useState<string | null>(null);
  const [publishableKey, setPublishableKey] = useState<string | undefined>();
  const [starting, setStarting] = useState(false);
  const canPayOnline =
    invoice.online_payable === true &&
    isStripeAvailable(publishableKey ?? stripeConfig?.publishable_key);

  async function startPay() {
    if (!isStripeAvailable()) {
      toast.error("Stripe is not configured");
      return;
    }
    setStarting(true);
    try {
      const res = await payIntent.mutateAsync(invoice.id);
      setClientSecret(res.client_secret);
      setPublishableKey(res.publishable_key);
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setStarting(false);
    }
  }

  const stripeForElements = clientSecret ? stripePromiseForIntent(publishableKey) : null;

  return (
    <FormDrawer
      open
      onClose={onClose}
      title="Pay invoice"
      subtitle={`${invoice.publisher_name ?? "Publisher"} · ${formatMoney(invoice.amount)}`}
      footer={
        !clientSecret ? (
          <>
            <Button variant="secondary" onClick={onClose}>
              Cancel
            </Button>
            {canPayOnline ? (
              <Button disabled={starting || payIntent.isPending} onClick={() => void startPay()}>
                Continue to payment
              </Button>
            ) : null}
          </>
        ) : undefined
      }
    >
      {!clientSecret && !canPayOnline && (
        <p className="text-sm text-gray-500">Contact your publisher to arrange payment.</p>
      )}
      {clientSecret && stripeForElements && (
        <Elements stripe={stripeForElements} options={{ clientSecret }} key={clientSecret}>
          <InvoicePaymentForm
            invoiceId={invoice.id}
            amount={invoice.amount}
            isDirect={isDirect}
            onDone={onClose}
            onCancel={onClose}
          />
        </Elements>
      )}
    </FormDrawer>
  );
}

function InvoicePaymentForm({
  invoiceId,
  amount,
  isDirect,
  onDone,
  onCancel,
}: {
  invoiceId: number;
  amount: number;
  isDirect: boolean;
  onDone: () => void;
  onCancel: () => void;
}) {
  const stripe = useStripe();
  const elements = useElements();
  const qc = useQueryClient();
  const { refetch: refetchBalance } = useBalance();
  const confirmPayment = useConfirmInvoicePayment();
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!stripe || !elements) return;
    setBusy(true);
    const { error, paymentIntent } = await stripe.confirmPayment({
      elements,
      confirmParams: { return_url: window.location.href },
      redirect: "if_required",
    });
    setBusy(false);
    if (error) {
      toast.error(error.message ?? "Payment failed");
      return;
    }
    if (paymentIntent?.status === "succeeded") {
      if (isDirect) {
        try {
          await confirmPayment.mutateAsync({ invoiceId, paymentIntentId: paymentIntent.id });
        } catch (e) {
          toast.error(errorMessage(e));
          return;
        }
      }
      toast.success("Invoice paid");
      await refetchBalance();
      void qc.invalidateQueries({ queryKey: ["transactions"] });
      void qc.invalidateQueries({ queryKey: ["invoices"] });
      onDone();
      return;
    }
    toast.info("Payment processing — refresh in a moment if balance does not update");
    void qc.invalidateQueries({ queryKey: ["invoices"] });
    onDone();
  }

  return (
    <form onSubmit={(e) => void submit(e)} className="space-y-3">
      <p className="text-sm text-gray-500">Pay {formatMoney(amount)} with your saved card or a new one.</p>
      <PaymentElement />
      <div className="flex gap-2">
        <Button type="submit" disabled={busy || !stripe}>
          Pay now
        </Button>
        <Button type="button" variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  );
}
