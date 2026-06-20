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
    website: string;
    contact_email: string;
    phone: string;
    address_line1: string;
    address_line2: string;
    city: string;
    state: string;
    postal_code: string;
    country: string;
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
  access_via?: "switch" | "impersonate";
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

export type RuleConditionDomain = "lead" | "pipeline" | "payload";
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
  /** Literal value, date mode object, or field copy: `{ from_field: "custom:foo" }` */
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
  country: string | null;
  address_place_id: string | null;
  source: string | null;
  external_id: string | null;
  pipeline_id: number | null;
  stage_id: number | null;
  publisher_pipeline_id?: number | null;
  publisher_stage_id?: number | null;
  board_stage_id?: number | null;
  position: number;
  assigned_user_id: number | null;
  action_at: string | null;
  status: "review" | "distributed" | "returned" | "closed" | "disputed";
  disqualification_reason_id: number | null;
  created_at: string;
  updated_at: string;
  custom_values: Record<string, unknown>;
  buyer_name?: string | null;
  preassigned_buyer_id?: number | null;
  preassigned_buyer_name?: string | null;
  source_name?: string | null;
  assignee_name?: string | null;
  assignee_avatar_url?: string | null;
  pipeline_name?: string | null;
  stage_name?: string | null;
  stage_entered_at?: string | null;
  tags?: string[];
  cost?: number | null;
  revenue?: number | null;
  gross_profit?: number | null;
  net_profit?: number | null;
  purchase_price?: number | null;
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

export type LeadHistoryKind =
  | "stage_change"
  | "account_transfer"
  | "route_run"
  | "purchase"
  | "refund"
  | "dispute_opened"
  | "dispute_resolved"
  | "webhook"
  | "outbound_webhook"
  | "integration"
  | "lead_created"
  | "pipeline_placed"
  | "status_change"
  | "field_change"
  | "assignee_change"
  | "tag_change"
  | "calendar_event"
  | "follower_added"
  | "follower_removed"
  | "lead_deleted"
  | "pipeline_cleared"
  | "imported"
  | "note_added";

export interface LeadHistoryEntry {
  id: number;
  kind: LeadHistoryKind;
  created_at: string;
  actor_type?: string | null;
  actor_name?: string | null;
  actor_detail?: string | null;
  status?: string | null;
  summary?: string | null;
  amount?: number | null;
  from_stage_name?: string | null;
  to_stage_name?: string | null;
  moved_by_name?: string | null;
  action_at_captured?: string | null;
  disqualification_reason?: string | null;
  account_name?: string | null;
  account_type?: "buyer" | "publisher" | null;
  transfer_kind?: "sold" | "returned" | "redistributed" | null;
  from_account_name?: string | null;
  to_account_name?: string | null;
  trigger_label?: string | null;
  field_name?: string | null;
  from_value?: string | null;
  to_value?: string | null;
  pipeline_name?: string | null;
  stage_name?: string | null;
  route_name?: string | null;
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
  folder_id?: number | null;
}

export interface CustomFieldFolder {
  id: number;
  name: string;
  position: number;
  is_system?: boolean;
  system_key?: string | null;
  contact_builtin_order?: string[] | null;
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
  buyer_id?: number | null;
  buyer_name?: string;
  buyer_account_type?: string;
  publisher_name?: string;
  name: string;
  description?: string;
  contract_type?: string;
  mirror_contract_id?: number | null;
  lead_type?: string;
  source_pipeline_id?: number | null;
  source_stage_id?: number | null;
  buyer_pipeline_id?: number | null;
  return_stage_id?: number | null;
  rate_per_lead: number;
  status: string;
  cap_period?: string;
  cap_total?: number | null;
  cap_max_daily?: number | null;
  lead_count?: number;
  delivery?: string;
  buyer_target_stage_id?: number | null;
  integration_connection_id?: number | null;
  outbound_webhook_id?: number | null;
  allowed_delivery_modes?: string[];
  distribution_strategy?: string;
  parent_contract_id?: number | null;
  invite_token?: string;
  participations?: ContractParticipation[];
}

