export type AccountType = "publisher" | "buyer";
export type Role = "admin" | "user" | "follower";

export interface CurrentUser {
  id: string;
  email: string;
  full_name: string;
  role: Role;
  account_type: AccountType;
  account_id: string;
}

export interface Me {
  user: {
    id: string;
    email: string;
    full_name: string;
    role: Role;
    is_active: boolean;
    prefs: Record<string, unknown>;
  };
  account: {
    id: string;
    type: AccountType;
    name: string;
    timezone: string;
  };
}

export interface Pipeline {
  id: number;
  public_id: string;
  name: string;
  position: number;
}

export interface Stage {
  id: number;
  public_id: string;
  pipeline_id: number;
  name: string;
  position: number;
  prompt_action_datetime: boolean;
  prompt_disqualification: boolean;
}

export interface Lead {
  id: number;
  public_id: string;
  owner_account_id: number;
  publisher_id: number;
  contract_id: number | null;
  first_name: string;
  last_name: string;
  phone: string | null;
  email: string | null;
  address: string | null;
  city: string | null;
  state: string | null;
  zip: string | null;
  campaign_name: string | null;
  pipeline_id: number | null;
  stage_id: number | null;
  position: number;
  assigned_user_id: number | null;
  action_at: string | null;
  status: "review" | "distributed" | "returned" | "closed";
  disqualification_reason_id: number | null;
  created_at: string;
  updated_at: string;
  custom_values: Record<string, unknown>;
}

export interface Note {
  id: number;
  lead_id: number;
  user_id: number | null;
  author_name: string;
  body: string;
  created_at: string;
}

export interface StageHistoryEntry {
  id: number;
  from_stage_id: number | null;
  from_stage_name: string | null;
  to_stage_id: number;
  to_stage_name: string | null;
  moved_by_name: string | null;
  action_at_captured: string | null;
  disqualification_reason: string | null;
  created_at: string;
}

export interface CustomField {
  id: number;
  name: string;
  field_key: string;
  type: "text" | "number" | "date" | "datetime" | "dropdown" | "checkbox";
  options: string[];
  position: number;
  is_active: boolean;
}

export interface DisqReason {
  id: number;
  label: string;
  position: number;
  is_active: boolean;
}

export interface Contract {
  id: number;
  public_id: string;
  buyer_id: number;
  buyer_name?: string;
  name: string;
  source_pipeline_id: number;
  source_stage_id: number;
  buyer_pipeline_id: number;
  return_stage_id: number;
  rate_per_lead: number;
  status: string;
}

export interface ReturnRule {
  id: number;
  contract_id: number;
  buyer_stage_id: number;
}

export interface Campaign {
  id: number;
  campaign_name: string;
  target_pipeline_id: number;
  target_stage_id: number;
  is_active: boolean;
}

export interface FieldMapEntry {
  id: number;
  campaign_id: number;
  source_key: string;
  target_type: "builtin" | "custom";
  builtin_field: string | null;
  custom_field_id: number | null;
}

export interface QueueItem {
  id: number;
  lead_id: number;
  first_name: string;
  last_name: string;
  phone: string | null;
  campaign_name: string | null;
  raw_payload: Record<string, unknown>;
  status: string;
  created_at: string;
}

export interface Transaction {
  id: number;
  public_id: string;
  buyer_id: number;
  lead_id: number | null;
  lead_name?: string | null;
  contract_id: number | null;
  type: "debit" | "credit" | "dispute_credit" | "manual_invoice";
  amount: number;
  balance_after: number;
  description: string;
  created_at: string;
}

export interface Dispute {
  id: number;
  transaction_id: number;
  buyer_id: number;
  buyer_name?: string;
  reason: string;
  status: "open" | "accepted" | "rejected";
  amount?: number;
  created_at: string;
}

export interface BuyerSummary {
  id: number;
  public_id: string;
  name: string;
  balance: number;
  lead_count: number;
}

export interface UserRow {
  id: number;
  public_id: string;
  email: string;
  full_name: string;
  role: Role;
  is_active: boolean;
}

export interface ApiKey {
  id: number;
  name: string;
  key_prefix: string;
  scopes: string[];
  last_used_at: string | null;
  revoked_at: string | null;
  created_at: string;
}

export interface CalendarEvent {
  lead_id: number;
  title: string;
  stage_id: number | null;
  pipeline_id: number | null;
  user_id: number | null;
  action_at: string;
  overdue: boolean;
}

export interface NotificationItem {
  id: number;
  type: string;
  payload: Record<string, unknown>;
  read_at: string | null;
  created_at: string;
}
