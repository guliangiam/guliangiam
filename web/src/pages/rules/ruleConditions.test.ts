import { describe, expect, it } from "vitest";
import type { IamRuleCondition } from "@/shared/api/iam";
import {
  deriveIPRange,
  deriveWAFPattern,
  editableConditions,
  isIPCondition,
  isWAFPatternCondition,
  mergeRuleConditions,
} from "./ruleConditions";

describe("ruleConditions", () => {
  it("derives and filters IP conditions", () => {
    const conditions: IamRuleCondition[] = [
      { field: "ip", operator: "cidr", value: "10.0.0.0/8" },
      { field: "ip", operator: "range", value: "192.168.1.1-192.168.1.9" },
      { field: "user_agent", operator: "eq", value: "bot" },
    ];
    expect(deriveIPRange(conditions)).toBe("10.0.0.0/8, 192.168.1.1-192.168.1.9");
    expect(editableConditions(conditions)).toEqual([conditions[2]]);
  });

  it("derives and filters WAF pattern conditions", () => {
    const conditions: IamRuleCondition[] = [
      { pattern: "<script>", target: "REQUEST_BODY" },
      { field: "ip", operator: "eq", value: "127.0.0.1" },
    ];
    expect(deriveWAFPattern(conditions)).toBe("<script>");
    expect(isIPCondition(conditions[1])).toBe(true);
    expect(editableConditions(conditions)).toEqual([]);
  });

  it("round-trips typed inputs into condition rows", () => {
    const merged = mergeRuleConditions(
      [{ field: "user_agent", operator: "eq", value: "bot" }],
      "10.0.0.0/8, 192.168.1.1-192.168.1.9, 127.0.0.1",
      '<script>alert(1)</script>\nSecRule ARGS "@rx foo{1,3}"',
      [{ pattern: "<script>alert(1)</script>", target: "REQUEST_BODY" }],
    );
    expect(merged).toEqual([
      { field: "user_agent", operator: "eq", value: "bot" },
      { field: "ip", operator: "cidr", value: "10.0.0.0/8" },
      { field: "ip", operator: "range", value: "192.168.1.1-192.168.1.9" },
      { field: "ip", operator: "eq", value: "127.0.0.1" },
      { pattern: "<script>alert(1)</script>", target: "REQUEST_BODY" },
      { pattern: 'SecRule ARGS "@rx foo{1,3}"' },
    ]);
  });

  it("does not treat non-IP field names as IP conditions", () => {
    expect(isIPCondition({ field: "user_agent", operator: "cidr", value: "x" })).toBe(false);
    expect(isWAFPatternCondition({ pattern: "   " })).toBe(false);
  });
});