export interface ContractParticipation {
  id: number;
  contract_id: number;
  buyer_id: number;
  buyer_name?: string;
  status: string;
  delivery?: string;
  buyer_pipeline_id?: number | null;
  buyer_target_stage_id?: number | null;
  source_pipeline_id?: number | null;
  source_stage_id?: number | null;
  return_stage_id?: number | null;
  integration_connection_id?: number | null;
  outbound_webhook_id?: number | null;
  counter_proposal?: unknown;
  contract_name?: string;
  publisher_name?: string;
  lead_type?: string;
  allowed_delivery_modes?: string[];
  contract_handler_id?: string;
  contract_status?: string;
  contract_description?: string;
  cap_period?: string;
  cap_total?: number | null;
  cap_max_daily?: number | null;
  lead_count?: number;
  rate_per_lead?: number;
  created_at?: string;
  updated_at?: string;
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

export interface ContractFieldMapEntry {
  id?: number;
  src_type: string;
  src_builtin?: string;
  src_custom_field_id?: number | null;
  dst_type: string;
  dst_builtin?: string;
  dst_custom_field_id?: number | null;
}

export interface ContractAvailableField {
  field_type: string;
  builtin_field?: string;
  custom_field_id?: number | null;
  label: string;
  key: string;
}

export interface ContractFieldMapOptions {
  available_fields: ContractAvailableField[];
  buyer_fields: CustomField[];
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
  payout_frequency?: string | null;
  payout_weekday?: number | null;
  payout_month_day?: number | null;
}

export interface ReturnRule {
  id: number;
  contract_id: number;
  participation_id?: number;
  buyer_stage_id: number;
  return_stage_id: number;
  buyer_stage_name?: string;
}

export interface ParticipationReturnRule extends ReturnRule {
  buyer_name: string;
  buyer_stage_name: string;
}

export type SourceType = "webhook";

export interface Source {
  id: number;
  name: string;
  slug: string;
  type: SourceType;
  is_active: boolean;
  api_key_required: boolean;
}

export interface RouteBranch {
  name?: string;
  position: number;
  condition_logic: "and" | "or";
  conditions: RuleCondition[];
  destination: "contract" | "pipeline" | "webhook" | "integration";
  delivery: "leads" | "leads_pipeline";
  target_pipeline_id: number | null;
  target_stage_id: number | null;
  contract_id: number | null;
  compensation_id?: number | null;
  dest_webhook_id: number | null;
}

export interface Route {
  id: number;
  buyer_id?: number | null;
  name: string;
  origin: "source" | "pipeline" | "webhook" | "integration";
  source_id: number | null;
  source_name?: string | null;
  origin_pipeline_id: number | null;
  origin_stage_id: number | null;
  origin_pipeline_name?: string | null;
  origin_stage_name?: string | null;
  origin_webhook_id?: number | null;
  origin_webhook_name?: string | null;
  origin_connection_id?: number | null;
  origin_connection_name?: string | null;
  destination: "contract" | "pipeline" | "webhook" | "integration";
  contract_id: number | null;
  compensation_id?: number | null;
  contract_name?: string | null;
  buyer_name?: string | null;
  delivery: "leads" | "leads_pipeline";
  target_pipeline_id: number | null;
  target_stage_id: number | null;
  target_pipeline_name?: string | null;
  target_stage_name?: string | null;
  dest_webhook_id?: number | null;
  dest_webhook_name?: string | null;
  branches: RouteBranch[];
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
  inbound_secret_required: boolean;
  outbound_enabled: boolean;
  outbound_sign_enabled: boolean;
  outbound_url?: string | null;
  outbound_format?: OutboundFormat;
  outbound_method?: OutboundMethod;
  outbound_payload_template?: string;
  outbound_field_map?: OutboundFieldMapEntry[];
  outbound_response_map?: ResponseMapEntry[];
  integration_connection_id?: number | null;
  integration_provider_slug?: string | null;
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
  note_source_key?: string | null;
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
  connection_name?: string;
  provider_slug?: string;
  event_id?: number | null;
  lead_id?: number | null;
  lead_public_id?: string | null;
  first_name?: string;
  last_name?: string;
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

export interface InboundLogItem {
  kind: "source" | "webhook" | "integration" | "route";
  direction: "inbound" | "outbound";
  id: number;
  created_at: string;
  origin: string;
  origin_slug: string;
  lead_label: string;
  lead_id?: number | null;
  status: string;
  unmapped_keys?: string[];
  first_name?: string;
  last_name?: string;
  phone?: string | null;
  source?: string | null;
  raw_payload?: Record<string, unknown>;
  webhook_id?: number;
  error_message?: string | null;
  provider_slug?: string;
  connection_name?: string;
  attempts?: number;
  route_id?: number | null;
  route_name?: string;
  trigger_type?: string;
  trigger_label?: string;
  target_account_name?: string;
  destination?: string;
  branch_position?: number;
  target_pipeline_name?: string | null;
  target_stage_name?: string | null;
  delivery?: string;
}

export interface InboundLogListResponse {
  items: InboundLogItem[];
  total: number;
  page: number;
  limit: number;
}

export interface HTTPRequestLog {
  method: string;
  url: string;
  headers?: Record<string, string>;
  body?: unknown;
}

export interface DeliveryRequestLog {
  mapped: Record<string, string>;
  http: HTTPRequestLog;
}

export interface IntegrationDeliveryAttemptLog {
  attempt_number: number;
  status: string;
  http_status?: number | null;
  request_body?: DeliveryRequestLog | Record<string, unknown> | null;
  response_body?: string;
  duration_ms?: number | null;
  error?: string | null;
  created_at: string;
}

export interface LabeledCustomField {
  id: number;
  field_key: string;
  name: string;
  value: unknown;
}

export interface IntegrationDeliveryDetail {
  id: number;
  status: string;
  connection_name: string;
  provider_slug: string;
  lead_public_id?: string;
  external_id?: string;
  payload: Record<string, unknown>;
  custom_fields_labeled?: LabeledCustomField[];
  last_error?: string | null;
  attempts: IntegrationDeliveryAttemptLog[];
}

export interface PayoutSummary {
  hold: number;
  cleared: number;
  prepay_balance: number;
  distributed_value: number;
  returned_value: number;
  cleared_from_prepay: number;
}

export interface CompensationPayoutRow {
  compensation_id: number;
  contract_id: number;
  contract_name: string;
  kind: string;
  buyer_kind: string;
  payout_frequency?: string | null;
  payout_weekday?: number | null;
  payout_month_day?: number | null;
  hold: number;
  cleared: number;
  next_period_end?: string | null;
  latest_transfer_status?: string | null;
  invoice_id?: number | null;
  invoice_status?: string | null;
  invoice_public_id?: string | null;
}

export interface PayoutLedgerRow {
  id: number;
  compensation_id: number;
  contract_id: number;
  contract_name: string;
  buyer_name: string;
  buyer_kind: string;
  amount: number;
  period_start: string;
  period_end: string;
  stripe_transfer_id?: string | null;
  stripe_transfer_status: string;
  invoice_status?: string | null;
  created_at: string;
}

export type TxnCategory =
  | "Sale"
  | "Return"
  | "Dispute"
  | "Stage"
  | "Purchase"
  | "Topup"
  | "Credit"
  | "Refund"
  | "Invoice";

export interface Transaction {
  id: number;
  public_id: string;
  buyer_id: number;
  lead_id: number | null;
  lead_name?: string | null;
  buyer_name?: string | null;
  publisher_name?: string | null;
  contract_id: number | null;
  type: string;
  side?: "sale" | "purchase" | "prepay";
  category?: TxnCategory | string;
  counterparty_name?: string | null;
  counterparty_account_type?: string | null;
  ledger_source?: "earning" | "transaction" | "legacy";
  lead_viewable?: boolean;
  amount: number;
  balance_after?: number | null;
  description: string;
  created_at: string;
}

export type DisputeParty = "buyer" | "publisher";

export interface Dispute {
  id: number;
  transaction_id: number;
  buyer_id: number;
  buyer_name?: string;
  counterparty_name?: string;
  counterparty_account_type?: string;
  reason: string;
  status: "open" | "accepted" | "rejected";
  amount?: number;
  created_at: string;
  initiated_by?: DisputeParty;
  lead_id?: number;
  lead_name?: string;
  contract_id?: number;
  deadline_days?: number;
  response_deadline_at?: string;
  awaiting_party?: DisputeParty;
  outcome?: string;
  winner_party?: DisputeParty;
  placement_party?: DisputeParty;
  placement_completed_at?: string;
}

export interface DisputeAttachment {
  id: number;
  message_id: number;
  filename: string;
  content_type: string;
  byte_size: number;
}

export interface DisputeMessage {
  id: number;
  dispute_id: number;
  author_party: DisputeParty;
  author_name?: string;
  kind: string;
  body: string;
  created_at: string;
  attachments?: DisputeAttachment[];
}

export type InvoiceStatus = "open" | "paid" | "void";
export type InvoiceKind = "starting_balance" | "prepay_request" | "compensation_payout";
export type InvoicePaymentMethod =
  | "stripe"
  | "bank_transfer"
  | "check"
  | "cash"
  | "other_digital"
  | "other";

export interface Invoice {
  id: number;
  public_id: string;
  publisher_id: number;
  buyer_id: number;
  buyer_name?: string | null;
  publisher_name?: string | null;
  amount: number;
  description: string;
  kind: InvoiceKind;
  status: InvoiceStatus;
  payment_method?: InvoicePaymentMethod | null;
  payment_note?: string | null;
  paid_at?: string | null;
  created_at: string;
  online_payable?: boolean;
}

export type BuyerKind = "direct" | "marketplace";

export interface BuyerSummary {
  id: number;
  public_id: string;
  handler_id: string;
  name: string;
  buyer_kind: BuyerKind;
  balance: number;
  lead_count: number;
  admin_provisioned?: boolean;
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
  buyer_kind: BuyerKind;
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

export type NotificationChannelPrefs = { in_app: boolean; email: boolean };
export type NotificationPrefs = Record<string, NotificationChannelPrefs>;

export interface NotificationSettingsResponse {
  account?: NotificationPrefs;
  personal: NotificationPrefs;
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

export interface SunbaseInboundWebhook {
  id: number;
  slug: string;
  endpoint: string;
  secret?: string | null;
  secret_required: boolean;
  setup_hint: string;
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
  inbound_webhook?: SunbaseInboundWebhook;
}

export interface SunbaseConnectionDetail {
  connection: IntegrationConnection;
  inbound_webhook?: SunbaseInboundWebhook;
  webhook_ids?: {
    outbound_post: number;
    outbound_get: number;
    inbound: number;
    inbound_webhook_slug?: string;
  };
}

export interface GhlConnectionDetail {
  connection: IntegrationConnection;
  inbound_webhook?: SunbaseInboundWebhook;
  webhook_ids?: {
    inbound: number;
    inbound_webhook_slug?: string;
  };
}

export interface RouteIntegration {
  id: number;
  route_id: number;
  branch_position: number;
  connection_id: number;
  connection_name: string;
  provider_slug: string;
  delivery_config: Record<string, unknown>;
  is_active: boolean;
}

