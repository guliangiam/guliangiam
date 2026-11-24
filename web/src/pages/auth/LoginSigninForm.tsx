import { Fragment, type ComponentType, type ReactNode } from "react";
import { Form, Input, type FormInstance } from "antd";
import { useTranslation } from "react-i18next";
import type { CaptchaState } from "@/shared/hooks/useCaptchaFlow";
import type { QrSigninState } from "@/shared/hooks/useQrSigninFlow";
import { sanitizeHtml, sanitizeCss } from "@/shared/lib/dom-purify";
import type {
  LoginMethodKey,
  SigninItemConfig,
  SigninMethodTab,
} from "@/shared/lib/auth-signin-items";
import {
  getUsernamePlaceholder,
  isCustomSigninItem,
  isSigninItemShown,
  LOGIN_SHELL_CSS_SCOPE,
} from "@/shared/lib/auth-signin-items";
import type { ProviderSigninOption } from "@/shared/lib/auth-provider-signin";
import type { DirectAuthProvider } from "@/shared/lib/provider-auth";
import type { ApplicationItem } from "@/shared/types/domain";
import {
  AgreementSigninItem,
  CaptchaSigninItem,
  ForgotPasswordSigninItem,
  LoginButtonSigninItem,
  LoginMethodSupportAlert,
  PasswordSigninItem,
  ProvidersSigninItem,
  SelectOrganizationSigninItem,
  SigninMethodsItem,
  SignupLinkSigninItem,
  UsernameSigninItem,
  VerificationCodeSigninItem,
} from "./LoginAuxiliarySigninItems";
import GoogleOneTapVirtualButton from "./GoogleOneTapVirtualButton";

type LoginFormValues = {
  organization: string;
  application: string;
  countryCode?: string;
  username: string;
  password?: string;
  code?: string;
  captchaToken?: string;
  agreement?: boolean;
  autoSignin?: boolean;
};

export type { LoginFormValues };

type QrSigninStateLike = QrSigninState;

type CaptchaWidgetProps = {
  captchaType: string;
  subType?: string | null;
  siteKey?: string | null;
  clientSecret?: string | null;
  clientId2?: string | null;
  clientSecret2?: string | null;
  onChange: (token: string) => void;
};

// ─────────────────────────────────────────────
// Structured sub-types for LoginSigninForm props
// ─────────────────────────────────────────────

/** Auth context state — read-only data */
export type SigninFormAuthState = {
  agreementRequired: boolean;
  availableCountryCodes: string[];
  currentLoginMethodSupported: boolean;
  forgetPasswordUrl: string | null;
  googleOneTapProvider: DirectAuthProvider | null;
  hasInlineSigninForm: boolean;
  hasProviderHintSupport: boolean;
  hasSupportedSigninMethod: boolean;
  isAuthApplicationError: boolean;
  isAuthApplicationLoading: boolean;
  isOauthAuthorize: boolean;
  isSamlLogin: boolean;
  isSupportedOauthResponseType: boolean;
  providerSigninOptions: ProviderSigninOption[];
  resolvedApplication: ApplicationItem | null;
  samlRequest?: string;
  showProviders: boolean;
  showSelectOrganization: boolean;
  signinItems: SigninItemConfig[];
  signinMethodTabs: SigninMethodTab[];
  signupUrl: string | null;
  termsOfUseEnabled: boolean;
};

/** Form flow state — captcha/wechat/verification code etc. */
export type SigninFormFlowState = {
  captchaReloadSeed: number;
  captchaRequired: boolean;
  captchaState: CaptchaState | null;
  reloadCaptcha: () => Promise<void>;
  sendCodePending: boolean;
  qrSigninState: QrSigninStateLike;
  onReloadQrSignin: () => void;
  onSendCode: () => void;
};

/** Social login state and actions */
export type SigninFormProviderState = {
  canRenderGoogleOneTap: boolean;
  handleProviderClick: (provider: ProviderSigninOption) => void;
};

/** Organization selection state */
export type SigninFormOrganizationState = {
  fallbackOrganizationInputDisabled: boolean;
  onSelectOrganization: (value: string) => void;
  organizationOptions: Array<{ label: string; value: string }>;
  organizationOptionsError?: string;
  organizationOptionsIsError: boolean;
  organizationOptionsLoading: boolean;
};

/** UI state and callbacks — routing/links/submit etc. */
export type SigninFormUiState = {
  itemSigninContextKey: string;
  loadButtonLoading: boolean;
  locationSearch: string;
  loginMethod: LoginMethodKey;
  onChangeLoginMethod: (value: LoginMethodKey) => void;
  onFinish: (values: LoginFormValues) => void;
  onOpenTerms: () => void;
  onStoreSigninUrl: () => void;
};

