import { create } from "zustand";

function initialSidebarOpen() {
  if (typeof window === "undefined") return false;
  return window.matchMedia("(min-width: 1024px)").matches;
}

interface UIState {
  sidebarOpen: boolean;
  toggleSidebar: () => void;
  closeSidebar: () => void;
  detailLeadId: number | null;
  requestedDetailLeadId: number | null;
  openDetail: (id: number) => void;
  closeDetail: () => void;
  completeDetailSwitch: () => void;
  abortDetailSwitch: () => void;
}

export const useUIStore = create<UIState>((set, get) => ({
  sidebarOpen: initialSidebarOpen(),
  toggleSidebar: () => set((s) => ({ sidebarOpen: !s.sidebarOpen })),
  closeSidebar: () => set({ sidebarOpen: false }),
  detailLeadId: null,
  requestedDetailLeadId: null,
  openDetail: (id) => {
    const current = get().detailLeadId;
    if (current === id) return;
    if (current != null) {
      set({ requestedDetailLeadId: id });
      return;
    }
    set({ detailLeadId: id });
  },
  closeDetail: () => set({ detailLeadId: null, requestedDetailLeadId: null }),
  completeDetailSwitch: () => {
    const next = get().requestedDetailLeadId;
    if (next != null) set({ detailLeadId: next, requestedDetailLeadId: null });
  },
  abortDetailSwitch: () => set({ requestedDetailLeadId: null }),
}));
