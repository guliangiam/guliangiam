import { BRAND_PALETTE } from "./brand-palette";

/**
 * 主题编辑器的预设配色目录。
 *
 * 每个预设都必须能让实心主色底上的文字达到 WCAG 2.1 AA（4.5:1）——由
 * `theme-presets.test.ts` 用 `pickReadableText` 逐条断言，新增预设必须先通过该断言。
 */
export const PRESET_THEMES = [
  {
    label: "GuliangIAM Teal",
    value: {
      primaryColor: BRAND_PALETTE.light.colorPrimary,
      darkPrimaryColor: BRAND_PALETTE.dark.colorPrimary,
    },
  },
  { label: "Ocean Blue", value: { primaryColor: "#0958d9", darkPrimaryColor: "#4096ff" } },
  { label: "Forest Green", value: { primaryColor: "#237804", darkPrimaryColor: "#52c41a" } },
  { label: "Royal Purple", value: { primaryColor: "#722ed1", darkPrimaryColor: "#9254de" } },
  { label: "Sunset Orange", value: { primaryColor: "#d4380d", darkPrimaryColor: "#ff4d4f" } },
  { label: "Rose Gold", value: { primaryColor: "#c41d7f", darkPrimaryColor: "#ff85c0" } },
] as const;
