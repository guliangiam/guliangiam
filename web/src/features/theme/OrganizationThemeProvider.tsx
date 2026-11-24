import ConfigProvider from "antd/es/config-provider";
import AntdApp from "antd/es/app";
import theme from "antd/es/theme";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo } from "react";
import type { PropsWithChildren } from "react";
import { getOrganizationTheme, getOrganizationPublicTheme } from "@/shared/api/iam/organization";
import { useSessionStore } from "@/features/session/session-store";
import { useThemeStore, resolvePrimaryColor } from "@/features/theme/theme-store";
import { useDiscoveredOrgStore } from "@/features/theme/discovered-org-store";
import { BRAND_PALETTE } from "@/app/theme/brand-palette";
import {
  pickReadableText,
  lightenToContrast,
  darkenToContrast,
  compositeOver,
  relativeLuminance,
} from "@/app/theme/contrast";
import { sanitizeOrganizationCss, sanitizeOrganizationTokens } from "./organization-theme";
import { useAntdLocale } from "@/shared/i18n/antd-locale";

const themeComponents = {
  Button: { primaryShadow: "none", borderRadius: 6, controlHeight: 36 },
  Input: { activeShadow: "none", borderRadius: 6 },
  Select: { borderRadius: 6 },
  Table: { fontWeightStrong: 600, headerBg: "var(--gl-color-bg-elevated)" },
  Menu: { itemFontSize: 14, itemHeight: 40 },
  Card: { borderRadiusLG: 14 },
  Modal: { borderRadiusLG: 14 },
  Form: { itemMarginBottom: 16 },
  Tag: { borderRadiusSM: 6 },
  Tooltip: { borderRadius: 8 },
  Dropdown: { borderRadiusLG: 10 },
  Message: { borderRadiusLG: 10 },
  Notification: { borderRadiusLG: 10 },
};

/**
 * 把组织配置的主题 token 与自定义 CSS 应用到控制台与登录前页面。
 *
 * - 有会话组织时走管理面 theme 子资源；无会话组织（登录/注册页）时按页面
 *   发布的组织 slug 走匿名公开投影（by-slug，含 theme、无凭据）；
 * - 无组织 / 未配置时完全沿用个人主题；
 * - token 走白名单，避免坏配置破坏全局；
 * - CSS 注入到独立 `<style data-organization-theme>`，切换组织时自动清理。
 */
