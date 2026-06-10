import { useState } from "react";
import { Elements, PaymentElement, useElements, useStripe } from "@stripe/react-stripe-js";
import {
  usePaymentMethods,
  useCreateSetupIntent,
  useDetachPaymentMethod,
  useCreateTopupIntent,
  useConfirmTopup,
  useBalance,
  useBuyerStripeConfig,
} from "@/features/admin/hooks";
import { isStripeAvailable, stripePromiseForIntent } from "@/features/billing/stripe";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Card, Spinner } from "@/components/ui/misc";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { useQueryClient } from "@tanstack/react-query";

export function BuyerStripeBilling() {
  const { data: stripeConfig, isLoading } = useBuyerStripeConfig();
  const publishableKey = stripeConfig?.publishable_key;
  const isDirect = stripeConfig?.buyer_kind === "direct";

  if (isLoading) return <Spinner className="h-6 w-6" />;

  if (!isStripeAvailable(publishableKey)) {
    return (
      <Card className="p-4 text-sm text-gray-500">
        {isDirect
          ? "Your publisher has not configured Stripe for online payments yet."
          : "Stripe is not configured. Set VITE_STRIPE_PUBLISHABLE_KEY to enable payments."}
      </Card>
    );
  }

  return (
    <div className="mb-6 grid gap-4 lg:grid-cols-2">
      <PaymentMethodsCard defaultPublishableKey={publishableKey} isDirect={isDirect} />
      <TopupCard defaultPublishableKey={publishableKey} isDirect={isDirect} />
    </div>
  );
}

function PaymentMethodsCard({
  defaultPublishableKey,
  isDirect,
}: {
  defaultPublishableKey?: string;
  isDirect: boolean;
}) {
  const { data: methods, isLoading, refetch } = usePaymentMethods();
  const detach = useDetachPaymentMethod();
  const createSetup = useCreateSetupIntent();
  const [clientSecret, setClientSecret] = useState<string | null>(null);
  const [publishableKey, setPublishableKey] = useState<string | undefined>(defaultPublishableKey);

  async function startAddCard() {
    try {
      const res = await createSetup.mutateAsync();
      setClientSecret(res.client_secret);
      setPublishableKey(res.publishable_key ?? defaultPublishableKey);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  const stripeForElements = clientSecret ? stripePromiseForIntent(publishableKey) : null;

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
      {clientSecret && stripeForElements ? (
        <Elements stripe={stripeForElements} options={{ clientSecret }} key={clientSecret}>
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
      {isDirect && (
        <p className="mt-2 text-xs text-gray-400">Cards are saved on your publisher&apos;s Stripe account.</p>
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

function TopupCard({
  defaultPublishableKey,
  isDirect,
}: {
  defaultPublishableKey?: string;
  isDirect: boolean;
}) {
  const [amount, setAmount] = useState(100);
  const [clientSecret, setClientSecret] = useState<string | null>(null);
  const [publishableKey, setPublishableKey] = useState<string | undefined>(defaultPublishableKey);
  const createIntent = useCreateTopupIntent();

  async function startTopup() {
    if (amount < 5) {
      toast.error("Minimum top-up is $5.00");
      return;
    }
    try {
      const res = await createIntent.mutateAsync(Math.round(amount * 100));
      setClientSecret(res.client_secret);
      setPublishableKey(res.publishable_key ?? defaultPublishableKey);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  const stripeForElements = clientSecret ? stripePromiseForIntent(publishableKey) : null;

  return (
    <Card className="p-4">
      <h3 className="mb-3 text-base font-semibold text-gray-800">Add funds</h3>
      {clientSecret && stripeForElements ? (
        <Elements stripe={stripeForElements} options={{ clientSecret }} key={clientSecret}>
          <TopupPaymentForm
            amount={amount}
            isDirect={isDirect}
            onDone={() => setClientSecret(null)}
            onCancel={() => setClientSecret(null)}
          />
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
  isDirect,
  onDone,
  onCancel,
}: {
  amount: number;
  isDirect: boolean;
  onDone: () => void;
  onCancel: () => void;
}) {
  const stripe = useStripe();
  const elements = useElements();
  const qc = useQueryClient();
  const { refetch: refetchBalance } = useBalance();
  const confirmTopup = useConfirmTopup();
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
          await confirmTopup.mutateAsync(paymentIntent.id);
        } catch (e) {
          toast.error(errorMessage(e));
          return;
        }
      }
      toast.success("Payment received");
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
