import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { del, get, patch, post } from "@/lib/api";
import { homePath } from "@/lib/homePath";
import { useAuthStore, userFromMe } from "@/store/authStore";
import type {
  AccountOperationalStatus,
  Me,
  PlatformAccount,
  SwitchableAccount,
  SwitchLoginResult,
} from "@/types";

export interface PlatformAccountListFilters {
  q?: string;
  page: number;
  limit: number;
}

export interface PlatformAccountListResult {
  items: PlatformAccount[];
  total: number;
  page: number;
  limit: number;
}

function platformAccountQuery(filters: PlatformAccountListFilters) {
  const params = new URLSearchParams();
  if (filters.q) params.set("q", filters.q);
  params.set("page", String(filters.page));
  params.set("limit", String(filters.limit));
  return params.toString();
}

export function useSwitchable() {
  return useQuery({
    queryKey: ["switchable"],
    queryFn: () => get<SwitchableAccount[]>("/auth/switchable"),
  });
}

export function useSwitchAccount() {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const startSwitch = useAuthStore((s) => s.startSwitch);

  return useMutation({
    mutationFn: (accountId: string) =>
      post<SwitchLoginResult>("/auth/switch", { account_id: accountId }),
    onSuccess: async (res) => {
      const { refreshToken } = useAuthStore.getState();
      const originUser = useAuthStore.getState().user;
      const originName = originUser?.account_name ?? originUser?.full_name ?? "Home";

      if (refreshToken) {
        useAuthStore.getState().setTokens(res.access, refreshToken);
      }

      const me = await get<Me>("/auth/me");
      startSwitch(
        res.access,
        {
          ...userFromMe(me),
          is_switched: true,
          switched_from: res.user.switched_from ?? me.switched_from,
          account_name: me.account.name,
        },
        originName
      );
      qc.clear();
      navigate(homePath(me.account.type));
    },
  });
}

export function useSwitchBack() {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const endSwitch = useAuthStore((s) => s.endSwitch);

  return useMutation({
    mutationFn: () => post<SwitchLoginResult>("/auth/switch-back"),
    onSuccess: async (res) => {
      const refresh =
        useAuthStore.getState().switchSession?.originRefreshToken ??
        useAuthStore.getState().refreshToken;
      endSwitch();
      if (refresh) useAuthStore.getState().setTokens(res.access, refresh);
      else useAuthStore.getState().setTokens(res.access, "");
      const me = await get<Me>("/auth/me");
      useAuthStore.setState({ user: userFromMe(me) });
      qc.clear();
      navigate(homePath(me.account.type));
    },
  });
}

export function usePlatformPublishers(filters: PlatformAccountListFilters) {
  return useQuery({
    queryKey: ["platform", "publishers", filters],
    queryFn: () =>
      get<PlatformAccountListResult>(`/platform/publishers?${platformAccountQuery(filters)}`),
  });
}

export function usePlatformBuyers(filters: PlatformAccountListFilters) {
  return useQuery({
    queryKey: ["platform", "buyers", filters],
    queryFn: () => get<PlatformAccountListResult>(`/platform/buyers?${platformAccountQuery(filters)}`),
  });
}

export function useCreatePublisher() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      name: string;
      admin_email: string;
      admin_first_name: string;
      admin_last_name: string;
      timezone?: string;
    }) => post<PlatformAccount>("/platform/publishers", body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["platform", "publishers"] });
      qc.invalidateQueries({ queryKey: ["switchable"] });
    },
  });
}

export function useUpdatePublisher() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      body,
    }: {
      id: string;
      body: {
        name?: string;
        timezone?: string;
        operational_status?: AccountOperationalStatus;
      };
    }) => patch<PlatformAccount>(`/platform/publishers/${id}`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["platform", "publishers"] });
      qc.invalidateQueries({ queryKey: ["switchable"] });
    },
  });
}

export function useUpdatePlatformBuyer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      body,
    }: {
      id: string;
      body: {
        name?: string;
        website?: string;
        timezone?: string;
        operational_status?: AccountOperationalStatus;
      };
    }) => patch<PlatformAccount>(`/platform/buyers/${id}`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["platform", "buyers"] });
      qc.invalidateQueries({ queryKey: ["switchable"] });
    },
  });
}

export function useRemovePlatformPublisher() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => del<{ ok: boolean }>(`/platform/publishers/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["platform", "publishers"] });
      qc.invalidateQueries({ queryKey: ["switchable"] });
    },
  });
}

export function useRemovePlatformBuyer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => del<{ ok: boolean }>(`/platform/buyers/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["platform", "buyers"] });
      qc.invalidateQueries({ queryKey: ["switchable"] });
    },
  });
}

export function useCreatePlatformBuyer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      name: string;
      admin_email: string;
      admin_first_name: string;
      admin_last_name: string;
      website?: string;
      timezone?: string;
      starting_balance?: number;
    }) => post<PlatformAccount>("/platform/buyers", body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["platform", "buyers"] });
      qc.invalidateQueries({ queryKey: ["switchable"] });
    },
  });
}
