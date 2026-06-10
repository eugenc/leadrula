import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, ns, patch } from "@/lib/api";
import type { NotificationItem, NotificationSettingsResponse, NotificationPrefs } from "@/types";

export function useNotifications() {
  return useQuery({
    queryKey: ["notifications"],
    queryFn: () => get<NotificationItem[]>(`${ns()}/notifications`),
    refetchInterval: 30_000,
  });
}

export function useMarkRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => patch(`${ns()}/notifications/${id}/read`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}

export function useNotificationSettings() {
  return useQuery({
    queryKey: ["notification-settings"],
    queryFn: () => get<NotificationSettingsResponse>(`${ns()}/notifications/settings`),
  });
}

export function useUpdateNotificationSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { account?: NotificationPrefs; personal?: NotificationPrefs }) =>
      patch<NotificationSettingsResponse>(`${ns()}/notifications/settings`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notification-settings"] }),
  });
}
