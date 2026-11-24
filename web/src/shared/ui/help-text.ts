import { useTranslation } from "react-i18next";

/**
 * 解析 help 命名空间中的说明文本；key 不存在时返回 undefined，让调用方决定
 * 回退到全局兜底文案或交给全局注入器处理。
 */
export function useHelpText(helpKey?: string): string | undefined {
  const { t, i18n } = useTranslation("help");
  if (!helpKey) return undefined;
  // 测试环境可能注入精简的 i18n 替身，没有 exists()；此时不走显式帮助，
  // 交给全局注入器或直接不渲染，避免渲染期抛错。
  if (!i18n || typeof i18n.exists !== "function") return undefined;
  if (!i18n.exists(helpKey, { ns: "help" })) return undefined;
  return t(helpKey);
}
