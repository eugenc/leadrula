import { describe, expect, it } from "vitest";
import { groupLeadsForBoard } from "./boardStage";
import type { Lead } from "@/types";

function lead(overrides: Partial<Lead> & Pick<Lead, "id" | "stage_id">): Lead {
  return {
    public_id: "x",
    owner_account_id: 1,
    publisher_id: 2,
    contract_id: null,
    first_name: "Test",
    last_name: "Lead",
    status: "review",
    created_at: "",
    updated_at: "",
    tags: [],
    ...overrides,
  };
}

describe("groupLeadsForBoard", () => {
  const pipelineStageIds = new Set([10, 20, 30]);

  it("groups leads into matching pipeline stages", () => {
    const items = [
      lead({ id: 1, stage_id: 10 }),
      lead({ id: 2, stage_id: 20 }),
    ];
    const { grouped, unplaced } = groupLeadsForBoard(items, pipelineStageIds, "buyer");
    expect(unplaced).toHaveLength(0);
    expect(grouped[10]).toHaveLength(1);
    expect(grouped[20]).toHaveLength(1);
  });

  it("puts leads with orphan stage ids in unplaced", () => {
    const items = [lead({ id: 1, stage_id: 999 })];
    const { grouped, unplaced } = groupLeadsForBoard(items, pipelineStageIds, "buyer");
    expect(unplaced).toHaveLength(1);
    expect(unplaced[0]?.id).toBe(1);
    expect(grouped[999]).toBeUndefined();
  });

  it("puts leads with null stage in unplaced", () => {
    const items = [lead({ id: 1, stage_id: null })];
    const { unplaced } = groupLeadsForBoard(items, pipelineStageIds, "buyer");
    expect(unplaced).toHaveLength(1);
  });

  it("puts cross-pipeline cached lead in unplaced on wrong pipeline board", () => {
    const solarFloridaStageIds = new Set([1, 2]);
    const items = [
      lead({
        id: 2593,
        first_name: "Steve",
        last_name: "Ward",
        pipeline_id: 8,
        stage_id: 44,
      }),
    ];
    const { unplaced, grouped } = groupLeadsForBoard(items, solarFloridaStageIds, "publisher");
    expect(unplaced).toHaveLength(1);
    expect(unplaced[0]?.id).toBe(2593);
    expect(grouped[44]).toBeUndefined();

    const teestStageIds = new Set([44, 45]);
    const onTeest = groupLeadsForBoard(items, teestStageIds, "publisher");
    expect(onTeest.unplaced).toHaveLength(0);
    expect(onTeest.grouped[44]).toHaveLength(1);
  });
});
