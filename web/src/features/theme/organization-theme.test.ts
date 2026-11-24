import { describe, expect, it } from "vitest";
import { sanitizeOrganizationTokens } from "./organization-theme";

describe("sanitizeOrganizationTokens", () => {
  it("accepts valid color strings and in-range numeric tokens", () => {
    const { tokens, warnings } = sanitizeOrganizationTokens({
      colorPrimary: "#1677ff",
      colorText: "rgba(0, 0, 0, 0.88)",
      borderRadius: 6,
      fontSize: 14,
    });
    expect(tokens).toEqual({
      colorPrimary: "#1677ff",
      colorText: "rgba(0, 0, 0, 0.88)",
      borderRadius: 6,
      fontSize: 14,
    });
    expect(warnings).toEqual([]);
  });

  it("ignores non-object input", () => {
    expect(sanitizeOrganizationTokens(null)).toEqual({ tokens: {}, warnings: [] });
    expect(sanitizeOrganizationTokens([1, 2])).toEqual({ tokens: {}, warnings: [] });
    expect(sanitizeOrganizationTokens("x")).toEqual({ tokens: {}, warnings: [] });
  });

  // INC-FE-07 回归：数值键越界/非有限值必须拒绝并提示，不得静默接受。
  it("rejects out-of-range and non-finite numeric tokens with a warning", () => {
    const { tokens, warnings } = sanitizeOrganizationTokens({
      fontSize: 0,
      borderRadius: -4,
    });
    expect(tokens).toEqual({});
    expect(warnings).toHaveLength(2);
    expect(warnings.some((w) => w.startsWith("fontSize"))).toBe(true);
    expect(warnings.some((w) => w.startsWith("borderRadius"))).toBe(true);

    const nan = sanitizeOrganizationTokens({ fontSize: Number.NaN, borderRadius: Infinity });
    expect(nan.tokens).toEqual({});
    expect(nan.warnings).toHaveLength(2);
  });

  it("rejects oversized numeric tokens", () => {
    const { tokens, warnings } = sanitizeOrganizationTokens({ fontSize: 999, borderRadius: 5000 });
    expect(tokens).toEqual({});
    expect(warnings).toHaveLength(2);
  });

  // INC-FE-07 回归：数值键传 string 会被 tokenNumber 静默忽略 → 假成功，须拒绝并提示。
  it("rejects string values on numeric keys with a warning", () => {
    const { tokens, warnings } = sanitizeOrganizationTokens({ borderRadius: "6", fontSize: "14" });
    expect(tokens).toEqual({});
    expect(warnings).toHaveLength(2);
  });

  it("rejects numeric values on color keys", () => {
    const { tokens, warnings } = sanitizeOrganizationTokens({ colorPrimary: 123 });
    expect(tokens).toEqual({});
    expect(warnings).toEqual(["colorPrimary is not a valid CSS color"]);
  });

  it("rejects invalid color strings and empty strings", () => {
    const { tokens, warnings } = sanitizeOrganizationTokens({
      colorPrimary: "url(javascript:alert(1))",
      colorText: "   ",
    });
    expect(tokens).toEqual({});
    expect(warnings).toHaveLength(2);
  });

  it("drops unknown keys silently", () => {
    const { tokens, warnings } = sanitizeOrganizationTokens({ notAWhitelistedKey: "#fff" });
    expect(tokens).toEqual({});
    expect(warnings).toEqual([]);
  });
});
