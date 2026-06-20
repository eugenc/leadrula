import { beforeEach, describe, expect, it } from "vitest";
import { loadCollapsedFolders, saveCollapsedFolders } from "./customFieldFolderCollapse";

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

describe("customFieldFolderCollapse", () => {
  beforeEach(installMemoryLocalStorage);

  it("defaults to empty (all expanded) when nothing is stored", () => {
    expect(loadCollapsedFolders("acct-1")).toEqual({});
  });

  it("persists and reloads collapse state per account", () => {
    saveCollapsedFolders("acct-1", { "5": true });
    expect(loadCollapsedFolders("acct-1")).toEqual({ "5": true });
    expect(loadCollapsedFolders("acct-2")).toEqual({});
  });

  it("returns empty on corrupt stored value", () => {
    localStorage.setItem("customFieldFolderCollapsed:acct-1", "{not json");
    expect(loadCollapsedFolders("acct-1")).toEqual({});
  });
});
