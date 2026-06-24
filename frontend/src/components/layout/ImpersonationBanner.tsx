import { useNavigate } from "react-router-dom";
import { LogOut } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAuthStore } from "@/store/authStore";
import { useEndImpersonation } from "@/features/admin/hooks";

export function ImpersonationBanner() {
  const impersonation = useAuthStore((s) => s.impersonation);
  const publisherUser = impersonation?.publisherUser;
  const navigate = useNavigate();
  const endImp = useEndImpersonation();

  if (!impersonation) return null;

  function exitImp() {
    endImp.mutate(undefined, {
      onSettled: () => {
        useAuthStore.getState().endImpersonation();
        navigate("/p/buyers");
      },
    });
  }

  return (
    <div className="flex shrink-0 flex-col gap-2 border-b border-amber-200 bg-amber-50 px-4 py-2 text-sm text-amber-900 sm:flex-row sm:items-center sm:justify-between sm:gap-3 dark:border-amber-800/60 dark:bg-amber-950 dark:text-amber-100">
      <span>
        Acting as <strong>{impersonation.buyerAccountName}</strong>
        {" "}
        — viewing leads and pipelines from your contracts only
        {publisherUser ? (
          <>
            {" "}
            (Publisher admin <strong>{publisherUser.full_name}</strong>)
          </>
        ) : null}
      </span>
      <Button variant="secondary" size="sm" onClick={exitImp} disabled={endImp.isPending}>
        <LogOut className="h-3.5 w-3.5" /> Exit
      </Button>
    </div>
  );
}
