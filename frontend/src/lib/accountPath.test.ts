import { describe, expect, it } from "vitest";
import { mapAccountPath } from "./accountPath";

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
    expect(mapAccountPath("/b/calendar", "", "publisher")).toBe("/p");
  });

  it("falls back to the dashboard for unrelated paths", () => {
    expect(mapAccountPath("/platform/buyers", "", "buyer")).toBe("/b");
    expect(mapAccountPath("/p", "", "buyer")).toBe("/b");
  });

  it("preserves the query string", () => {
    expect(mapAccountPath("/p/leads", "?page=2", "buyer")).toBe("/b/leads?page=2");
  });
});
