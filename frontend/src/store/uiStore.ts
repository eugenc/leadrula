import { create } from "zustand";

interface UIState {
  sidebarOpen: boolean;
  toggleSidebar: () => void;
  detailLeadId: number | null;
  openDetail: (id: number) => void;
  closeDetail: () => void;
}

export const useUIStore = create<UIState>((set) => ({
  sidebarOpen: true,
  toggleSidebar: () => set((s) => ({ sidebarOpen: !s.sidebarOpen })),
  detailLeadId: null,
  openDetail: (id) => set({ detailLeadId: id }),
  closeDetail: () => set({ detailLeadId: null }),
}));
