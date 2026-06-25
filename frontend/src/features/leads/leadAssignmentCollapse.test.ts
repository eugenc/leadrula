import { beforeEach, describe, expect, it } from "vitest";
import { loadLeadAssignmentCollapsed, saveLeadAssignmentCollapsed } from "./leadAssignmentCollapse";

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

describe("leadAssignmentCollapse", () => {
  beforeEach(installMemoryLocalStorage);

  it("defaults to expanded when nothing is stored", () => {
    expect(loadLeadAssignmentCollapsed("acct-1")).toBe(false);
  });

  it("persists and reloads collapse state per account", () => {
    saveLeadAssignmentCollapsed("acct-1", true);
    expect(loadLeadAssignmentCollapsed("acct-1")).toBe(true);
    expect(loadLeadAssignmentCollapsed("acct-2")).toBe(false);
  });

  it("returns expanded on corrupt stored value", () => {
    localStorage.setItem("leadAssignmentCollapsed:acct-1", "maybe");
    expect(loadLeadAssignmentCollapsed("acct-1")).toBe(false);
  });
});
