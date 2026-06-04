import { create } from "zustand";

const STORAGE_KEY = "theme-dark";

function readStored(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) === "true";
  } catch {
    return false;
  }
}

function applyDark(on: boolean) {
  document.documentElement.classList.toggle("dark", on);
}

interface ThemeState {
  dark: boolean;
  toggle: () => void;
  setDark: (on: boolean) => void;
}

export const useThemeStore = create<ThemeState>((set, get) => ({
  dark: false,
  toggle: () => {
    const next = !get().dark;
    applyDark(next);
    try {
      localStorage.setItem(STORAGE_KEY, String(next));
    } catch {
      /* ignore */
    }
    set({ dark: next });
  },
  setDark: (on) => {
    applyDark(on);
    try {
      localStorage.setItem(STORAGE_KEY, String(on));
    } catch {
      /* ignore */
    }
    set({ dark: on });
  },
}));

export function initTheme() {
  const dark = readStored();
  applyDark(dark);
  useThemeStore.setState({ dark });
}
