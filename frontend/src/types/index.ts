export type AccountType = "publisher" | "buyer" | "platform";
export type Role = "admin" | "user" | "follower";

export interface CurrentUser {
  id: string;
  email: string;
  full_name: string;
  role: Role;
  account_type: AccountType;
  account_id: string;
  avatar_url?: string | null;
  impersonating?: boolean;
  buyer_account_name?: string;
  impersonator?: { id: string; full_name?: string; account_id: string };
  is_switched?: boolean;
  switched_from?: string;
  account_name?: string;
}

export interface Me {
  user: {
    id: string;
    email: string;
    full_name: string;
    role: Role;
    is_active: boolean;
    prefs: Record<string, unknown>;
    avatar_url?: string | null;
  };
  account: {
    id: string;
    handler_id: string;
    type: AccountType;
    name: string;
    timezone: string;
  };
  impersonating?: boolean;
  buyer_account_name?: string;
  impersonator?: { id: string; account_id: string };
  is_switched?: boolean;
  switched_from?: string;
  switchable_count?: number;
}

export type AccountOperationalStatus = "active" | "suspended";

export interface PlatformAccount {
  id: string;
  handler_id: string;
  type: string;
  name: string;
  timezone: string;
  operational_status: AccountOperationalStatus;
  created_at: string;
}

export interface SwitchableAccount {
  id: string;
  handler_id: string;
  type: "publisher" | "buyer";
  name: string;
}

export interface SwitchLoginResult {
  access: string;
  user: {
    id: string;
    account_id: string;
    account_type: "publisher" | "buyer";
    account_name?: string;
    switched_from?: string;
  };
}

export interface Pipeline {
  id: number;
  public_id: string;
  name: string;
  position: number;
}

export type StageType = "standard" | "action" | "disqualification" | "won";

export interface Stage {
  id: number;
  public_id: string;
  pipeline_id: number;
  name: string;
  position: number;
  color: string;
  stage_type: StageType;
}

export type RuleConditionOp =
  | "eq"
  | "neq"
  | "gt"
  | "lt"
  | "contains"
  | "empty"
  | "not_empty";

export type RuleConditionDomain = "lead" | "pipeline";
export type RuleActionDomain = "lead" | "pipeline" | "user";

export interface RuleCondition {
  domain: RuleConditionDomain;
  field: string;
  op: RuleConditionOp;
  value?: unknown;
}

export interface RuleAction {
  verb: "update";
  domain: RuleActionDomain;
  field: string;
  value?: unknown;
}

export interface StageRule {
  id: number;
  stage_id: number;
  position: number;
  condition_logic: "and" | "or";
  conditions: RuleCondition[];
  actions: RuleAction[];
  created_at: string;
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
  source: string | null;
  external_id: string | null;
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
  buyer_name?: string | null;
  assignee_name?: string | null;
  assignee_avatar_url?: string | null;
  pipeline_name?: string | null;
  stage_name?: string | null;
  stage_entered_at?: string | null;
  tags?: string[];
}

export interface LeadListResponse {
  items: Lead[];
  total: number;
  page: number;
  limit: number;
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
  format?: string | null;
  options: string[];
  position: number;
  is_active: boolean;
}

export interface DisqReason {
  id: number;
  stage_id?: number;
  stage_name?: string;
  label: string;
  position: number;
  is_active: boolean;
}

export interface Contract {
  id: number;
  public_id: string;
  handler_id: string;
  buyer_id: number;
  buyer_name?: string;
  buyer_account_type?: string;
  publisher_name?: string;
  name: string;
  description?: string;
  contract_type?: string;
  mirror_contract_id?: number | null;
  lead_type?: string;
  source_pipeline_id: number;
  source_stage_id: number;
  buyer_pipeline_id: number;
  return_stage_id: number;
  rate_per_lead: number;
  status: string;
  cap_period?: string;
  cap_total?: number | null;
  cap_max_daily?: number | null;
  lead_count?: number;
}

export interface ContractLeadCriteria {
  required_fields: {
    field_type: string;
    builtin_field?: string;
    custom_field_id?: number | null;
  }[];
  field_map: {
    src_type: string;
    src_builtin?: string;
    src_custom_field_id?: number | null;
    dst_type: string;
    dst_builtin?: string;
    dst_custom_field_id?: number | null;
  }[];
  filter_rules: {
    field_type: string;
    builtin_field?: string;
    custom_field_id?: number | null;
    operator: string;
    value: string;
  }[];
  quality_rules: { buyer_stage_id: number; on_fail: string }[];
}

