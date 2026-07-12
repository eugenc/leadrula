import { create } from "zustand";

export type ChatView = "inbox" | "incoming" | "sent" | "archived" | "thread" | "new";

// PendingOpen requests the widget to open (or create) a thread with a recipient.
export interface PendingOpen {
  recipientAccountId?: string;
  leadId?: string;
  contractId?: string;
  context?: "general" | "lead" | "contract";
}

interface ChatState {
  isOpen: boolean;
  view: ChatView;
  activeThreadId: string | null;
  pendingOpen: PendingOpen | null;
  open: () => void;
  close: () => void;
  toggle: () => void;
  setView: (v: ChatView) => void;
  openThread: (id: string) => void;
  back: () => void;
  openWithRecipient: (recipientAccountId: string, opts?: Omit<PendingOpen, "recipientAccountId">) => void;
  clearPending: () => void;
}

export const useChatStore = create<ChatState>((set) => ({
  isOpen: false,
  view: "inbox",
  activeThreadId: null,
  pendingOpen: null,
  open: () => set({ isOpen: true }),
  close: () => set({ isOpen: false }),
  toggle: () => set((s) => ({ isOpen: !s.isOpen })),
  setView: (view) => set({ view, activeThreadId: null }),
  openThread: (id) => set({ isOpen: true, view: "thread", activeThreadId: id, pendingOpen: null }),
  back: () => set({ view: "inbox", activeThreadId: null }),
  openWithRecipient: (recipientAccountId, opts) =>
    set({
      isOpen: true,
      view: "inbox",
      pendingOpen: { recipientAccountId, context: opts?.context ?? "general", ...opts },
    }),
  clearPending: () => set({ pendingOpen: null }),
}));
