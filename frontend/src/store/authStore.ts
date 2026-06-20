import { create } from "zustand";
import { persist } from "zustand/middleware";
import { queryClient } from "@/lib/queryClient";
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
  targetAccountId: string;
}

export interface StartSwitchParams {
  switchedAccess: string;
  user: CurrentUser;
  originAccountName: string;
  targetAccountId: string;
  originAccessToken: string;
  originRefreshToken: string;
  originUser: CurrentUser;
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
  startSwitch: (params: StartSwitchParams) => void;
  renewSwitchedSession: (switchedAccess: string, originAccess: string, originRefresh: string) => void;
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
        queryClient.clear();
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
        queryClient.clear();
      },
      startSwitch: (params) => {
        set({
          switchSession: {
            originAccessToken: params.originAccessToken,
            originRefreshToken: params.originRefreshToken,
            originUser: params.originUser,
            originAccountName: params.originAccountName,
            targetAccountId: params.targetAccountId,
          },
          accessToken: params.switchedAccess,
          refreshToken: params.originRefreshToken,
          user: {
            ...params.user,
            is_switched: true,
            switched_from: params.user.switched_from,
          },
        });
      },
      renewSwitchedSession: (switchedAccess, originAccess, originRefresh) => {
        const sw = get().switchSession;
        if (!sw) return;
        set({
          switchSession: {
            ...sw,
            originAccessToken: originAccess,
            originRefreshToken: originRefresh,
          },
          accessToken: switchedAccess,
          refreshToken: originRefresh,
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
