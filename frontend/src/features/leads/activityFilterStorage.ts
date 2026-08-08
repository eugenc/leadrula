import { useCallback, useEffect, useState } from "react";
import type { LeadHistoryEntry, LeadHistoryKind } from "@/types";

export type ActivityFilterGroup = LeadHistoryKind | "dispute" | "follower" | "return";

const storageKey = (userId: string) => `lead-activity-hidden-groups:${userId}`;

export function activityFilterGroup(kind: LeadHistoryKind): ActivityFilterGroup {
  switch (kind) {
    case "dispute_opened":
    case "dispute_resolved":
      return "dispute";
    case "follower_added":
    case "follower_removed":
      return "follower";
    case "return_scheduled":
    case "return_cancelled":
      return "return";
    default:
      return kind;
  }
}

export function activityGroupLabel(group: ActivityFilterGroup): string {
  switch (group) {
    case "stage_change":
      return "Stage";
    case "account_transfer":
      return "Transfer";
    case "purchase":
      return "Purchase";
    case "refund":
      return "Refund";
    case "dispute":
      return "Dispute";
    case "webhook":
      return "Webhook";
    case "outbound_webhook":
      return "Outbound";
    case "integration":
      return "CRM";
    case "lead_created":
      return "Created";
    case "pipeline_placed":
      return "Placement";
    case "status_change":
      return "Status";
    case "field_change":
      return "Field";
    case "assignee_change":
      return "Assignee";
    case "tag_change":
      return "Tags";
    case "calendar_event":
      return "Calendar";
    case "follower":
      return "Follower";
    case "lead_deleted":
      return "Deleted";
    case "pipeline_cleared":
      return "Pipeline";
    case "imported":
      return "Import";
    case "note_added":
      return "Note";
    case "route_run":
      return "Route";
    case "return":
      return "Return";
    default:
      return "Activity";
  }
}

export function activityKindLabel(kind: LeadHistoryKind): string {
  return activityGroupLabel(activityFilterGroup(kind));
}

export function presentActivityGroups(history: LeadHistoryEntry[]): ActivityFilterGroup[] {
  const seen = new Set<ActivityFilterGroup>();
  const groups: ActivityFilterGroup[] = [];
  for (const entry of history) {
    const group = activityFilterGroup(entry.kind);
    if (seen.has(group)) continue;
    seen.add(group);
    groups.push(group);
  }
  return groups;
}

export function loadHiddenActivityGroups(userId: string): Set<ActivityFilterGroup> {
  try {
    const raw = localStorage.getItem(storageKey(userId));
    if (!raw) return new Set();
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return new Set();
    return new Set(parsed.filter((item): item is ActivityFilterGroup => typeof item === "string"));
  } catch {
    return new Set();
  }
}

export function saveHiddenActivityGroups(userId: string, groups: Set<ActivityFilterGroup>): void {
  try {
    localStorage.setItem(storageKey(userId), JSON.stringify([...groups]));
  } catch {
    // localStorage may be unavailable (private mode); filter state is non-critical.
  }
}

export function useActivityGroupFilters(userId: string | undefined) {
  const [hiddenGroups, setHiddenGroups] = useState<Set<ActivityFilterGroup>>(() =>
    userId ? loadHiddenActivityGroups(userId) : new Set()
  );

  useEffect(() => {
    if (userId) setHiddenGroups(loadHiddenActivityGroups(userId));
  }, [userId]);

  const toggleGroup = useCallback(
    (group: ActivityFilterGroup) => {
      setHiddenGroups((prev) => {
        const next = new Set(prev);
        if (next.has(group)) next.delete(group);
        else next.add(group);
        if (userId) saveHiddenActivityGroups(userId, next);
        return next;
      });
    },
    [userId]
  );

  const isVisible = useCallback(
    (group: ActivityFilterGroup) => !hiddenGroups.has(group),
    [hiddenGroups]
  );

  return { hiddenGroups, toggleGroup, isVisible };
}
