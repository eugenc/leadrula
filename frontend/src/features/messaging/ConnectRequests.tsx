import { Check, X, Users } from "lucide-react";
import { errorMessage } from "@/lib/api";
import { toast } from "@/store/toastStore";
import {
  useIncomingConnects,
  useSentConnects,
  useGroupInvites,
  useAcceptConnect,
  useDeclineConnect,
  useAcceptInvite,
  useDeclineInvite,
} from "./hooks";

export function IncomingList({ onOpen }: { onOpen: (id: string) => void }) {
  const { data: connects } = useIncomingConnects();
  const { data: invites } = useGroupInvites();
  const accept = useAcceptConnect();
  const decline = useDeclineConnect();
  const acceptInvite = useAcceptInvite();
  const declineInvite = useDeclineInvite();

  const empty = (connects?.length ?? 0) === 0 && (invites?.length ?? 0) === 0;
  if (empty) {
    return <p className="p-6 text-center text-sm text-gray-400">No incoming requests.</p>;
  }

  return (
    <div className="flex flex-col">
      {(connects?.length ?? 0) > 0 && (
        <SectionLabel>Connect requests</SectionLabel>
      )}
      {connects?.map((c) => (
        <div key={c.id} className="border-b border-gray-50 px-3 py-2.5">
          <div className="flex items-center gap-2">
            <span className="truncate text-sm font-semibold text-gray-800">{c.account_name}</span>
            <span className="ml-auto font-mono text-[11px] text-gray-400">{c.handler_id}</span>
          </div>
          {c.preview && <p className="mt-0.5 line-clamp-2 text-xs text-gray-500">{c.preview}</p>}
          <div className="mt-1.5 flex gap-2">
            <ActionBtn
              variant="accept"
              onClick={() =>
                accept.mutate(c.id, { onSuccess: (t) => onOpen(t.id), onError: (e) => toast.error(errorMessage(e)) })
              }
            >
              <Check className="h-3.5 w-3.5" /> Accept
            </ActionBtn>
            <ActionBtn variant="decline" onClick={() => decline.mutate(c.id)}>
              <X className="h-3.5 w-3.5" /> Decline
            </ActionBtn>
          </div>
        </div>
      ))}

      {(invites?.length ?? 0) > 0 && <SectionLabel>Group invites</SectionLabel>}
      {invites?.map((t) => (
        <div key={t.id} className="border-b border-gray-50 px-3 py-2.5">
          <div className="flex items-center gap-2">
            <Users className="h-4 w-4 text-gray-400" />
            <span className="truncate text-sm font-semibold text-gray-800">{t.display_name}</span>
          </div>
          <div className="mt-1.5 flex gap-2">
            <ActionBtn variant="accept" onClick={() => acceptInvite.mutate(t.id)}>
              <Check className="h-3.5 w-3.5" /> Join
            </ActionBtn>
            <ActionBtn variant="decline" onClick={() => declineInvite.mutate(t.id)}>
              <X className="h-3.5 w-3.5" /> Decline
            </ActionBtn>
          </div>
        </div>
      ))}
    </div>
  );
}

export function SentList() {
  const { data: sent } = useSentConnects();
  if ((sent?.length ?? 0) === 0) {
    return <p className="p-6 text-center text-sm text-gray-400">No sent requests.</p>;
  }
  return (
    <div className="flex flex-col">
      {sent?.map((c) => (
        <div key={c.id} className="flex items-center gap-2 border-b border-gray-50 px-3 py-2.5">
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm font-semibold text-gray-800">{c.account_name}</div>
            {c.preview && <div className="truncate text-xs text-gray-400">{c.preview}</div>}
          </div>
          <StatusPill status={c.status} />
        </div>
      ))}
    </div>
  );
}

function StatusPill({ status }: { status: string }) {
  const map: Record<string, string> = {
    pending: "bg-amber-50 text-amber-700",
    declined: "bg-gray-100 text-gray-500",
    blocked: "bg-gray-100 text-gray-500",
  };
  const label = status === "blocked" ? "Declined" : status.charAt(0).toUpperCase() + status.slice(1);
  return (
    <span className={`shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium ${map[status] ?? "bg-gray-100 text-gray-500"}`}>
      {label}
    </span>
  );
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <div className="bg-gray-50 px-3 py-1 text-[11px] font-semibold uppercase tracking-wide text-gray-400">
      {children}
    </div>
  );
}

function ActionBtn({
  variant,
  onClick,
  children,
}: {
  variant: "accept" | "decline";
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={
        "flex items-center gap-1 rounded-md px-2.5 py-1 text-xs font-semibold " +
        (variant === "accept"
          ? "bg-jade-500 text-white hover:bg-jade-600"
          : "border border-gray-200 text-gray-600 hover:bg-gray-100")
      }
    >
      {children}
    </button>
  );
}
