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
  openDetail: (id: number) => void;
  closeDetail: () => void;
}

export const useUIStore = create<UIState>((set) => ({
  sidebarOpen: initialSidebarOpen(),
  toggleSidebar: () => set((s) => ({ sidebarOpen: !s.sidebarOpen })),
  closeSidebar: () => set({ sidebarOpen: false }),
  detailLeadId: null,
  openDetail: (id) => set({ detailLeadId: id }),
  closeDetail: () => set({ detailLeadId: null }),
}));
