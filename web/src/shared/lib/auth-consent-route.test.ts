import { describe, it, expect } from "vitest";

// Test consent route builder logic
function buildConsentRoute(values: { applicationName: string; locationSearch: string }) {
  return `/consent/${encodeURIComponent(values.applicationName)}${values.locationSearch}`;
}

describe("buildConsentRoute", () => {
  it("builds consent route with application name", () => {
    const result = buildConsentRoute({ applicationName: "my-app", locationSearch: "" });
    expect(result).toBe("/consent/my-app");
  });

  it("encodes special characters in application name", () => {
    const result = buildConsentRoute({ applicationName: "my app", locationSearch: "" });
    expect(result).toBe("/consent/my%20app");
  });

  it("appends location search params", () => {
    const result = buildConsentRoute({
      applicationName: "my-app",
      locationSearch: "?redirect=/dashboard",
    });
    expect(result).toBe("/consent/my-app?redirect=/dashboard");
  });

  it("handles empty application name", () => {
    const result = buildConsentRoute({ applicationName: "", locationSearch: "" });
    expect(result).toBe("/consent/");
  });
});
