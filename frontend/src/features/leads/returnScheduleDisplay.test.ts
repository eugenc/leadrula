import { describe, expect, it } from "vitest";
import { formatPendingReturnLabel } from "./returnScheduleDisplay";

describe("formatPendingReturnLabel", () => {
  it("returns null for missing iso", () => {
    expect(formatPendingReturnLabel(null, "America/New_York", "card")).toBeNull();
  });

  it("formats card label in contract timezone", () => {
    const label = formatPendingReturnLabel(
      "2026-01-09T14:00:00.000Z",
      "America/New_York",
      "card"
    );
    expect(label).toMatch(/^Returns /);
    expect(label).toContain("9:00");
  });

  it("formats detail label in contract timezone", () => {
    const label = formatPendingReturnLabel(
      "2026-01-09T14:00:00.000Z",
      "America/New_York",
      "detail"
    );
    expect(label).toMatch(/^Returns /);
    expect(label).toContain("Jan");
  });

  it("falls back when timezone is invalid", () => {
    const label = formatPendingReturnLabel(
      "2026-01-09T14:00:00.000Z",
      "Not/A_Timezone",
      "card"
    );
    expect(label).toMatch(/^Returns /);
  });
});
