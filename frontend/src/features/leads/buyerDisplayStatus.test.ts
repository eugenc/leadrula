import { describe, expect, it } from "vitest";
import { buyerDisplayStatus } from "./leadsListColumns";
import type { Lead } from "@/types";

function lead(
  overrides: Partial<Lead> & Pick<Lead, "status">
): Pick<Lead, "status" | "stage_id" | "stage_move_count"> {
  return {
    stage_id: null,
    stage_move_count: 0,
    ...overrides,
  };
}

describe("buyerDisplayStatus", () => {
  it("inbox lead with no stage is new", () => {
    expect(buyerDisplayStatus(lead({ status: "review" }))).toBe("new");
  });

  it("pipeline lead at landing stage is new", () => {
    expect(
      buyerDisplayStatus(lead({ status: "distributed", stage_id: 10, stage_move_count: 0 }))
    ).toBe("new");
  });

  it("pipeline lead after stage move is active", () => {
    expect(
      buyerDisplayStatus(lead({ status: "distributed", stage_id: 20, stage_move_count: 1 }))
    ).toBe("active");
  });

  it("inbox lead on first pipeline stage is new", () => {
    expect(
      buyerDisplayStatus(lead({ status: "review", stage_id: 10, stage_move_count: 1 }))
    ).toBe("new");
  });

  it("inbox lead moved to second stage is active", () => {
    expect(
      buyerDisplayStatus(lead({ status: "review", stage_id: 20, stage_move_count: 2 }))
    ).toBe("active");
  });

  it("terminal statuses unchanged", () => {
    expect(buyerDisplayStatus(lead({ status: "closed" }))).toBe("won");
    expect(buyerDisplayStatus(lead({ status: "returned" }))).toBe("returned");
    expect(buyerDisplayStatus(lead({ status: "disputed" }))).toBe("disputed");
  });
});
