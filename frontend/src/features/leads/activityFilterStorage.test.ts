import { beforeEach, describe, expect, it } from "vitest";
import type { LeadHistoryEntry } from "@/types";
import {
  activityFilterGroup,
  activityGroupLabel,
  activityKindLabel,
  loadHiddenActivityGroups,
  presentActivityGroups,
  saveHiddenActivityGroups,
} from "./activityFilterStorage";

function installMemoryLocalStorage() {
  const store = new Map<string, string>();
  globalThis.localStorage = {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, v),
    removeItem: (k: string) => void store.delete(k),
    clear: () => store.clear(),
    key: (i: number) => Array.from(store.keys())[i] ?? null,
    get length() {
      return store.size;
    },
  } as Storage;
}

function entry(kind: LeadHistoryEntry["kind"], id: number): LeadHistoryEntry {
  return { id, kind, created_at: "2026-01-01T00:00:00Z" };
}

describe("activityFilterStorage", () => {
  beforeEach(installMemoryLocalStorage);

  it("groups dispute and follower kinds", () => {
    expect(activityFilterGroup("dispute_opened")).toBe("dispute");
    expect(activityFilterGroup("dispute_resolved")).toBe("dispute");
    expect(activityFilterGroup("follower_added")).toBe("follower");
    expect(activityFilterGroup("follower_removed")).toBe("follower");
    expect(activityFilterGroup("return_scheduled")).toBe("return");
    expect(activityFilterGroup("return_cancelled")).toBe("return");
    expect(activityFilterGroup("note_added")).toBe("note_added");
  });

  it("maps return kinds to Return label", () => {
    expect(activityKindLabel("return_scheduled")).toBe("Return");
    expect(activityGroupLabel("return")).toBe("Return");
  });

  it("maps group ids to the same labels shown on activity badges", () => {
    expect(activityKindLabel("note_added")).toBe("Note");
    expect(activityGroupLabel("dispute")).toBe("Dispute");
    expect(activityGroupLabel("follower")).toBe("Follower");
  });

  it("returns present groups in feed order without duplicates", () => {
    const history = [
      entry("note_added", 1),
      entry("stage_change", 2),
      entry("note_added", 3),
      entry("dispute_opened", 4),
      entry("dispute_resolved", 5),
    ];
    expect(presentActivityGroups(history)).toEqual(["note_added", "stage_change", "dispute"]);
  });

  it("defaults to no hidden groups when nothing is stored", () => {
    expect(loadHiddenActivityGroups("user-1").size).toBe(0);
  });

  it("persists and reloads hidden groups per user", () => {
    saveHiddenActivityGroups("user-1", new Set(["note_added", "field_change"]));
    expect(loadHiddenActivityGroups("user-1")).toEqual(new Set(["note_added", "field_change"]));
    expect(loadHiddenActivityGroups("user-2").size).toBe(0);
  });

  it("returns empty set on corrupt stored value", () => {
    localStorage.setItem("lead-activity-hidden-groups:user-1", "{not json");
    expect(loadHiddenActivityGroups("user-1").size).toBe(0);
  });
});