export interface ContractCompensation {
  id: number;
  contract_id: number;
  kind: string;
  flat_amount?: number | null;
  bid_min?: number | null;
  bid_max?: number | null;
  rev_percent?: number | null;
  profit_percent?: number | null;
  cap_period: string;
  cap_total?: number | null;
  cap_max_daily?: number | null;
  trigger: string;
  trigger_stage_id?: number | null;
  source_pipeline_id?: number | null;
  source_stage_id?: number | null;
  counterparty_pipeline_id?: number | null;
  counterparty_stage_id?: number | null;
  return_stage_id?: number | null;
  delivery: string;
  position: number;
}

export interface ReturnRule {
  id: number;
  contract_id: number;
  buyer_stage_id: number;
  return_stage_id: number;
}

export type SourceType = "webhook";

export interface Source {
  id: number;
  name: string;
  slug: string;
  type: SourceType;
  is_active: boolean;
}

export interface Route {
  id: number;
  name: string;
  origin: "source" | "pipeline";
  source_id: number | null;
  source_name?: string | null;
  origin_pipeline_id: number | null;
  origin_stage_id: number | null;
  origin_pipeline_name?: string | null;
  origin_stage_name?: string | null;
  destination: "publisher" | "buyer";
  contract_id: number | null;
  compensation_id?: number | null;
  contract_name?: string | null;
  buyer_name?: string | null;
  delivery: "leads" | "leads_pipeline";
  target_pipeline_id: number | null;
  target_stage_id: number | null;
  target_pipeline_name?: string | null;
  target_stage_name?: string | null;
  is_active: boolean;
}

export interface FieldMapEntry {
  id: number;
  source_id: number;
  source_key: string;
  target_type: "builtin" | "custom" | "ignore";
  builtin_field: string | null;
  custom_field_id: number | null;
}

export interface SourceSamplePayload {
  payload: Record<string, unknown> | null;
  received_at?: string;
}

export type OutboundFormat = "json" | "url";
export type OutboundMethod = "GET" | "POST";

export interface OutboundFieldMapEntry {
  dest_key: string;
  source_type: "builtin" | "custom" | "static" | "meta";
  builtin_field?: string;
  custom_field_id?: number;
  static_value?: string;
  meta_field?: string;
}

export interface Webhook {
  id: number;
  name: string;
  slug: string;
  secret_prefix: string;
  is_active: boolean;
  inbound_enabled: boolean;
  outbound_enabled: boolean;
  outbound_url?: string | null;
  outbound_format?: OutboundFormat;
  outbound_method?: OutboundMethod;
  outbound_payload_template?: string;
  outbound_field_map?: OutboundFieldMapEntry[];
  outbound_response_map?: ResponseMapEntry[];
  created_at: string;
}

export type OutboundTriggerEvent =
  | "lead.create"
  | "lead.update"
  | "lead.delete"
  | "pipeline.move_stage"
  | "pipeline.place"
  | "pipeline.stage_rule_applied";

export interface ResponseMapEntry {
  response_key: string;
  target_type: "builtin" | "custom";
  builtin_field?: string;
  custom_field_id?: number;
}

export interface WebhookOutboundTrigger {
  id: number;
  webhook_id: number;
  trigger_event: OutboundTriggerEvent;
  condition_logic: "and" | "or";
  conditions: unknown[];
  position: number;
  is_active: boolean;
}

export interface InboundCondition {
  field: string;
  op: "eq" | "neq" | "contains" | "empty" | "not_empty";
  value?: string;
}

export interface WebhookEvent {
  id: number;
  webhook_id: number;
  action: "create" | "update" | "delete" | "move_stage";
  duplicate_mode?: "update" | "duplicate" | "reject" | null;
  lookup_by?: "external_id" | "public_id" | "phone" | "email" | null;
  lookup_source_key?: string | null;
  target_stage_id?: number | null;
  target_pipeline_id?: number | null;
  position: number;
  condition_logic: "and" | "or";
  conditions: InboundCondition[];
  created_at: string;
}

export interface WebhookFieldMapEntry {
  id: number;
  event_id: number;
  source_key: string;
  target_type: "builtin" | "custom";
  builtin_field: string | null;
  custom_field_id: number | null;
}

export interface WebhookSamplePayload {
  payload: Record<string, unknown> | null;
  received_at?: string;
}

export interface WebhookDelivery {
  id: number;
  webhook_id: number;
  webhook_name?: string;
  webhook_slug?: string;
  event_id?: number | null;
  lead_id?: number | null;
  lead_public_id?: string | null;
  status: "success" | "error" | "skipped";
  error_message?: string | null;
  request_payload?: Record<string, unknown> | null;
  created_at: string;
}

