import { useEffect } from "react";
import { create } from "zustand";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { apiBaseURL, messagingAccessToken, messagingAccountType } from "@/lib/api";
import { useAuthStore } from "@/store/authStore";
import type { WSEvent } from "./types";

// typingStore holds ephemeral "who is typing" state keyed by thread id.
interface TypingState {
  byThread: Record<string, Record<number, string>>;
  set: (threadId: string, userId: number, name: string) => void;
  clear: (threadId: string, userId: number) => void;
}

const useTypingStore = create<TypingState>((set) => ({
  byThread: {},
  set: (threadId, userId, name) =>
    set((s) => ({
      byThread: { ...s.byThread, [threadId]: { ...(s.byThread[threadId] ?? {}), [userId]: name } },
    })),
  clear: (threadId, userId) =>
    set((s) => {
      const t = { ...(s.byThread[threadId] ?? {}) };
      delete t[userId];
      return { byThread: { ...s.byThread, [threadId]: t } };
    }),
}));

const typingTimers: Record<string, ReturnType<typeof setTimeout>> = {};

function handleEvent(evt: WSEvent, qc: QueryClient) {
  const home = messagingAccountType();
  switch (evt.type) {
    case "new_message":
    case "message_edited":
    case "message_deleted":
      if (evt.thread_id) {
        qc.invalidateQueries({ queryKey: ["messages", home, "messages", evt.thread_id] });
      }
      qc.invalidateQueries({ queryKey: ["messages", home, "threads"] });
      break;
    case "connect_accepted":
    case "thread_created":
    case "thread_updated":
      qc.invalidateQueries({ queryKey: ["messages"] });
      break;
    case "user_typing": {
      const p = evt.payload as { thread_id: string; user_id: number; user_name: string };
      if (!p?.thread_id) break;
      useTypingStore.getState().set(p.thread_id, p.user_id, p.user_name);
      const key = `${p.thread_id}:${p.user_id}`;
      clearTimeout(typingTimers[key]);
      typingTimers[key] = setTimeout(() => useTypingStore.getState().clear(p.thread_id, p.user_id), 6000);
      break;
    }
    case "user_stopped_typing": {
      const p = evt.payload as { thread_id: string; user_id: number };
      if (p?.thread_id) useTypingStore.getState().clear(p.thread_id, p.user_id);
      break;
    }
  }
}

let socket: WebSocket | null = null;
let refCount = 0;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

function wsURL(token: string): string {
  const httpBase = apiBaseURL.replace(/\/$/, "");
  const wsBase = httpBase.replace(/^http/, "ws");
  return `${wsBase}/ws/messages?token=${encodeURIComponent(token)}`;
}

function connect(qc: QueryClient) {
  const token = messagingAccessToken();
  if (!token || socket) return;
  const ws = new WebSocket(wsURL(token));
  socket = ws;
  ws.onmessage = (e) => {
    try {
      handleEvent(JSON.parse(e.data) as WSEvent, qc);
    } catch {
      /* ignore malformed frames */
    }
  };
  ws.onclose = () => {
    socket = null;
    if (refCount > 0 && !reconnectTimer) {
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        connect(qc);
      }, 3000);
    }
  };
  ws.onerror = () => ws.close();
}

function disconnect() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
  socket?.close();
  socket = null;
}

export function sendTyping(threadId: string, typing: boolean) {
  if (socket?.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify({ type: typing ? "typing" : "stop_typing", thread_id: threadId }));
  }
}

// useMessagingSocket keeps a single shared connection alive while mounted.
export function useMessagingSocket() {
  const qc = useQueryClient();
  const impersonation = useAuthStore((s) => s.impersonation);
  const switchSession = useAuthStore((s) => s.switchSession);
  const accessToken = useAuthStore((s) => s.accessToken);

  useEffect(() => {
    refCount += 1;
    disconnect();
    connect(qc);
    return () => {
      refCount -= 1;
      if (refCount <= 0) disconnect();
    };
  }, [qc, impersonation, switchSession, accessToken]);
}

// useTypingNames returns the names of other members currently typing in a thread.
export function useTypingNames(threadId: string | null): string[] {
  const byThread = useTypingStore((s) => s.byThread);
  if (!threadId) return [];
  return Object.values(byThread[threadId] ?? {});
}
