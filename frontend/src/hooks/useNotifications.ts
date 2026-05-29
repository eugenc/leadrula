import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, ns, patch } from "@/lib/api";
import type { NotificationItem } from "@/types";

export function useNotifications() {
  return useQuery({
    queryKey: ["notifications"],
    queryFn: () => get<NotificationItem[]>(`${ns()}/notifications`),
    refetchInterval: 30_000, // poll every 30s (v1)
  });
}

export function useMarkRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => patch(`${ns()}/notifications/${id}/read`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}
