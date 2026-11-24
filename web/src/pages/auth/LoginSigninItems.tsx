import type { ComponentType } from "react";
import { Alert, Button, Form, Input, Space, Skeleton, Typography } from "antd";
import { useTranslation } from "react-i18next";
import type { LoginMethodKey } from "@/shared/lib/auth-signin-items";
import type { ProviderSigninOption } from "@/shared/lib/auth-provider-signin";
import { CountryCodeSelect } from "@/shared/ui/CountryCodeSelect";
import { GoCaptchaInteraction } from "@/shared/ui/GoCaptchaInteraction";
import { ProviderIcon } from "@/shared/ui/ProviderIcon";
import { socialAuthorize } from "@/shared/api/iam/auth";
import { isSafeRedirectUrl } from "@/shared/lib/url";
import type { QrSigninState } from "@/shared/hooks/useQrSigninFlow";
import type { BaseSigninItemProps } from "./LoginAuxiliaryLayout";
import QrSigninCard from "./LoginQrcodeCard";
import { Web3LoginPanel } from "./Web3LoginPanel";
import { useLoginPageRouteContext } from "./useLoginPageRouteContext";

// ─────────────────────────────────────────────
// Username Signin Item
// ─────────────────────────────────────────────

type UsernameSigninItemProps = BaseSigninItemProps & {
  availableCountryCodes: string[];
  form?: FormInstance;
  loginMethod: LoginMethodKey;
  onReloadQrSignin: () => void;
  qrSigninState: QrSigninState;
  submitting?: boolean;
  usernamePlaceholder: string;
};

function isValidPhoneNumber(value: string, countryCode = "") {
  const phoneCnRegex = /^1(3\d|4[5-9]|5[0-35-9]|6[2567]|7[0-8]|8\d|9[0-35-9])\d{8}$/;
  const phoneRegex = /^[0-9]{4,15}$/;

  return countryCode === "CN" ? phoneCnRegex.test(value) : phoneRegex.test(value);
}

export function UsernameSigninItem({
  availableCountryCodes,
  form,
  item,
  itemKey,
  loginMethod,
  onReloadQrSignin,
  qrSigninState,
  renderSigninItemCss,
  submitting,
  usernamePlaceholder,
}: UsernameSigninItemProps) {
  const { t } = useTranslation("auth");
  // Hooks 必须在顶层无条件调用 — React 规则。webauthn case 后续用这个值。
  const ctx = useLoginPageRouteContext();
  const organizationSlug = ctx?.loginDefaults?.organization ?? "built-in";

  if (loginMethod === "qrcode") {
    return (
      <div key={itemKey}>
        {renderSigninItemCss(item)}
        {/* key 绑定二维码载荷：刷新/换会话即重新挂载，倒计时随之重置 */}
        <QrSigninCard
          key={qrSigninState.code ?? "qr-loading"}
          state={qrSigninState}
          onRefresh={onReloadQrSignin}
        />
      </div>
    );
  }

  if (loginMethod === "webauthn") {
    return (
      <div key={itemKey}>
        {renderSigninItemCss(item)}
        {/* 面板只负责用户名输入；真正的提交由表单 onFinish 的 webauthn 分支完成，
            这样协议上下文（responseType/applicationId）与会话落地只有一条链路。 */}
        <Form.Item name="username" hidden rules={[{ required: true, message: t("enterUsername") }]}>
          <Input />
        </Form.Item>
        <Web3LoginPanel
          organizationSlug={organizationSlug}
          initialMode="login"
          hideRegister
          submitting={submitting}
          onSubmit={
            form
              ? (username) => {
                  form.setFieldValue("username", username);
                  void form
                    .validateFields()
                    .then(() => form.submit())
                    .catch(() => undefined);
                }
              : undefined
          }
          onAuthenticated={() => {
            const returnTo = ctx?.returnTo;
            window.location.assign(
              typeof returnTo === "string" && isSafeRedirectUrl(returnTo) ? returnTo : "/",
            );
          }}
        />
      </div>
    );
  }

  if (loginMethod === "verificationCodePhone") {
    return (
      <div key={itemKey}>
        {renderSigninItemCss(item)}
        <Form.Item required style={{ marginBottom: 24 }}>
          <Input.Group compact>
            <Form.Item
              name="countryCode"
              noStyle
              rules={[{ required: true, message: t("selectCountryCode") }]}
            >
              <CountryCodeSelect
                ariaLabel={t("countryCode")}
                placeholder={t("countryCode")}
                style={{ width: "34%" }}
                // Values are ISO alpha-2; when the public application projection
                // has no tenant subset, fall back to the full phone-capable list.
                countryCodes={availableCountryCodes.length > 0 ? availableCountryCodes : undefined}
              />
            </Form.Item>
            <Form.Item
              name="username"
              noStyle
              dependencies={["countryCode"]}
              rules={[
                { required: true, message: t("enterPhone") },
                ({ getFieldValue }) => ({
                  validator: async (_, value: string | undefined) => {
                    if (!value) {
                      return;
                    }

                    if (!isValidPhoneNumber(value, getFieldValue("countryCode") ?? "")) {
                      throw new Error(t("enterValidPhone"));
                    }
                  },
                }),
              ]}
            >
              <Input
                autoComplete="tel"
                placeholder={usernamePlaceholder}
                style={{ width: "66%" }}
              />
            </Form.Item>
          </Input.Group>
        </Form.Item>
      </div>
    );
  }

  return (
    <div key={itemKey}>
      {renderSigninItemCss(item)}
      <Form.Item
        label={item.label || t("account")}
        name="username"
        rules={[{ required: true, message: t("enterUsername") }]}
      >
        <Input autoComplete="username" placeholder={usernamePlaceholder} />
      </Form.Item>
    </div>
  );
}

