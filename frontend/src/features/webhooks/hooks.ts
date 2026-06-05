import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { get, ns, post, patch, del } from "@/lib/api";
import type {
  Webhook,
  WebhookEvent,
  WebhookFieldMapEntry,
  WebhookSamplePayload,
  WebhookDeliveryListResponse,
  WebhookOutboundTrigger,
} from "@/types";

function base() {
  return `${ns()}/webhooks`;
}

export function useWebhooks() {
  return useQuery({ queryKey: ["webhooks"], queryFn: () => get<Webhook[]>(base()) });
}

export function useCreateWebhook() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: Record<string, unknown>) =>
      post<{ webhook: Webhook; secret: string }>(base(), body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["webhooks"] }),
  });
}

export function useUpdateWebhook() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: Record<string, unknown> }) => patch<Webhook>(`${base()}/${id}`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["webhooks"] }),
  });
}

export function useDeleteWebhook() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => del(`${base()}/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["webhooks"] }),
  });
}

export function useRotateWebhookSecret() {
  return useMutation({
    mutationFn: (id: number) => post<{ secret: string }>(`${base()}/${id}/rotate-secret`, {}),
  });
}

export function useWebhookEvents(webhookId: number | null) {
  return useQuery({
    queryKey: ["webhook-events", webhookId],
    queryFn: () => get<WebhookEvent[]>(`${base()}/${webhookId}/events`),
    enabled: !!webhookId,
  });
}

export function useCreateWebhookEvent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ webhookId, body }: { webhookId: number; body: Record<string, unknown> }) =>
      post<WebhookEvent>(`${base()}/${webhookId}/events`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["webhook-events"] }),
  });
}

export function useUpdateWebhookEvent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ webhookId, eventId, body }: { webhookId: number; eventId: number; body: Record<string, unknown> }) =>
      patch<WebhookEvent>(`${base()}/${webhookId}/events/${eventId}`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["webhook-events"] }),
  });
}

export function useDeleteWebhookEvent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ webhookId, eventId }: { webhookId: number; eventId: number }) =>
      del(`${base()}/${webhookId}/events/${eventId}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["webhook-events"] }),
  });
}

export function useWebhookFieldMap(webhookId: number | null, eventId: number | null) {
  return useQuery({
    queryKey: ["webhook-field-map", webhookId, eventId],
    queryFn: () => get<WebhookFieldMapEntry[]>(`${base()}/${webhookId}/events/${eventId}/field-map`),
    enabled: !!webhookId && !!eventId,
  });
}

export function useWebhookSamplePayload(webhookId: number | null, poll = false) {
  return useQuery({
    queryKey: ["webhook-sample-payload", webhookId],
    queryFn: () => get<WebhookSamplePayload>(`${base()}/${webhookId}/sample-payload`),
    enabled: !!webhookId,
    refetchInterval: poll ? 5000 : false,
  });
}

export function useAddWebhookFieldMap() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ webhookId, eventId, body }: { webhookId: number; eventId: number; body: Record<string, unknown> }) =>
      post(`${base()}/${webhookId}/events/${eventId}/field-map`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["webhook-field-map"] }),
  });
}

export function useDeleteWebhookFieldMap() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => del(`${ns()}/webhook-field-map/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["webhook-field-map"] }),
  });
}

export function useWebhookDeliveries(webhookId: number | null) {
  return useQuery({
    queryKey: ["webhook-deliveries", webhookId],
    queryFn: () => get<WebhookDeliveryListResponse>(`${base()}/${webhookId}/deliveries`),
    enabled: !!webhookId,
  });
}

export function useRotateWebhookOutboundSecret() {
  return useMutation({
    mutationFn: (id: number) => post<{ secret: string }>(`${base()}/${id}/rotate-outbound-secret`, {}),
  });
}

export function useWebhookOutboundTriggers(webhookId: number | null) {
  return useQuery({
    queryKey: ["webhook-outbound-triggers", webhookId],
    queryFn: () => get<WebhookOutboundTrigger[]>(`${base()}/${webhookId}/outbound-triggers`),
    enabled: !!webhookId,
  });
}

export function useCreateWebhookOutboundTrigger() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ webhookId, body }: { webhookId: number; body: Record<string, unknown> }) =>
      post<WebhookOutboundTrigger>(`${base()}/${webhookId}/outbound-triggers`, body),
    onSuccess: (_, { webhookId }) =>
      qc.invalidateQueries({ queryKey: ["webhook-outbound-triggers", webhookId] }),
  });
}

export function useUpdateWebhookOutboundTrigger() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      webhookId,
      triggerId,
      body,
    }: {
      webhookId: number;
      triggerId: number;
      body: Record<string, unknown>;
    }) => patch<WebhookOutboundTrigger>(`${base()}/${webhookId}/outbound-triggers/${triggerId}`, body),
    onSuccess: (_, { webhookId }) =>
      qc.invalidateQueries({ queryKey: ["webhook-outbound-triggers", webhookId] }),
  });
}

export function useDeleteWebhookOutboundTrigger() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ webhookId, triggerId }: { webhookId: number; triggerId: number }) =>
      del(`${base()}/${webhookId}/outbound-triggers/${triggerId}`),
    onSuccess: (_, { webhookId }) =>
      qc.invalidateQueries({ queryKey: ["webhook-outbound-triggers", webhookId] }),
  });
}
