import { format } from "date-fns";
import {
  Building2,
  Calendar,
  CalendarClock,
  CalendarPlus,
  CheckSquare,
  CircleDot,
  FileText,
  Hash,
  List,
  Mail,
  MapPin,
  Import,
  Phone,
  Tag,
  Type,
  User,
  Zap,
  type LucideIcon,
} from "lucide-react";
import type { CustomField, Lead } from "@/types";
import { formatMoney } from "@/lib/utils";
import { formatCustomDateForDisplay } from "./customFieldDate";

export const STATUS_LABELS: Record<string, string> = {
  review: "In Review",
  distributed: "Distributed",
  returned: "Returned",
  closed: "Closed",
};

export function formatStatus(status: string): string {
  return STATUS_LABELS[status] ?? status.charAt(0).toUpperCase() + status.slice(1);
}

export function leadSourceLabel(lead: Lead): string {
  return lead.source_name ?? lead.source ?? "—";
}

export interface SystemColumn {
  id: string;
  label: string;
  sortKey?: string;
}

export const SYSTEM_COLUMNS: SystemColumn[] = [
  { id: "name", label: "Name", sortKey: "first_name" },
  { id: "phone", label: "Phone", sortKey: "phone" },
  { id: "email", label: "Email", sortKey: "email" },
  { id: "source", label: "Source", sortKey: "source_name" },
  { id: "buyer", label: "Buyer", sortKey: "buyer_name" },
  { id: "assignee", label: "Assignee", sortKey: "assignee_name" },
  { id: "pipeline", label: "Pipeline", sortKey: "pipeline_name" },
  { id: "stage", label: "Pipeline Stage", sortKey: "stage_name" },
  { id: "status", label: "Status", sortKey: "status" },
  { id: "tags", label: "Tags" },
  { id: "action_at", label: "Action At", sortKey: "action_at" },
  { id: "created_at", label: "Created", sortKey: "created_at" },
  { id: "address", label: "Address", sortKey: "address" },
  { id: "city", label: "City", sortKey: "city" },
  { id: "state", label: "State", sortKey: "state" },
  { id: "zip", label: "Zip", sortKey: "zip" },
  { id: "cost", label: "Cost" },
  { id: "revenue", label: "Revenue" },
  { id: "net_profit", label: "Net Profit" },
];

export const PIPELINE_COLUMNS: SystemColumn[] = [
  { id: "stage_entered_at", label: "Time in stage", sortKey: "stage_entered_at" },
  { id: "position", label: "Manual order", sortKey: "position" },
];

export const DEFAULT_VISIBLE_COLUMNS = [
  "name",
  "phone",
  "source",
  "buyer",
  "assignee",
  "pipeline",
  "stage",
  "status",
  "action_at",
  "created_at",
];

export const DEFAULT_BOARD_CARD_FIELDS = [
  "name",
  "assignee",
  "action_at",
  "status",
  "phone",
  "email",
  "source",
];

export const BOARD_CARD_FIELDS_PREF_KEY = "board_card_fields";

export function columnsEqual(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((c, i) => c === b[i]);
}

export function resetToDefaultColumns(defaultCols: string[], validIds: string[]): string[] {
  const next = defaultCols.filter((id) => validIds.includes(id));
  return next.length ? next : [...defaultCols];
}

export function parseBoardCardFields(raw: unknown): string[] | null {
  if (!Array.isArray(raw)) return null;
  const cols = raw.filter((c): c is string => typeof c === "string" && c.length > 0);
  return cols.length ? cols : null;
}

export function resolveBoardCardFields(prefs: Record<string, unknown> | undefined): string[] {
  const saved = parseBoardCardFields(prefs?.[BOARD_CARD_FIELDS_PREF_KEY]);
  return saved ?? [...DEFAULT_BOARD_CARD_FIELDS];
}

export function normalizeBoardCardFields(cols: string[], validIds: string[]): string[] {
  const filtered = cols.filter((id) => validIds.includes(id));
  return filtered.length ? filtered : [...DEFAULT_BOARD_CARD_FIELDS];
}

export function boardCardFields(columns?: string[]): string[] {
  const filtered = (columns ?? []).filter((c) => c !== "name");
  return filtered.length ? filtered : [...DEFAULT_BOARD_CARD_FIELDS];
}

const STORAGE_KEY = "leads-table-columns";

export function loadVisibleColumns(): string[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_VISIBLE_COLUMNS;
    const parsed = JSON.parse(raw) as string[];
    const cols = parsed.map((c) => (c === "campaign" ? "source" : c));
    return cols.length ? cols : DEFAULT_VISIBLE_COLUMNS;
  } catch {
    return DEFAULT_VISIBLE_COLUMNS;
  }
}

export function saveVisibleColumns(cols: string[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(cols));
}

export function formatTimeInStage(enteredAt: string): string {
  const ms = Date.now() - new Date(enteredAt).getTime();
  if (ms < 0) return "—";
  const days = Math.floor(ms / 86400000);
  if (days >= 1) return `${days} day${days === 1 ? "" : "s"}`;
  const hours = Math.floor(ms / 3600000);
  if (hours >= 1) return `${hours} hour${hours === 1 ? "" : "s"}`;
  const mins = Math.floor(ms / 60000);
  if (mins >= 1) return `${mins} min${mins === 1 ? "" : "s"}`;
  return "Just now";
}

