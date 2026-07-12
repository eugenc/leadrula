import { useEffect } from "react";
import { MessageSquare, X, Plus, ChevronLeft } from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/store/authStore";
import { useChatStore } from "@/store/chatStore";
import { useMessagingSocket } from "./useMessagingSocket";
import { useThreads, useIncomingConnects, useGroupInvites, useCreateDirect, useOpenSupportThread } from "./hooks";
import { ThreadList } from "./ThreadList";
import { MessageThread } from "./MessageThread";
import { IncomingList, SentList } from "./ConnectRequests";
import { NewThreadDialog } from "./NewThreadDialog";
import { BroadcastComposer } from "./BroadcastComposer";
import { toast } from "@/store/toastStore";
import { errorMessage, homeAccountType } from "@/lib/api";

export function ChatWidget() {
  const user = useAuthStore((s) => s.user);
  if (!user) return null;
  return <ChatWidgetInner />;
}

function ChatWidgetInner() {
  const user = useAuthStore((s) => s.user);
  const { isOpen, view, activeThreadId, pendingOpen, toggle, close, setView, openThread, clearPending } =
    useChatStore();
  useMessagingSocket();

  const { data: threads } = useThreads();
  const { data: incoming } = useIncomingConnects();
  const { data: invites } = useGroupInvites();
  const createDirect = useCreateDirect();
  const openSupport = useOpenSupportThread();

  const unread = (threads ?? []).reduce((n, t) => n + t.unread_count, 0);
  const pendingCount = (incoming?.length ?? 0) + (invites?.length ?? 0);
  const badge = unread + pendingCount;

  // Handle contextual "Message" requests: create/open the target thread.
  useEffect(() => {
    if (!pendingOpen?.recipientAccountId || createDirect.isPending) return;
    createDirect.mutate(
      {
        recipient_account_id: pendingOpen.recipientAccountId,
        context: pendingOpen.context ?? "general",
        lead_id: pendingOpen.leadId,
        contract_id: pendingOpen.contractId,
      },
      {
        onSuccess: (t) => openThread(t.id),
        onError: (e) => {
          toast.error(errorMessage(e));
          clearPending();
        },
      }
    );
    clearPending();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pendingOpen]);

  const isPublisher = homeAccountType() === "publisher";
  const isPlatform = homeAccountType() === "platform";
  const homeInbox = !!user?.is_switched || !!user?.impersonating;

  function contactSupport() {
    openSupport.mutate(undefined, {
      onSuccess: (t) => openThread(t.id),
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  return (
    <>
      {!isOpen && (
        <button
          type="button"
          onClick={toggle}
          aria-label="Open messages"
          className="fixed bottom-20 right-4 z-40 flex h-14 w-14 items-center justify-center rounded-full bg-jade-500 text-white shadow-lg transition-colors hover:bg-jade-600 lg:bottom-6"
        >
          <MessageSquare className="h-6 w-6" />
          {badge > 0 && (
            <span className="absolute -right-1 -top-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-danger px-1 text-xs font-bold text-white">
              {badge > 99 ? "99+" : badge}
            </span>
          )}
        </button>
      )}

      {isOpen && (
        <div className="fixed bottom-0 right-0 z-40 flex h-[100dvh] w-full flex-col overflow-hidden border border-gray-200 bg-surface-card shadow-xl sm:bottom-6 sm:right-4 sm:h-[560px] sm:w-[384px] sm:rounded-xl">
          <Header
            view={view}
            onBack={() => setView("inbox")}
            onClose={close}
            showBack={view === "thread" || view === "new"}
            title={view === "thread" ? "" : "Messages"}
            subtitle={homeInbox ? "Your messages" : undefined}
          />

          {view !== "thread" && view !== "new" && (
            <div className="flex items-center gap-1 border-b border-gray-100 px-2 py-1.5">
              <Tab label="Inbox" active={view === "inbox"} onClick={() => setView("inbox")} />
              <Tab
                label="Incoming"
                active={view === "incoming"}
                badge={pendingCount}
                onClick={() => setView("incoming")}
              />
              <Tab label="Sent" active={view === "sent"} onClick={() => setView("sent")} />
              <div className="ml-auto flex items-center gap-1">
                {isPublisher && <BroadcastComposer />}
                {!isPlatform && (
                  <button
                    type="button"
                    onClick={contactSupport}
                    disabled={openSupport.isPending}
                    className="rounded-md px-2 py-1 text-xs font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700 disabled:opacity-50"
                  >
                    Support
                  </button>
                )}
                <button
                  type="button"
                  onClick={() => setView("new")}
                  aria-label="New conversation"
                  className="flex h-7 w-7 items-center justify-center rounded-md text-gray-500 hover:bg-gray-100 hover:text-gray-700"
                >
                  <Plus className="h-4 w-4" />
                </button>
              </div>
            </div>
          )}

          <div className="min-h-0 flex-1 overflow-y-auto">
            {view === "inbox" && <ThreadList onOpen={openThread} />}
            {view === "archived" && <ThreadList onOpen={openThread} archived />}
            {view === "incoming" && <IncomingList onOpen={openThread} />}
            {view === "sent" && <SentList />}
            {view === "thread" && activeThreadId && (
              <MessageThread threadId={activeThreadId} onBack={() => setView("inbox")} />
            )}
            {view === "new" && <NewThreadDialog onCreated={openThread} onCancel={() => setView("inbox")} />}
          </div>
        </div>
      )}
    </>
  );
}

function Header({
  view,
  onBack,
  onClose,
  showBack,
  title,
  subtitle,
}: {
  view: string;
  onBack: () => void;
  onClose: () => void;
  showBack: boolean;
  title: string;
  subtitle?: string;
}) {
  return (
    <div className="flex items-center justify-between border-b border-gray-100 px-3 py-2.5">
      <div className="flex min-w-0 items-center gap-2">
        {showBack ? (
          <button
            type="button"
            onClick={onBack}
            aria-label="Back"
            className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-gray-500 hover:bg-gray-100"
          >
            <ChevronLeft className="h-4 w-4" />
          </button>
        ) : (
          <MessageSquare className="h-4 w-4 shrink-0 text-jade-500" />
        )}
        <div className="min-w-0">
          <span className="text-sm font-semibold text-gray-800">
            {view === "new" ? "New conversation" : title}
          </span>
          {subtitle && <p className="truncate text-xs text-gray-400">{subtitle}</p>}
        </div>
      </div>
      <button
        type="button"
        onClick={onClose}
        aria-label="Close"
        className="flex h-7 w-7 items-center justify-center rounded-md text-gray-400 hover:bg-gray-100 hover:text-gray-700"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}

function Tab({
  label,
  active,
  badge,
  onClick,
}: {
  label: string;
  active: boolean;
  badge?: number;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "relative rounded-md px-2.5 py-1 text-sm font-medium transition-colors",
        active ? "bg-jade-50 text-jade-700" : "text-gray-500 hover:bg-gray-100 hover:text-gray-700"
      )}
    >
      {label}
      {badge != null && badge > 0 && (
        <span className="ml-1 rounded-full bg-danger px-1.5 text-[10px] font-bold text-white">{badge}</span>
      )}
    </button>
  );
}
