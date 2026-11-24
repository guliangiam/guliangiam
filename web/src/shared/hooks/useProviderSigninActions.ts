import { t } from "i18next";
import { useCallback } from "react";
import {
  resolveProviderLaunchPlan,
  runSamlProviderSignin,
  startDirectProviderSignin,
  type ProviderSigninOption,
} from "@/shared/lib/auth-provider-signin";
import type { OAuthQueryContext } from "@/shared/lib/auth-oauth-context";
import type { DirectAuthProvider } from "@/shared/lib/provider-auth";
import type { ApplicationItem } from "@/shared/types/domain";
import { localizedError } from "@/shared/lib/server-error";

type UseProviderSigninActionsValues = {
  agreementRequired: boolean;
  isAgreementAccepted: boolean;
  application: ApplicationItem | null;
  oauthContext: OAuthQueryContext | null;
  validateAgreement: () => Promise<unknown>;
  showError: (message: string) => void;
  startDirectAuthProviderSignin: (
    provider: DirectAuthProvider,
    options?: { replace?: boolean },
  ) => Promise<void>;
};

function isInIframe() {
  try {
    return window.self !== window.top;
  } catch {
    return true;
  }
}

export function useProviderSigninActions(values: UseProviderSigninActionsValues) {
  const {
    agreementRequired,
    application,
    isAgreementAccepted,
    oauthContext,
    showError,
    startDirectAuthProviderSignin,
    validateAgreement,
  } = values;

  const handleProviderClick = useCallback(
    async (provider: ProviderSigninOption) => {
      if (agreementRequired && !isAgreementAccepted) {
        try {
          await validateAgreement();
        } catch {
          return;
        }
      }

      const launchPlan = resolveProviderLaunchPlan(provider);

      if (launchPlan.kind === "saml") {
        try {
          await runSamlProviderSignin({
            application,
            provider,
            oauthContext,
          });
        } catch (error) {
          showError(localizedError(error, t, "auth:signin.saml.loginFailed"));
        }
        return;
      }

      if (launchPlan.kind === "oauth") {
        try {
          await startDirectAuthProviderSignin(provider);
        } catch (error) {
          showError(localizedError(error, t, "auth:signin.provider.oauthLaunchFailed"));
        }
        return;
      }

      if (launchPlan.kind === "web3") {
        try {
          await startDirectAuthProviderSignin(provider);
        } catch (error) {
          showError(localizedError(error, t, "auth:signin.provider.web3LaunchFailed"));
        }
        return;
      }

      if (launchPlan.kind === "url") {
        window.location.assign(launchPlan.url);
        return;
      }

      try {
        await startDirectProviderSignin({
          application,
          provider,
          unsupportedMessage: t("auth:signin.saml.providerUnsupported"),
          missingUrlMessage: t("auth:signin.saml.providerMissingUrl"),
        });
      } catch (error) {
        showError(localizedError(error, t, "auth:signin.provider.oauthLaunchFailed"));
      }
    },
    [
      agreementRequired,
      application,
      isAgreementAccepted,
      oauthContext,
      showError,
      startDirectAuthProviderSignin,
      validateAgreement,
    ],
  );

  const shouldRenderGoogleOneTap = useCallback(
    (provider?: DirectAuthProvider | null) => Boolean(application && provider && !isInIframe()),
    [application],
  );

  return {
    handleProviderClick,
    shouldRenderGoogleOneTap,
  };
}