function renderCustomValue(v: unknown, field?: CustomField): string {
  if (v == null) return "—";
  if (field && (field.type === "date" || field.type === "datetime") && typeof v === "string") {
    return formatCustomDateForDisplay(v, field.type, field.format);
  }
  if (typeof v === "string") {
    if (!v) return "—";
    return v;
  }
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  return JSON.stringify(v);
}

export function cellValue(lead: Lead, colId: string, customFields: CustomField[]): string {
  switch (colId) {
    case "name":
      return `${lead.first_name} ${lead.last_name}`.trim() || "—";
    case "phone":
      return lead.phone ?? "—";
    case "email":
      return lead.email ?? "—";
    case "source":
      return leadSourceLabel(lead);
    case "buyer":
      return lead.buyer_name ?? lead.preassigned_buyer_name ?? "—";
    case "assignee":
      return lead.assignee_name ?? "—";
    case "pipeline":
      return lead.pipeline_name ?? "—";
    case "stage":
      return lead.stage_name ?? "—";
    case "status":
      return formatStatus(lead.status);
    case "tags":
      return (lead.tags ?? []).length ? (lead.tags ?? []).join(", ") : "—";
    case "action_at":
      return lead.action_at ? format(new Date(lead.action_at), "MMM d, h:mm a") : "—";
    case "created_at":
      return format(new Date(lead.created_at), "MMM d, yyyy");
    case "stage_entered_at":
      return lead.stage_entered_at ? formatTimeInStage(lead.stage_entered_at) : "—";
    case "address":
      return lead.address ?? "—";
    case "city":
      return lead.city ?? "—";
    case "state":
      return lead.state ?? "—";
    case "zip":
      return lead.zip ?? "—";
    case "cost":
      return lead.cost != null ? formatMoney(lead.cost) : "—";
    case "revenue":
      return lead.revenue != null ? formatMoney(lead.revenue) : "—";
    case "net_profit":
      return lead.net_profit != null ? formatMoney(lead.net_profit) : "—";
    default:
      if (colId.startsWith("custom_")) {
        const fieldId = colId.slice(7);
        const field = customFields.find((f) => String(f.id) === fieldId);
        if (!field) return "—";
        return renderCustomValue(lead.custom_values?.[fieldId], field);
      }
      return "—";
  }
}

export function columnLabel(colId: string, customFields: CustomField[]): string {
  if (colId.startsWith("custom_")) {
    const field = customFields.find((f) => String(f.id) === colId.slice(7));
    return field?.name ?? colId;
  }
  return [...SYSTEM_COLUMNS, ...PIPELINE_COLUMNS].find((c) => c.id === colId)?.label ?? colId;
}

export function columnSortKey(colId: string): string | undefined {
  if (colId.startsWith("custom_")) return colId;
  return [...SYSTEM_COLUMNS, ...PIPELINE_COLUMNS].find((c) => c.id === colId)?.sortKey;
}

export function sortKeyLabel(sortKey: string, customFields: CustomField[]): string {
  if (sortKey.startsWith("custom_")) {
    return columnLabel(sortKey, customFields);
  }
  for (const c of [...SYSTEM_COLUMNS, ...PIPELINE_COLUMNS]) {
    if (c.sortKey === sortKey) return c.label;
  }
  return sortKey;
}

export function boardSortOptions(customFields: CustomField[]): { group: string; sortKey: string; label: string }[] {
  const lead = SYSTEM_COLUMNS.filter((c) => c.sortKey).map((c) => ({
    group: "Lead fields",
    sortKey: c.sortKey!,
    label: c.label,
  }));
  const custom = customFields
    .filter((f) => f.is_active)
    .map((f) => ({
      group: "Lead fields",
      sortKey: `custom_${f.id}`,
      label: f.name,
    }));
  const pipeline = PIPELINE_COLUMNS.filter((c) => c.sortKey).map((c) => ({
    group: "Pipeline",
    sortKey: c.sortKey!,
    label: c.label,
  }));
  return [...lead, ...custom, ...pipeline];
}

const SYSTEM_COLUMN_ICONS: Record<string, LucideIcon> = {
  phone: Phone,
  email: Mail,
  source: Import,
  buyer: Building2,
  assignee: User,
  status: CircleDot,
  tags: Tag,
  action_at: Zap,
  created_at: CalendarPlus,
  stage_entered_at: CalendarClock,
  address: MapPin,
  city: MapPin,
  state: MapPin,
  zip: MapPin,
};

function customFieldIcon(type: CustomField["type"]): LucideIcon {
  switch (type) {
    case "text":
      return Type;
    case "number":
      return Hash;
    case "date":
      return Calendar;
    case "datetime":
      return CalendarClock;
    case "dropdown":
      return List;
    case "checkbox":
      return CheckSquare;
    default:
      return FileText;
  }
}

export function columnIcon(colId: string, customFields: CustomField[] = []): LucideIcon {
  if (colId.startsWith("custom_")) {
    const field = customFields.find((f) => String(f.id) === colId.slice(7));
    return field ? customFieldIcon(field.type) : FileText;
  }
  return SYSTEM_COLUMN_ICONS[colId] ?? FileText;
}
