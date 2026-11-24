import {
  buildProviderSigninOptions,
  type ProviderSigninOption,
} from "@/shared/lib/auth-provider-signin";
import type { OAuthQueryContext } from "@/shared/lib/auth-oauth-context";
import {
  DEFAULT_APPLICATION_NAME,
  DEFAULT_ORGANIZATION_NAME,
  getForgetPasswordUrl,
  getFormPositionStyles,
  getIsSplitLayout,
  getOuterBackground,
  getSigninItems,
  getSigninMethodTabs,
  getSignupUrl,
  isSigninItemShown,
  type LoginMethodKey,
  type SigninCapabilities,
  type SigninMethodTab,
} from "@/shared/lib/auth-signin-items";
import { getFaceIdProvider } from "@/shared/lib/auth-signin-providers";
import { useBrowserCapabilities } from "./useBrowserCapabilities";
import type { ApplicationItem } from "@/shared/types/domain";

type SessionAccountLike = {
  owner?: string | null;
};

type ApplicationProviderListItem = NonNullable<ApplicationItem["providers"]>[number];
type ApplicationProviderItem = Extract<ApplicationProviderListItem, { provider?: unknown }>;

function isProviderVisible(providerItem: ApplicationProviderItem) {
  if (providerItem.canSignIn === false) {
    return false;
  }

  const provider = providerItem.provider;
  if (!provider?.category) {
    return false;
  }

  if (!["OAuth", "SAML", "Web3"].includes(provider.category)) {
    return false;
  }

  if (provider.type === "WeChatMiniProgram") {
    return false;
  }

  return true;
}

function isProviderConfigItem(
  providerItem: ApplicationProviderListItem,
): providerItem is ApplicationProviderItem {
  return typeof providerItem === "object" && providerItem !== null && "provider" in providerItem;
}

function getApplicationExtraString(application: ApplicationItem | null, key: string) {
  if (!application) {
    return "";
  }

  const value = (application as Record<string, unknown>)[key];
  return typeof value === "string" ? value : "";
}

function isSharedApplication(application: ApplicationItem | null) {
  if (!application) {
    return false;
  }

  return (application as Record<string, unknown>).isShared === true;
}

function getSignupItem(application: ApplicationItem | null, name: string) {
  return application?.signupItems?.find((item) => item.name === name) ?? null;
}

function isAgreementRequired(application: ApplicationItem | null) {
  const agreementItem = getSignupItem(application, "Agreement");
  if (!agreementItem?.rule || agreementItem.rule === "None") {
    return false;
  }

  return agreementItem.required === true;
}

function getAgreementDefaultValue(application: ApplicationItem | null) {
  const agreementItem = getSignupItem(application, "Agreement");
  return isAgreementRequired(application) && agreementItem?.rule === "Signin (Default True)";
}

function getAutoSigninDefaultValue(application: ApplicationItem | null) {
  const forgotPasswordItem = application?.signinItems?.find(
    (item) => item.name === "Forgot password?",
  );

  return forgotPasswordItem?.rule !== "Auto sign in - False";
}

function isApplicationAutoSigninEnabled(application: ApplicationItem | null) {
  if (!application) {
    return false;
  }

  return (application as Record<string, unknown>).enableAutoSignin === true;
}

type UseLoginPageViewModelValues = {
  authApplication: ApplicationItem | null | undefined;
  brandedApplication: ApplicationItem | null | undefined;
  pathname: string;
  search: string;
  oauthContext: OAuthQueryContext | null;
  providerHint: string | null;
  loginMethod: LoginMethodKey;
  watchedOrganization: string;
  watchedApplication: string;
  sessionAccount: SessionAccountLike | null;
};

