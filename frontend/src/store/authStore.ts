import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { CurrentUser } from "@/types";

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  user: CurrentUser | null;
  setAuth: (access: string, refresh: string, user: CurrentUser) => void;
  setTokens: (access: string, refresh: string) => void;
  setUserAvatar: (url: string | null) => void;
  syncUserProfile: (
    patch: Partial<Pick<CurrentUser, "full_name" | "email" | "role" | "avatar_url">>
  ) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      accessToken: null,
      refreshToken: null,
      user: null,
      setAuth: (access, refresh, user) =>
        set({ accessToken: access, refreshToken: refresh, user }),
      setTokens: (access, refresh) => set({ accessToken: access, refreshToken: refresh }),
      setUserAvatar: (url) =>
        set((s) => (s.user ? { user: { ...s.user, avatar_url: url } } : {})),
      syncUserProfile: (patch) =>
        set((s) => (s.user ? { user: { ...s.user, ...patch } } : {})),
      logout: () => set({ accessToken: null, refreshToken: null, user: null }),
    }),
    { name: "leadrula-auth" }
  )
);
