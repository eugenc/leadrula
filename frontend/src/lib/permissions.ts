import type { AccountType, CurrentUser, Role } from "@/types";

export const LeadScopeAll = "all";
export const LeadScopeAssigned = "assigned";
export const LeadScopeFollowed = "followed";
export const LeadScopeAssignedAndFollowed = "assigned_and_followed";

export const ActionSettingsAdmin = "settings_admin";
export const ActionPipelinesRouting = "pipelines_routing";
export const ActionContractsPartners = "contracts_partners";
export const ActionBilling = "billing";
export const ActionAppointmentsManage = "appointments_manage";

export const NavDashboard = "dashboard";
export const NavLeads = "leads";
export const NavFields = "fields";
export const NavAppointments = "appointments";
export const NavCalendars = "calendars";
export const NavCalls = "calls";
export const NavBoard = "board";
export const NavPipelines = "pipelines";
export const NavBuyers = "buyers";
export const NavPublishers = "publishers";
export const NavContracts = "contracts";
export const NavCollaboration = "collaboration";
export const NavSources = "sources";
export const NavWebhooks = "webhooks";
export const NavRouting = "routing";
export const NavLogs = "logs";
export const NavRoutes = "routes";
export const NavSettings = "settings";
export const NavBilling = "billing";
export const NavIntegrations = "integrations";

export type LeadScope =
  | typeof LeadScopeAll
  | typeof LeadScopeAssigned
  | typeof LeadScopeFollowed
  | typeof LeadScopeAssignedAndFollowed;

export type ActionKey =
  | typeof ActionSettingsAdmin
  | typeof ActionPipelinesRouting
  | typeof ActionContractsPartners
  | typeof ActionBilling
  | typeof ActionAppointmentsManage;

export interface EffectivePermissions {
  nav: Record<string, boolean>;
  lead_scope: LeadScope;
  actions: Record<ActionKey, boolean>;
}

export interface PermissionOverrides {
  nav?: Record<string, boolean>;
  lead_scope?: LeadScope;
  actions?: Partial<Record<ActionKey, boolean>>;
}

function adminOnlyNav(key: string, accountType: AccountType): boolean {
  switch (key) {
    case NavFields:
    case NavPipelines:
    case NavCollaboration:
    case NavWebhooks:
    case NavIntegrations:
      return true;
    case NavBuyers:
    case NavSources:
    case NavRouting:
      return accountType === "publisher";
    case NavLogs:
      return true;
    default:
      return false;
  }
}

function defaultNavKeys(accountType: AccountType): string[] {
  const keys = [
    NavDashboard, NavLeads, NavFields, NavAppointments, NavCalendars, NavCalls,
    NavBoard, NavPipelines, NavContracts, NavCollaboration, NavWebhooks,
    NavSettings, NavBilling, NavIntegrations,
  ];
  if (accountType === "publisher") return [...keys, NavBuyers, NavSources, NavRouting, NavLogs];
  if (accountType === "buyer") return [...keys, NavPublishers, NavRoutes, NavLogs];
  return keys;
}

export function presetForRole(role: Role, accountType: AccountType): EffectivePermissions {
  const nav: Record<string, boolean> = {};
  for (const k of defaultNavKeys(accountType)) nav[k] = false;

  const actions: Record<ActionKey, boolean> = {
    settings_admin: false,
    pipelines_routing: false,
    contracts_partners: false,
    billing: false,
    appointments_manage: false,
  };

  let lead_scope: LeadScope = LeadScopeAssigned;

  if (role === "admin") {
    lead_scope = LeadScopeAll;
    for (const k of Object.keys(nav)) nav[k] = true;
    for (const k of Object.keys(actions) as ActionKey[]) actions[k] = true;
  } else if (role === "follower") {
    lead_scope = LeadScopeFollowed;
    for (const k of Object.keys(nav)) {
      if (!adminOnlyNav(k, accountType)) nav[k] = true;
    }
  } else {
    for (const k of Object.keys(nav)) {
      if (!adminOnlyNav(k, accountType)) nav[k] = true;
    }
  }

  return { nav, lead_scope, actions };
}

