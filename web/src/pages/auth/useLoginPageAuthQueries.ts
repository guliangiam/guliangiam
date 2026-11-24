import { useQuery } from "@tanstack/react-query";
import { getOrganizationOptions } from "@/shared/api/iam/organization";
import { useLoginPageViewModel } from "@/shared/hooks/useLoginPageViewModel";
import {
  loadCasApplication,
  loadOAuthApplication,
  loadOwnedApplicationDetail,
} from "@/shared/lib/auth-application-loader";
import type { LoginMethodKey } from "@/shared/lib/auth-signin-items";
import type { LoginPageRouteContextState } from "./useLoginPageRouteContext";

type SessionAccountLike = {
  owner?: string | null;
} | null;

type UseLoginPageAuthQueriesValues = {
  routeContext: LoginPageRouteContextState;
  pathname: string;
  search: string;
  loginMethod: LoginMethodKey;
  watchedOrganization: string;
  watchedApplication: string;
  sessionAccount: SessionAccountLike;
};

export function useLoginPageAuthQueries(values: UseLoginPageAuthQueriesValues) {
  const {
    loginMethod,
    pathname,
    routeContext,
    search,
    sessionAccount,
    watchedApplication,
    watchedOrganization,
  } = values;
  const { casContext, isCasLogin, isOauthAuthorize, oauthContext, providerHint } = routeContext;

  const authApplicationQuery = useQuery({
    queryKey: [
      "auth",
      "application",
      oauthContext?.clientId,
      oauthContext?.redirectUri,
      casContext?.applicationId,
      casContext?.service,
    ],
    enabled: isOauthAuthorize || isCasLogin,
    queryFn: async () => {
      if (casContext) {
        return loadCasApplication({
          applicationId: casContext.applicationId,
          redirectUri: casContext.service,
        });
      }

      return loadOAuthApplication(oauthContext!);
    },
  });

  const brandedApplicationQuery = useQuery({
    queryKey: ["auth", "branding", watchedOrganization, watchedApplication],
    enabled: Boolean(watchedOrganization && watchedApplication) && !isOauthAuthorize && !isCasLogin,
    queryFn: async () => loadOwnedApplicationDetail(watchedOrganization, watchedApplication),
  });

  const viewModel = useLoginPageViewModel({
    authApplication: authApplicationQuery.data,
    brandedApplication: brandedApplicationQuery.data,
    pathname,
    search,
    oauthContext,
    providerHint,
    loginMethod,
    watchedOrganization,
    watchedApplication,
    sessionAccount,
  });

  const organizationOptionsQuery = useQuery({
    queryKey: ["auth", "organization-options"],
    enabled: viewModel.showSelectOrganization,
    queryFn: async () => getOrganizationOptions(),
  });

  return {
    ...viewModel,
    authApplicationQuery,
    brandedApplicationQuery,
    organizationOptionsQuery,
  };
}
