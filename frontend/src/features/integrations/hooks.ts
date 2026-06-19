import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { ApiError, get, ns, post, del, patch, api, type ApiErrorShape } from "@/lib/api";
import type {
  IntegrationConnection,
  IntegrationProvider,
  RouteIntegration,
  SunbaseConnectionDetail,
  GhlConnectionDetail,
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
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["integration-connections"] });
      qc.invalidateQueries({ queryKey: ["google-maps-status"] });
    },
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

export function useGhlConnectionDetail(id: number | null) {
  return useQuery({
    queryKey: ["ghl-connection", id],
    queryFn: () => get<GhlConnectionDetail>(`${ns()}/integrations/connections/${id}/ghl`),
    enabled: id != null,
  });
}

export function useGhlPipelines(connectionId: number | null) {
  return useQuery({
    queryKey: ["ghl-pipelines", connectionId],
    queryFn: () =>
      get<{ pipelines: { id: string; name: string; stages?: { id: string; name: string }[] }[] }>(
        `${ns()}/integrations/connections/${connectionId}/ghl/pipelines`
      ),
    enabled: connectionId != null,
  });
}

export function useGhlCalendars(connectionId: number | null) {
  return useQuery({
    queryKey: ["ghl-calendars", connectionId],
    queryFn: () =>
      get<{ calendars: { id: string; name: string }[] }>(
        `${ns()}/integrations/connections/${connectionId}/ghl/calendars`
      ),
    enabled: connectionId != null,
  });
}

export type GhlMetadataPreview = {
  pipelines: { id: string; name: string; stages?: { id: string; name: string }[] }[];
  calendars: { id: string; name: string }[];
};

export function useFetchGhlMetadata() {
  return useMutation({
    mutationFn: (body: {
      credentials: { private_integration_token: string };
      config: { location_id: string };
    }) => post<GhlMetadataPreview>(`${ns()}/integrations/ghl/metadata`, body),
  });
}

export function useDeleteIntegrationConnection() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => del(`${ns()}/integrations/connections/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["integration-connections"] });
      qc.invalidateQueries({ queryKey: ["route-integrations"] });
      qc.invalidateQueries({ queryKey: ["google-maps-status"] });
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

export type GoogleMapsSuggestion = {
  place_id: string;
  description: string;
  main_text?: string;
  secondary_text?: string;
};

export type GoogleMapsPlaceDetails = {
  place_id: string;
  formatted_address: string;
  address: string;
  city: string;
  state: string;
  zip: string;
  country: string;
  lat: number;
  lng: number;
};

export function useGoogleMapsStatus() {
  return useQuery({
    queryKey: ["google-maps-status"],
    queryFn: () => get<{ connected: boolean }>(`${ns()}/google-maps/status`),
  });
}

export async function fetchGoogleMapsAutocomplete(input: string, sessionToken: string) {
  const params = new URLSearchParams({ input, session_token: sessionToken });
  const res = await get<{ suggestions: GoogleMapsSuggestion[] }>(
    `${ns()}/google-maps/autocomplete?${params}`
  );
  return res.suggestions ?? [];
}

export async function fetchGoogleMapsPlaceDetails(placeId: string) {
  return post<GoogleMapsPlaceDetails>(`${ns()}/google-maps/place-details`, { place_id: placeId });
}

export async function fetchGoogleMapsSatelliteMap(placeId: string, zoom = 18) {
  const params = new URLSearchParams({ place_id: placeId, zoom: String(zoom) });
  try {
    const res = await api.get(`${ns()}/google-maps/satellite-map?${params}`, { responseType: "blob" });
    return res.data as Blob;
  } catch (err) {
    if (axios.isAxiosError(err) && err.response?.data instanceof Blob) {
      const text = await err.response.data.text();
      try {
        const parsed = JSON.parse(text) as { error?: ApiErrorShape };
        if (parsed.error?.message) {
          throw new ApiError(err.response.status, parsed.error.code ?? "error", parsed.error.message);
        }
      } catch (parseErr) {
        if (parseErr instanceof ApiError) throw parseErr;
      }
    }
    throw err;
  }
}

