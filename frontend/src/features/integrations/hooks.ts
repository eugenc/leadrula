import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { get, ns, post, del, patch } from "@/lib/api";
import type {
  IntegrationConnection,
  IntegrationProvider,
  RouteIntegration,
  SunbaseConnectionDetail,
} from "@/types";

export function useOAuthConnect() {
  return useMutation({
    mutationFn: async ({
      provider,
      name,
      config,
    }: {
      provider: string;
      name: string;
      config?: Record<string, unknown>;
    }) => {
      const res = await post<{ url: string }>(`${ns()}/integrations/oauth/${provider}/start`, {
        name,
        config: config ?? {},
      });
      return res.url;
    },
  });
}

export function useIntegrationProviders() {
  return useQuery({
    queryKey: ["integration-providers"],
    queryFn: () => get<IntegrationProvider[]>(`${ns()}/integrations/providers`),
  });
}

export function useIntegrationConnections() {
  return useQuery({
    queryKey: ["integration-connections"],
    queryFn: () => get<IntegrationConnection[]>(`${ns()}/integrations/connections`),
  });
}

export function useCreateIntegrationConnection() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      provider_slug: string;
      name: string;
      credentials: Record<string, unknown>;
      config?: Record<string, unknown>;
    }) => post<IntegrationConnection>(`${ns()}/integrations/connections`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["integration-connections"] }),
  });
}

export function useTestIntegrationConnection() {
  return useMutation({
    mutationFn: (body: {
      provider_slug: string;
      credentials: Record<string, unknown>;
      config?: Record<string, unknown>;
    }) =>
      post<{ ok: boolean; message?: string }>(`${ns()}/integrations/connections/test`, body),
  });
}

export function useUpdateIntegrationConnection() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      credentials,
      config,
    }: {
      id: number;
      credentials?: Record<string, unknown>;
      config?: Record<string, unknown>;
    }) => patch<IntegrationConnection>(`${ns()}/integrations/connections/${id}`, { credentials, config }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["integration-connections"] }),
  });
}

export function useSunbaseConnectionDetail(id: number | null) {
  return useQuery({
    queryKey: ["sunbase-connection", id],
    queryFn: () => get<SunbaseConnectionDetail>(`${ns()}/integrations/connections/${id}/sunbase`),
    enabled: id != null,
  });
}

export function useDeleteIntegrationConnection() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => del(`${ns()}/integrations/connections/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["integration-connections"] });
      qc.invalidateQueries({ queryKey: ["route-integrations"] });
    },
  });
}

export function useRouteIntegrations(routeId: number | null) {
  return useQuery({
    queryKey: ["route-integrations", routeId],
    queryFn: () => get<RouteIntegration[]>(`${ns()}/integrations/routes/${routeId}`),
    enabled: routeId != null,
  });
}

export function useAttachRouteIntegration() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      routeId,
      branch_position,
      connection_id,
      delivery_config,
    }: {
      routeId: number;
      branch_position?: number;
      connection_id: number;
      delivery_config?: Record<string, unknown>;
    }) =>
      post(`${ns()}/integrations/routes/${routeId}/attach`, {
        connection_id,
        branch_position: branch_position ?? 0,
        delivery_config: delivery_config ?? {},
      }),
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ["route-integrations", v.routeId] }),
  });
}

export function useDetachRouteIntegration() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, routeId }: { id: number; routeId: number }) =>
      del(`${ns()}/integrations/route-integrations/${id}`),
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ["route-integrations", v.routeId] }),
  });
}

