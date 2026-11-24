import type { CSSProperties, ReactNode } from "react";
import { Card, Space } from "antd";
import { sanitizeHtml, sanitizeCss } from "@/shared/lib/dom-purify";
import { LOGIN_SHELL_CSS_SCOPE } from "@/shared/lib/auth-signin-items";
import { useMediaQuery } from "@/shared/hooks/useMediaQuery";
import { SplitLayoutSide } from "./LoginAuxiliarySigninItems";

/**
 * 登录壳真正消费的字段子集。
 *
 * 用结构化子集而非完整 `ApplicationItem`，让预览面板可以用表单当前值
 * 构造同一份输入，保证「预览」与「真实登录页」走同一套渲染逻辑。
 */
export type LoginShellApplication = {
  description?: string | null;
  displayName?: string | null;
  name?: string | null;
  formCss?: string | null;
  formCssMobile?: string | null;
  formSideHtml?: string | null;
  formBackgroundUrlMobile?: string | null;
  headerHtml?: string | null;
  footerHtml?: string | null;
  customCss?: string | null;
};

type LoginShellLayoutProps = {
  application: LoginShellApplication | null;
  customSigninHtml: string;
  outerBackground: string;
  formPositionStyles: CSSProperties;
  isSplitLayout: boolean;
  /** 预览容器需要固定高度；真实页面保持整屏。 */
  contentMinHeight?: number | string;
  /**
   * 预览断点模拟：真实页面靠窗口宽度的 @media 生效，预览容器缩放不会触发
   * 媒体查询。传入 "desktop"/"mobile" 时直接输出对应断点的规则（不包
   * @media），并跳过另一断点的规则，让两套 CSS 在预览里可观测地互斥生效。
   */
  previewBreakpoint?: "desktop" | "mobile";
  children: ReactNode;
  overlays?: ReactNode;
};

export function LoginShellLayout(props: LoginShellLayoutProps) {
  const {
    application,
    children,
    customSigninHtml,
    formPositionStyles,
    isSplitLayout,
    outerBackground,
    previewBreakpoint,
  } = props;
  // 自定义整页 HTML 优先级最高：它替代整个登录页，其余装饰不再叠加。
  const overlays = props.overlays;
  const contentMinHeight = props.contentMinHeight ?? "100vh";
  const emulateMobile = previewBreakpoint === "mobile";
  const emulateDesktop = previewBreakpoint === "desktop";
  const isMobileViewport = useMediaQuery("(max-width: 768px)");

  // 移动背景图走内联样式而非 <style> 注入：sanitizeCss 的 url() 值过滤会把
  // 含 url() 的 background 声明整条丢弃（安全 allow-list 不区分来源）。
  // 内联 background 由 React 经 CSSOM 赋值，无 HTML 注入面；预览断点模拟优先。
  // 含括号/引号/反斜杠/空白的值可逃逸 url() token 注入额外内联声明，直接拒绝。
  const mobileBackgroundUrl = application?.formBackgroundUrlMobile ?? "";
  const safeMobileBackgroundUrl = /^[^)"'\\\s]+$/.test(mobileBackgroundUrl)
    ? mobileBackgroundUrl
    : "";
  const useMobileBackground =
    Boolean(safeMobileBackgroundUrl) && (previewBreakpoint ? emulateMobile : isMobileViewport);
  const background = useMobileBackground
    ? `linear-gradient(color-mix(in srgb, var(--gl-color-bg) 82%, transparent), color-mix(in srgb, var(--gl-color-bg) 92%, transparent)), url(${safeMobileBackgroundUrl}) center / cover no-repeat`
    : outerBackground;

  if (customSigninHtml.trim()) {
    return (
      <div
        className="login-signin-html-page"
        dangerouslySetInnerHTML={{ __html: sanitizeHtml(customSigninHtml) }}
      />
    );
  }

  return (
    <div
      style={{
        minHeight: contentMinHeight,
        position: "relative",
        display: "flex",
        padding: 24,
        background,
        ...formPositionStyles,
      }}
      className="login-content"
    >
      {/* 应用级 CSS 叠加顺序（同特异性时后者覆盖前者）：customCss（整壳通用）
          → formCss（桌面表单）→ formCssMobile（移动覆盖）。全部经 sanitizeCss
          净化并加 .login-content 作用域，租户 CSS 不波及壳外（portal/body）。
          预览帧（emulateDesktop/emulateMobile）始终注入 formCss + formCssMobile，
          由窄优先级覆盖：formCss 为基础样式，formCssMobile 为移动设备覆盖。 */}
      {application?.customCss ? (
        <style
          dangerouslySetInnerHTML={{
            __html: sanitizeCss(application.customCss, LOGIN_SHELL_CSS_SCOPE),
          }}
        />
      ) : null}
      {application?.formCss ? (
        <style
          dangerouslySetInnerHTML={{
            __html: sanitizeCss(
              emulateDesktop || emulateMobile
                ? application.formCss
                : `@media (min-width: 769px) { ${application.formCss} }`,
              LOGIN_SHELL_CSS_SCOPE,
            ),
          }}
        />
      ) : null}
      {application?.formCssMobile ? (
        <style
          dangerouslySetInnerHTML={{
            __html: sanitizeCss(
              emulateDesktop || emulateMobile
                ? application.formCssMobile
                : `@media (max-width: 768px) { ${application.formCssMobile} }`,
              LOGIN_SHELL_CSS_SCOPE,
            ),
          }}
        />
      ) : null}
      {application?.headerHtml ? (
        <div
          className="login-custom-header"
          style={{ position: "absolute", top: 0, left: 0, right: 0, zIndex: 1 }}
          dangerouslySetInnerHTML={{ __html: sanitizeHtml(application.headerHtml) }}
        />
      ) : null}
      {application?.footerHtml ? (
        <div
          className="login-custom-footer"
          style={{ position: "absolute", bottom: 0, left: 0, right: 0, zIndex: 1 }}
          dangerouslySetInnerHTML={{ __html: sanitizeHtml(application.footerHtml) }}
        />
      ) : null}
      <Card
        variant="borderless"
        style={{ width: isSplitLayout ? "min(100%, 980px)" : "min(100%, 460px)" }}
        className={`gl-panel ${isSplitLayout ? "login-panel" : "login-panel login-panel-compact"}`}
      >
        <Space orientation="vertical" size={20} style={{ width: "100%" }}>
          <div
            className="login-shell-grid"
            style={{
              display: "grid",
              gridTemplateColumns: isSplitLayout
                ? "minmax(260px, 360px) minmax(0, 1fr)"
                : "minmax(0, 1fr)",
              gap: 24,
              alignItems: "stretch",
            }}
          >
            <SplitLayoutSide
              description={application?.description ?? undefined}
              displayName={application?.displayName ?? undefined}
              isSplitLayout={isSplitLayout}
              name={application?.name ?? undefined}
              sideHtml={application?.formSideHtml ?? undefined}
            />

            <div className="login-form">{children}</div>
          </div>
        </Space>
      </Card>

      {overlays}
    </div>
  );
}
