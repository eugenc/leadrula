import type { CollaborationAuditEntry } from "@/types";

interface AuditChange {
  field?: string;
  from?: string;
  to?: string;
}

const AREA_LABELS: Record<string, string> = {
  leads: "Leads",
  pipelines: "Pipelines",
  stages: "Pipelines",
  "stage-rules": "Pipelines",
  "custom-fields": "Custom fields",
  billing: "Billing",
  collaboration: "Collaboration",
  contract: "Contracts",
  partnerships: "Partnerships",
  "api-keys": "API keys",
  notifications: "Notifications",
  users: "Users",
  routes: "Routes",
};

function str(v: unknown): string {
  return typeof v === "string" ? v : "";
}

function parseChanges(metadata: Record<string, unknown>): AuditChange[] {
  const raw = metadata.changes;
  if (!Array.isArray(raw)) return [];
  return raw.filter((c): c is AuditChange => typeof c === "object" && c !== null);
}

function formatChangeLine(change: AuditChange): string {
  const field = str(change.field);
  const from = str(change.from);
  const to = str(change.to);
  if (field === "Stage") return `${from} > ${to}`;
  if (field) return `${field}: ${from} > ${to}`;
  return `${from} > ${to}`;
}

function formatChanges(metadata: Record<string, unknown>, path: string): string | null {
  const changes = parseChanges(metadata);
  if (changes.length === 0) return null;

  const normalized = path.replace(/^\/buyer\/?/, "").replace(/^\//, "");
  const areaKey = normalized.split("/").filter(Boolean)[0] ?? "Activity";
  const area = AREA_LABELS[areaKey] ?? areaKey.replace(/-/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());

  if (changes.length === 1 && changes[0].field === "Stage") {
    return `${area} > ${formatChangeLine(changes[0])}`;
  }
  return `${area} > ${changes.map(formatChangeLine).join(", ")}`;
}

function actionFromPath(method: string, segments: string[]): string | null {
  const joined = segments.join("/");

  if (joined === "leads/import") return "Imported leads";
  if (joined === "leads/bulk") return "Bulk updated leads";
  if (joined.match(/^leads\/[^/]+\/stage$/)) return "Changed stage";
  if (joined.match(/^leads\/[^/]+\/action$/)) return "Updated action";
  if (joined.match(/^leads\/[^/]+\/notes$/)) return method === "POST" ? "Added note" : "Updated notes";
  if (joined.match(/^leads\/[^/]+\/followers$/)) return "Added follower";
  if (joined.match(/^leads\/[^/]+\/followers\/[^/]+$/)) return "Removed follower";
  if (joined === "leads/views") return method === "POST" ? "Created view" : "Updated view";
  if (joined.match(/^leads\/views\/[^/]+$/)) return method === "DELETE" ? "Deleted view" : "Updated view";
  if (joined === "leads") {
    if (method === "POST") return "Created lead";
    return null;
  }
  if (joined.match(/^leads\/[^/]+$/)) {
    if (method === "DELETE") return "Deleted lead";
    if (method === "PATCH") return "Updated lead";
    return null;
  }

  if (joined === "pipelines") return method === "POST" ? "Created pipeline" : null;
  if (joined.match(/^pipelines\/[^/]+$/)) {
    if (method === "DELETE") return "Deleted pipeline";
    if (method === "PATCH") return "Updated pipeline";
    return null;
  }
  if (joined.match(/^pipelines\/[^/]+\/stages$/)) return method === "POST" ? "Added stage" : null;
  if (joined.match(/^pipelines\/[^/]+\/stages\/reorder$/)) return "Reordered stages";
  if (joined.match(/^stages\/[^/]+$/)) {
    if (method === "DELETE") return "Deleted stage";
    if (method === "PATCH") return "Updated stage";
    return null;
  }
  if (joined.match(/^stages\/[^/]+\/rules$/)) return "Added stage rule";
  if (joined.match(/^stage-rules\/[^/]+$/)) {
    if (method === "DELETE") return "Deleted stage rule";
    if (method === "PATCH") return "Updated stage rule";
    return null;
  }

  if (joined === "custom-fields") return method === "POST" ? "Created field" : null;
  if (joined === "custom-fields/import") return "Imported fields";
  if (joined.match(/^custom-fields\/[^/]+$/)) {
    if (method === "DELETE") return "Deleted field";
    if (method === "PATCH") return "Updated field";
    return null;
  }
  if (joined.match(/^stages\/[^/]+\/disqualification-reasons$/)) {
    return method === "POST" ? "Created disqualification reason" : null;
  }
  if (joined === "disqualification-reasons") return method === "POST" ? "Created reason" : null;
  if (joined.match(/^disqualification-reasons\/[^/]+$/)) {
    if (method === "DELETE") return "Deleted reason";
    if (method === "PATCH") return "Updated reason";
    return null;
  }

  if (joined === "billing/balance/topup") return "Topped up balance";
  if (joined === "billing/balance/topup-intent") return "Started balance top-up";
  if (joined === "billing/disputes") return method === "POST" ? "Opened dispute" : null;

  if (joined === "collaboration/invite") return "Sent invitation";
  if (joined === "collaboration/accept") return "Accepted invitation";
  if (joined === "collaboration/reject") return "Rejected invitation";
  if (joined === "collaboration") return method === "DELETE" ? "Revoked access" : null;

  if (joined === "contract/return-rules") return method === "POST" ? "Added return rule" : null;
  if (joined.match(/^contract\/return-rules\/[^/]+$/)) {
    if (method === "DELETE") return "Deleted return rule";
    if (method === "PATCH") return "Updated return rule";
    return null;
  }

  if (joined === "partnerships/request") return "Sent partnership request";
  if (joined.match(/^partnerships\/[^/]+\/accept$/)) return "Accepted partnership";
  if (joined.match(/^partnerships\/[^/]+\/reject$/)) return "Rejected partnership";

  if (joined === "api-keys") return method === "POST" ? "Created API key" : null;
  if (joined.match(/^api-keys\/[^/]+$/)) return "Revoked API key";

  if (joined.match(/^notifications\/[^/]+\/read$/)) return "Marked notification read";

  if (joined === "users/invite") return "Invited user";
  if (joined.match(/^users\/invites\/[^/]+$/)) {
    if (method === "DELETE") return "Deleted invite";
    if (method === "PATCH") return "Updated invite";
    return null;
  }
  if (joined.match(/^users\/invites\/[^/]+\/resend$/)) return "Resent invite";
  if (joined.match(/^users\/[^/]+$/)) {
    if (method === "DELETE") return "Deleted user";
    if (method === "PATCH") return "Updated user";
    return null;
  }
  if (joined.match(/^users\/[^/]+\/avatar$/)) return "Updated avatar";

  return null;
}

function genericAction(method: string, segments: string[]): string {
  const tail = segments.slice(1).join("/") || segments[0] || "resource";
  if (method === "POST") return `Created ${tail}`;
  if (method === "PATCH" || method === "PUT") return `Updated ${tail}`;
  if (method === "DELETE") return `Deleted ${tail}`;
  return `${method} ${tail}`;
}

function formatAction(method: string, path: string): string {
  const normalized = path.replace(/^\/buyer\/?/, "").replace(/^\//, "");
  const segments = normalized.split("/").filter(Boolean);
  if (segments.length === 0) return `${method} request`;

  const areaKey = segments[0];
  const area = AREA_LABELS[areaKey] ?? areaKey.replace(/-/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
  const action = actionFromPath(method, segments) ?? genericAction(method, segments);
  return `${area} > ${action}`;
}

function formatSessionStart(metadata: Record<string, unknown>): string {
  const buyerName = str(metadata.buyer_name);
  if (buyerName) return `Session > Started (${buyerName})`;
  return "Session > Started";
}

const LIFECYCLE_LABELS: Record<string, string> = {
  granted: "Access > Granted",
  revoked: "Access > Revoked",
  request_sent: "Access > Invitation sent",
  request_accepted: "Access > Invitation accepted",
  request_rejected: "Access > Invitation rejected",
};

export function formatCollaborationAuditEntry(entry: CollaborationAuditEntry): string {
  switch (entry.event_type) {
    case "impersonation_start":
      return formatSessionStart(entry.metadata);
    case "impersonation_end":
      return "Session > Ended";
    case "impersonation_action": {
      const path = str(entry.metadata.path);
      const fromChanges = formatChanges(entry.metadata, path);
      if (fromChanges) return fromChanges;
      const method = str(entry.metadata.method).toUpperCase() || "REQUEST";
      if (path) return formatAction(method, path);
      return "Session > Action";
    }
    default:
      return LIFECYCLE_LABELS[entry.event_type] ?? entry.event_type.replace(/_/g, " ");
  }
}
