export type ThreadType = "direct" | "group" | "internal" | "broadcast";
export type ThreadContext = "general" | "lead" | "contract" | "connect";
export type ThreadStatus = "active" | "pending" | "archived" | "blocked";

export interface LeadCard {
  id: string;
  name: string;
  phone?: string;
  city?: string;
  state?: string;
}

export interface ReplyRef {
  id: string;
  sender_name: string;
  body?: string | null;
}

export interface Attachment {
  id: string;
  filename: string;
  content_type: string;
  byte_size: number;
}

export interface Message {
  id: string;
  thread_id: string;
  sender_name: string;
  mine: boolean;
  body?: string | null;
  type: string;
  lead_id?: string | null;
  lead_card?: LeadCard | null;
  reply_to?: ReplyRef | null;
  attachments?: Attachment[];
  edited_at?: string | null;
  deleted_at?: string | null;
  can_edit: boolean;
  can_delete: boolean;
  created_at: string;
}

export interface ThreadMember {
  name: string;
  role: string;
  invite_status: string;
  muted: boolean;
  last_read_at?: string | null;
}

export interface Thread {
  id: string;
  type: ThreadType;
  context: ThreadContext;
  status: ThreadStatus;
  title?: string | null;
  lead_id?: string | null;
  contract_id?: string | null;
  context_label?: string;
  display_name: string;
  last_message_at?: string | null;
  last_message?: Message | null;
  unread_count: number;
  muted: boolean;
  can_send: boolean;
  blocked_by_me: boolean;
  members?: ThreadMember[];
  created_at: string;
}

export interface ConnectRequest {
  id: string;
  thread_id: string;
  account_name: string;
  handler_id: string;
  preview?: string;
  status: string;
  created_at: string;
}

export interface BroadcastJob {
  id: string;
  status: string;
  total_count: number;
  sent_count: number;
  failed_count: number;
}

export interface BroadcastRecipient {
  id: string;
  name: string;
  handler_id: string;
  type: "buyer" | "publisher";
}

// WebSocket event envelope.
export interface WSEvent {
  type: string;
  thread_id?: string;
  payload?: unknown;
}
