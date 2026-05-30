import { create } from "zustand";

export interface Toast {
  id: number;
  message: string;
  variant: "default" | "success" | "error";
}

interface PushOptions {
  persistent?: boolean;
}

interface ToastState {
  toasts: Toast[];
  push: (message: string, variant?: Toast["variant"], options?: PushOptions) => number;
  update: (id: number, message: string) => void;
  dismiss: (id: number) => void;
}

let nextId = 1;

export const useToastStore = create<ToastState>((set) => ({
  toasts: [],
  push: (message, variant = "default", options) => {
    const id = nextId++;
    set((s) => ({ toasts: [...s.toasts, { id, message, variant }] }));
    if (!options?.persistent) {
      setTimeout(() => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })), 4000);
    }
    return id;
  },
  update: (id, message) =>
    set((s) => ({
      toasts: s.toasts.map((t) => (t.id === id ? { ...t, message } : t)),
    })),
  dismiss: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
}));

export const toast = {
  success: (m: string) => useToastStore.getState().push(m, "success"),
  error: (m: string) => useToastStore.getState().push(m, "error"),
  info: (m: string) => useToastStore.getState().push(m, "default"),
  progress: (m: string) => useToastStore.getState().push(m, "default", { persistent: true }),
  update: (id: number, m: string) => useToastStore.getState().update(id, m),
  dismiss: (id: number) => useToastStore.getState().dismiss(id),
};
