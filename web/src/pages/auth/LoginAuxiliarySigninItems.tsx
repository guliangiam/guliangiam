import { ArrowLeftOutlined } from "@ant-design/icons";
import { Alert, Button, Checkbox, Form, Input, Select, Tabs, Tooltip, Typography } from "antd";
import { useTranslation } from "react-i18next";
import {
  LoginMethodKey,
  SigninMethodTab,
  isVerificationCodeMethod,
} from "@/shared/lib/auth-signin-items";
import {
  ProtocolAlerts,
  LoginMethodSupportAlert,
  SplitLayoutSide,
  FormHeading,
  ProviderOnlyIntro,
  SignedInContinuation,
  type BaseSigninItemProps,
} from "./LoginAuxiliaryLayout";
import {
  UsernameSigninItem,
  ProvidersSigninItem,
  CaptchaSigninItem,
  type CaptchaStateLike,
  type CaptchaWidgetProps,
} from "./LoginSigninItems";

// Re-export all components for backward compatibility
export {
  ProtocolAlerts,
  LoginMethodSupportAlert,
  SplitLayoutSide,
  FormHeading,
  ProviderOnlyIntro,
  SignedInContinuation,
  UsernameSigninItem,
  ProvidersSigninItem,
  CaptchaSigninItem,
};
export type { CaptchaStateLike, CaptchaWidgetProps };

// ─────────────────────────────────────────────
// Header Signin Item
// ─────────────────────────────────────────────

type HeaderSigninItemProps = BaseSigninItemProps & {
  availableLanguages: string[];
  logoAlt: string;
  logoUrl?: string | null;
  onBack: () => void;
  onChangeLanguage: (nextLanguage: string) => void;
  preferredLanguage: string;
};

export function HeaderSigninItem({
  availableLanguages,
  item,
  itemKey,
  logoAlt,
  logoUrl,
  onBack,
  onChangeLanguage,
  preferredLanguage,
  renderSigninItemCss,
}: HeaderSigninItemProps) {
  const { t } = useTranslation("auth");
  switch (item.name) {
    case "Back button":
      return (
        <div key={itemKey}>
          {renderSigninItemCss(item)}
          <Button
            type="text"
            icon={<ArrowLeftOutlined />}
            onClick={onBack}
            style={{ paddingInline: 0 }}
          >
            {item.label || t("back")}
          </Button>
        </div>
      );
    case "Languages":
      if (availableLanguages.length <= 1) {
        return null;
      }

      return (
        <div key={itemKey} style={{ display: "flex", justifyContent: "flex-end" }}>
          {renderSigninItemCss(item)}
          <Select
            value={preferredLanguage}
            size="small"
            style={{ minWidth: 132 }}
            options={availableLanguages.map((language) => ({
              label: language,
              value: language,
            }))}
            onChange={onChangeLanguage}
          />
        </div>
      );
    case "Logo":
      if (!logoUrl) {
        return null;
      }

      return (
        <div key={itemKey}>
          {renderSigninItemCss(item)}
          <img
            className="panel-logo"
            src={logoUrl}
            alt={logoAlt}
            style={{
              maxWidth: 220,
              maxHeight: 72,
              objectFit: "contain",
              marginBottom: 16,
            }}
          />
        </div>
      );
    default:
      return null;
  }
}

// ─────────────────────────────────────────────
// Signin Methods Item
// ─────────────────────────────────────────────

type SigninMethodsItemProps = BaseSigninItemProps & {
  loginMethod: LoginMethodKey;
  onChangeLoginMethod: (nextKey: LoginMethodKey) => void;
  signinMethodTabs: SigninMethodTab[];
};

export function SigninMethodsItem({
  item,
  itemKey,
  loginMethod,
  onChangeLoginMethod,
  renderSigninItemCss,
  signinMethodTabs,
}: SigninMethodsItemProps) {
  // 只有 1 个 enabled tab 才不需要切换器 — disabled 的 tab 虽然不可点击，
  // 但 antd Tabs 组件可以正确展示它们，让用户知道有哪些方式可用。
  const enabledTabs = signinMethodTabs.filter((t) => !t.disabled);
  if (enabledTabs.length <= 1) {
    return null;
  }

  return (
    <div key={itemKey}>
      {renderSigninItemCss(item)}
      <Tabs
        size="small"
        centered
        activeKey={loginMethod}
        items={signinMethodTabs.map((tab) => ({
          key: tab.key,
          label:
            tab.disabled && tab.disabledReason ? (
              <Tooltip title={tab.disabledReason}>{tab.label}</Tooltip>
            ) : (
              tab.label
            ),
          disabled: tab.disabled,
        }))}
        onChange={(activeKey) => {
          onChangeLoginMethod(activeKey as LoginMethodKey);
        }}
      />
    </div>
  );
}

