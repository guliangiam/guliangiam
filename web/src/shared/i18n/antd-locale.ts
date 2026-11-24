import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { Locale } from "antd/es/locale";
import zhCN from "antd/es/locale/zh_CN";

// antd 组件内置文案（Popconfirm 的 OK/Cancel、Pagination、DatePicker 等）随界面语言本地化。
// 与 i18n 资源一致按语言懒加载，zh-CN 作为静态兜底（应用 fallback 语言）。
const loaders: Record<string, () => Promise<{ default: Locale }>> = {
  de: () => import("antd/es/locale/de_DE"),
  en: () => import("antd/es/locale/en_US"),
  es: () => import("antd/es/locale/es_ES"),
  fr: () => import("antd/es/locale/fr_FR"),
  ja: () => import("antd/es/locale/ja_JP"),
  ko: () => import("antd/es/locale/ko_KR"),
  pl: () => import("antd/es/locale/pl_PL"),
  pt: () => import("antd/es/locale/pt_PT"),
  ru: () => import("antd/es/locale/ru_RU"),
  tr: () => import("antd/es/locale/tr_TR"),
  uk: () => import("antd/es/locale/uk_UA"),
  vi: () => import("antd/es/locale/vi_VN"),
  "zh-TW": () => import("antd/es/locale/zh_TW"),
};

// zh-CN 已静态导入作兜底，不再动态 import（避免同模块静态+动态重复打包告警）。
const zhCNLoader = () => Promise.resolve({ default: zhCN });

export function useAntdLocale(): Locale {
  const { i18n } = useTranslation();
  const lang = i18n.language || "zh-CN";
  const [locale, setLocale] = useState<Locale>(zhCN);

  useEffect(() => {
    const load = loaders[lang] ?? loaders[lang.split("-")[0]] ?? zhCNLoader;
    let alive = true;
    void (async () => {
      try {
        const m = await load();
        if (alive) setLocale(m.default);
      } catch {
        if (alive) setLocale(zhCN);
      }
    })();
    return () => {
      alive = false;
    };
  }, [lang]);

  return locale;
}
