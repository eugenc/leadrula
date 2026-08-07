import { describe, expect, it } from "vitest";
import { mapAccountPath, pathAfterAccountSwitch } from "./accountPath";

describe("mapAccountPath", () => {
  it("keeps the same suffix across account types", () => {
    expect(mapAccountPath("/p/board", "", "buyer")).toBe("/b/board");
    expect(mapAccountPath("/b/board", "", "publisher")).toBe("/p/board");
    expect(mapAccountPath("/p/settings/users", "", "buyer")).toBe("/b/settings/users");
    expect(mapAccountPath("/p/calls", "", "buyer")).toBe("/b/calls");
    expect(mapAccountPath("/b/calls", "", "publisher")).toBe("/p/calls");
  });

  it("maps known mismatched suffixes", () => {
    expect(mapAccountPath("/p/buyers", "", "buyer")).toBe("/b/publishers");
    expect(mapAccountPath("/b/publishers", "", "publisher")).toBe("/p/buyers");
    expect(mapAccountPath("/p/contracts", "", "buyer")).toBe("/b/contract");
    expect(mapAccountPath("/p/routing", "", "buyer")).toBe("/b/routes");
    expect(mapAccountPath("/p/log", "", "buyer")).toBe("/b/logs");
  });

  it("falls back to the dashboard when there is no equivalent", () => {
    expect(mapAccountPath("/p/sources", "", "buyer")).toBe("/b");
    expect(mapAccountPath("/b/calendar", "", "publisher")).toBe("/p/calendar");
  });

  it("falls back to the dashboard for unrelated paths", () => {
    expect(mapAccountPath("/platform/buyers", "", "buyer")).toBe("/b");
    expect(mapAccountPath("/p", "", "buyer")).toBe("/b");
  });

  it("preserves the query string", () => {
    expect(mapAccountPath("/p/leads", "?page=2", "buyer")).toBe("/b/leads?page=2");
  });
});

describe("pathAfterAccountSwitch", () => {
  it("keeps the same path for same account type", () => {
    expect(pathAfterAccountSwitch("/b/appointments", "", "buyer")).toBe("/b/appointments");
    expect(pathAfterAccountSwitch("/b/appointments", "?page=2", "buyer")).toBe("/b/appointments?page=2");
    expect(pathAfterAccountSwitch("/p/calendar", "", "publisher")).toBe("/p/calendar");
  });

  it("maps cross-type tenant routes", () => {
    expect(pathAfterAccountSwitch("/p/calendar", "", "buyer")).toBe("/b/calendar");
    expect(pathAfterAccountSwitch("/b/routes", "", "publisher")).toBe("/p/routing");
    expect(pathAfterAccountSwitch("/p/leads", "?page=2", "buyer")).toBe("/b/leads?page=2");
  });

  it("falls back to platform home from tenant routes", () => {
    expect(pathAfterAccountSwitch("/b/leads", "", "platform")).toBe("/platform");
    expect(pathAfterAccountSwitch("/p/settings/users", "", "platform")).toBe("/platform");
  });

  it("keeps platform routes when switching back to platform", () => {
    expect(pathAfterAccountSwitch("/platform/buyers", "", "platform")).toBe("/platform/buyers");
    expect(pathAfterAccountSwitch("/platform/settings/users", "?q=1", "platform")).toBe(
      "/platform/settings/users?q=1"
    );
  });

  it("falls back to tenant home from platform routes", () => {
    expect(pathAfterAccountSwitch("/platform/settings/users", "", "buyer")).toBe("/b");
    expect(pathAfterAccountSwitch("/platform/publishers", "", "publisher")).toBe("/p");
  });
});
