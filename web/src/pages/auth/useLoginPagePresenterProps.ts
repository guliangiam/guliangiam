import { useState, type ComponentProps } from "react";
import { useTranslation } from "react-i18next";
import type { FormInstance } from "antd";
import type { FaceRecognitionMode } from "@/shared/hooks/useFaceRecognitionFlow";
import type { LoginMethodKey } from "@/shared/lib/auth-signin-items";
import { LoginOverlaySection } from "./LoginOverlaySection";
import { LoginPanelSection } from "./LoginPanelSection";
import { LoginSigninContent } from "./LoginSigninContent";
import type { LoginPageAuthActionsState, LoginPageFormValues } from "./useLoginPageAuthActions";
import type { LoginPageAuthContextState } from "./useLoginPageAuthContext";
import type { LoginPageFormFlowsState } from "./useLoginPageFormFlows";
import type { LoginPageProviderActionsState } from "./useLoginPageProviderActions";

type SessionAccountLike = ComponentProps<typeof LoginPanelSection>["sessionAccount"];

type UseLoginPagePresenterPropsValues = {
  authContext: LoginPageAuthContextState;
  authActions: LoginPageAuthActionsState;
  form: FormInstance<LoginPageFormValues>;
  formFlows: LoginPageFormFlowsState;
  providerActions: LoginPageProviderActionsState;
  shellActions: {
    changeLanguage: (nextLanguage: string) => void;
    changeLoginMethod: (value: LoginMethodKey) => void;
    preferredLanguage: string;
    storeSigninUrl: () => void;
  };
  faceRecognition: {
    faceRecognitionMode: FaceRecognitionMode | null;
    onFaceRecognitionCancel: () => void;
    onFaceRecognitionImagesOk: (faceIdImage: string[]) => void;
    onFaceRecognitionVectorOk: (faceId: number[]) => void;
  };
  locationSearch: string;
  loginMethod: LoginMethodKey;
  organizationName?: string;
  sessionAccount: SessionAccountLike;
};

function getSessionAccountLabel(account: SessionAccountLike) {
  if (!account) {
    return "";
  }

  if (account.displayName && account.displayName !== account.name) {
    return `${account.name} (${account.displayName})`;
  }

  return account.displayName || account.name;
}

