/**
 * Scalar API Reference 配置工具
 *
 * 负责把 GuliangIAM 前端的 i18n 语言码 / theme-store 主题状态
 * 映射到 Scalar ApiReferenceReact 的 configuration。
 */

import type {
  ApiReferenceLocale,
  ApiReferenceTextDirectionPreference,
} from "@scalar/types/api-reference";

/** Scalar 内建支持的 UI 翻译语言（ApiReferenceBuiltInLocale） */
const SCALAR_BUILTIN_LOCALES: ApiReferenceLocale[] = [
  "en",
  "ru",
  "es",
  "fr",
  "de",
  "zh-CN",
  "ar",
  "pt",
];

/** 我们支持但 Scalar 没内建的语言 → 降级到哪个 Scalar locale */
const LOCALE_FALLBACK: Record<string, ApiReferenceLocale> = {
  ja: "en", // 日语 → 英语（无内建）
  ko: "en", // 韩语 → 英语
  pl: "en", // 波兰语 → 英语
  tr: "en", // 土耳其语 → 英语
  uk: "en", // 乌克兰语 → 英语
  vi: "en", // 越南语 → 英语
  "zh-TW": "zh-CN", // 繁中 → 简中
};

/** 我们的语言码 → Scalar locale 归一化 */
export function toScalarLocale(language: string): ApiReferenceLocale {
  // 去掉 country 后缀（例如 "en-US" → "en"）
  const tag = language.toLowerCase();
  // 先尝试完整匹配
  if (SCALAR_BUILTIN_LOCALES.includes(tag as ApiReferenceLocale)) {
    return tag as ApiReferenceLocale;
  }
  // zh-* 归一到 zh-CN（Scalar 只有简中内建）
  if (tag.startsWith("zh")) return "zh-CN";
  // 取主语言码
  const primary = tag.split("-")[0];
  if (SCALAR_BUILTIN_LOCALES.includes(primary as ApiReferenceLocale)) {
    return primary as ApiReferenceLocale;
  }
  return LOCALE_FALLBACK[primary] ?? "en";
}

/** RTL 语言列表 */
const RTL_LOCALES = new Set(["ar", "he", "fa", "ur"]);

export function toScalarDirection(language: string): ApiReferenceTextDirectionPreference {
  const tag = language.toLowerCase().split("-")[0];
  return RTL_LOCALES.has(tag) ? "rtl" : "auto";
}
