import { describe, expect, it } from "vitest";
import { USER_DETAIL_TAB_KEYS, resolveUserDetailTab } from "./user-detail-tabs";

describe("resolveUserDetailTab", () => {
  it("keeps every canonical section key", () => {
    for (const key of USER_DETAIL_TAB_KEYS) {
      expect(resolveUserDetailTab(key)).toBe(key);
    }
  });

  it("folds the old 12-tab keys into the merged business sections", () => {
    expect(resolveUserDetailTab("profile")).toBe("basic");
    expect(resolveUserDetailTab("properties")).toBe("basic");
    expect(resolveUserDetailTab("custom")).toBe("basic");
    expect(resolveUserDetailTab("real-name")).toBe("realname");
    expect(resolveUserDetailTab("consents")).toBe("sessions");
    expect(resolveUserDetailTab("finance")).toBe("assets");
    expect(resolveUserDetailTab("managed-accounts")).toBe("assets");
  });

  it("is case-insensitive for legacy values", () => {
    expect(resolveUserDetailTab("managedAccounts")).toBe("assets");
    expect(resolveUserDetailTab("  ")).toBe("basic");
  });

  it("falls back to basic for unknown values", () => {
    expect(resolveUserDetailTab("unknown")).toBe("basic");
    expect(resolveUserDetailTab("")).toBe("basic");
    expect(resolveUserDetailTab(null)).toBe("basic");
  });
});
