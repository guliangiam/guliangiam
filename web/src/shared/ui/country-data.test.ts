import { describe, expect, it } from "vitest";
import {
  getCountryName,
  isIsoCountryCode,
  normalizeIsoCountryCode,
  toDialCode,
  toE164,
} from "./country-data";

describe("country-data", () => {
  it("normalizes only supported ISO alpha-2 codes", () => {
    expect(normalizeIsoCountryCode("cn")).toBe("CN");
    expect(normalizeIsoCountryCode("US")).toBe("US");
    expect(normalizeIsoCountryCode("+86")).toBe("");
    expect(normalizeIsoCountryCode("86")).toBe("");
    expect(normalizeIsoCountryCode("USA")).toBe("");
    expect(isIsoCountryCode("CN")).toBe(true);
    expect(isIsoCountryCode("+86")).toBe(false);
  });

  it("derives the calling code from the ISO country, never from a dial code", () => {
    expect(toDialCode("CN")).toBe("+86");
    expect(toDialCode("US")).toBe("+1");
    expect(toDialCode("+86")).toBe("");
    expect(toDialCode("")).toBe("");
  });

  it("resolves localized country names from the existing data source", () => {
    expect(getCountryName("CN", "en")).toBe("People's Republic of China");
    expect(getCountryName("CN", "zh-CN")).toBe("中国");
    expect(getCountryName("+86", "en")).toBe("");
  });

  it("builds E.164 from an ISO country and a local number", () => {
    expect(toE164("CN", "13800138000")).toBe("+8613800138000");
    expect(toE164("CN", "138 0013 8000")).toBe("+8613800138000");
    expect(toE164("US", "4155550123")).toBe("+14155550123");
  });

  it("keeps an existing E.164 number regardless of the selected country", () => {
    expect(toE164("CN", "+14155550123")).toBe("+14155550123");
    expect(toE164("US", "+8613800138000")).toBe("+8613800138000");
  });

  it("accepts only a legacy dial prefix, not an ISO code, when composing", () => {
    expect(toE164("+86", "13800138000")).toBe("+8613800138000");
    expect(toE164("86", "13800138000")).toBe("+8613800138000");
    expect(toE164("", "13800138000")).toBe("");
    expect(toE164("CN", "abc")).toBe("");
    expect(toE164("CN", "+86abc")).toBe("");
  });
});
