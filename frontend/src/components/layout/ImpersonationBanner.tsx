import { useNavigate } from "react-router-dom";
import { LogOut } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAuthStore } from "@/store/authStore";
import { useEndImpersonation } from "@/features/admin/hooks";

export function ImpersonationBanner() {
  const impersonation = useAuthStore((s) => s.impersonation);
  const publisherUser = impersonation?.publisherUser;
  const navigate = useNavigate();
  const end = useEndImpersonation();

  if (!impersonation) return null;

  function exit() {
    end.mutate(undefined, {
      onSettled: () => {
        useAuthStore.getState().endImpersonation();
        navigate("/p/buyers");
      },
    });
  }

  return (
    <div className="flex shrink-0 items-center justify-between gap-3 border-b border-amber-200 bg-amber-50 px-4 py-2 text-sm text-amber-900">
      <span>
        Acting as <strong>{impersonation.buyerAccountName}</strong>
        {publisherUser ? (
          <>
            {" "}
            — Publisher admin <strong>{publisherUser.full_name}</strong>
          </>
        ) : null}
      </span>
      <Button variant="secondary" size="sm" onClick={exit} disabled={end.isPending}>
        <LogOut className="h-3.5 w-3.5" /> Exit
      </Button>
    </div>
  );
}