export function useLoginPagePresenterProps(values: UseLoginPagePresenterPropsValues) {
  const {
    authActions,
    authContext,
    faceRecognition,
    form,
    formFlows,
    locationSearch,
    loginMethod,
    organizationName,
    providerActions,
    sessionAccount,
    shellActions,
  } = values;
  const {
    agreementRequired,
    authApplicationQuery,
    availableCountryCodes,
    availableLanguages,
    brandedApplicationQuery,
    casContext,
    currentLoginMethodSupported,
    forgetPasswordUrl,
    googleOneTapProvider,
    hasInlineSigninMethod,
    hasProviderHintSupport,
    hasSupportedSigninMethod,
    headerSigninItems,
    isSplitLayout,
    isCasLogin,
    isOauthAuthorize,
    isSamlLogin,
    isSupportedOauthResponseType,
    loginDefaults,
    mfaSetupCompleted,
    oauthContext,
    organizationOptionsQuery,
    providerSigninOptions,
    resolvedApplication,
    samlContext,
    showProviders,
    showSelectOrganization,
    signinItems,
    signinMethodTabs,
    signupUrl,
    watchedOrganization,
  } = authContext;
  const {
    clearPendingMfa,
    loadButtonLoading,
    onContinue,
    onUseOther,
    pendingMfa,
    qrSigninState,
    reloadQrSignin,
    sendCodePending,
    sendVerificationCode,
    shouldShowSignedInContinuation,
    submitLogin,
  } = authActions;
  const { captchaReloadSeed, captchaRequired, captchaState, reloadCaptcha } = formFlows;
  const { canRenderGoogleOneTap, handleProviderClick, switchLoginOrganization } = providerActions;
  const { changeLanguage, changeLoginMethod, preferredLanguage, storeSigninUrl } = shellActions;
  const {
    faceRecognitionMode,
    onFaceRecognitionCancel,
    onFaceRecognitionImagesOk,
    onFaceRecognitionVectorOk,
  } = faceRecognition;
  const [termsOpen, setTermsOpen] = useState(false);
  const { t } = useTranslation();
  const { t: ta } = useTranslation("auth");

  const shouldShowProviderOnlyIntro =
    !pendingMfa && !shouldShowSignedInContinuation && !hasInlineSigninMethod && showProviders;
  const shouldShowFormHeading =
    !isSplitLayout &&
    !pendingMfa &&
    !shouldShowSignedInContinuation &&
    !shouldShowProviderOnlyIntro;

  const signinContentProps: ComponentProps<typeof LoginSigninContent> = {
    protocolAlertsProps: {
      authApplicationError: authApplicationQuery.isError
        ? t("login.oauthContextLoadFailed")
        : undefined,
      authApplicationName: authApplicationQuery.data?.name || "",
      brandedApplicationError: brandedApplicationQuery.isError
        ? ta("appBrandLoadFailed")
        : undefined,
      isBrandedApplicationError: brandedApplicationQuery.isError,
      isCasLogin,
      isOauthAuthorize,
      isSamlLogin,
      isSupportedOauthResponseType,
      loginDefaultsApplication: loginDefaults.application,
      mfaSetupCompleted,
      pendingMfa: Boolean(pendingMfa),
      samlApplicationName: samlContext?.applicationName,
      samlRequest: samlContext?.samlRequest,
      service: casContext?.service,
      unsupportedOauthResponseType: oauthContext?.responseType,
    },
    loginSigninFormProps: {
      authState: {
        agreementRequired,
        availableCountryCodes,
        currentLoginMethodSupported,
        forgetPasswordUrl,
        googleOneTapProvider,
        hasInlineSigninForm: hasInlineSigninMethod,
        hasProviderHintSupport,
        hasSupportedSigninMethod,
        isAuthApplicationError: authApplicationQuery.isError,
        isAuthApplicationLoading: Boolean(authApplicationQuery.isLoading),
        isOauthAuthorize,
        isSamlLogin,
        isSupportedOauthResponseType,
        providerSigninOptions,
        resolvedApplication,
        samlRequest: samlContext?.samlRequest,
        showProviders,
        showSelectOrganization,
        signinItems,
        signinMethodTabs,
        signupUrl,
        termsOfUseEnabled: Boolean(resolvedApplication?.termsOfUse),
      },
      flowState: {
        captchaReloadSeed,
        captchaRequired,
        captchaState,
        reloadCaptcha,
        sendCodePending,
        qrSigninState,
        onReloadQrSignin: () => {
          reloadQrSignin();
        },
        onSendCode: sendVerificationCode,
      },
      providerState: {
        canRenderGoogleOneTap,
        handleProviderClick: (provider) => {
          void handleProviderClick(provider);
        },
      },
      organizationState: {
        fallbackOrganizationInputDisabled:
          Boolean(organizationName) || isOauthAuthorize || isCasLogin || isSamlLogin,
        onSelectOrganization: switchLoginOrganization,
        organizationOptions: (organizationOptionsQuery.data ?? []).map((organization) => ({
          label: organization.displayName || organization.slug,
          value: organization.slug,
        })),
        organizationOptionsError: organizationOptionsQuery.isError
          ? ta("orgListLoadFailed")
          : undefined,
        organizationOptionsIsError: organizationOptionsQuery.isError,
        organizationOptionsLoading: Boolean(organizationOptionsQuery.isLoading),
      },
      uiState: {
        itemSigninContextKey: `${resolvedApplication?.organization ?? watchedOrganization}_${resolvedApplication?.name ?? "app"}`,
        loadButtonLoading,
        locationSearch,
        loginMethod,
        onChangeLoginMethod: changeLoginMethod,
        onFinish: submitLogin,
        onOpenTerms: () => {
          setTermsOpen(true);
        },
        onStoreSigninUrl: storeSigninUrl,
      },
      form,
    },
  };

  const panelSectionProps: Omit<ComponentProps<typeof LoginPanelSection>, "signinContent"> = {
    application: resolvedApplication,
    availableLanguages,
    headerSigninItems,
    preferredLanguage,
    onChangeLanguage: changeLanguage,
    shouldShowFormHeading,
    shouldShowProviderOnlyIntro,
    shouldShowSignedInContinuation,
    sessionAccount,
    sessionAccountLabel: getSessionAccountLabel(sessionAccount),
    onContinue,
    onUseOther,
    pendingMfa,
    onBackPendingMfa: clearPendingMfa,
    onVerifyPendingMfa: authActions.verifyPendingMfa,
  };

  const overlaySectionProps: ComponentProps<typeof LoginOverlaySection> = {
    termsOpen,
    termsOfUse: resolvedApplication?.termsOfUse,
    onCloseTerms: () => {
      setTermsOpen(false);
    },
    faceRecognitionMode,
    onFaceRecognitionCancel,
    onFaceRecognitionVectorOk,
    onFaceRecognitionImagesOk,
  };

  return {
    overlaySectionProps,
    panelSectionProps,
    signinContentProps,
  };
}
