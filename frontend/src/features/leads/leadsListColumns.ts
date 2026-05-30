import { format } from "date-fns";
import {
  Building2,
  Calendar,
  CalendarClock,
  CheckSquare,
  CircleDot,
  FileText,
  Hash,
  List,
  Mail,
  MapPin,
  Megaphone,
  Phone,
  Tag,
  Type,
  User,
  type LucideIcon,
} from "lucide-react";
import type { CustomField, Lead } from "@/types";

export const STATUS_LABELS: Record<string, string> = {
  review: "In Review",
  distributed: "Distributed",
  returned: "Returned",
  closed: "Closed",
};

export function formatStatus(status: string): string {
  return STATUS_LABELS[status] ?? status.charAt(0).toUpperCase() + status.slice(1);
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
  { id: "campaign", label: "Campaign", sortKey: "campaign_name" },
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
];

export const DEFAULT_VISIBLE_COLUMNS = [
  "name",
  "phone",
  "campaign",
  "buyer",
  "assignee",
  "pipeline",
  "stage",
  "status",
  "action_at",
  "created_at",
];

export const DEFAULT_BOARD_CARD_FIELDS = ["action_at", "phone", "tags"];

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
    return parsed.length ? parsed : DEFAULT_VISIBLE_COLUMNS;
  } catch {
    return DEFAULT_VISIBLE_COLUMNS;
  }
}

export function saveVisibleColumns(cols: string[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(cols));
}

function renderCustomValue(v: unknown): string {
  if (v == null) return "—";
  if (typeof v === "string") return v || "—";
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
    case "campaign":
      return lead.campaign_name ?? "—";
    case "buyer":
      return lead.buyer_name ?? "—";
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
      return lead.action_at ? format(new Date(lead.action_at), "MMM d, h:mma") : "—";
    case "created_at":
      return format(new Date(lead.created_at), "MMM d, yyyy");
    case "address":
      return lead.address ?? "—";
    case "city":
      return lead.city ?? "—";
    case "state":
      return lead.state ?? "—";
    case "zip":
      return lead.zip ?? "—";
    default:
      if (colId.startsWith("custom_")) {
        const fieldId = colId.slice(7);
        const field = customFields.find((f) => String(f.id) === fieldId);
        if (!field) return "—";
        return renderCustomValue(lead.custom_values?.[fieldId]);
      }
      return "—";
  }
}

export function columnLabel(colId: string, customFields: CustomField[]): string {
  if (colId.startsWith("custom_")) {
    const field = customFields.find((f) => String(f.id) === colId.slice(7));
    return field?.name ?? colId;
  }
  return SYSTEM_COLUMNS.find((c) => c.id === colId)?.label ?? colId;
}

export function columnSortKey(colId: string): string | undefined {
  return SYSTEM_COLUMNS.find((c) => c.id === colId)?.sortKey;
}

const SYSTEM_COLUMN_ICONS: Record<string, LucideIcon> = {
  phone: Phone,
  email: Mail,
  campaign: Megaphone,
  buyer: Building2,
  assignee: User,
  status: CircleDot,
  tags: Tag,
  action_at: CalendarClock,
  created_at: Calendar,
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
    case "datetime":
      return Calendar;
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
