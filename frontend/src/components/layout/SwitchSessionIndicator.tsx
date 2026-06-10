import { useNavigate } from "react-router-dom";
import { LogOut } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAuthStore } from "@/store/authStore";
import { useSwitchBack } from "@/features/auth/switchHooks";
import { homePath } from "@/lib/homePath";

export function SwitchSessionIndicator() {
  const switchSession = useAuthStore((s) => s.switchSession);
  const user = useAuthStore((s) => s.user);
  const navigate = useNavigate();
  const switchBack = useSwitchBack();

  if (!switchSession && !user?.is_switched) return null;

  const actingAs = user?.account_name ?? user?.account_type ?? "account";
  const origin = switchSession?.originAccountName ?? "home";

  return (
    <div className="flex max-w-xs items-center gap-2 rounded-md border border-sky-200 bg-sky-50 px-2.5 py-1 text-sm text-sky-900 dark:border-sky-800/60 dark:bg-sky-950 dark:text-sky-100 sm:max-w-md">
      <span className="truncate">
        Acting as <strong>{actingAs}</strong> — from <strong>{origin}</strong>
      </span>
      <Button
        variant="secondary"
        size="sm"
        className="shrink-0"
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
