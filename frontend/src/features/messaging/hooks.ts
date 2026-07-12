import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { del, get, homeAccountType, messagingNs, patch, post, postForm } from "@/lib/api";
import type { BroadcastJob, BroadcastRecipient, ConnectRequest, Message, Thread } from "./types";

const base = () => `${messagingNs()}/messages`;
const home = () => homeAccountType();

export function useThreads(archived = false, q = "") {
  return useQuery({
    queryKey: ["messages", home(), "threads", { archived, q }],
    queryFn: () => {
      const params = new URLSearchParams();
      if (archived) params.set("archived", "true");
      if (q.trim().length >= 2) params.set("q", q.trim());
      const qs = params.toString();
      return get<Thread[]>(`${base()}/threads${qs ? `?${qs}` : ""}`);
    },
  });
}

export function useThread(threadId: string | null) {
  return useQuery({
    queryKey: ["messages", home(), "thread", threadId],
    queryFn: () => get<Thread>(`${base()}/threads/${threadId}`),
    enabled: !!threadId,
  });
}

export function useMessages(threadId: string | null) {
  return useQuery({
    queryKey: ["messages", home(), "messages", threadId],
    queryFn: () => get<Message[]>(`${base()}/threads/${threadId}/messages`),
    enabled: !!threadId,
  });
}

export function useIncomingConnects() {
  return useQuery({
    queryKey: ["messages", home(), "connect-incoming"],
    queryFn: () => get<ConnectRequest[]>(`${base()}/connect-requests`),
  });
}

export function useSentConnects() {
  return useQuery({
    queryKey: ["messages", home(), "connect-sent"],
    queryFn: () => get<ConnectRequest[]>(`${base()}/connect-requests/sent`),
  });
}

export function useGroupInvites() {
  return useQuery({
    queryKey: ["messages", home(), "group-invites"],
    queryFn: () => get<Thread[]>(`${base()}/group-invites`),
  });
}

function invalidateAll(qc: ReturnType<typeof useQueryClient>) {
  qc.invalidateQueries({ queryKey: ["messages"] });
}

export function useSendMessage() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      threadId,
      body,
      replyToId,
      leadId,
      files,
    }: {
      threadId: string;
      body: string;
      replyToId?: string;
      leadId?: string;
      files?: File[];
    }) => {
      if (files && files.length > 0) {
        const form = new FormData();
        form.set("body", body);
        if (replyToId) form.set("reply_to_id", replyToId);
        if (leadId) form.set("lead_id", leadId);
        files.forEach((f) => form.append("files", f));
        return postForm<Message>(`${base()}/threads/${threadId}/messages`, form);
      }
      return post<Message>(`${base()}/threads/${threadId}/messages`, {
        body,
        reply_to_id: replyToId,
        lead_id: leadId,
      });
    },
    onSuccess: (_m, v) => {
      qc.invalidateQueries({ queryKey: ["messages", home(), "messages", v.threadId] });
      qc.invalidateQueries({ queryKey: ["messages", home(), "threads"] });
    },
  });
}

export function useCreateDirect() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      recipient_account_id: string;
      context?: string;
      lead_id?: string;
      contract_id?: string;
      body?: string;
    }) => post<Thread>(`${base()}/threads`, body),
    onSuccess: () => invalidateAll(qc),
  });
}

export function useCreateInternalDirect() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { user_id: string; body?: string }) =>
      post<Thread>(`${base()}/threads/internal`, body),
    onSuccess: () => invalidateAll(qc),
  });
}

export function useCreateGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      title: string;
      internal?: boolean;
      member_ids?: string[];
      user_ids?: string[];
      body?: string;
    }) => post<Thread>(`${base()}/threads/group`, body),
    onSuccess: () => invalidateAll(qc),
  });
}

export function useAcceptConnect() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => post<Thread>(`${base()}/connect-requests/${id}/accept`),
    onSuccess: () => invalidateAll(qc),
  });
}

export function useDeclineConnect() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => post(`${base()}/connect-requests/${id}/decline`),
    onSuccess: () => invalidateAll(qc),
  });
}

export function useAcceptInvite() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (threadId: string) => post(`${base()}/threads/${threadId}/invite/accept`),
    onSuccess: () => invalidateAll(qc),
  });
}

export function useDeclineInvite() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (threadId: string) => post(`${base()}/threads/${threadId}/invite/decline`),
    onSuccess: () => invalidateAll(qc),
  });
}

export function useMarkRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (threadId: string) => post(`${base()}/threads/${threadId}/read`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["messages", home(), "threads"] }),
  });
}

export function useSetMuted() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ threadId, muted }: { threadId: string; muted: boolean }) =>
      post(`${base()}/threads/${threadId}/mute`, { muted }),
    onSuccess: () => invalidateAll(qc),
  });
}

export function useSetArchived() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ threadId, archived }: { threadId: string; archived: boolean }) =>
      post(`${base()}/threads/${threadId}/archive`, { archived }),
    onSuccess: () => invalidateAll(qc),
  });
}

export function useBlockThread() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (threadId: string) => post(`${base()}/threads/${threadId}/block`),
    onSuccess: () => invalidateAll(qc),
  });
}

export function useUnblockThread() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (threadId: string) => post(`${base()}/threads/${threadId}/unblock`),
    onSuccess: () => invalidateAll(qc),
  });
}

export function useEditMessage() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ messageId, body }: { messageId: string; body: string }) =>
      patch<Message>(`${base()}/${messageId}`, { body }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["messages", home(), "messages"] }),
  });
}

export function useDeleteMessage() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (messageId: string) => del(`${base()}/${messageId}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["messages", home(), "messages"] }),
  });
}

export function useOpenLeadThread() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (leadId: string) => post<Thread>(`${base()}/threads/by-lead/${leadId}`),
    onSuccess: () => invalidateAll(qc),
  });
}

export function useOpenContractThread() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (contractId: string) => post<Thread>(`${base()}/threads/by-contract/${contractId}`),
    onSuccess: () => invalidateAll(qc),
  });
}

export function useOpenSupportThread() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => post<Thread>(`${base()}/threads/support`),
    onSuccess: () => invalidateAll(qc),
  });
}

export function useBroadcastRecipients() {
  return useQuery({
    queryKey: ["messages", home(), "broadcast-recipients"],
    queryFn: () => get<BroadcastRecipient[]>(`${base()}/broadcast-recipients`),
  });
}

export function useBroadcast() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { body: string; recipient_account_ids: string[] }) =>
      post<BroadcastJob>(`${base()}/broadcasts`, body),
    onSuccess: () => invalidateAll(qc),
  });
}
