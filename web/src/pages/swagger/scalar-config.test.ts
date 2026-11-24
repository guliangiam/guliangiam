import { describe, expect, it } from "vitest";
import { toScalarDirection, toScalarLocale } from "./scalar-config";

describe("toScalarLocale", () => {
  // 直接命中 Scalar 内建 8 种
  it.each([
    ["en", "en"],
    ["EN", "en"],
    ["en-US", "en"],
    ["ru", "ru"],
    ["es", "es"],
    ["fr", "fr"],
    ["de", "de"],
    ["pt-BR", "pt"],
    ["pt-PT", "pt"],
  ])("%s → %s", (input, expected) => {
    expect(toScalarLocale(input)).toBe(expected);
  });

  // 中文特殊处理
  it("zh-CN → zh-CN", () => expect(toScalarLocale("zh-CN")).toBe("zh-CN"));
  it("zh-TW → zh-CN", () => expect(toScalarLocale("zh-TW")).toBe("zh-CN"));
  it("zh-HK → zh-CN", () => expect(toScalarLocale("zh-HK")).toBe("zh-CN"));

  // 降级到 en（Scalar 无内建）
  it.each([
    ["ja", "en"],
    ["ko", "en"],
    ["pl", "en"],
    ["tr", "en"],
    ["uk", "en"],
    ["vi", "en"],
  ])("%s → %s (fallback)", (input, expected) => {
    expect(toScalarLocale(input)).toBe(expected);
  });

  // 空值 → en
  it("空字符串 → en", () => expect(toScalarLocale("")).toBe("en"));
});

describe("toScalarDirection", () => {
  it("ar → rtl", () => expect(toScalarDirection("ar")).toBe("rtl"));
  it("ar-SA → rtl", () => expect(toScalarDirection("ar-SA")).toBe("rtl"));
  it("en → auto", () => expect(toScalarDirection("en")).toBe("auto"));
  it("zh-CN → auto", () => expect(toScalarDirection("zh-CN")).toBe("auto"));
});
