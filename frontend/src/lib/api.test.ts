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

import { requestAuthToken, messagingNs, messagingAccountType, messagingAccessToken } from "./api";

describe("requestAuthToken", () => {
  beforeEach(() => {
    authState.accessToken = "switched-token";
    authState.impersonation = null;
    authState.switchSession = null;
    authState.user = { account_type: "publisher" };
  });

  it("uses switched token for active-namespace messaging when platform admin switches", () => {
    authState.switchSession = {
      originAccessToken: "platform-token",
      originUser: { account_type: "platform" },
    };
    authState.user = { account_type: "publisher" };

    expect(messagingNs()).toBe("/publisher");
    expect(messagingAccountType()).toBe("publisher");
    expect(messagingAccessToken()).toBe("switched-token");
    expect(requestAuthToken("/publisher/messages/threads")).toBe("switched-token");
    expect(requestAuthToken("/publisher/leads")).toBe("switched-token");
  });

  it("uses origin token for home-namespace messaging while publisher-origin switch", () => {
    authState.switchSession = {
      originAccessToken: "publisher-token",
      originUser: { account_type: "publisher" },
    };
    authState.user = { account_type: "buyer" };

    expect(messagingNs()).toBe("/publisher");
    expect(messagingAccountType()).toBe("publisher");
    expect(messagingAccessToken()).toBe("publisher-token");
    expect(requestAuthToken("/publisher/messages/threads")).toBe("publisher-token");
    expect(requestAuthToken("/buyer/leads")).toBe("switched-token");
  });

  it("does not treat platform namespace as home when platform admin switches to tenant", () => {
    authState.switchSession = {
      originAccessToken: "platform-token",
      originUser: { account_type: "platform" },
    };
    authState.user = { account_type: "publisher" };

    expect(requestAuthToken("/platform/messages/threads")).toBe("switched-token");
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
