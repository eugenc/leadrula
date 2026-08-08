import { describe, expect, it } from "vitest";
import { contactFieldsFromLead, dirtyContactPatch } from "./leadContactFields";
import type { Lead } from "@/types";

function minimalLead(overrides: Partial<Lead> = {}): Lead {
  return {
    id: 1,
    public_id: "L-1",
    first_name: "Jane",
    last_name: "Doe",
    phone: "555-0100",
    email: "jane@example.com",
    address: "123 Main St",
    city: "Austin",
    state: "TX",
    zip: "78701",
    country: "US",
    address_place_id: "place-1",
    status: "active",
    ...overrides,
  } as Lead;
}

describe("contactFieldsFromLead", () => {
  it("maps builtins and address fields from lead", () => {
    const lead = minimalLead();
    expect(contactFieldsFromLead(lead)).toEqual({
      first_name: "Jane",
      last_name: "Doe",
      phone: "555-0100",
      email: "jane@example.com",
      address: "123 Main St",
      city: "Austin",
      state: "TX",
      zip: "78701",
      country: "US",
      address_place_id: "place-1",
    });
  });

  it("uses empty strings for null values", () => {
    const lead = minimalLead({
      phone: null as unknown as string,
      address_place_id: null,
    });
    expect(contactFieldsFromLead(lead).phone).toBe("");
    expect(contactFieldsFromLead(lead).address_place_id).toBe("");
  });
});

describe("dirtyContactPatch", () => {
  it("returns null when nothing changed", () => {
    const lead = minimalLead();
    expect(dirtyContactPatch(contactFieldsFromLead(lead), lead)).toBeNull();
  });

  it("includes only changed builtin fields", () => {
    const lead = minimalLead();
    const fields = { ...contactFieldsFromLead(lead), first_name: "Janet" };
    expect(dirtyContactPatch(fields, lead)).toEqual({ fields: { first_name: "Janet" } });
  });

  it("clears address_place_id when manual address fields change", () => {
    const lead = minimalLead();
    const fields = { ...contactFieldsFromLead(lead), city: "Dallas" };
    expect(dirtyContactPatch(fields, lead)).toEqual({
      fields: { city: "Dallas", address_place_id: null },
    });
  });

  it("includes address_place_id when only place id changed", () => {
    const lead = minimalLead({ address_place_id: null });
    const fields = { ...contactFieldsFromLead(lead), address_place_id: "place-2" };
    expect(dirtyContactPatch(fields, lead)).toEqual({
      fields: { address_place_id: "place-2" },
    });
  });
});
