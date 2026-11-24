import { Alert, Card, Form, Input, Segmented, Select, Space } from "antd";
import { Fragment, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { ApplicationItem } from "@/shared/types/domain";
import type { ProviderSigninOption } from "@/shared/lib/auth-provider-signin";
import type { QrSigninState } from "@/shared/hooks/useQrSigninFlow";
import {
  getForgetPasswordUrl,
  getFormPositionStyles,
  getIsSplitLayout,
  getOuterBackground,
  getSigninMethodTabs,
  getUsernamePlaceholder,
  isCustomSigninItem,
  isSigninItemShown,
  LOGIN_SHELL_CSS_SCOPE,
  type LoginMethodKey,
  type SigninCapabilities,
  type SigninItemConfig,
} from "@/shared/lib/auth-signin-items";
import { sanitizeCss, sanitizeHtml } from "@/shared/lib/dom-purify";
import { LoginShellLayout } from "@/pages/auth/LoginShellLayout";
import { LoginHeaderSection } from "@/pages/auth/LoginHeaderSection";
import { FormHeading, LoginMethodSupportAlert } from "@/pages/auth/LoginAuxiliaryLayout";
import {
  AgreementSigninItem,
  ForgotPasswordSigninItem,
  LoginButtonSigninItem,
  PasswordSigninItem,
  SelectOrganizationSigninItem,
  SigninMethodsItem,
  SignupLinkSigninItem,
  VerificationCodeSigninItem,
} from "@/pages/auth/LoginAuxiliarySigninItems";
import {
  CaptchaSigninItem,
  ProvidersSigninItem,
  UsernameSigninItem,
  type CaptchaWidgetProps,
} from "@/pages/auth/LoginSigninItems";
import type { SigninItem } from "@/shared/api/iam/application";

export type PreviewBreakpoint = "desktop" | "mobile";

type LoginPreviewPanelProps = {
  formBackgroundUrl?: string;
  formOffset?: string | number;
  formCss?: string;
  formSideHtml?: string;
  headerHtml?: string;
  footerHtml?: string;
  customCss?: string;
  formBackgroundUrlMobile?: string;
  formCssMobile?: string;
  signinHtml?: string;
  applicationName?: string;
  applicationDescription?: string;
  logoUrl?: string;
  signinItems?: SigninItem[];

  /** 嵌入模式：由外层固定断点（不显示 Segmented/语言切换），用于 Grid 两帧并排。 */
  compactBreakpoint?: PreviewBreakpoint;
  /** 紧凑渲染：不包 Card，直接返回帧内容（外层帧容器提供 border/shadow/overflow）。 */
  compact?: boolean;
};

/**
 * 预览环境的能力探测桩：WebAuthn/Face ID/扫码在配置页预览中不可用，
 * 对应 tab 由真实 getSigninMethodTabs 派生为 disabled（与真实页同一逻辑）。
 */
const PREVIEW_CAPABILITIES: SigninCapabilities = {
  hasFaceIdProvider: false,
  supportsWebAuthn: false,
  isSecureContext: false,
  hasMediaDevices: false,
  supportsPlatformAuth: false,
};

const IDLE_QR_STATE: QrSigninState = {
  status: "idle",
  code: null,
  expiresIn: null,
  error: null,
};

/** 预览用第三方登录桩数据：真实 ProvidersSigninItem 渲染，样式与真实页零漂移。 */
const PREVIEW_PROVIDERS: ProviderSigninOption[] = [
  { name: "google", displayName: "Google", type: "Google", canStartDirectly: true },
  { name: "wechat", displayName: "WeChat", type: "WeChat", canStartDirectly: true },
  { name: "apple", displayName: "Apple", type: "Apple", canStartDirectly: true },
];

/** 预览不加载真实验证码组件；type=default 分支不会渲染 widget，此桩仅满足 props 契约。 */
function PreviewCaptchaWidget(_props: CaptchaWidgetProps) {
  return null;
}

/**
 * 登录预览面板 —— 对标 casdoor ApplicationPreview：直接复用真实登录页的
 * item 渲染组件（LoginSigninForm 的同一批导出），而非重写 switch-case。
 *
 * 结构对齐真实链路：
 *   LoginShellLayout（真实壳）
 *     ├─ LoginHeaderSection（真实 Header：Logo / Languages / Back button）
 *     ├─ FormHeading（真实标题区）
 *     └─ Form
 *         ├─ hidden application / organization
 *         ├─ LoginMethodSupportAlert（真实组件）
 *         └─ signinItems.map → 真实 item 组件逐 name 分支
 *
 * tabs 由 getSigninMethodTabs(pseudoApp, capabilities) 派生：关闭 Password
 * 登录项 → password/ldap tab 即时 disabled；loginMethod 自动回退首个可用方式；
 * placeholder/rule/customCss 全部由真实组件消费，预览与真实页零漂移。
 */
export function LoginPreviewPanel({
  formBackgroundUrl,
  formOffset,
  formCss,
  formSideHtml,
  headerHtml,
  footerHtml,
  customCss,
  formBackgroundUrlMobile,
  formCssMobile,
  signinHtml,
  applicationName,
  applicationDescription,
  logoUrl,
  signinItems,
  compactBreakpoint,
  compact,
}: LoginPreviewPanelProps) {
  const { t, i18n } = useTranslation("auth");
  const { t: ta } = useTranslation("resource");
  const [innerBreakpoint, setInnerBreakpoint] = useState<PreviewBreakpoint>("desktop");
  const [form] = Form.useForm();
  const [loginMethod, setLoginMethod] = useState<LoginMethodKey>("password");

  const breakpoint = compactBreakpoint ?? innerBreakpoint;
  const isMobile = breakpoint === "mobile";
  const isSplitLayout = getIsSplitLayout(formOffset);
  const activeBackgroundUrl = isMobile ? formBackgroundUrlMobile : formBackgroundUrl;
  const outerBackground = getOuterBackground(activeBackgroundUrl);
  const hasCustomSigninHtml = Boolean(signinHtml && signinHtml.trim());

  const configItems = useMemo(
    () => (signinItems ?? []) as unknown as SigninItemConfig[],
    [signinItems],
  );

  // 伪应用对象：getSigninMethodTabs / getSigninItems 只消费 name/signinItems；
  // 无 id/organizationId → qrcode tab disabled（与真实页同一判定）。
  const pseudoApplication = useMemo(
    () =>
      ({
        name: applicationName ?? "app-built-in",
        displayName: applicationName ?? "app-built-in",
        organization: "built-in",
        signinItems: configItems,
      }) as unknown as ApplicationItem,
    [applicationName, configItems],
  );

  const signinMethodTabs = useMemo(
    () => getSigninMethodTabs(pseudoApplication, PREVIEW_CAPABILITIES),
    [pseudoApplication],
  );

  // 当前方式对应 tab 被禁用（如关闭 Password 登录项）时，自动回退首个可用方式 ——
  // 与真实页 getDefaultLoginMethod 同一语义。
  const effectiveLoginMethod: LoginMethodKey = useMemo(() => {
    const current = signinMethodTabs.find((tab) => tab.key === loginMethod);
    if (current && !current.disabled) {
      return loginMethod;
    }
    return signinMethodTabs.find((tab) => !tab.disabled)?.key ?? "password";
  }, [signinMethodTabs, loginMethod]);

  const renderSigninItemCss = (item: SigninItemConfig) => {
    if (!item.customCss?.trim() || isCustomSigninItem(item)) {
      return null;
    }
    return (
      <style
        dangerouslySetInnerHTML={{ __html: sanitizeCss(item.customCss, LOGIN_SHELL_CSS_SCOPE) }}
      />
    );
  };

  // Header 项（Logo/Languages/Back button）与 Form 项分离 —— 与真实 LoginPanelSection 一致。
  const headerNames = new Set(["Logo", "Languages", "Back button"]);
  const headerItems = configItems.filter((item) => headerNames.has(item.name ?? ""));
  const formItems = configItems.filter((item) => !headerNames.has(item.name ?? ""));
  const showSelectOrganization = formItems.some(
    (item) => item.name === "Select organization" && isSigninItemShown(item),
  );

  // 与真实 LoginSigninForm.renderOrderedSigninFormItem 同一 switch 结构，
  // 分支内直接调用真实 item 组件，stub 仅用于满足交互态 props。
  const renderFormItem = (item: SigninItemConfig, index: number) => {
    if (!isSigninItemShown(item) && item.name !== "Forgot password?") {
      return null;
    }

    const itemKey = `preview_${item.name ?? "item"}_${index}`;

    if (isCustomSigninItem(item)) {
      if (!item.customCss?.trim()) {
        return null;
      }
      return (
        <div
          key={itemKey}
          className="login-custom-signin-item"
          dangerouslySetInnerHTML={{ __html: sanitizeHtml(item.customCss) }}
        />
      );
    }

    switch (item.name) {
      case "Signin methods":
        return (
          <SigninMethodsItem
            item={item}
            itemKey={itemKey}
            loginMethod={effectiveLoginMethod}
            onChangeLoginMethod={setLoginMethod}
            renderSigninItemCss={renderSigninItemCss}
            signinMethodTabs={signinMethodTabs}
          />
        );
      case "Select organization":
        return (
          <SelectOrganizationSigninItem
            fallbackInputDisabled={false}
            isError={false}
            isLoading={false}
            item={item}
            itemKey={itemKey}
            onSelectOrganization={() => undefined}
            options={[{ label: "Built-in Organization", value: "built-in" }]}
            showSelectOrganization
            renderSigninItemCss={renderSigninItemCss}
          />
        );
      case "Username":
        return (
          <UsernameSigninItem
            availableCountryCodes={[]}
            form={form}
            item={item}
            itemKey={itemKey}
            loginMethod={effectiveLoginMethod}
            onReloadQrSignin={() => undefined}
            qrSigninState={IDLE_QR_STATE}
            renderSigninItemCss={renderSigninItemCss}
            submitting={false}
            usernamePlaceholder={getUsernamePlaceholder(effectiveLoginMethod)}
          />
        );
      case "Password":
        return (
          <PasswordSigninItem
            item={item}
            itemKey={itemKey}
            loginMethod={effectiveLoginMethod}
            renderSigninItemCss={renderSigninItemCss}
          />
        );
      case "Verification code":
        return (
          <VerificationCodeSigninItem
            item={item}
            itemKey={itemKey}
            loginMethod={effectiveLoginMethod}
            onSendCode={() => undefined}
            renderSigninItemCss={renderSigninItemCss}
            sendCodePending={false}
          />
        );
      case "Agreement":
        return (
          <AgreementSigninItem
            agreementRequired
            hasTermsOfUse
            item={item}
            itemKey={itemKey}
            loginMethod={effectiveLoginMethod}
            onOpenTerms={() => undefined}
            renderSigninItemCss={renderSigninItemCss}
          />
        );
      case "Captcha":
        return (
          <CaptchaSigninItem
            CaptchaWidget={PreviewCaptchaWidget}
            captchaReloadSeed={0}
            captchaRequired
            captchaState={{ type: "default", captchaImage: null }}
            form={form}
            item={item}
            itemKey={itemKey}
            loginMethod={effectiveLoginMethod}
            reloadCaptcha={async () => undefined}
            renderSigninItemCss={renderSigninItemCss}
          />
        );
      case "Login button":
        return (
          <LoginButtonSigninItem
            currentLoginMethodSupported
            disabled={false}
            hasSupportedSigninMethod
            isOauthAuthorize={false}
            isSamlLogin={false}
            isSupportedOauthResponseType
            item={item}
            itemKey={itemKey}
            loading={false}
            loginMethod={effectiveLoginMethod}
            renderSigninItemCss={renderSigninItemCss}
          />
        );
      case "Forgot password?":
        return (
          <ForgotPasswordSigninItem
            forgetPasswordUrl={getForgetPasswordUrl(applicationName ?? "app-built-in")}
            item={item}
            itemKey={itemKey}
            renderSigninItemCss={renderSigninItemCss}
          />
        );
      case "Signup link":
        return (
          <SignupLinkSigninItem
            enableSignUp
            item={item}
            itemKey={itemKey}
            onStoreSigninUrl={() => undefined}
            renderSigninItemCss={renderSigninItemCss}
            search=""
            signupUrl="/signup"
          />
        );
      case "Providers": {
        const displayMode = item.rule === "big" || item.rule === "small" ? item.rule : "small";
        return (
          <ProvidersSigninItem
            displayMode={displayMode}
            hasProviderHintSupport
            item={item}
            itemKey={itemKey}
            onProviderClick={() => undefined}
            providers={PREVIEW_PROVIDERS}
            renderSigninItemCss={renderSigninItemCss}
            showProviders
          />
        );
      }
      default:
        return null;
    }
  };

  const body = (
    <>
      {hasCustomSigninHtml ? (
        <div style={{ padding: 8 }}>
          <Alert type="info" showIcon title={ta("application.previewCustomHtmlActive")} />
        </div>
      ) : null}
      <div
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
          customSigninHtml={signinHtml ?? ""}
          outerBackground={outerBackground}
          formPositionStyles={getFormPositionStyles(formOffset)}
          isSplitLayout={isSplitLayout}
          contentMinHeight={560}
          previewBreakpoint={breakpoint}
        >
          {/* 外层间距结构对齐真实 LoginPanelSection（Space size=20 + header/heading 分组） */}
          <Space orientation="vertical" size={20} style={{ width: "100%" }}>
            <div>
              <LoginHeaderSection
                availableLanguages={["zh-CN", "en"]}
                headerSigninItems={headerItems}
                logoAlt={applicationName ?? "Application"}
                logoUrl={logoUrl}
                onBack={() => undefined}
                onChangeLanguage={(nextLanguage) => {
                  void i18n.changeLanguage(nextLanguage);
                }}
                preferredLanguage={i18n.language}
                renderSigninItemCss={renderSigninItemCss}
                resolvedApplicationName={applicationName ?? "app-built-in"}
                resolvedApplicationOrganization="built-in"
              />

              <FormHeading
                description={applicationDescription || t("defaultFormDescription")}
                title={applicationName || "app-built-in"}
                visible
              />
            </div>

            {/* 预览嵌在详情页外层 Form 内：component="div" 保留表单 store 又不产生嵌套 <form> */}
            <Form form={form} component="div" layout="vertical" onFinish={() => undefined}>
              <Form.Item hidden name="application" initialValue={applicationName ?? "app-built-in"}>
                <Input disabled />
              </Form.Item>
              {!showSelectOrganization ? (
                <Form.Item hidden name="organization" initialValue="built-in">
                  <Input disabled />
                </Form.Item>
              ) : null}

              <LoginMethodSupportAlert
                currentLoginMethodSupported
                signinMethodTabsCount={signinMethodTabs.length}
              />

              {formItems.map((item, index) => (
                <Fragment key={`preview_${item.name ?? "item"}_${index}`}>
                  {renderFormItem(item, index)}
                </Fragment>
              ))}
            </Form>
          </Space>
        </LoginShellLayout>
      </div>
    </>
  );

  if (compact) return body;

  return (
    <Card
      title={ta("application.preview")}
      size="small"
      style={{ position: "sticky", top: 16 }}
      styles={{ body: { padding: 0, overflow: "hidden" } }}
      extra={
        <Space size={8}>
          <Segmented
            size="small"
            value={innerBreakpoint}
            onChange={(value) => setInnerBreakpoint(value as PreviewBreakpoint)}
            options={[
              { label: ta("application.previewDesktop"), value: "desktop" },
              { label: ta("application.previewMobile"), value: "mobile" },
            ]}
          />
          <Select
            size="small"
            style={{ width: 110 }}
            aria-label={ta("application.previewLanguage")}
            value={i18n.language}
            onChange={(value) => void i18n.changeLanguage(value as string)}
            options={[
              { label: "中文", value: "zh-CN" },
              { label: "English", value: "en" },
            ]}
          />
        </Space>
      }
    >
      {body}
    </Card>
  );
}