// ─────────────────────────────────────────────
// Providers Signin Item
// ─────────────────────────────────────────────

type ProvidersSigninItemProps = BaseSigninItemProps & {
  displayMode: "big" | "small";
  hasProviderHintSupport: boolean;
  onProviderClick: (provider: ProviderSigninOption) => void;
  providers: ProviderSigninOption[];
  showProviders: boolean;
};

export function ProvidersSigninItem({
  displayMode,
  hasProviderHintSupport,
  item,
  itemKey,
  onProviderClick,
  providers,
  renderSigninItemCss,
  showProviders,
}: ProvidersSigninItemProps) {
  const { t } = useTranslation("auth");
  if (!showProviders) {
    return null;
  }

  const hasDirectSigninProvider = providers.some(
    (provider) => provider.canStartDirectly || Boolean(provider.url),
  );

  async function handleProviderClick(provider: ProviderSigninOption) {
    // PKCE providers must go through the browser auth-URL builder: it creates
    // the verifier/challenge pair and stores the verifier for the callback.
    // The server-side authorize endpoint cannot persist that verifier yet.
    if (provider.category === "OAuth" && provider.name && !provider.enablePkce) {
      try {
        const result = await socialAuthorize({
          providerId: provider.name,
        });
        if (isSafeRedirectUrl(result.redirectUrl)) {
          window.location.assign(result.redirectUrl);
        } else {
          window.location.assign("/");
        }
        return;
      } catch {
        // Fallback to the original handler if socialAuthorize fails
      }
    }
    onProviderClick(provider);
  }

  return (
    <div key={itemKey}>
      <div style={{ marginBlock: 8, textAlign: "center", color: "var(--gl-color-text-secondary)" }}>
        {t("orUseThirdParty")}
      </div>
      {renderSigninItemCss(item)}
      {!hasProviderHintSupport && !hasDirectSigninProvider ? (
        <Alert showIcon type="warning" title={t("missingClientId")} />
      ) : (
        <div
          style={
            displayMode === "small"
              ? {
                  display: "grid",
                  gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))",
                  gap: 12,
                  width: "100%",
                }
              : {
                  display: "flex",
                  flexDirection: "column",
                  gap: 12,
                  width: "100%",
                }
          }
        >
          {providers.map((provider) => (
            <Button
              key={provider.name}
              block={displayMode === "big"}
              size={displayMode === "big" ? "large" : "middle"}
              type={displayMode === "big" ? "default" : "text"}
              onClick={() => handleProviderClick(provider)}
              disabled={!provider.canStartDirectly && !provider.url}
              icon={
                provider.customLogo ? (
                  <img
                    src={provider.customLogo}
                    alt=""
                    width={displayMode === "big" ? 22 : 18}
                    height={displayMode === "big" ? 22 : 18}
                    style={{ objectFit: "contain" }}
                  />
                ) : (
                  <ProviderIcon
                    type={provider.type || provider.name || ""}
                    size={displayMode === "big" ? 22 : 18}
                  />
                )
              }
              style={
                displayMode === "big"
                  ? {
                      minHeight: 52,
                      justifyContent: "center",
                      borderRadius: 14,
                    }
                  : {
                      justifyContent: "center",
                      borderRadius: 12,
                      border: "1px solid var(--gl-color-border)",
                      background: "var(--gl-color-bg)",
                    }
              }
            >
              {provider.displayName || provider.name}
            </Button>
          ))}
        </div>
      )}
    </div>
  );
}

