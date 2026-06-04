import { useState } from "react";
import { Elements, PaymentElement, useElements, useStripe } from "@stripe/react-stripe-js";
import {
  usePaymentMethods,
  useCreateSetupIntent,
  useDetachPaymentMethod,
  useCreateTopupIntent,
  useBalance,
} from "@/features/admin/hooks";
import { stripeConfigured, stripePromise } from "@/features/billing/stripe";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Card, Spinner } from "@/components/ui/misc";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { useQueryClient } from "@tanstack/react-query";

export function BuyerStripeBilling() {
  if (!stripeConfigured || !stripePromise) {
    return (
      <Card className="p-4 text-sm text-gray-500">
        Stripe is not configured. Set{" "}
        <code className="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-xs text-gray-700">
          VITE_STRIPE_PUBLISHABLE_KEY
        </code>{" "}
        to enable payments.
      </Card>
    );
  }

  return (
    <div className="mb-6 grid gap-4 lg:grid-cols-2">
      <PaymentMethodsCard />
      <TopupCard />
    </div>
  );
}

function PaymentMethodsCard() {
  const { data: methods, isLoading, refetch } = usePaymentMethods();
  const detach = useDetachPaymentMethod();
  const createSetup = useCreateSetupIntent();
  const [clientSecret, setClientSecret] = useState<string | null>(null);

  async function startAddCard() {
    try {
      const res = await createSetup.mutateAsync();
      setClientSecret(res.client_secret);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  return (
    <Card className="p-4">
      <h3 className="mb-3 text-base font-semibold text-gray-800">Payment methods</h3>
      {isLoading ? (
        <Spinner className="h-5 w-5" />
      ) : (
        <ul className="mb-3 space-y-2">
          {(methods ?? []).length === 0 ? (
            <li className="text-sm text-gray-400">No cards saved yet.</li>
          ) : (
            (methods ?? []).map((m) => (
              <li key={m.id} className="flex items-center justify-between text-sm">
                <span className="capitalize text-gray-700">
                  {m.brand} •••• {m.last4}
                  <span className="ml-2 text-gray-400">
                    {m.exp_month}/{m.exp_year}
                  </span>
                  {m.is_default && <span className="ml-2 text-jade-600">default</span>}
                </span>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() =>
                    detach.mutate(m.id, {
                      onSuccess: () => toast.success("Card removed"),
                      onError: (e) => toast.error(errorMessage(e)),
                    })
                  }
                >
                  Remove
                </Button>
              </li>
            ))
          )}
        </ul>
      )}
      {clientSecret && stripePromise ? (
        <Elements stripe={stripePromise} options={{ clientSecret }} key={clientSecret}>
          <SetupCardForm
            onDone={() => {
              setClientSecret(null);
              void refetch();
            }}
            onCancel={() => setClientSecret(null)}
          />
        </Elements>
      ) : (
        <Button variant="secondary" disabled={createSetup.isPending} onClick={() => void startAddCard()}>
          Add card
        </Button>
      )}
    </Card>
  );
}

function SetupCardForm({ onDone, onCancel }: { onDone: () => void; onCancel: () => void }) {
  const stripe = useStripe();
  const elements = useElements();
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!stripe || !elements) return;
    setBusy(true);
    const { error } = await stripe.confirmSetup({
      elements,
      confirmParams: { return_url: window.location.href },
      redirect: "if_required",
    });
    setBusy(false);
    if (error) {
      toast.error(error.message ?? "Could not save card");
      return;
    }
    toast.success("Card saved");
    onDone();
  }

  return (
    <form onSubmit={(e) => void submit(e)} className="space-y-3">
      <PaymentElement />
      <div className="flex gap-2">
        <Button type="submit" disabled={busy || !stripe}>
          Save card
        </Button>
        <Button type="button" variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  );
}

function TopupCard() {
  const [amount, setAmount] = useState(100);
  const [clientSecret, setClientSecret] = useState<string | null>(null);
  const createIntent = useCreateTopupIntent();

  async function startTopup() {
    if (amount < 5) {
      toast.error("Minimum top-up is $5.00");
      return;
    }
    try {
      const res = await createIntent.mutateAsync(Math.round(amount * 100));
      setClientSecret(res.client_secret);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  return (
    <Card className="p-4">
      <h3 className="mb-3 text-base font-semibold text-gray-800">Add funds</h3>
      {clientSecret && stripePromise ? (
        <Elements stripe={stripePromise} options={{ clientSecret }} key={clientSecret}>
          <TopupPaymentForm amount={amount} onDone={() => setClientSecret(null)} onCancel={() => setClientSecret(null)} />
        </Elements>
      ) : (
        <div className="flex items-end gap-2">
          <div>
            <Label>Amount (USD)</Label>
            <Input type="number" min={5} step={1} value={amount} onChange={(e) => setAmount(Number(e.target.value))} className="w-32" />
          </div>
          <Button disabled={createIntent.isPending} onClick={() => void startTopup()}>
            Continue to payment
          </Button>
        </div>
      )}
    </Card>
  );
}

function TopupPaymentForm({
  amount,
  onDone,
  onCancel,
}: {
  amount: number;
  onDone: () => void;
  onCancel: () => void;
}) {
  const stripe = useStripe();
  const elements = useElements();
  const qc = useQueryClient();
  const { refetch: refetchBalance } = useBalance();
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
      toast.success("Payment received — balance updating shortly");
      await refetchBalance();
      void qc.invalidateQueries({ queryKey: ["transactions"] });
      onDone();
      return;
    }
    toast.info("Payment processing — refresh in a moment if balance does not update");
    onDone();
  }

  return (
    <form onSubmit={(e) => void submit(e)} className="space-y-3">
      <p className="text-sm text-gray-500">Pay ${amount.toFixed(2)} to add funds to your balance.</p>
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
