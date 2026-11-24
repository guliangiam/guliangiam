import { describe, expect, it } from "vitest";
import { userDetailPath, userRouteSlug } from "./user-route";

const slugById = new Map<string, string>([
  ["org-target", "target-org"],
  ["org-session", "session-org"],
]);

describe("userRouteSlug", () => {
  it("resolves the target organization slug", () => {
    expect(userRouteSlug("org-target", slugById, "session-org")).toBe("target-org");
  });

  it("falls back to the session slug when the organization is unknown", () => {
    expect(userRouteSlug(undefined, slugById, "session-org")).toBe("session-org");
    expect(userRouteSlug("org-missing", slugById, "session-org")).toBe("session-org");
  });
});

describe("userDetailPath", () => {
  it("keeps the same-organization path unchanged", () => {
    expect(userDetailPath("org-session", "alice", slugById, "session-org")).toBe(
      "/users/session-org/alice",
    );
  });

  it("lands cross-organization impersonation in the target organization", () => {
    expect(userDetailPath("org-target", "bob", slugById, "session-org")).toBe(
      "/users/target-org/bob",
    );
  });
});
