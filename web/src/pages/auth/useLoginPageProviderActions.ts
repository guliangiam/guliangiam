import { useCallback } from "react";
import type { FormInstance } from "antd";
import type { NavigateFunction } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useProviderSigninActions } from "@/shared/hooks/useProviderSigninActions";
import { useProviderRedirectFlow } from "@/shared/hooks/useProviderRedirectFlow";
import { startDirectProviderSignin } from "@/shared/lib/auth-provider-signin";
import { resolveLoginOrganizationSwitch } from "@/shared/lib/auth-route-context";
import type { DirectAuthProvider } from "@/shared/lib/provider-auth";
import type { LoginPageAuthContextState } from "./useLoginPageAuthContext";

type LoginFormProviderActionValues = {
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

type UseLoginPageProviderActionsValues = {
  form: FormInstance<LoginFormProviderActionValues>;
  pathname: string;
  search: string;
  navigate: NavigateFunction;
  authContext: LoginPageAuthContextState;
  showError: (message: string) => void;
};

export function useLoginPageProviderActions(values: UseLoginPageProviderActionsValues) {
  const { authContext, form, navigate, pathname, search, showError } = values;
  const {
    agreementRequired,
    googleOneTapProvider,
    isSharedApplication,
    oauthContext,
    providerHint,
    resolvedApplication,
    shouldAutoRedirectSingleProvider,
    visibleDirectProviders,
    watchedApplicationFallback,
    watchedOrganizationFallback,
  } = authContext;
  const { t } = useTranslation();

  const switchLoginOrganization = useCallback(
    (nextOrganization: string) => {
      const currentApplicationName = resolvedApplication?.name || watchedApplicationFallback;
      const currentOrganizationName =
        resolvedApplication?.organization || watchedOrganizationFallback;
      const { navigateTo, nextApplicationName } = resolveLoginOrganizationSwitch({
        pathname,
        search,
        currentApplicationName,
        currentOrganizationName,
        nextOrganization,
        isSharedApplication,
      });

      form.setFieldsValue({
        organization: nextOrganization,
        application: nextApplicationName,
        password: "",
        code: "",
        captchaToken: "",
      });
      navigate(navigateTo);
    },
    [
      form,
      isSharedApplication,
      navigate,
      pathname,
      resolvedApplication,
      search,
      watchedApplicationFallback,
      watchedOrganizationFallback,
    ],
  );

  const startDirectAuthProviderSignin = useCallback(
    async (providerOption: DirectAuthProvider, options?: { replace?: boolean }) => {
      await startDirectProviderSignin({
        application: resolvedApplication,
        provider: providerOption,
        replace: options?.replace,
        unsupportedMessage: t("login.providerNotSupported"),
        missingUrlMessage: t("login.providerMissingUrl"),
      });
    },
    [resolvedApplication, t],
  );

  const { handleProviderClick, shouldRenderGoogleOneTap } = useProviderSigninActions({
    agreementRequired,
    application: resolvedApplication,
    isAgreementAccepted: Boolean(form.getFieldValue("agreement")),
    oauthContext,
    validateAgreement: () => form.validateFields(["agreement"]),
    showError,
    startDirectAuthProviderSignin,
  });

  useProviderRedirectFlow({
    pathname,
    search,
    providerHint,
    shouldAutoRedirectSingleProvider,
    application: resolvedApplication,
    visibleDirectProviders,
    oauthContext,
    startDirectAuthProviderSignin,
  });

  return {
    canRenderGoogleOneTap: shouldRenderGoogleOneTap(googleOneTapProvider),
    handleProviderClick,
    switchLoginOrganization,
  };
}

export type LoginPageProviderActionsState = ReturnType<typeof useLoginPageProviderActions>;
