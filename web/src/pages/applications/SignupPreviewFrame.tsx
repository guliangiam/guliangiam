import { Button, Form, Input, Space, Typography } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import { useMemo } from "react";
import { Trans, useTranslation } from "react-i18next";
import type { SignupItem } from "@/shared/api/iam/application";
import { SignupFormItems } from "@/pages/auth/SignupFormItems";
import { LoginShellLayout } from "@/pages/auth/LoginShellLayout";
import {
  getFormPositionStyles,
  getIsSplitLayout,
  getOuterBackground,
} from "@/shared/lib/auth-signin-items";
import type { PreviewBreakpoint } from "@/pages/applications/tabs/LoginPreviewPanel";

type SignupPreviewFrameProps = {
  signupItems?: SignupItem[];
  formBackgroundUrl?: string;
  formOffset?: string | number;
  formCss?: string;
  formSideHtml?: string;
  headerHtml?: string;
  footerHtml?: string;
  customCss?: string;
  formBackgroundUrlMobile?: string;
  formCssMobile?: string;
  signupHtml?: string;
  applicationName?: string;
  applicationDescription?: string;
  /** 外层固定断点（与登录预览帧一致）：desktop/mobile 互斥生效两套 CSS。 */
  compactBreakpoint?: PreviewBreakpoint;
};

/**
 * 注册预览帧 —— 与真实注册页（SignupPage）走同一壳渲染链：
 * `LoginShellLayout` 消费 customCss/formCss/formCssMobile/headerHtml/footerHtml/
 * formSideHtml/背景图/formOffset，`SignupFormItems` 渲染字段（与真实页同一组件）。
 * 背景、位置、自定义 HTML/CSS 的改动在注册预览里与登录预览同步可观测，零漂移。
 * 外层由 UiCustomizationTab 提供 overflow-auto 帧容器与断点切换。
 */
export function SignupPreviewFrame({
  signupItems,
  formBackgroundUrl,
  formOffset,
  formCss,
  formSideHtml,
  headerHtml,
  footerHtml,
  customCss,
  formBackgroundUrlMobile,
  formCssMobile,
  signupHtml,
  applicationName,
  applicationDescription,
  compactBreakpoint = "desktop",
}: SignupPreviewFrameProps) {
  const { t } = useTranslation("auth");
  const [form] = Form.useForm();

  const items = useMemo(
    () => (signupItems ?? []).filter((it) => it.visible !== false),
    [signupItems],
  );
  // 「Captcha」注册项此前在预览里完全不可见，管理员无法判断它是否生效。
  // 与真实页规则一致：可见 = 该应用启用图形验证（还要求组织有验证码供应商）。
  const captchaItemVisible = items.some((it) => it.name === "Captcha");

  const passwordValue = String(Form.useWatch("password", form) ?? "");

  const isMobile = compactBreakpoint === "mobile";
  const activeBackgroundUrl = isMobile ? formBackgroundUrlMobile : formBackgroundUrl;
  const outerBackground = getOuterBackground(activeBackgroundUrl);
  const isSplitLayout = getIsSplitLayout(formOffset);

  return (
    <div
      data-testid="preview-signup-frame"
      style={{
        height: 560,
        width: isMobile ? "min(375px, 100%)" : "100%",
        margin: isMobile ? "0 auto" : undefined,
        overflow: "hidden",
      }}
    >
      <LoginShellLayout
        application={{
          name: applicationName,
          displayName: applicationName,
          description: applicationDescription,
          formCss,
          formCssMobile,
          formSideHtml,
          formBackgroundUrlMobile,
          headerHtml,
          footerHtml,
          customCss,
        }}
        customSigninHtml={signupHtml ?? ""}
        outerBackground={outerBackground}
        formPositionStyles={getFormPositionStyles(formOffset)}
        isSplitLayout={isSplitLayout}
        contentMinHeight={560}
        previewBreakpoint={compactBreakpoint}
      >
        {/* 标题/副标题结构对齐真实 SignupPage（gl-auth-form__title / __subtitle） */}
        <div>
          <h1 className="gl-auth-form__title">{t("auth:createAccount")}</h1>
          <p className="gl-auth-form__subtitle">
            <Trans
              i18nKey="auth:registerInOrg"
              values={{ org: "built-in" }}
              components={{ org: <strong /> }}
            />
          </p>
        </div>

        {/* 预览帧嵌在详情页外层 Form 内：component="div" 保留表单 store 又不产生嵌套 <form> */}
        <Form
          form={form}
          component="div"
          layout="vertical"
          autoComplete="off"
          onFinish={() => undefined}
        >
          <SignupFormItems items={items} passwordValue={passwordValue} showCaptcha={false} t={t} />

          {captchaItemVisible && (
            <div style={{ marginBottom: 16 }} data-testid="preview-signup-captcha">
              <Typography.Text strong style={{ display: "block", marginBottom: 8 }}>
                {t("humanCaptcha")}
              </Typography.Text>
              <Space.Compact style={{ width: "100%" }}>
                <Input disabled placeholder={t("enterCaptcha")} />
                <Button disabled icon={<ReloadOutlined />} aria-label={t("reloadCaptcha")} />
              </Space.Compact>
            </div>
          )}

          <Form.Item>
            <Button type="primary" onClick={() => form.submit()} block>
              {t("auth:signup")}
            </Button>
          </Form.Item>
        </Form>

        {/* 已有账户登录链接 —— 对齐真实 SignupPage 的 Button type="link" */}
        <div style={{ textAlign: "center" }}>
          {t("auth:alreadyHaveAccount")} <Button type="link">{t("auth:signIn")}</Button>
        </div>
      </LoginShellLayout>
    </div>
  );
}
