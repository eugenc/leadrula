import { beforeEach, describe, expect, it, vi } from "vitest";

const authState = {
  accessToken: "switched-token",
  impersonation: null as null | {
    publisherAccessToken: string;
    publisherUser: { account_type: "publisher" | "buyer" | "platform" };
  },
  switchSession: null as null | {
    originAccessToken: string;
    originUser: { account_type: "publisher" | "buyer" | "platform" };
  },
  user: { account_type: "publisher" as const },
};

vi.mock("@/store/authStore", () => ({
  useAuthStore: {
    getState: () => authState,
  },
}));

vi.mock("@/store/toastStore", () => ({
  toast: { error: vi.fn() },
}));

import { requestAuthToken } from "./api";

describe("requestAuthToken", () => {
  beforeEach(() => {
    authState.accessToken = "switched-token";
    authState.impersonation = null;
    authState.switchSession = null;
    authState.user = { account_type: "publisher" };
  });

  it("uses origin token for home-namespace messaging while switched", () => {
    authState.switchSession = {
      originAccessToken: "platform-token",
      originUser: { account_type: "platform" },
    };
    authState.user = { account_type: "publisher" };

    expect(requestAuthToken("/platform/messages/threads")).toBe("platform-token");
    expect(requestAuthToken("/publisher/leads")).toBe("switched-token");
  });

  it("uses publisher token for home-namespace messaging while impersonating", () => {
    authState.impersonation = {
      publisherAccessToken: "publisher-token",
      publisherUser: { account_type: "publisher" },
    };
    authState.user = { account_type: "buyer" };

    expect(requestAuthToken("/publisher/messages/threads/by-lead/abc")).toBe("publisher-token");
    expect(requestAuthToken("/buyer/leads")).toBe("switched-token");
  });

  it("uses session token when not switched or impersonating", () => {
    authState.user = { account_type: "platform" };
    authState.accessToken = "platform-token";

    expect(requestAuthToken("/platform/messages/threads")).toBe("platform-token");
  });
});
