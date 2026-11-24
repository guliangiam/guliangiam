import { useEffect } from "react";
import { runProviderHintRedirect } from "@/shared/lib/auth-provider-hint";
import { resolveProviderHintUrl } from "@/shared/lib/auth-route-context";
import type { OAuthQueryContext } from "@/shared/lib/auth-oauth-context";
import type { DirectAuthProvider } from "@/shared/lib/provider-auth";
import type { ApplicationItem } from "@/shared/types/domain";

type ProviderRedirectTarget = DirectAuthProvider & {
  category?: string | null;
  name?: string | null;
};

type UseProviderRedirectFlowValues = {
  pathname: string;
  search: string;
  providerHint: string | null;
  shouldAutoRedirectSingleProvider: boolean;
  application: ApplicationItem | null;
  visibleDirectProviders: Array<{ provider?: ProviderRedirectTarget | null }>;
  oauthContext: OAuthQueryContext | null;
  startDirectAuthProviderSignin: (
    provider: DirectAuthProvider,
    options?: { replace?: boolean },
  ) => Promise<void>;
};

export function useProviderRedirectFlow(values: UseProviderRedirectFlowValues) {
  const {
    application,
    oauthContext,
    pathname,
    providerHint,
    search,
    shouldAutoRedirectSingleProvider,
    startDirectAuthProviderSignin,
    visibleDirectProviders,
  } = values;

  useEffect(() => {
    if (pathname !== "/login/oauth/authorize" || !providerHint) {
      return;
    }

    return runProviderHintRedirect({
      currentHref: window.location.href,
    });
  }, [pathname, providerHint]);

  useEffect(() => {
    if (!shouldAutoRedirectSingleProvider || !application) {
      return;
    }

    const soleProvider = visibleDirectProviders[0]?.provider;
    if (!soleProvider?.name) {
      return;
    }

    const redirectUrl = resolveProviderHintUrl({
      providerName: soleProvider.name,
      pathname,
      currentHref: window.location.href,
      origin: window.location.origin,
      hash: window.location.hash,
      search,
      oauthContext,
      applicationClientId: application.clientId,
      applicationName: application.name,
    });

    if (!redirectUrl) {
      if (soleProvider.category === "OAuth" || soleProvider.category === "Web3") {
        void startDirectAuthProviderSignin(soleProvider, {
          replace: true,
        }).catch(() => {
          return;
        });
      }
      return;
    }

    window.location.replace(redirectUrl);
  }, [
    application,
    oauthContext,
    pathname,
    search,
    shouldAutoRedirectSingleProvider,
    startDirectAuthProviderSignin,
    visibleDirectProviders,
  ]);
}
