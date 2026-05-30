import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { CurrentUser } from "@/types";

interface ImpersonationStack {
  publisherAccessToken: string;
  publisherRefreshToken: string;
  publisherUser: CurrentUser;
  buyerAccountName: string;
}

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  user: CurrentUser | null;
  impersonation: ImpersonationStack | null;
  setAuth: (access: string, refresh: string, user: CurrentUser) => void;
  setTokens: (access: string, refresh: string) => void;
  setUserAvatar: (url: string | null) => void;
  syncUserProfile: (
    patch: Partial<Pick<CurrentUser, "full_name" | "email" | "role" | "avatar_url">>
  ) => void;
  startImpersonation: (access: string, user: CurrentUser, buyerAccountName: string) => void;
  endImpersonation: () => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      accessToken: null,
      refreshToken: null,
      user: null,
      impersonation: null,
      setAuth: (access, refresh, user) =>
        set({ accessToken: access, refreshToken: refresh, user, impersonation: null }),
      setTokens: (access, refresh) => set({ accessToken: access, refreshToken: refresh }),
      setUserAvatar: (url) =>
        set((s) => (s.user ? { user: { ...s.user, avatar_url: url } } : {})),
      syncUserProfile: (patch) =>
        set((s) => (s.user ? { user: { ...s.user, ...patch } } : {})),
      startImpersonation: (access, user, buyerAccountName) => {
        const { accessToken, refreshToken, user: publisherUser } = get();
        if (!accessToken || !refreshToken || !publisherUser) return;
        set({
          impersonation: {
            publisherAccessToken: accessToken,
            publisherRefreshToken: refreshToken,
            publisherUser,
            buyerAccountName,
          },
          accessToken: access,
          refreshToken: null,
          user: { ...user, impersonating: true, buyer_account_name: buyerAccountName },
        });
      },
      endImpersonation: () => {
        const imp = get().impersonation;
        if (!imp) return;
        set({
          accessToken: imp.publisherAccessToken,
          refreshToken: imp.publisherRefreshToken,
          user: imp.publisherUser,
          impersonation: null,
        });
      },
      logout: () => set({ accessToken: null, refreshToken: null, user: null, impersonation: null }),
    }),
    { name: "leadrula-auth" }
  )
);
