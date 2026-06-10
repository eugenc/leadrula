import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import {
  useStripeConnect,
  useStripeConnectStatus,
  useStripeKeysStatus,
  useSaveStripeKeys,
} from "@/features/admin/hooks";
import { useAuthStore } from "@/store/authStore";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Badge, Spinner } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";

const CONNECT_STATUS_COPY: Record<
  string,
  { label: string; variant: "default" | "distributed" | "pending" | "overdue"; hint: string }
> = {
  none: {
    label: "Not connected",
    variant: "default",
    hint: "Required to receive scheduled marketplace payouts. Link your existing Stripe account.",
  },
  pending: {
    label: "Onboarding incomplete",
    variant: "pending",
    hint: "Finish Stripe onboarding to enable marketplace payouts.",
  },
  active: {
    label: "Connected",
    variant: "distributed",
    hint: "Your Stripe account is ready for marketplace payouts.",
  },
  restricted: {
    label: "Restricted",
    variant: "overdue",
    hint: "Stripe needs additional information before payouts can be enabled.",
  },
};

const KEYS_STATUS_COPY: Record<
  string,
  { label: string; variant: "default" | "distributed" | "pending" | "overdue"; hint: string }
> = {
  none: {
    label: "Not configured",
    variant: "default",
    hint: "Add your Stripe API keys so direct buyers can pay invoices and top up online.",
  },
  verified: {
    label: "Configured",
    variant: "distributed",
    hint: "Direct buyer payments run on your Stripe account.",
  },
  invalid: {
    label: "Invalid",
    variant: "overdue",
    hint: "Stripe rejected the saved keys — enter valid test or live keys.",
  },
};

function DirectBuyerBillingSection({ isAdmin }: { isAdmin: boolean }) {
  const { data, isLoading, refetch } = useStripeKeysStatus();
  const saveKeys = useSaveStripeKeys();
  const [secretKey, setSecretKey] = useState("");
  const [publishableKey, setPublishableKey] = useState("");

  if (isLoading) return <Spinner className="h-5 w-5" />;

  const status = data?.status ?? "none";
  const copy = KEYS_STATUS_COPY[status] ?? KEYS_STATUS_COPY.none;

  async function handleSave() {
    try {
      await saveKeys.mutateAsync({ secret_key: secretKey, publishable_key: publishableKey });
      setSecretKey("");
      setPublishableKey("");
      toast.success("Stripe keys saved");
      void refetch();
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  return (
    <div className="space-y-4 border-b border-gray-100 pb-6">
      <div className="flex items-center justify-between">
        <h3 className="text-base font-semibold text-gray-800">Direct Buyer Billing</h3>
        <Badge variant={copy.variant}>{copy.label}</Badge>
      </div>
      <p className="text-sm text-gray-500">{copy.hint}</p>
      {data?.publishable_key_prefix && (
        <p className="text-xs text-gray-400">Publishable key: {data.publishable_key_prefix}</p>
      )}
      {isAdmin ? (
        <div className="space-y-3">
          <div>
            <Label>Secret key</Label>
            <Input
              type="password"
              autoComplete="off"
              placeholder="sk_test_…"
              value={secretKey}
              onChange={(e) => setSecretKey(e.target.value)}
            />
          </div>
          <div>
            <Label>Publishable key</Label>
            <Input
              type="text"
              autoComplete="off"
              placeholder="pk_test_…"
              value={publishableKey}
              onChange={(e) => setPublishableKey(e.target.value)}
            />
          </div>
          <Button
            disabled={saveKeys.isPending || !secretKey || !publishableKey}
            onClick={() => void handleSave()}
          >
            Save Stripe keys
          </Button>
        </div>
      ) : (
        <p className="text-sm text-gray-400">Only account admins can manage Stripe keys.</p>
      )}
    </div>
  );
}

function MarketplacePayoutSection({ isAdmin }: { isAdmin: boolean }) {
  const { data, isLoading } = useStripeConnectStatus();
  const connect = useStripeConnect();

  if (isLoading) return <Spinner className="h-5 w-5" />;

  const status = data?.status ?? "none";
  const copy = CONNECT_STATUS_COPY[status] ?? CONNECT_STATUS_COPY.none;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-base font-semibold text-gray-800">Marketplace Payout</h3>
        <Badge variant={copy.variant}>{copy.label}</Badge>
      </div>
      <p className="text-sm text-gray-500">{copy.hint}</p>
      {isAdmin ? (
        <Button
          disabled={connect.isPending}
          onClick={() =>
            connect.mutate(undefined, {
              onSuccess: ({ oauth_url }) => {
                window.location.href = oauth_url;
              },
              onError: (e) => toast.error(errorMessage(e)),
            })
          }
        >
          {status === "none" ? "Connect existing Stripe account" : "Reconnect Stripe"}
        </Button>
      ) : (
        <p className="text-sm text-gray-400">Only account admins can manage Stripe Connect.</p>
      )}
    </div>
  );
}

export function StripeOAuthReturnHandler({ onReturn }: { onReturn: () => void }) {
  const [params, setParams] = useSearchParams();
  const { refetch } = useStripeConnectStatus();

  const stripeParam = params.get("stripe");
  useEffect(() => {
    if (stripeParam === "complete" || stripeParam === "refresh") {
      void refetch();
      params.delete("stripe");
      setParams(params, { replace: true });
      onReturn();
      if (stripeParam === "complete") {
        toast.success("Stripe connection updated");
      }
    }
  }, [stripeParam, refetch, params, setParams, onReturn]);

  return null;
}

export function StripeSetupDrawer({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const isAdmin = useAuthStore((s) => s.user?.role === "admin");

  return (
    <FormDrawer open={open} onClose={onClose} title="Stripe">
      <div className="space-y-6">
        <DirectBuyerBillingSection isAdmin={isAdmin} />
        <MarketplacePayoutSection isAdmin={isAdmin} />
      </div>
    </FormDrawer>
  );
}