export function resolvePermissions(
  role: Role,
  accountType: AccountType,
  overrides?: PermissionOverrides | Record<string, unknown>
): EffectivePermissions {
  const preset = presetForRole(role, accountType);
  if (!overrides || Object.keys(overrides).length === 0) return preset;

  const o = overrides as PermissionOverrides;
  const out = { ...preset, nav: { ...preset.nav }, actions: { ...preset.actions } };
  if (o.lead_scope) out.lead_scope = o.lead_scope;
  if (o.nav) {
    for (const [k, v] of Object.entries(o.nav)) {
      if (k in out.nav) out.nav[k] = v;
    }
  }
  if (o.actions) {
    for (const [k, v] of Object.entries(o.actions)) {
      if (k in out.actions) out.actions[k as ActionKey] = v;
    }
  }
  return out;
}

export function effectivePermissions(user: CurrentUser | null | undefined): EffectivePermissions | null {
  if (!user?.effective_permissions) return null;
  const ep = user.effective_permissions;
  return {
    nav: ep.nav ?? {},
    lead_scope: (ep.lead_scope as LeadScope) ?? LeadScopeAssigned,
    actions: {
      settings_admin: !!ep.actions?.settings_admin,
      pipelines_routing: !!ep.actions?.pipelines_routing,
      contracts_partners: !!ep.actions?.contracts_partners,
      billing: !!ep.actions?.billing,
      appointments_manage: !!ep.actions?.appointments_manage,
    },
  };
}

export function canNav(user: CurrentUser | null | undefined, key: string): boolean {
  if (!user) return false;
  if (user.account_type === "platform") return true;
  if (user.impersonating || user.is_switched) return true;
  const ep = effectivePermissions(user);
  return ep?.nav[key] ?? presetForRole(user.role, user.account_type).nav[key] ?? false;
}

export function canAction(user: CurrentUser | null | undefined, key: ActionKey): boolean {
  if (!user) return false;
  if (user.account_type === "platform") return true;
  if (user.impersonating || user.is_switched) return true;
  const ep = effectivePermissions(user);
  return ep?.actions[key] ?? false;
}

export function leadScope(user: CurrentUser | null | undefined): LeadScope {
  if (!user) return LeadScopeAssigned;
  if (user.impersonating || user.is_switched) return LeadScopeAll;
  const ep = effectivePermissions(user);
  return ep?.lead_scope ?? presetForRole(user.role, user.account_type).lead_scope;
}

export function leadScopeToFlags(scope: LeadScope): { assigned: boolean; followed: boolean } {
  return {
    assigned: scope === LeadScopeAssigned || scope === LeadScopeAssignedAndFollowed,
    followed: scope === LeadScopeFollowed || scope === LeadScopeAssignedAndFollowed,
  };
}

export function leadScopeFromFlags(
  assigned: boolean,
  followed: boolean,
  role: Role
): LeadScope | null {
  if (assigned && followed) return LeadScopeAssignedAndFollowed;
  if (assigned) return LeadScopeAssigned;
  if (followed) return LeadScopeFollowed;
  if (role === "admin") return LeadScopeAll;
  return null;
}

export function isValidLeadVisibility(role: Role, scope: LeadScope): boolean {
  if (scope === LeadScopeAll) return role === "admin";
  return scope === LeadScopeAssigned || scope === LeadScopeFollowed || scope === LeadScopeAssignedAndFollowed;
}

export function canCreateLead(user: CurrentUser | null | undefined): boolean {
  return leadScope(user) !== LeadScopeFollowed;
}

export function canEditLead(
  user: CurrentUser | null | undefined,
  lead: { assigned_user_id?: number | null }
): boolean {
  const scope = leadScope(user);
  if (scope === LeadScopeAll) return true;
  if (scope === LeadScopeFollowed) return false;
  return lead.assigned_user_id === Number(user?.id);
}