// ─────────────────────────────────────────────
// Select Organization Signin Item
// ─────────────────────────────────────────────

type SelectOrganizationSigninItemProps = BaseSigninItemProps & {
  fallbackInputDisabled: boolean;
  isError: boolean;
  isLoading: boolean;
  onSelectOrganization: (value: string) => void;
  options: Array<{ label: string; value: string }>;
  errorMessage?: string;
  showSelectOrganization: boolean;
};

export function SelectOrganizationSigninItem({
  errorMessage,
  fallbackInputDisabled,
  isError,
  isLoading,
  item,
  itemKey,
  onSelectOrganization,
  options,
  showSelectOrganization,
}: SelectOrganizationSigninItemProps) {
  const { t } = useTranslation("auth");
  return (
    <div key={itemKey}>
      <Form.Item
        label={item.label || t("organization")}
        name="organization"
        rules={[{ required: true, message: t("enterOrganization") }]}
      >
        {showSelectOrganization && !isError ? (
          <Select
            showSearch
            placeholder={t("selectOrganization")}
            loading={isLoading}
            options={options}
            filterOption={(input, option) =>
              `${option?.label ?? ""}`.toLowerCase().includes(input.toLowerCase())
            }
            onChange={onSelectOrganization}
          />
        ) : (
          <Input placeholder={t("orgExample")} disabled={fallbackInputDisabled} />
        )}
      </Form.Item>
      {showSelectOrganization && isError ? (
        <Alert showIcon type="warning" title={t("orgListLoadFailed")} description={errorMessage} />
      ) : null}
    </div>
  );
}

// ─────────────────────────────────────────────
// Password Signin Item
// ─────────────────────────────────────────────

type PasswordSigninItemProps = BaseSigninItemProps & {
  loginMethod: LoginMethodKey;
};

export function PasswordSigninItem({
  item,
  itemKey,
  loginMethod,
  renderSigninItemCss,
}: PasswordSigninItemProps) {
  const { t } = useTranslation("auth");
  if (loginMethod !== "password" && loginMethod !== "ldap") {
    return null;
  }

  return (
    <div key={itemKey}>
      {renderSigninItemCss(item)}
      <Form.Item
        label={item.label || t("password")}
        name="password"
        rules={[{ required: true, message: t("enterPassword") }]}
      >
        <Input.Password autoComplete="current-password" />
      </Form.Item>
    </div>
  );
}

// ─────────────────────────────────────────────
// Verification Code Signin Item
// ─────────────────────────────────────────────

type VerificationCodeSigninItemProps = BaseSigninItemProps & {
  loginMethod: LoginMethodKey;
  sendCodePending: boolean;
  onSendCode: () => void;
};

export function VerificationCodeSigninItem({
  item,
  itemKey,
  loginMethod,
  onSendCode,
  renderSigninItemCss,
  sendCodePending,
}: VerificationCodeSigninItemProps) {
  const { t } = useTranslation("auth");
  if (!isVerificationCodeMethod(loginMethod)) {
    return null;
  }

  return (
    <div key={itemKey}>
      {renderSigninItemCss(item)}
      <Form.Item
        label={item.label || t("verificationCode")}
        name="code"
        rules={[{ required: true, message: t("enterCode") }]}
      >
        <Input
          placeholder={t("enterCode")}
          autoComplete="one-time-code"
          suffix={
            <Button type="link" size="small" loading={sendCodePending} onClick={onSendCode}>
              {t("sendCode")}
            </Button>
          }
        />
      </Form.Item>
    </div>
  );
}

// ─────────────────────────────────────────────
// Agreement Signin Item
// ─────────────────────────────────────────────

type AgreementSigninItemProps = BaseSigninItemProps & {
  agreementRequired: boolean;
  loginMethod: LoginMethodKey;
  hasTermsOfUse: boolean;
  onOpenTerms: () => void;
};

export function AgreementSigninItem({
  agreementRequired,
  hasTermsOfUse,
  item,
  itemKey,
  loginMethod,
  onOpenTerms,
  renderSigninItemCss,
}: AgreementSigninItemProps) {
  const { t } = useTranslation("auth");
  if (!agreementRequired || loginMethod === "qrcode") {
    return null;
  }

  return (
    <div key={itemKey}>
      {renderSigninItemCss(item)}
      <Form.Item
        name="agreement"
        valuePropName="checked"
        className="login-agreement"
        rules={[
          {
            validator: (_, value) => {
              if (!agreementRequired || value) {
                return Promise.resolve();
              }

              return Promise.reject(new Error(t("acceptTermsFirst")));
            },
          },
        ]}
      >
        <Checkbox>
          {t("accept")}{" "}
          {hasTermsOfUse ? (
            <Button
              type="link"
              style={{ paddingInline: 4 }}
              onClick={(event) => {
                event.preventDefault();
                onOpenTerms();
              }}
            >
              {t("termsOfUse")}
            </Button>
          ) : (
            <span>{t("termsOfUse")}</span>
          )}
        </Checkbox>
      </Form.Item>
    </div>
  );
}

