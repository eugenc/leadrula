import { describe, expect, it } from "vitest";
import {
  CONTACT_SYSTEM_KEY,
  DEFAULT_CONTACT_BUILTIN_ORDER,
  CONTACT_LOCKED_BUILTIN_KEYS,
  CONTACT_REORDERABLE_BUILTIN_KEYS,
  isContactFolder,
  isLockedContactField,
  resolveContactBuiltinOrder,
  resolveContactBuiltinTail,
  isDefaultContactBuiltinOrder,
  orderedContactSystemFields,
} from "./contactSection";

describe("isContactFolder", () => {
  it("returns true for the contact system folder", () => {
    expect(isContactFolder({ is_system: true, system_key: CONTACT_SYSTEM_KEY })).toBe(true);
  });

  it("returns false for regular folders", () => {
    expect(isContactFolder({ is_system: false, system_key: null })).toBe(false);
    expect(isContactFolder({ is_system: true, system_key: "other" })).toBe(false);
    expect(isContactFolder({ is_system: false, system_key: CONTACT_SYSTEM_KEY })).toBe(false);
  });
});

describe("isLockedContactField", () => {
  it("identifies locked name fields", () => {
    expect(isLockedContactField("first_name")).toBe(true);
    expect(isLockedContactField("last_name")).toBe(true);
    expect(isLockedContactField("phone")).toBe(false);
  });
});

describe("resolveContactBuiltinTail", () => {
  it("returns default tail when order is empty", () => {
    expect(resolveContactBuiltinTail(null)).toEqual([...CONTACT_REORDERABLE_BUILTIN_KEYS]);
  });

  it("preserves reorderable field order and ignores locked keys", () => {
    expect(resolveContactBuiltinTail(["phone", "email", "first_name", "last_name", "address", "tags"])).toEqual([
      "phone",
      "email",
      "address",
      "tags",
    ]);
  });
});

describe("resolveContactBuiltinOrder", () => {
  it("returns default order when null or empty", () => {
    expect(resolveContactBuiltinOrder(null)).toEqual(DEFAULT_CONTACT_BUILTIN_ORDER);
    expect(resolveContactBuiltinOrder([])).toEqual(DEFAULT_CONTACT_BUILTIN_ORDER);
  });

  it("always pins first_name and last_name first", () => {
    const custom = ["phone", "email", "first_name", "last_name", "address", "tags"];
    expect(resolveContactBuiltinOrder(custom)).toEqual([
      "first_name",
      "last_name",
      "phone",
      "email",
      "address",
      "tags",
    ]);
  });

  it("fills missing reorderable keys from default", () => {
    expect(resolveContactBuiltinOrder(["phone", "email"])).toEqual([
      "first_name",
      "last_name",
      "phone",
      "email",
      "address",
      "tags",
    ]);
  });
});

describe("isDefaultContactBuiltinOrder", () => {
  it("detects default and custom order", () => {
    expect(isDefaultContactBuiltinOrder(null)).toBe(true);
    expect(isDefaultContactBuiltinOrder([...DEFAULT_CONTACT_BUILTIN_ORDER])).toBe(true);
    expect(
      isDefaultContactBuiltinOrder(["first_name", "last_name", "phone", "email", "tags", "address"])
    ).toBe(false);
  });
});

describe("orderedContactSystemFields", () => {
  it("orders fields with locked prefix and custom tail", () => {
    const ordered = orderedContactSystemFields(["email", "phone", "first_name", "last_name", "address", "tags"]);
    expect(ordered.map((f) => f.key)).toEqual([
      "first_name",
      "last_name",
      "email",
      "phone",
      "address",
      "tags",
    ]);
  });
});
