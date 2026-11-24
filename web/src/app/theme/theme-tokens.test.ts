import { describe, it, expect } from "vitest";
import { BRAND_PALETTE, THEME_TOKEN_CSS_VARS } from "./brand-palette";
import {
  AA_TEXT_CONTRAST,
  contrastRatio,
  darkenToContrast,
  lightenToContrast,
  pickReadableText,
  relativeLuminance,
} from "./contrast";
import { PRESET_THEMES } from "./theme-presets";

describe("brand palette", () => {
  it("mirrors every mapped CSS token", () => {
    for (const key of Object.keys(THEME_TOKEN_CSS_VARS)) {
      expect(BRAND_PALETTE.light).toHaveProperty(key);
      expect(BRAND_PALETTE.dark).toHaveProperty(key);
    }
  });

  it("uses light-mode text colors that are readable on their surfaces", () => {
    const { colorText, colorTextSecondary, colorTextTertiary, colorBgContainer } =
      BRAND_PALETTE.light;
    for (const color of [colorText, colorTextSecondary, colorTextTertiary]) {
      const ratio = contrastRatio(color, colorBgContainer);
      expect(ratio, `${color} on ${colorBgContainer}`).toBeGreaterThanOrEqual(AA_TEXT_CONTRAST);
    }
  });

  it("uses dark-mode text colors that are readable on their surfaces", () => {
    const { colorText, colorTextSecondary, colorTextTertiary, colorBgContainer } =
      BRAND_PALETTE.dark;
    for (const color of [colorText, colorTextSecondary, colorTextTertiary]) {
      const ratio = contrastRatio(color, colorBgContainer);
      expect(ratio, `${color} on ${colorBgContainer}`).toBeGreaterThanOrEqual(AA_TEXT_CONTRAST);
    }
  });
});

describe("contrast helpers", () => {
  it("computes WCAG relative luminance for black and white", () => {
    expect(relativeLuminance("#000000")).toBeCloseTo(0, 5);
    expect(relativeLuminance("#ffffff")).toBeCloseTo(1, 5);
  });

  it("computes the documented ratio for a known pair", () => {
    expect(contrastRatio("#ffffff", "#000000")).toBeCloseTo(21, 1);
  });

  it("picks the brighter foreground on dark brand colors", () => {
    expect(pickReadableText(BRAND_PALETTE.light.colorPrimary)).toBe(
      BRAND_PALETTE.light.colorBgContainer,
    );
  });

  it("picks the darker foreground on bright brand colors", () => {
    expect(pickReadableText(BRAND_PALETTE.dark.colorPrimary)).toBe(BRAND_PALETTE.light.colorText);
  });

  it("darkens the antd default error color to AA on white and zebra surfaces", () => {
    // 内置组织主题种子存的 #ff4d4f 在白底 3.26:1 / 斑马纹底 3.18:1（E2E-02 实测）。
    for (const bg of ["#ffffff", "#fcfcfc"]) {
      const clamped = darkenToContrast("#ff4d4f", bg);
      expect(contrastRatio(clamped, bg), `${clamped} on ${bg}`).toBeGreaterThanOrEqual(
        AA_TEXT_CONTRAST,
      );
    }
  });

  it("keeps an already-compliant error color unchanged", () => {
    expect(darkenToContrast(BRAND_PALETTE.light.colorError, "#ffffff")).toBe(
      BRAND_PALETTE.light.colorError,
    );
  });

  it("falls back to black when no darkened step can reach the target", () => {
    expect(darkenToContrast("#808080", "#808080", 21)).toBe("#000000");
  });

  it("lightens the dark-mode error color to AA on the brightest elevated surface", () => {
    const clamped = lightenToContrast(BRAND_PALETTE.dark.colorError, "#2a2a2a");
    expect(contrastRatio(clamped, "#2a2a2a")).toBeGreaterThanOrEqual(AA_TEXT_CONTRAST);
  });
});

describe("theme presets", () => {
  it("keeps every preset label above the AA threshold in both modes", () => {
    for (const preset of PRESET_THEMES) {
      for (const color of [preset.value.primaryColor, preset.value.darkPrimaryColor]) {
        const onColor = pickReadableText(color);
        const ratio = contrastRatio(onColor, color);
        expect(ratio, `${preset.label}: ${color} with ${onColor}`).toBeGreaterThanOrEqual(
          AA_TEXT_CONTRAST,
        );
      }
    }
  });
});
