import { describe, expect, it } from "vitest";
import {
  buyerDeliveryBody,
  buyerDeliveryDirty,
  buyerDeliveryDraftFrom,
} from "./buyerDeliveryDirty";

describe("buyerDeliveryDraftFrom", () => {
  it("defaults delivery from pipeline when set", () => {
    expect(buyerDeliveryDraftFrom({ buyer_pipeline_id: 5 }, ["leads", "leads_pipeline"])).toEqual({
      delivery: "leads_pipeline",
      pipelineId: 5,
      stageId: 0,
      webhookId: 0,
      integrationId: 0,
    });
  });
});

describe("buyerDeliveryDirty", () => {
  it("returns false when unchanged", () => {
    const server = {
      delivery: "leads",
      buyer_pipeline_id: null,
      buyer_target_stage_id: null,
      outbound_webhook_id: null,
      integration_connection_id: null,
    };
    const local = buyerDeliveryDraftFrom(server, ["leads", "leads_pipeline"]);
    expect(buyerDeliveryDirty(local, server, ["leads", "leads_pipeline"])).toBe(false);
  });

  it("detects pipeline change", () => {
    const server = {
      delivery: "leads_pipeline",
      buyer_pipeline_id: 1,
      buyer_target_stage_id: 2,
    };
    const local = buyerDeliveryDraftFrom(server, ["leads", "leads_pipeline"]);
    expect(buyerDeliveryDirty({ ...local, pipelineId: 3 }, server, ["leads", "leads_pipeline"])).toBe(
      true
    );
  });
});

describe("buyerDeliveryBody", () => {
  it("includes pipeline fields for leads_pipeline", () => {
    expect(
      buyerDeliveryBody({
        delivery: "leads_pipeline",
        pipelineId: 1,
        stageId: 2,
        webhookId: 0,
        integrationId: 0,
      })
    ).toEqual({
      delivery: "leads_pipeline",
      buyer_pipeline_id: 1,
      buyer_target_stage_id: 2,
    });
  });
});