// ─────────────────────────────────────────────
// Captcha Signin Item
// ─────────────────────────────────────────────

type CaptchaStateLike = {
  type?: string | null;
  subType?: string | null;
  clientId?: string | null;
  clientId2?: string | null;
  captchaConfig?: string | null;
  captchaImage?: string | null;
  shape?: string | null;
  captchaImageSub?: string | null;
};

type CaptchaWidgetProps = {
  captchaType: string;
  subType?: string | null;
  siteKey?: string | null;
  clientId2?: string | null;
  captchaConfig?: string | null;
  onChange: (token: string) => void;
};

export type { CaptchaStateLike, CaptchaWidgetProps };

type CaptchaSigninItemProps = BaseSigninItemProps & {
  captchaRequired: boolean;
  captchaState: CaptchaStateLike | null;
  captchaReloadSeed: number;
  form: FormInstance;
  loginMethod: LoginMethodKey;
  reloadCaptcha: () => Promise<void>;
  CaptchaWidget: ComponentType<CaptchaWidgetProps>;
};

import { Suspense } from "react";
import type { FormInstance } from "antd";

export function CaptchaSigninItem({
  CaptchaWidget,
  captchaReloadSeed,
  captchaRequired,
  captchaState,
  form,
  item,
  itemKey,
  loginMethod,
  reloadCaptcha,
  renderSigninItemCss,
}: CaptchaSigninItemProps) {
  const { t } = useTranslation("auth");
  if (!captchaRequired || loginMethod === "qrcode") {
    return null;
  }

  if (captchaState?.type === "default") {
    return (
      <div key={itemKey}>
        {renderSigninItemCss(item)}
        <Form.Item
          label={item.label || t("captcha")}
          name="captchaToken"
          rules={[{ required: true, message: t("enterCaptcha") }]}
        >
          <Input
            placeholder={t("enterCaptcha")}
            suffix={
              captchaState?.captchaImage ? (
                <button
                  type="button"
                  style={{ border: 0, background: "transparent", padding: 0 }}
                  onClick={() => {
                    void reloadCaptcha();
                  }}
                >
                  <img
                    src={`data:image/png;base64,${captchaState.captchaImage}`}
                    alt="captcha"
                    style={{ height: 30, cursor: "pointer", display: "block" }}
                  />
                </button>
              ) : null
            }
          />
        </Form.Item>
      </div>
    );
  }

  if (captchaState?.type === "go_captcha") {
    return (
      <div key={itemKey}>
        {renderSigninItemCss(item)}
        <Form.Item
          label={item.label || t("captcha")}
          name="captchaToken"
          rules={[{ required: true, message: t("enterCaptcha") }]}
        >
          <GoCaptchaInteraction
            key={`${captchaState.shape ?? "click"}-${captchaReloadSeed}`}
            shape={captchaState.shape}
            masterImage={captchaState.captchaImage}
            subImage={captchaState.captchaImageSub}
          />
        </Form.Item>
        <Button
          size="small"
          type="link"
          style={{ paddingInline: 0 }}
          onClick={() => void reloadCaptcha()}
        >
          {t("reloadCaptcha")}
        </Button>
      </div>
    );
  }

  return (
    <div key={itemKey}>
      {renderSigninItemCss(item)}
      <Form.Item
        label={item.label || t("humanCaptcha")}
        required
        validateStatus={!form.getFieldValue("captchaToken") ? "warning" : undefined}
        help={
          form.getFieldValue("captchaToken") ? t("captchaCompleted") : t("completeCaptchaFirst")
        }
      >
        <Space orientation="vertical" style={{ width: "100%" }} size={12}>
          <Suspense fallback={<Skeleton active paragraph={{ rows: 2 }} />}>
            <CaptchaWidget
              key={`${captchaState?.type ?? "none"}-${captchaReloadSeed}`}
              captchaType={captchaState?.type ?? "none"}
              subType={captchaState?.subType}
              siteKey={captchaState?.clientId}
              clientId2={captchaState?.clientId2}
              captchaConfig={captchaState?.captchaConfig}
              onChange={(token) => {
                form.setFieldValue("captchaToken", token);
              }}
            />
          </Suspense>
          <Space>
            <Button
              type="default"
              onClick={() => {
                void reloadCaptcha();
              }}
            >
              {t("reloadCaptcha")}
            </Button>
            {form.getFieldValue("captchaToken") ? (
              <Typography.Text type="secondary">{t("captchaTokenObtained")}</Typography.Text>
            ) : null}
          </Space>
        </Space>
      </Form.Item>
    </div>
  );
}
