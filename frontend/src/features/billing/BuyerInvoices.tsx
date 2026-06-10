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
import { INVOICE_KIND_LABELS } from "@/features/billing/invoices";
import { Button } from "@/components/ui/button";
import { Card, Badge } from "@/components/ui/misc";
import { formatMoney } from "@/lib/utils";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { Invoice } from "@/types";

export function BuyerOutstandingInvoices() {
  const { data: invoices, isLoading } = useInvoices("buyer", "open");
  const { data: stripeConfig } = useBuyerStripeConfig();
  const open = invoices ?? [];

  if (isLoading) return null;
  if (open.length === 0) return null;

  return (
    <Card className="mb-6 p-4">
      <h3 className="mb-3 text-base font-semibold text-gray-800">Outstanding invoices</h3>
      <ul className="space-y-3">
        {open.map((inv) => (
          <InvoicePayRow key={inv.id} invoice={inv} isDirect={stripeConfig?.buyer_kind === "direct"} />
        ))}
      </ul>
    </Card>
  );
}

function InvoicePayRow({ invoice, isDirect }: { invoice: Invoice; isDirect: boolean }) {
  const { data: stripeConfig } = useBuyerStripeConfig();
  const payIntent = usePayInvoiceIntent();
  const [clientSecret, setClientSecret] = useState<string | null>(null);
  const [publishableKey, setPublishableKey] = useState<string | undefined>();
  const [paying, setPaying] = useState(false);
  const canPayOnline =
    invoice.online_payable === true &&
    isStripeAvailable(publishableKey ?? stripeConfig?.publishable_key);

  async function startPay() {
    if (!isStripeAvailable()) {
      toast.error("Stripe is not configured");
      return;
    }
    setPaying(true);
    try {
      const res = await payIntent.mutateAsync(invoice.id);
      setClientSecret(res.client_secret);
      setPublishableKey(res.publishable_key);
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setPaying(false);
    }
  }

  const stripeForElements = clientSecret ? stripePromiseForIntent(publishableKey) : null;

  return (
    <li className="rounded-lg border border-gray-100 p-3">
      <div className="mb-2 flex flex-wrap items-start justify-between gap-2">
        <div>
          <div className="font-medium text-gray-800">{formatMoney(invoice.amount)}</div>
          <div className="text-sm text-gray-500">
            {invoice.publisher_name ?? "Publisher"} · {invoice.description}
          </div>
          <div className="mt-1 flex items-center gap-2 text-xs text-gray-400">
            <Badge variant="overdue">{INVOICE_KIND_LABELS[invoice.kind] ?? invoice.kind}</Badge>
            <span>{format(new Date(invoice.created_at), "MMM d, yyyy")}</span>
          </div>
        </div>
        {!clientSecret && canPayOnline && (
          <Button size="sm" disabled={paying || payIntent.isPending} onClick={() => void startPay()}>
            Pay invoice
          </Button>
        )}
        {!clientSecret && !canPayOnline && (
          <p className="max-w-xs text-right text-sm text-gray-400">
            Contact your publisher to arrange payment
          </p>
        )}
      </div>
      {clientSecret && stripeForElements && (
        <Elements stripe={stripeForElements} options={{ clientSecret }} key={clientSecret}>
          <InvoicePaymentForm
            invoiceId={invoice.id}
            amount={invoice.amount}
            isDirect={isDirect}
            onDone={() => {
              setClientSecret(null);
              setPublishableKey(undefined);
            }}
            onCancel={() => {
              setClientSecret(null);
              setPublishableKey(undefined);
            }}
          />
        </Elements>
      )}
    </li>
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
    <form onSubmit={(e) => void submit(e)} className="mt-2 space-y-3 border-t border-gray-100 pt-3">
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