export function useLoginPageViewModel(values: UseLoginPageViewModelValues) {
  const {
    authApplication,
    brandedApplication,
    loginMethod,
    oauthContext,
    pathname,
    providerHint,
    search,
    sessionAccount,
    watchedApplication,
    watchedOrganization,
  } = values;
  const resolvedApplication = authApplication ?? brandedApplication ?? null;
  const customSigninHtml = getApplicationExtraString(resolvedApplication, "signinHtml");
  const outerBackground = getOuterBackground(resolvedApplication?.formBackgroundUrl);
  const isSplitLayout = getIsSplitLayout(resolvedApplication?.formOffset);
  const formPositionStyles = getFormPositionStyles(resolvedApplication?.formOffset);
  const forgetPasswordUrl = resolvedApplication
    ? // 忘记密码页会把 application 参数当作 applicationId 交给验证码发送接口，
      // 用于优先命中该应用绑定的短信/邮件提供商；因此这里优先传应用 UUID，
      // 只有缺失时才退回名称（后端会把非 UUID 视为未绑定并回退组织级提供商）。
      getForgetPasswordUrl(
        resolvedApplication.id ?? resolvedApplication.name,
        resolvedApplication.forgetUrl,
      )
    : null;
  const signupUrl = resolvedApplication ? getSignupUrl(resolvedApplication, pathname) : null;
  const signinItems = getSigninItems(resolvedApplication);
  const availableCountryCodes =
    resolvedApplication?.organizationObj?.countryCodes?.filter(Boolean) ?? [];
  const defaultCountryCode = availableCountryCodes[0] ?? undefined;
  const headerSigninItems = signinItems.filter(
    (item) => item.name === "Logo" || item.name === "Back button" || item.name === "Languages",
  );
  const selectOrganizationItem =
    signinItems.find((item) => item.name === "Select organization") ?? null;
  const availableLanguages = resolvedApplication?.organizationObj?.languages ?? [];
  const providersItem = signinItems.find((item) => item.name === "Providers") ?? null;
  const visibleProviders = (resolvedApplication?.providers ?? [])
    .filter(isProviderConfigItem)
    .filter(isProviderVisible);
  const googleOneTapProviderItem =
    visibleProviders.find(
      (providerItem) => providerItem.rule === "OneTap" && providerItem.provider?.type === "Google",
    ) ?? null;
  const googleOneTapProvider = googleOneTapProviderItem?.provider ?? null;
  const visibleDirectProviders = visibleProviders.filter(
    (providerItem) => providerItem.provider?.category !== "SAML",
  );
  const providerSigninOptions: ProviderSigninOption[] = buildProviderSigninOptions({
    providerItems: visibleProviders,
    pathname,
    currentHref: typeof window === "undefined" ? `${pathname}${search}` : window.location.href,
    origin: typeof window === "undefined" ? "" : window.location.origin,
    hash: typeof window === "undefined" ? "" : window.location.hash,
    search,
    oauthContext,
    applicationClientId: resolvedApplication?.clientId,
    applicationName: resolvedApplication?.name,
  });
  const hasFaceIdProvider = Boolean(getFaceIdProvider(resolvedApplication));
  const browserCaps = useBrowserCapabilities();
  const signinCapabilities: SigninCapabilities = {
    hasFaceIdProvider,
    supportsWebAuthn: browserCaps.supportsWebAuthn && browserCaps.isSecureContext,
    isSecureContext: browserCaps.isSecureContext,
    hasMediaDevices: browserCaps.hasMediaDevices,
    supportsPlatformAuth: browserCaps.supportsPlatformAuth,
  };
  // 微信扫码 tab 已随 legacy 端点一并移除：微信登录保留在 provider 列表里
  // （走 getDirectOAuthProviderAuthUrl 的授权跳转），扫码登录改用自建 /qrcode 契约。
  const signinMethodTabs: SigninMethodTab[] = getSigninMethodTabs(
    resolvedApplication,
    signinCapabilities,
  );
  const enabledSigninMethodTabs = signinMethodTabs.filter((item) => !item.disabled);
  const currentLoginMethodSupported = signinMethodTabs.some(
    (item) => item.key === loginMethod && !item.disabled,
  );
  const hasInlineSigninForm = signinMethodTabs.some(
    (item) =>
      (item.key === "password" ||
        item.key === "ldap" ||
        item.key === "verificationCode" ||
        item.key === "webauthn") &&
      !item.disabled,
  );
  const hasInlineSigninMethod = hasInlineSigninForm;
  const hasSupportedSigninMethod = enabledSigninMethodTabs.length > 0;
  const hasProviderHintSupport = Boolean(oauthContext || resolvedApplication?.clientId);
  const showProviders = isSigninItemShown(providersItem) && visibleProviders.length > 0;
  const shouldAutoRedirectSingleProvider =
    !providerHint && !hasInlineSigninMethod && visibleDirectProviders.length === 1;
  const agreementRequired = isAgreementRequired(resolvedApplication);
  const agreementDefaultValue = getAgreementDefaultValue(resolvedApplication);
  const showSelectOrganization = isSigninItemShown(selectOrganizationItem);
  const autoSigninEnabled = isApplicationAutoSigninEnabled(resolvedApplication);
  const autoSigninDefaultValue = getAutoSigninDefaultValue(resolvedApplication);
  const autoSigninOrganization = resolvedApplication?.organization || watchedOrganization;
  const canAutoSigninWithCurrentSession =
    Boolean(sessionAccount) &&
    Boolean(autoSigninOrganization) &&
    sessionAccount?.owner === autoSigninOrganization;

  return {
    agreementDefaultValue,
    agreementRequired,
    autoSigninDefaultValue,
    autoSigninEnabled,
    autoSigninOrganization,
    availableCountryCodes,
    availableLanguages,
    canAutoSigninWithCurrentSession,
    currentLoginMethodSupported,
    customSigninHtml,
    defaultCountryCode,
    enabledSigninMethodTabs,
    forgetPasswordUrl,
    formPositionStyles,
    googleOneTapProvider,
    hasFaceIdProvider,
    hasInlineSigninMethod,
    hasProviderHintSupport,
    hasSupportedSigninMethod,
    headerSigninItems,
    isSharedApplication: isSharedApplication(resolvedApplication),
    isSplitLayout,
    outerBackground,
    providerSigninOptions,
    resolvedApplication,
    showProviders,
    showSelectOrganization,
    shouldAutoRedirectSingleProvider,
    signinItems,
    signinMethodTabs,
    signupUrl,
    visibleDirectProviders,
    watchedApplicationFallback: watchedApplication || DEFAULT_APPLICATION_NAME,
    watchedOrganizationFallback: watchedOrganization || DEFAULT_ORGANIZATION_NAME,
  };
}
