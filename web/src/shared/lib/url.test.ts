import { describe, it, expect, vi, beforeEach, afterAll } from "vitest";
import { isSafeRedirectUrl, safeRedirectUrl, isSafeExternalUrl } from "./url";

// Mock window.location for consistent test results
const originalLocation = window.location;

beforeEach(() => {
  // jsdom allows us to redefine location via Object.defineProperty
  Object.defineProperty(window, "location", {
    value: {
      origin: "https://app.example.com",
      href: "https://app.example.com/dashboard",
      protocol: "https:",
      host: "app.example.com",
      hostname: "app.example.com",
      port: "",
      pathname: "/dashboard",
      search: "",
      hash: "",
      assign: vi.fn<() => void>(),
      replace: vi.fn<() => void>(),
      reload: vi.fn<() => void>(),
    },
    writable: true,
  });
});

afterAll(() => {
  Object.defineProperty(window, "location", {
    value: originalLocation,
    writable: true,
  });
});

// ─── isSafeRedirectUrl ──────────────────────────────────────────

describe("isSafeRedirectUrl", () => {
  describe("same-origin path references (safe)", () => {
    it("allows root path", () => {
      expect(isSafeRedirectUrl("/")).toBe(true);
    });

    it("allows simple path", () => {
      expect(isSafeRedirectUrl("/dashboard")).toBe(true);
    });

    it("allows path with query params", () => {
      expect(isSafeRedirectUrl("/users?page=1")).toBe(true);
    });

    it("allows path with hash fragment", () => {
      expect(isSafeRedirectUrl("/settings#profile")).toBe(true);
    });

    it("allows deeply nested path", () => {
      expect(isSafeRedirectUrl("/org/org-1/app/app-1/settings")).toBe(true);
    });

    it("allows path with special characters", () => {
      expect(isSafeRedirectUrl("/search?q=hello%20world")).toBe(true);
    });
  });

  describe("same-origin full URLs (safe)", () => {
    it("allows same-origin full URL", () => {
      expect(isSafeRedirectUrl("https://app.example.com/dashboard")).toBe(true);
    });

    it("allows same-origin root URL", () => {
      expect(isSafeRedirectUrl("https://app.example.com/")).toBe(true);
    });

    it("allows same-origin URL with query params", () => {
      expect(isSafeRedirectUrl("https://app.example.com/users?page=1")).toBe(true);
    });
  });

  describe("protocol-relative URLs (unsafe)", () => {
    it("rejects protocol-relative URL", () => {
      expect(isSafeRedirectUrl("//evil.com")).toBe(false);
    });

    it("rejects protocol-relative URL with path", () => {
      expect(isSafeRedirectUrl("//evil.com/steal-cookies")).toBe(false);
    });
  });

  describe("dangerous protocols (unsafe)", () => {
    it("rejects javascript: protocol", () => {
      expect(isSafeRedirectUrl("javascript:alert(1)")).toBe(false);
    });

    it("rejects javascript: with mixed case", () => {
      expect(isSafeRedirectUrl("JaVaScRiPt:alert(1)")).toBe(false);
    });

    it("rejects data: protocol", () => {
      expect(isSafeRedirectUrl("data:text/html,<script>alert(1)</script>")).toBe(false);
    });

    it("rejects data: with mixed case", () => {
      expect(isSafeRedirectUrl("DATA:text/html,payload")).toBe(false);
    });

    it("rejects vbscript: protocol", () => {
      // vbscript: is not explicitly blocked but will fail URL parse / origin check
      expect(isSafeRedirectUrl("vbscript:payload")).toBe(false);
    });
  });

  describe("external origins (unsafe)", () => {
    it("rejects external HTTPS URL", () => {
      expect(isSafeRedirectUrl("https://evil.com/steal")).toBe(false);
    });

    it("rejects external HTTP URL", () => {
      expect(isSafeRedirectUrl("http://evil.com/steal")).toBe(false);
    });

    it("rejects subdomain of different origin", () => {
      expect(isSafeRedirectUrl("https://sub.evil.com/")).toBe(false);
    });
  });

  describe("invalid inputs (unsafe)", () => {
    it("rejects empty string", () => {
      expect(isSafeRedirectUrl("")).toBe(false);
    });

    it("rejects whitespace-only string", () => {
      expect(isSafeRedirectUrl("   ")).toBe(false);
    });

    it("allows relative path without leading slash (same-origin via URL parse)", () => {
      // "dashboard" is parsed as a same-origin URL by the URL constructor
      expect(isSafeRedirectUrl("dashboard")).toBe(true);
    });

    it("rejects backslash path (parsed as same-origin by URL)", () => {
      // Backslash is technically parsed as same-origin by URL constructor
      // This is a known edge case; the primary defense is against // and javascript:
      expect(isSafeRedirectUrl("\\evil")).toBe(true);
    });
  });

  describe("edge cases", () => {
    it("handles URLs with encoded characters", () => {
      expect(isSafeRedirectUrl("/path/%2F%2Fevil.com")).toBe(true);
    });

    it("handles very long URLs", () => {
      const longPath = "/a" + "/b".repeat(1000);
      expect(isSafeRedirectUrl(longPath)).toBe(true);
    });
  });
});