type LoginSigninFormProps = {
  CaptchaWidget: ComponentType<CaptchaWidgetProps>;
  authState: SigninFormAuthState;
  flowState: SigninFormFlowState;
  providerState: SigninFormProviderState;
  organizationState: SigninFormOrganizationState;
  uiState: SigninFormUiState;
  form: FormInstance<LoginFormValues>;
};

export function LoginSigninForm(props: LoginSigninFormProps) {
  const { authState, flowState, providerState, organizationState, uiState, form, CaptchaWidget } =
    props;

  const {
    agreementRequired,
    availableCountryCodes,
    currentLoginMethodSupported,
    forgetPasswordUrl,
    googleOneTapProvider,
    hasInlineSigninForm,
    hasProviderHintSupport,
    hasSupportedSigninMethod,
    isAuthApplicationError,
    isAuthApplicationLoading,
    isOauthAuthorize,
    isSamlLogin,
    isSupportedOauthResponseType,
    providerSigninOptions,
    resolvedApplication,
    samlRequest,
    showProviders,
    showSelectOrganization,
    signinItems,
    signinMethodTabs,
    signupUrl,
    termsOfUseEnabled,
  } = authState;

  const {
    captchaReloadSeed,
    captchaRequired,
    captchaState,
    reloadCaptcha,
    sendCodePending,
    qrSigninState,
    onReloadQrSignin,
    onSendCode,
  } = flowState;

  const { canRenderGoogleOneTap, handleProviderClick } = providerState;

  const {
    fallbackOrganizationInputDisabled,
    onSelectOrganization,
    organizationOptions,
    organizationOptionsError,
    organizationOptionsIsError,
    organizationOptionsLoading,
  } = organizationState;

  const {
    itemSigninContextKey,
    loadButtonLoading,
    locationSearch,
    loginMethod,
    onChangeLoginMethod,
    onFinish,
    onOpenTerms,
    onStoreSigninUrl,
  } = uiState;

  const { t } = useTranslation("auth");

  const renderSigninItemCss = (item: SigninItemConfig): ReactNode => {
    if (!item.customCss?.trim() || isCustomSigninItem(item)) {
      return null;
    }

    return (
      <style
        dangerouslySetInnerHTML={{ __html: sanitizeCss(item.customCss, LOGIN_SHELL_CSS_SCOPE) }}
      />
    );
  };

  // 品牌回退失败（如新组织无默认应用、resolve 404）时 resolvedApplication
  // 为 null：登录表单必须 fail-open 渲染默认项，错误仅由上层提示条展示，
  // 不能整体吞掉表单导致用户无法登录。
  const renderOrderedSigninFormItem = (item: SigninItemConfig, index: number) => {
    if (!isSigninItemShown(item) && item.name !== "Forgot password?") {
      return null;
    }

    const itemKey = `${itemSigninContextKey}_${item.name ?? "item"}_${index}`;

    if (isCustomSigninItem(item)) {
      if (!item.customCss?.trim()) {
        return null;
      }

      return (
        <div
          key={itemKey}
          className="login-custom-signin-item"
          dangerouslySetInnerHTML={{ __html: sanitizeHtml(item.customCss ?? "") }}
        />
      );
    }

    switch (item.name) {
      case "Signin methods":
        return (
          <SigninMethodsItem
            item={item}
            itemKey={itemKey}
            loginMethod={loginMethod}
            onChangeLoginMethod={onChangeLoginMethod}
            renderSigninItemCss={renderSigninItemCss}
            signinMethodTabs={signinMethodTabs}
          />
        );
      case "Select organization":
        return (
          <SelectOrganizationSigninItem
            errorMessage={organizationOptionsError}
            fallbackInputDisabled={fallbackOrganizationInputDisabled}
            isError={organizationOptionsIsError}
            isLoading={organizationOptionsLoading}
            item={item}
            itemKey={itemKey}
            onSelectOrganization={onSelectOrganization}
            options={organizationOptions}
            showSelectOrganization={showSelectOrganization}
            renderSigninItemCss={renderSigninItemCss}
          />
        );
      case "Username":
        return (
          <UsernameSigninItem
            availableCountryCodes={availableCountryCodes}
            form={form}
            item={item}
            itemKey={itemKey}
            loginMethod={loginMethod}
            onReloadQrSignin={onReloadQrSignin}
            qrSigninState={qrSigninState}
            renderSigninItemCss={renderSigninItemCss}
            submitting={loadButtonLoading}
            usernamePlaceholder={getUsernamePlaceholder(loginMethod)}
          />
        );
      case "Password":
        return (
          <PasswordSigninItem
            item={item}
            itemKey={itemKey}
            loginMethod={loginMethod}
            renderSigninItemCss={renderSigninItemCss}
          />
        );
      case "Verification code":
        return (
          <VerificationCodeSigninItem
            item={item}
            itemKey={itemKey}
            loginMethod={loginMethod}
            onSendCode={onSendCode}
            renderSigninItemCss={renderSigninItemCss}
            sendCodePending={sendCodePending}
          />
        );
      case "Agreement":
        return (
          <AgreementSigninItem
            agreementRequired={agreementRequired}
            hasTermsOfUse={termsOfUseEnabled}
            item={item}
            itemKey={itemKey}
            loginMethod={loginMethod}
            onOpenTerms={onOpenTerms}
            renderSigninItemCss={renderSigninItemCss}
          />
        );
      case "Captcha":
        return (
          <CaptchaSigninItem
            CaptchaWidget={CaptchaWidget}
            captchaReloadSeed={captchaReloadSeed}
            captchaRequired={captchaRequired}
            captchaState={captchaState}
            form={form}
            item={item}
            itemKey={itemKey}
            loginMethod={loginMethod}
            reloadCaptcha={reloadCaptcha}
            renderSigninItemCss={renderSigninItemCss}
          />
        );
      case "Login button":
        return (
          <LoginButtonSigninItem
            currentLoginMethodSupported={currentLoginMethodSupported}
            disabled={isAuthApplicationLoading || isAuthApplicationError}
            hasSupportedSigninMethod={hasSupportedSigninMethod}
            isOauthAuthorize={isOauthAuthorize}
            isSamlLogin={isSamlLogin}
            isSupportedOauthResponseType={isSupportedOauthResponseType}
            item={item}
            itemKey={itemKey}
            loading={loadButtonLoading}
            loginMethod={loginMethod}
            renderSigninItemCss={renderSigninItemCss}
            samlRequest={samlRequest}
          />
        );
      case "Forgot password?":
        return (
          <ForgotPasswordSigninItem
            forgetPasswordUrl={forgetPasswordUrl}
            item={item}
            itemKey={itemKey}
            renderSigninItemCss={renderSigninItemCss}
          />
        );
      case "Signup link":
        return (
          <SignupLinkSigninItem
            enableSignUp={resolvedApplication?.enableSignUp !== false}
            item={item}
            itemKey={itemKey}
            onStoreSigninUrl={onStoreSigninUrl}
            renderSigninItemCss={renderSigninItemCss}
            search={locationSearch}
            signupUrl={signupUrl}
          />
        );
      case "Providers": {
        const providerDisplayMode =
          item.rule === "small" || item.rule === "big"
            ? item.rule
            : hasInlineSigninForm
              ? "small"
              : "big";

        return (
          <>
            <ProvidersSigninItem
              displayMode={providerDisplayMode}
              hasProviderHintSupport={hasProviderHintSupport}
              item={item}
              itemKey={itemKey}
              onProviderClick={handleProviderClick}
              providers={providerSigninOptions}
              renderSigninItemCss={renderSigninItemCss}
              showProviders={showProviders}
            />
            {canRenderGoogleOneTap && resolvedApplication ? (
              <GoogleOneTapVirtualButton
                application={resolvedApplication}
                provider={googleOneTapProvider!}
              />
            ) : null}
          </>
        );
      }
      default:
        return null;
    }
  };

  return (
    <Form form={form} layout="vertical" onFinish={onFinish}>
      <Form.Item
        hidden
        label={t("application")}
        name="application"
        rules={[{ required: true, message: t("enterApplication") }]}
      >
        <Input disabled />
      </Form.Item>

      {!showSelectOrganization ? (
        <Form.Item
          hidden
          label={t("organization")}
          name="organization"
          rules={[{ required: true, message: t("enterOrganization") }]}
        >
          <Input disabled />
        </Form.Item>
      ) : null}

      <LoginMethodSupportAlert
        currentLoginMethodSupported={currentLoginMethodSupported}
        signinMethodTabsCount={signinMethodTabs.length}
      />

      {signinItems.map((item, index) => (
        <Fragment key={`${itemSigninContextKey}_${item.name ?? "item"}_${index}`}>
          {renderOrderedSigninFormItem(item, index)}
        </Fragment>
      ))}
    </Form>
  );
}
