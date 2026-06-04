import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { AccountType, CurrentUser, Role } from "@/types";

interface ImpersonationStack {
  publisherAccessToken: string;
  publisherRefreshToken: string;
  publisherUser: CurrentUser;
  buyerAccountName: string;
}

interface SwitchStack {
  originAccessToken: string;
  originRefreshToken: string;
  originUser: CurrentUser;
  originAccountName: string;
}

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  user: CurrentUser | null;
  impersonation: ImpersonationStack | null;
  switchSession: SwitchStack | null;
  setAuth: (access: string, refresh: string, user: CurrentUser) => void;
  setTokens: (access: string, refresh: string) => void;
  setUserAvatar: (url: string | null) => void;
  syncUserProfile: (
    patch: Partial<Pick<CurrentUser, "full_name" | "email" | "role" | "avatar_url">>
  ) => void;
  syncFromMe: (patch: Partial<CurrentUser>) => void;
  startImpersonation: (access: string, user: CurrentUser, buyerAccountName: string) => void;
  endImpersonation: () => void;
  startSwitch: (access: string, user: CurrentUser, originAccountName: string) => void;
  endSwitch: () => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      accessToken: null,
      refreshToken: null,
      user: null,
      impersonation: null,
      switchSession: null,
      setAuth: (access, refresh, user) =>
        set({ accessToken: access, refreshToken: refresh, user, impersonation: null, switchSession: null }),
      setTokens: (access, refresh) => set({ accessToken: access, refreshToken: refresh }),
      setUserAvatar: (url) =>
        set((s) => (s.user ? { user: { ...s.user, avatar_url: url } } : {})),
      syncUserProfile: (patch) =>
        set((s) => (s.user ? { user: { ...s.user, ...patch } } : {})),
      syncFromMe: (patch) =>
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
      startSwitch: (access, user, originAccountName) => {
        const { accessToken, refreshToken, user: originUser } = get();
        if (!accessToken || !refreshToken || !originUser) return;
        set({
          switchSession: {
            originAccessToken: accessToken,
            originRefreshToken: refreshToken,
            originUser,
            originAccountName,
          },
          accessToken: access,
          user: {
            ...user,
            is_switched: true,
            switched_from: user.switched_from,
          },
        });
      },
      endSwitch: () => {
        const sw = get().switchSession;
        if (!sw) return;
        set({
          accessToken: sw.originAccessToken,
          refreshToken: sw.originRefreshToken,
          user: sw.originUser,
          switchSession: null,
        });
      },
      logout: () =>
        set({
          accessToken: null,
          refreshToken: null,
          user: null,
          impersonation: null,
          switchSession: null,
        }),
    }),
    { name: "leadrula-auth" }
  )
);

export function userFromMe(me: {
  user: { id: string; email: string; full_name: string; role: Role; avatar_url?: string | null };
  account: { id: string; type: AccountType; name: string };
  impersonating?: boolean;
  buyer_account_name?: string;
  is_switched?: boolean;
  switched_from?: string;
}): CurrentUser {
  return {
    id: me.user.id,
    email: me.user.email,
    full_name: me.user.full_name,
    role: me.user.role,
    account_type: me.account.type,
    account_id: me.account.id,
    avatar_url: me.user.avatar_url,
    impersonating: me.impersonating,
    buyer_account_name: me.buyer_account_name,
    is_switched: me.is_switched,
    switched_from: me.switched_from,
    account_name: me.account.name,
  };
}