// ─────────────────────────────────────────────
// Login Button Signin Item
// ─────────────────────────────────────────────

type LoginButtonSigninItemProps = BaseSigninItemProps & {
  currentLoginMethodSupported: boolean;
  disabled: boolean;
  hasSupportedSigninMethod: boolean;
  isOauthAuthorize: boolean;
  isSamlLogin: boolean;
  isSupportedOauthResponseType: boolean;
  loginMethod: LoginMethodKey;
  loading: boolean;
  samlRequest?: string;
};

export function LoginButtonSigninItem({
  currentLoginMethodSupported,
  disabled,
  hasSupportedSigninMethod,
  isOauthAuthorize,
  isSamlLogin,
  isSupportedOauthResponseType,
  item,
  itemKey,
  loginMethod,
  loading,
  renderSigninItemCss,
  samlRequest,
}: LoginButtonSigninItemProps) {
  const { t } = useTranslation("auth");
  // 扫码登录没有提交按钮：会话在选中 tab 时创建，登录在手机确认后由轮询驱动。
  // WebAuthn 同样不重复渲染提交按钮：Passkey 面板自带「使用 Passkey 登录」按钮，
  // 它会把用户名写回表单并触发同一个 onFinish（协议上下文/会话落地不分叉）。
  if (loginMethod === "qrcode" || loginMethod === "webauthn") {
    return null;
  }

  return (
    <div key={itemKey}>
      {renderSigninItemCss(item)}
      <Button
        type="primary"
        htmlType="submit"
        block
        loading={loading}
        disabled={
          loading ||
          disabled ||
          (isOauthAuthorize && !isSupportedOauthResponseType) ||
          (isSamlLogin && !samlRequest) ||
          !hasSupportedSigninMethod ||
          !currentLoginMethodSupported
        }
      >
        {item.label ||
          (loginMethod === "faceId"
            ? t("signInWithFaceId")
            : isVerificationCodeMethod(loginMethod)
              ? t("signInWithCode")
              : t("signIn"))}
      </Button>
    </div>
  );
}

// ─────────────────────────────────────────────
// Forgot Password Signin Item
// ─────────────────────────────────────────────

type ForgotPasswordSigninItemProps = BaseSigninItemProps & {
  forgetPasswordUrl: string | null;
};

export function ForgotPasswordSigninItem({
  forgetPasswordUrl,
  item,
  itemKey,
  renderSigninItemCss,
}: ForgotPasswordSigninItemProps) {
  const { t } = useTranslation("auth");
  if (!forgetPasswordUrl && item.visible !== false) {
    return null;
  }

  return (
    <div key={itemKey} className="login-forget-password" style={{ width: "100%" }}>
      {renderSigninItemCss(item)}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 12,
        }}
      >
        <Form.Item name="autoSignin" valuePropName="checked" noStyle>
          <Checkbox>{t("autoSignIn")}</Checkbox>
        </Form.Item>
        {item.visible !== false && forgetPasswordUrl ? (
          <Button type="link" href={forgetPasswordUrl} style={{ paddingInlineEnd: 0 }}>
            {item.label || t("forgotPassword.link")}
          </Button>
        ) : null}
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────
// Signup Link Signin Item
// ─────────────────────────────────────────────

type SignupLinkSigninItemProps = BaseSigninItemProps & {
  enableSignUp: boolean;
  onStoreSigninUrl: () => void;
  search: string;
  signupUrl: string | null;
};

export function SignupLinkSigninItem({
  enableSignUp,
  item,
  itemKey,
  onStoreSigninUrl,
  renderSigninItemCss,
  search,
  signupUrl,
}: SignupLinkSigninItemProps) {
  const { t } = useTranslation("auth");
  if (!enableSignUp || !signupUrl) {
    return null;
  }

  return (
    <div key={itemKey} style={{ width: "100%" }} className="login-signup-link">
      {renderSigninItemCss(item)}
      {item.label ? (
        <Button block type="link" href={`${signupUrl}${search}`} onClick={onStoreSigninUrl}>
          {item.label}
        </Button>
      ) : (
        <Typography.Paragraph style={{ marginBottom: 0, textAlign: "right" }} type="secondary">
          {t("noAccount")}
          <Button
            type="link"
            href={`${signupUrl}${search}`}
            style={{ paddingInlineEnd: 0 }}
            onClick={onStoreSigninUrl}
          >
            {t("signUpNow")}
          </Button>
        </Typography.Paragraph>
      )}
    </div>
  );
}
