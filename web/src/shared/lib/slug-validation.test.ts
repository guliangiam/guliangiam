import { describe, it, expect } from "vitest";

// Organization slug validation logic
function isValidSlug(slug: string): boolean {
  if (!slug || slug.length === 0) return false;
  // Slug must be lowercase alphanumeric + hyphens, no leading/trailing hyphens
  return /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(slug);
}

function sanitizeSlug(input: string): string {
  return input
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, "-")
    .replace(/^-+|-+$/g, "")
    .replace(/-+/g, "-");
}

describe("isValidSlug", () => {
  it("accepts valid slug", () => {
    expect(isValidSlug("my-org")).toBe(true);
    expect(isValidSlug("org123")).toBe(true);
    expect(isValidSlug("a")).toBe(true);
    expect(isValidSlug("my-org-123")).toBe(true);
  });

  it("rejects empty slug", () => {
    expect(isValidSlug("")).toBe(false);
  });

  it("rejects uppercase letters", () => {
    expect(isValidSlug("My-Org")).toBe(false);
  });

  it("rejects leading hyphen", () => {
    expect(isValidSlug("-my-org")).toBe(false);
  });

  it("rejects trailing hyphen", () => {
    expect(isValidSlug("my-org-")).toBe(false);
  });

  it("rejects spaces", () => {
    expect(isValidSlug("my org")).toBe(false);
  });

  it("rejects special characters", () => {
    expect(isValidSlug("my_org")).toBe(false);
    expect(isValidSlug("my.org")).toBe(false);
  });
});

describe("sanitizeSlug", () => {
  it("converts to lowercase", () => {
    expect(sanitizeSlug("MyOrg")).toBe("myorg");
  });

  it("replaces spaces with hyphens", () => {
    expect(sanitizeSlug("my org")).toBe("my-org");
  });

  it("replaces multiple hyphens with single", () => {
    expect(sanitizeSlug("my---org")).toBe("my-org");
  });

  it("removes leading and trailing hyphens", () => {
    expect(sanitizeSlug("--my-org--")).toBe("my-org");
  });

  it("handles special characters", () => {
    expect(sanitizeSlug("my_org.name")).toBe("my-org-name");
  });

  it("preserves valid slugs", () => {
    expect(sanitizeSlug("my-org-123")).toBe("my-org-123");
  });
});