export interface WebhookDeliveryListResponse {
  items: WebhookDelivery[];
  total: number;
  page: number;
  limit: number;
}

export interface RouteFieldMapEntry {
  id: number;
  route_id: number;
  src_type: "builtin" | "custom";
  src_builtin: string | null;
  src_custom_field_id: number | null;
  src_label?: string | null;
  dst_type: "builtin" | "custom";
  dst_builtin: string | null;
  dst_custom_field_id: number | null;
  dst_label?: string | null;
}

export interface RouteFieldMapOptions {
  buyer_name: string;
  publisher_fields: CustomField[];
  buyer_fields: CustomField[];
}

export interface QueueItem {
  id: number;
  lead_id: number;
  first_name: string;
  last_name: string;
  phone: string | null;
  source: string | null;
  raw_payload: Record<string, unknown>;
  status: string;
  unmapped_keys?: string[];
  created_at: string;
}

export interface QueueListResponse {
  items: QueueItem[];
  total: number;
  page: number;
  limit: number;
}

export interface Transaction {
  id: number;
  public_id: string;
  buyer_id: number;
  lead_id: number | null;
  lead_name?: string | null;
  contract_id: number | null;
  type: "debit" | "credit" | "dispute_credit" | "manual_invoice" | "topup";
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
  handler_id: string;
  name: string;
  balance: number;
  lead_count: number;
}

export interface BuyerDetail {
  id: number;
  public_id: string;
  handler_id: string;
  name: string;
  website: string;
  timezone: string;
  balance: number;
  type: string;
  admin_name: string;
  admin_email: string;
  admin_status?: string;
}

export interface PublisherSummary {
  id: number;
  public_id: string;
  handler_id: string;
  name: string;
  lead_count: number;
  collaboration_status: string;
}

export interface PublisherDetail {
  id: number;
  public_id: string;
  handler_id: string;
  name: string;
  website: string;
  timezone: string;
  type: string;
  admin_name: string;
  admin_email: string;
}

export type UserStatus = "pending" | "active" | "inactive";

export interface UserRow {
  id: number;
  invite_id: number;
  public_id?: string;
  email: string;
  full_name: string;
  role: Role;
  status: UserStatus;
  avatar_url?: string | null;
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

export interface CollaborationAuditEntry {
  id: number;
  event_type: string;
  actor_user_id?: number;
  actor_name?: string;
  buyer_name?: string;
  metadata: Record<string, unknown>;
  created_at: string;
}

export interface AuditLogListResponse {
  items: CollaborationAuditEntry[];
  total: number;
  page: number;
  limit: number;
}

export interface AuditLogActor {
  id: number;
  name: string;
}

export interface CollaborationStatus {
  status: "none" | "active" | "revoked" | "pending_buyer" | "pending_publisher";
  version?: number;
  auto_granted?: boolean;
  publisher_name?: string;
  buyer_name?: string;
  buyer_id?: string;
  target_publisher_user_name?: string;
  requested_by_name?: string;
  created_at?: string;
  revoked_at?: string;
  audit_log?: CollaborationAuditEntry[];
}

export interface BuyerCollabSummary {
  buyer_id: number;
  status: string;
  version: number;
}

export interface BuyerPublisher {
  id: string;
  name: string;
  website?: string;
  collaboration_status: string;
}

export interface Partnership {
  id: number;
  status: string;
  requested_by: string;
  partner_name: string;
  partner_handler_id: string;
  created_at: string;
}

export interface IntegrationProvider {
  slug: string;
  name: string;
  description: string;
  auth_type: "none" | "api_key" | "oauth2";
  direction: string;
  config_schema: Record<string, { type: string; label: string; required?: boolean; enum?: string[] }>;
}

export interface IntegrationConnection {
  id: number;
  public_id: string;
  provider_slug: string;
  provider_name: string;
  name: string;
  config: Record<string, unknown>;
  status: string;
  last_error?: string | null;
  last_used_at?: string | null;
  created_at: string;
}

export interface RouteIntegration {
  id: number;
  route_id: number;
  connection_id: number;
  connection_name: string;
  provider_slug: string;
  delivery_config: Record<string, unknown>;
  is_active: boolean;
}

export interface IntegrationDeliveryItem {
  id: number;
  lead_id: number;
  provider: string;
  status: string;
  attempts: number;
  external_id?: string | null;
  last_error?: string | null;
  delivered_at?: string | null;
  created_at: string;
}
