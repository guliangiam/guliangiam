import { useQuery } from "@tanstack/react-query";
import { ApiReferenceReact } from "@scalar/api-reference-react";
import "@scalar/api-reference-react/style.css";
import { useThemeStore } from "@/features/theme/theme-store";
import { useTranslation } from "react-i18next";
import { useMemo } from "react";
import { toScalarDirection, toScalarLocale } from "./scalar-config";
import { requestText } from "@/shared/api/http";

/**
 * Swagger 页面 — Scalar API Reference
 *
 * 跟随全局语言 + 明暗主题切换：
 *   - i18n.language 变化 → Scalar configuration.localization.locale 同步切换
 *   - theme-store.resolvedTheme 变化 → Scalar darkMode 同步切换（force 模式，隐藏内建 toggle）
 *
 * 为什么用 key={locale + theme} 强制 remount？
 *   Scalar 的 ApiReferenceReact 内部是 Vue SPA，initial configuration 只在 mount 时读取一次，
 *   运行时 props 变化不会 re-init。通过 key 触发 React unmount/mount，保证 locale / darkMode 都生效。
 */
export default function SwaggerPage() {
  const { t, i18n } = useTranslation("common");
  const resolvedTheme = useThemeStore((s) => s.resolvedTheme);

  const {
    data: spec,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["openapi-spec"],
    queryFn: async () => {
      try {
        return await requestText("/openapi/openapi.json");
      } catch (err: unknown) {
        const status =
          err && typeof err === "object" && "status" in (err as Record<string, unknown>)
            ? (err as Record<string, unknown>).status
            : "unknown";
        throw new Error(`Failed to fetch OpenAPI spec: ${status}`, { cause: err });
      }
    },
    staleTime: 5 * 60 * 1000, // 5min cache (matches server Cache-Control)
  });

  const configuration = useMemo(() => {
    const locale = toScalarLocale(i18n.language);
    const direction = toScalarDirection(i18n.language);
    const isDark = resolvedTheme === "dark";

    // TypeScript 7 不再允许条件表达式上的 `as const` 断言
    const darkModeState = isDark ? "dark" : "light";

    return {
      content: spec,
      layout: "modern" as const,

      // 色板预设：暗色用 deepSpace，亮色用 alternate
      theme: isDark ? ("deepSpace" as const) : ("alternate" as const),

      // 强制 Scalar 的明暗状态跟随我们的 theme-store（关键：隐藏内建 toggle，全局一把钥匙）
      darkMode: isDark,
      forceDarkModeState: darkModeState,
      hideDarkModeToggle: true,

      // 多语言
      localization: {
        locale,
        direction,
      },

      // 其他行为
      hideModels: false,
      hideSearch: false,
      hideClientButton: false,
      hideTestRequestButton: false,
      searchHotKey: "k",
      withDefaultFonts: true,
      defaultOpenFirstTag: true,
      orderSchemaPropertiesBy: "alpha" as const,
      orderRequiredPropertiesFirst: true,
      operationTitleSource: "summary" as const,
      showOperationId: false,
    } as import("@scalar/types/api-reference").AnyApiReferenceConfiguration;
  }, [spec, i18n.language, resolvedTheme]);

  if (isLoading) {
    return <div className="gl-api-reference gl-api-reference--loading">{t("swagger.loading")}</div>;
  }

  if (error) {
    return (
      <div className="gl-api-reference gl-api-reference--error">{t("swagger.loadFailed")}</div>
    );
  }

  // key = locale + theme 强制 remount（Scalar 内部 Vue SPA 不响应 props 变化）
  const key = `${toScalarLocale(i18n.language)}:${resolvedTheme}`;

  return (
    <div className="gl-api-reference">
      <ApiReferenceReact key={key} configuration={configuration} />
    </div>
  );
}
