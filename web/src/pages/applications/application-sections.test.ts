import { describe, expect, it } from "vitest";
import { APP_SECTION_KEYS, resolveAppSection } from "./application-sections";

describe("resolveAppSection", () => {
  it("keeps every canonical section key", () => {
    for (const key of APP_SECTION_KEYS) {
      expect(resolveAppSection(key)).toBe(key);
    }
  });

  it("folds legacy tab keys into the new business sections", () => {
    expect(resolveAppSection("overview")).toBe("basics");
    expect(resolveAppSection("basic")).toBe("basics");
    expect(resolveAppSection("auth")).toBe("authentication");
    expect(resolveAppSection("auth-protocol")).toBe("authentication");
    expect(resolveAppSection("custom-scopes")).toBe("oidc");
    expect(resolveAppSection("scopes")).toBe("oidc");
    expect(resolveAppSection("integration")).toBe("providers");
    expect(resolveAppSection("ui-customization")).toBe("ui");
    expect(resolveAppSection("experience")).toBe("ui");
    expect(resolveAppSection("signin-log")).toBe("security");
    expect(resolveAppSection("operations")).toBe("security");
    expect(resolveAppSection("reverse-proxy")).toBe("reverseProxy");
  });

  it("is case-insensitive for legacy values", () => {
    expect(resolveAppSection("OIDC")).toBe("oidc");
    expect(resolveAppSection(" ui ")).toBe("ui");
  });

  it("returns null for unknown or empty values", () => {
    expect(resolveAppSection("unknown")).toBeNull();
    expect(resolveAppSection("")).toBeNull();
    expect(resolveAppSection(null)).toBeNull();
  });
});
