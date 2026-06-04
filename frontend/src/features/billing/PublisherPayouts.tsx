import { useEffect } from "react";
import { useSearchParams } from "react-router-dom";
import { useStripeConnect, useStripeConnectStatus } from "@/features/admin/hooks";
import { useAuthStore } from "@/store/authStore";
import { Button } from "@/components/ui/button";
import { Card, Badge, Spinner } from "@/components/ui/misc";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";

const STATUS_COPY: Record<string, { label: string; variant: "default" | "distributed" | "pending" | "overdue"; hint: string }> = {
  none: {
    label: "Not connected",
    variant: "default",
    hint: "Connect Stripe to receive payouts to your bank account.",
  },
  pending: {
    label: "Onboarding incomplete",
    variant: "pending",
    hint: "Finish Stripe onboarding to enable payouts.",
  },
  active: {
    label: "Connected",
    variant: "distributed",
    hint: "Your Stripe account is ready to receive payouts.",
  },
  restricted: {
    label: "Restricted",
    variant: "overdue",
    hint: "Stripe needs additional information before payouts can be enabled.",
  },
};

export function PublisherPayouts() {
  const [params, setParams] = useSearchParams();
  const isAdmin = useAuthStore((s) => s.user?.role === "admin");
  const { data, isLoading, refetch } = useStripeConnectStatus();
  const connect = useStripeConnect();

  const stripeParam = params.get("stripe");
  useEffect(() => {
    if (stripeParam === "complete" || stripeParam === "refresh") {
      void refetch();
      params.delete("stripe");
      setParams(params, { replace: true });
      if (stripeParam === "complete") {
        toast.success("Stripe onboarding updated");
      }
    }
  }, [stripeParam, refetch, params, setParams]);

  if (isLoading) return <Spinner className="h-6 w-6" />;

  const status = data?.status ?? "none";
  const copy = STATUS_COPY[status] ?? STATUS_COPY.none;

  return (
    <Card className="max-w-lg space-y-4 p-5">
      <div className="flex items-center justify-between">
        <h3 className="text-base font-semibold text-gray-800">Stripe payouts</h3>
        <Badge variant={copy.variant}>{copy.label}</Badge>
      </div>
      <p className="text-sm text-gray-500">{copy.hint}</p>
      {isAdmin ? (
        <Button
          disabled={connect.isPending}
          onClick={() =>
            connect.mutate(undefined, {
              onSuccess: ({ onboarding_url }) => {
                window.location.href = onboarding_url;
              },
              onError: (e) => toast.error(errorMessage(e)),
            })
          }
        >
          {status === "none" ? "Connect with Stripe" : "Continue Stripe setup"}
        </Button>
      ) : (
        <p className="text-sm text-gray-400">Only account admins can manage Stripe Connect.</p>
      )}
    </Card>
  );
}
