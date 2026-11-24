import {
  DEFAULT_APPLICATION_NAME,
  DEFAULT_ORGANIZATION_NAME,
  type LoginMethodKey,
} from "@/shared/lib/auth-signin-items";
import {
  resolveCasLoginContext,
  resolveLoginDefaults,
  resolveLoginPageUrlState,
  resolveOAuthAuthorizeContext,
  resolveSamlLoginContext,
} from "@/shared/lib/auth-route-context";

type LocationStateLike = {
  from?: {
    pathname?: string;
    search?: string;
  };
} | null;

type UseLoginPageRouteContextValues = {
  loginMethod?: LoginMethodKey;
  pathname?: string;
  search?: string;
  state?: LocationStateLike;
  organizationName?: string;
  owner?: string;
  casApplicationName?: string;
  applicationName?: string;
};

export function useLoginPageRouteContext(
  values: UseLoginPageRouteContextValues = {} as UseLoginPageRouteContextValues,
) {
  const {
    applicationName,
    casApplicationName,
    organizationName,
    owner,
    pathname = "",
    search = "",
    state = null,
  } = values;

  const { mfaSetupCompleted, oauthResponseMode, providerHint, returnTo, silentSignin } =
    resolveLoginPageUrlState({
      pathname,
      search,
      state,
    });
  const samlContext = resolveSamlLoginContext({
    pathname,
    search,
    owner,
    applicationName,
  });
  const loginDefaults = resolveLoginDefaults({
    organizationNameParam: organizationName,
    search,
    samlContext,
    lastLoginOrganization: localStorage.getItem("lastLoginOrg"),
    lastLoginApplication: localStorage.getItem("lastLoginApplication"),
    defaultOrganizationName: DEFAULT_ORGANIZATION_NAME,
    defaultApplicationName: DEFAULT_APPLICATION_NAME,
  });
  const oauthContext = resolveOAuthAuthorizeContext({
    pathname,
    search,
  });
  const casContext = resolveCasLoginContext({
    pathname,
    search,
    owner,
    applicationName: casApplicationName,
  });
  const isOauthAuthorize = oauthContext !== null;
  const isCasLogin = casContext !== null;
  const isSamlLogin = samlContext !== null;
  const isSupportedOauthResponseType =
    oauthContext?.responseType === "code" ||
    oauthContext?.responseType === "token" ||
    oauthContext?.responseType === "id_token";
  return {
    casContext,
    isCasLogin,
    isOauthAuthorize,
    isSamlLogin,
    isSupportedOauthResponseType,
    loginDefaults,
    mfaSetupCompleted,
    oauthContext,
    oauthResponseMode,
    providerHint,
    returnTo,
    samlContext,
    silentSignin,
  };
}

export type LoginPageRouteContextState = ReturnType<typeof useLoginPageRouteContext>;
