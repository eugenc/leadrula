import { useNavigate } from "react-router-dom";
import { LogOut } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAuthStore } from "@/store/authStore";
import { useEndImpersonation } from "@/features/admin/hooks";
import { useSwitchBack } from "@/features/auth/switchHooks";
import { homePath } from "@/lib/homePath";

export function ImpersonationBanner() {
  const impersonation = useAuthStore((s) => s.impersonation);
  const switchSession = useAuthStore((s) => s.switchSession);
  const user = useAuthStore((s) => s.user);
  const publisherUser = impersonation?.publisherUser;
  const navigate = useNavigate();
  const endImp = useEndImpersonation();
  const switchBack = useSwitchBack();

  if (impersonation) {
    function exitImp() {
      endImp.mutate(undefined, {
        onSettled: () => {
          useAuthStore.getState().endImpersonation();
          navigate("/p/buyers");
        },
      });
    }
    return (
      <div className="flex shrink-0 items-center justify-between gap-3 border-b border-amber-200 bg-amber-50 px-4 py-2 text-sm text-amber-900 dark:border-amber-800/60 dark:bg-amber-950 dark:text-amber-100">
        <span>
          Acting as <strong>{impersonation.buyerAccountName}</strong>
          {publisherUser ? (
            <>
              {" "}
              — Publisher admin <strong>{publisherUser.full_name}</strong>
            </>
          ) : null}
        </span>
        <Button variant="secondary" size="sm" onClick={exitImp} disabled={endImp.isPending}>
          <LogOut className="h-3.5 w-3.5" /> Exit
        </Button>
      </div>
    );
  }

  if (switchSession || user?.is_switched) {
    const actingAs = user?.account_name ?? user?.account_type ?? "account";
    const origin = switchSession?.originAccountName ?? "home";
    return (
      <div className="flex shrink-0 items-center justify-between gap-3 border-b border-sky-200 bg-sky-50 px-4 py-2 text-sm text-sky-900 dark:border-sky-800/60 dark:bg-sky-950 dark:text-sky-100">
        <span>
          Acting as <strong>{actingAs}</strong> — from <strong>{origin}</strong>
        </span>
        <Button
          variant="secondary"
          size="sm"
          onClick={() =>
            switchBack.mutate(undefined, {
              onError: () => {
                useAuthStore.getState().endSwitch();
                navigate(homePath("platform"));
              },
            })
          }
          disabled={switchBack.isPending}
        >
          <LogOut className="h-3.5 w-3.5" /> Switch back
        </Button>
      </div>
    );
  }

  return null;
}