export function canSeeAllLeads(user: CurrentUser | null | undefined): boolean {
  return leadScope(user) === LeadScopeAll;
}

export function deltaFromEffective(
  role: Role,
  accountType: AccountType,
  effective: EffectivePermissions
): PermissionOverrides {
  const preset = presetForRole(role, accountType);
  const overrides: PermissionOverrides = {};
  if (effective.lead_scope !== preset.lead_scope) overrides.lead_scope = effective.lead_scope;

  const nav: Record<string, boolean> = {};
  for (const k of Object.keys(preset.nav)) {
    if (effective.nav[k] !== preset.nav[k]) nav[k] = effective.nav[k];
  }
  if (Object.keys(nav).length > 0) overrides.nav = nav;

  const actions: Partial<Record<ActionKey, boolean>> = {};
  for (const k of Object.keys(preset.actions) as ActionKey[]) {
    if (effective.actions[k] !== preset.actions[k]) actions[k] = effective.actions[k];
  }
  if (Object.keys(actions).length > 0) overrides.actions = actions;

  return overrides;
}

export interface NavSection {
  label: string;
  keys: { key: string; label: string }[];
}

export function navSections(accountType: AccountType): NavSection[] {
  const leads = [
    { key: NavLeads, label: "Leads" },
    { key: NavFields, label: "Custom Fields" },
  ];
  const appointments = [
    { key: NavAppointments, label: "Appointments" },
    { key: NavCalendars, label: "Calendars" },
  ];
  const pipeline = [
    { key: NavBoard, label: "Pipeline" },
    { key: NavPipelines, label: "Pipelines" },
  ];
  const settings = [
    { key: NavSettings, label: "Settings" },
    { key: NavBilling, label: "Billing" },
    { key: NavIntegrations, label: "Integrations" },
  ];

  if (accountType === "publisher") {
    return [
      { label: "Overview", keys: [{ key: NavDashboard, label: "Dashboard" }] },
      { label: "Leads", keys: leads },
      { label: "Appointments", keys: appointments },
      { label: "Calls", keys: [{ key: NavCalls, label: "Calls" }] },
      { label: "Pipeline", keys: pipeline },
      {
        label: "Buyers",
        keys: [
          { key: NavBuyers, label: "Buyers" },
          { key: NavContracts, label: "Contracts" },
          { key: NavCollaboration, label: "Collaboration" },
        ],
      },
      {
        label: "Routing",
        keys: [
          { key: NavSources, label: "Sources" },
          { key: NavWebhooks, label: "Webhooks" },
          { key: NavRouting, label: "Routing" },
          { key: NavLogs, label: "Logs" },
        ],
      },
      { label: "Settings", keys: settings },
    ];
  }

  return [
    { label: "Overview", keys: [{ key: NavDashboard, label: "Dashboard" }] },
    { label: "Leads", keys: leads },
    { label: "Appointments", keys: appointments },
    { label: "Calls", keys: [{ key: NavCalls, label: "Calls" }] },
    { label: "Pipeline", keys: pipeline },
    {
      label: "Publishers",
      keys: [
        { key: NavPublishers, label: "Publishers" },
        { key: NavContracts, label: "Contracts" },
        { key: NavCollaboration, label: "Collaboration" },
      ],
    },
    {
      label: "Routing",
      keys: [
        { key: NavRoutes, label: "Routes" },
        { key: NavWebhooks, label: "Webhooks" },
        { key: NavLogs, label: "Logs" },
      ],
    },
    { label: "Settings", keys: settings },
  ];
}

export const ACTION_OPTIONS: { key: ActionKey; label: string }[] = [
  { key: ActionSettingsAdmin, label: "Manage users, API keys & business profile" },
  { key: ActionPipelinesRouting, label: "Manage pipelines, fields & routing" },
  { key: ActionContractsPartners, label: "Manage contracts & partners" },
  { key: ActionBilling, label: "Manage billing" },
  { key: ActionAppointmentsManage, label: "Manage calendars & booking slots" },
];