export function OrganizationThemeProvider({ children }: PropsWithChildren) {
  const resolvedTheme = useThemeStore((s) => s.resolvedTheme);
  const themeConfig = useThemeStore((s) => s.config);
  const organizationId = useSessionStore((s) => s.organization?.id);
  const discoveredOrgSlug = useDiscoveredOrgStore((s) => s.orgSlug);
  const antdLocale = useAntdLocale();

  const themeQuery = useQuery({
    queryKey: ["organizations", "theme", organizationId],
    queryFn: () => getOrganizationTheme(organizationId as string),
    enabled: Boolean(organizationId),
    staleTime: 60_000,
    refetchOnWindowFocus: true,
  });

  const publicThemeQuery = useQuery({
    queryKey: ["organizations", "public-theme", discoveredOrgSlug],
    queryFn: () => getOrganizationPublicTheme(discoveredOrgSlug as string),
    enabled: !organizationId && Boolean(discoveredOrgSlug),
    staleTime: 60_000,
    retry: false,
  });

  const themeData = organizationId ? themeQuery.data : (publicThemeQuery.data ?? undefined);

  const sanitized = useMemo(
    () => sanitizeOrganizationTokens(themeData?.tokens),
    [themeData?.tokens],
  );

  const customCss = themeData?.customCss ?? "";

  useEffect(() => {
    const selector = "style[data-organization-theme]";
    const existing = document.querySelector<HTMLStyleElement>(selector);
    if (!customCss.trim()) {
      existing?.remove();
      return;
    }
    const style = existing ?? document.createElement("style");
    style.dataset.organizationTheme = organizationId ?? discoveredOrgSlug ?? "";
    style.textContent = sanitizeOrganizationCss(customCss);
    if (!existing) document.head.appendChild(style);
  }, [customCss, organizationId, discoveredOrgSlug]);

  const isDark = resolvedTheme === "dark";
  const algorithm = isDark ? theme.darkAlgorithm : theme.defaultAlgorithm;
  const palette = BRAND_PALETTE[isDark ? "dark" : "light"];
  const tokenString = (key: keyof typeof sanitized.tokens) =>
    typeof sanitized.tokens[key] === "string" ? (sanitized.tokens[key] as string) : undefined;
  /**
   * 表面/文字类 token（背景、边框、正文色）的组织覆盖只在浅色模式生效。
   *
   * 组织配置的 `colorBgContainer:#ffffff` 之类白底若原样进入暗黑模式，会让
   * 输入框、按钮、InputNumber、卡片全部变白底，文本对比度跌破 WCAG 2.1 AA。
   * 暗黑模式统一回落 `BRAND_PALETTE.dark` 深灰系（bg #1f1f1f / text #e8e8e8），
   * hover/focus/active/disabled 状态由 antd darkAlgorithm 从深底派生。
   */
  const surfaceToken = (key: keyof typeof sanitized.tokens) =>
    isDark ? undefined : tokenString(key);
  const tokenNumber = (key: keyof typeof sanitized.tokens) =>
    typeof sanitized.tokens[key] === "number" ? (sanitized.tokens[key] as number) : undefined;
  const colorPrimary =
    tokenString("colorPrimary") ?? resolvePrimaryColor(resolvedTheme, themeConfig);
  /**
   * 暗黑模式链接/文字按钮前景（≥ AA 4.5:1 的提亮品牌色）。
   *
   * `colorLink` 在 antd 6 是 seed token：经 `theme.token` 传入会被
   * `generateColorPalettes` 吸附到预设色板 index 6（深品牌色在深底只有 ~4:1），
   * 精确值无法保留；组件级 token 在算法之后直接合并，不重映射，因此链接色
   * 走 `components.Button/Typography` 覆盖。hover 更亮、active 略亮于常态，
   * 三档全部 ≥ 4.5:1（以最浅的 elevated 底 #2a2a2a 为基准，更深的容器底对比度更高）。
   */
  const linkColor = isDark ? lightenToContrast(String(colorPrimary), "#2a2a2a") : undefined;
  const linkHoverColor = isDark
    ? lightenToContrast(String(colorPrimary), "#2a2a2a", 5.5)
    : undefined;
  const linkActiveColor = isDark
    ? lightenToContrast(String(colorPrimary), "#2a2a2a", 4.9)
    : undefined;
  const containerBg = surfaceToken("colorBgContainer") ?? palette.colorBgContainer;
  /**
   * 组织覆盖的错误色是危险链接/文字按钮的前景种子，必须同样满足 AA：
   * 内置组织主题种子存的 antd 默认 #ff4d4f 在白底只有 3.26:1（axe serious）。
   * 浅色落底取容器底与斑马纹底（elevated #ffffff 混 1% 黑，即 #fcfcfc）中更暗者；
   * 暗黑与链接色同口径，以最浅 elevated #2a2a2a 为基准提亮。
   */
  const zebra = compositeOver("rgba(0, 0, 0, 0.01)", "#ffffff");
  const zebraBg = `rgb(${zebra.r}, ${zebra.g}, ${zebra.b})`;
  const errorBase = tokenString("colorError") ?? palette.colorError;
  let colorError: string = palette.colorError;
  try {
    colorError = isDark
      ? lightenToContrast(errorBase, "#2a2a2a")
      : darkenToContrast(
          errorBase,
          relativeLuminance(containerBg) <= relativeLuminance(zebraBg) ? containerBg : zebraBg,
        );
  } catch {
    // 组织 token 白名单允许命名色/hsl（parseColor 只认 hex/rgb），解析失败回落品牌错误色。
  }

  return (
    <ConfigProvider
      locale={antdLocale}
      theme={{
        algorithm: [algorithm],
        token: {
          colorPrimary,
          // 链接色只作为 seed 把整条 link 色板移到提亮后的品牌色相族；
          // 最终达标值由下方组件 token 提供（seed 会被色板吸附，见上文注释）。
          // seed token 绝不能传 undefined：antd 以 {...seed, ...token} 合并，
          // undefined 会覆盖默认 seed（fontSize=14 / colorLink=colorPrimary），
          // 使派生令牌算出 NaN（如 --ant-font-size-sm: NaNpx），进而让 Switch
          // 的 track-height/handle-size 变成 NaNpx → 尺寸塌缩 0×0 而不可见。
          colorLink: linkColor ?? colorPrimary,
          colorInfo: tokenString("colorInfo") ?? colorPrimary,
          colorSuccess: tokenString("colorSuccess") ?? palette.colorSuccess,
          colorWarning: tokenString("colorWarning") ?? palette.colorWarning,
          colorError,
          borderRadius: tokenNumber("borderRadius") ?? themeConfig.borderRadius,
          fontSize: tokenNumber("fontSize") ?? 14,
          fontFamily: '"IBM Plex Sans", "Segoe UI", sans-serif',
          colorBgContainer: containerBg,
          colorBgElevated: isDark ? "#2a2a2a" : "#ffffff",
          colorBgLayout: surfaceToken("colorBgLayout") ?? "transparent",
          colorBorder: surfaceToken("colorBorder") ?? palette.colorBorder,
          colorBorderSecondary: isDark ? "rgba(255, 255, 255, 0.05)" : "rgba(16, 42, 44, 0.05)",
          colorText: surfaceToken("colorText") ?? palette.colorText,
          colorTextSecondary: palette.colorTextSecondary,
          colorTextTertiary: palette.colorTextTertiary,
          // antd 默认 placeholder 色 #bfbfbf 在白底只有 1.83:1（axe color-contrast
          // 直接判 serious）。占位符属于承载语义的文本，必须与 tertiary 同档。
          colorTextPlaceholder: palette.colorTextTertiary,
        },
        components: {
          ...themeComponents,
          Button: {
            ...themeComponents.Button,
            primaryColor: pickReadableText(String(colorPrimary)),
            ...(isDark
              ? {
                  colorLink: linkColor,
                  colorLinkHover: linkHoverColor,
                  colorLinkActive: linkActiveColor,
                }
              : null),
          },
          ...(isDark
            ? {
                Typography: {
                  colorLink: linkColor,
                  colorLinkHover: linkHoverColor,
                  colorLinkActive: linkActiveColor,
                },
                // 分页激活项文本/描边消费 colorPrimary：深品牌色在深底只有 ~3.2:1，
                // 组件级覆盖为提亮值（仅影响分页族，不改全局主色）。
                Pagination: { colorPrimary: linkColor },
              }
            : null),
        },
      }}
    >
      <AntdApp>{children}</AntdApp>
    </ConfigProvider>
  );
}