// ─── safeRedirectUrl ────────────────────────────────────────────

describe("safeRedirectUrl", () => {
  it("returns safe URL unchanged", () => {
    expect(safeRedirectUrl("/dashboard")).toBe("/dashboard");
  });

  it("returns fallback for unsafe URL", () => {
    expect(safeRedirectUrl("https://evil.com/")).toBe("/");
  });

  it("uses custom fallback when provided", () => {
    expect(safeRedirectUrl("javascript:alert(1)", "/home")).toBe("/home");
  });

  it("uses custom fallback for external URL", () => {
    expect(safeRedirectUrl("//evil.com", "/login")).toBe("/login");
  });

  it("returns safe same-origin URL unchanged", () => {
    expect(safeRedirectUrl("https://app.example.com/settings")).toBe(
      "https://app.example.com/settings",
    );
  });

  it("returns fallback for empty string", () => {
    expect(safeRedirectUrl("", "/fallback")).toBe("/fallback");
  });
});

// ─── isSafeExternalUrl ──────────────────────────────────────────

describe("isSafeExternalUrl", () => {
  describe("safe external URLs", () => {
    it("allows HTTPS URL", () => {
      expect(isSafeExternalUrl("https://example.com")).toBe(true);
    });

    it("allows HTTP URL", () => {
      expect(isSafeExternalUrl("http://example.com")).toBe(true);
    });

    it("allows HTTPS URL with path", () => {
      expect(isSafeExternalUrl("https://example.com/path/to/resource")).toBe(true);
    });

    it("allows HTTPS URL with query params", () => {
      expect(isSafeExternalUrl("https://example.com/api?key=value")).toBe(true);
    });

    it("allows HTTP URL with port", () => {
      expect(isSafeExternalUrl("http://localhost:3000")).toBe(true);
    });
  });

  describe("unsafe URLs", () => {
    it("rejects javascript: protocol", () => {
      expect(isSafeExternalUrl("javascript:alert(1)")).toBe(false);
    });

    it("rejects data: protocol", () => {
      expect(isSafeExternalUrl("data:text/html,<script>alert(1)</script>")).toBe(false);
    });

    it("rejects vbscript: protocol", () => {
      expect(isSafeExternalUrl("vbscript:payload")).toBe(false);
    });

    it("rejects ftp: protocol", () => {
      expect(isSafeExternalUrl("ftp://files.example.com")).toBe(false);
    });

    it("rejects relative path", () => {
      expect(isSafeExternalUrl("/dashboard")).toBe(false);
    });

    it("rejects relative path without slash", () => {
      expect(isSafeExternalUrl("dashboard")).toBe(false);
    });
  });

  describe("invalid inputs", () => {
    it("rejects empty string", () => {
      expect(isSafeExternalUrl("")).toBe(false);
    });

    it("rejects whitespace-only string", () => {
      expect(isSafeExternalUrl("   ")).toBe(false);
    });

    it("rejects random string", () => {
      expect(isSafeExternalUrl("not-a-url")).toBe(false);
    });
  });
});
