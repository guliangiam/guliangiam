import type { CasLoginContext, SamlLoginContext } from "@/shared/lib/auth-finishers";
import { parseOAuthQueryContext, type OAuthQueryContext } from "@/shared/lib/auth-oauth-context";

export function resolveAuthApplicationName(values: {
  routeApplicationName?: string;
  search: string;
}) {
  return values.routeApplicationName ?? new URLSearchParams(values.search).get("application") ?? "";
}

type LoginPageLocationState = {
  from?: {
    pathname?: string;
    search?: string;
  };
} | null;

export type LoginPageUrlState = {
  returnTo: string;
  silentSignin: string | null;
  providerHint: string | null;
  oauthResponseMode: string;
  mfaSetupCompleted: boolean;
};

export type LoginDefaults = {
  organization: string;
  application: string;
  username: string;
};

export type LoginOrganizationSwitchResult = {
  nextApplicationName: string;
  navigateTo: string;
};

export function resolveProviderHintUrl(values: {
  providerName: string;
  pathname: string;
  currentHref: string;
  origin: string;
  hash?: string;
  search?: string;
  oauthContext: OAuthQueryContext | null;
  applicationClientId?: string | null;
  applicationName?: string | null;
}) {
  if (!values.providerName) {
    return null;
  }

  if (values.oauthContext && values.pathname === "/login/oauth/authorize") {
    const nextUrl = new URL(values.currentHref);
    nextUrl.searchParams.set("provider_hint", values.providerName);
    return `${nextUrl.pathname}${nextUrl.search}${nextUrl.hash}`;
  }

  if (!values.applicationClientId) {
    return null;
  }

  const searchParams = new URLSearchParams({
    client_id: values.applicationClientId,
    response_type: "code",
    redirect_uri: `${values.origin}/callback`,
    scope: "read",
    state: values.applicationName || "guliangiam",
    provider_hint: values.providerName,
  });

  return `/login/oauth/authorize?${searchParams.toString()}`;
}

export function resolveProviderHintFallbackUrl(currentHref: string) {
  const nextUrl = new URL(currentHref);
  nextUrl.searchParams.delete("provider_hint");
  return `${nextUrl.pathname}${nextUrl.search}${nextUrl.hash}`;
}

function resolveOrganizationSwitchedApplicationName(values: {
  currentApplicationName: string;
  currentOrganizationName: string;
  nextOrganization: string;
  isSharedApplication: boolean;
}) {
  if (!values.currentApplicationName) {
    return "app-built-in";
  }

  if (values.isSharedApplication) {
    return `${values.currentApplicationName}-org-${values.nextOrganization}`;
  }

  if (values.currentOrganizationName) {
    const currentOrganizationSuffix = `-org-${values.currentOrganizationName}`;
    if (values.currentApplicationName.endsWith(currentOrganizationSuffix)) {
      return `${values.currentApplicationName.slice(0, -currentOrganizationSuffix.length)}-org-${values.nextOrganization}`;
    }
  }

  return values.currentApplicationName;
}

export function resolveLoginPageUrlState(values: {
  pathname: string;
  search: string;
  state?: LoginPageLocationState;
}): LoginPageUrlState {
  const params = new URLSearchParams(values.search);
  const fromPath = values.state?.from?.pathname;

  let returnTo = "/";
  if (fromPath && fromPath.startsWith("/") && !fromPath.startsWith("//")) {
    returnTo = `${fromPath}${values.state?.from?.search ?? ""}`;
  } else {
    const returnToParam = params.get("returnTo");
    if (returnToParam && returnToParam.startsWith("/") && !returnToParam.startsWith("//")) {
      returnTo = returnToParam;
    } else if (values.pathname === "/login/oauth/authorize") {
      returnTo = `${values.pathname}${values.search}`;
    }
  }

  return {
    returnTo,
    silentSignin: params.get("silentSignin"),
    providerHint: params.get("provider_hint"),
    oauthResponseMode: params.get("response_mode") || "query",
    mfaSetupCompleted: params.get("mfaSetup") === "completed",
  };
}

export function resolveLoginDefaults(values: {
  organizationNameParam?: string;
  search: string;
  samlContext: SamlLoginContext | null;
  lastLoginOrganization?: string | null;
  lastLoginApplication?: string | null;
  defaultOrganizationName: string;
  defaultApplicationName: string;
}): LoginDefaults {
  const params = new URLSearchParams(values.search);

  return {
    organization:
      values.samlContext?.owner ||
      values.organizationNameParam ||
      params.get("organization") ||
      params.get("owner") ||
      values.lastLoginOrganization ||
      values.defaultOrganizationName,
    application:
      values.samlContext?.applicationName ||
      params.get("application") ||
      params.get("applicationName") ||
      values.lastLoginApplication ||
      values.defaultApplicationName,
    username:
      values.samlContext?.username || values.samlContext?.loginHint || params.get("username") || "",
  };
}

export function resolveLoginOrganizationSwitch(values: {
  pathname: string;
  search: string;
  currentApplicationName: string;
  currentOrganizationName: string;
  nextOrganization: string;
  isSharedApplication: boolean;
}): LoginOrganizationSwitchResult {
  const nextApplicationName = resolveOrganizationSwitchedApplicationName({
    currentApplicationName: values.currentApplicationName,
    currentOrganizationName: values.currentOrganizationName,
    nextOrganization: values.nextOrganization,
    isSharedApplication: values.isSharedApplication,
  });
  const searchParams = new URLSearchParams(values.search);
  const clientId = searchParams.get("client_id");

  if (clientId) {
    const clientIdSplitted = clientId.split("-org-");
    searchParams.set("client_id", `${clientIdSplitted[0]}-org-${values.nextOrganization}`);
    return {
      nextApplicationName,
      navigateTo: `/login/oauth/authorize?${searchParams.toString()}`,
    };
  }

  if (values.pathname.startsWith("/login/saml/authorize")) {
    return {
      nextApplicationName,
      navigateTo: `/login/saml/authorize/${encodeURIComponent(values.nextOrganization)}/${encodeURIComponent(`${values.currentApplicationName}-org-${values.nextOrganization}`)}?${searchParams.toString()}`,
    };
  }

  if (values.pathname.startsWith("/cas")) {
    return {
      nextApplicationName,
      navigateTo: `/cas/${encodeURIComponent(`${values.currentApplicationName}-org-${values.nextOrganization}`)}/${encodeURIComponent(values.nextOrganization)}/login?${searchParams.toString()}`,
    };
  }

  searchParams.set("application", nextApplicationName);
  searchParams.set("orgChoiceMode", "None");
  return {
    nextApplicationName,
    navigateTo: `/login/${encodeURIComponent(values.nextOrganization)}?${searchParams.toString()}`,
  };
}

export function resolveOAuthAuthorizeContext(values: {
  pathname: string;
  search: string;
}): OAuthQueryContext | null {
  if (values.pathname !== "/login/oauth/authorize") {
    return null;
  }

  return parseOAuthQueryContext(values.search);
}

export function resolveCasLoginContext(values: {
  pathname: string;
  search: string;
  owner?: string;
  applicationName?: string;
}): CasLoginContext | null {
  if (!values.pathname.startsWith("/cas/") || !values.applicationName) {
    return null;
  }

  const params = new URLSearchParams(values.search);
  return {
    applicationId: `${values.owner || "admin"}/${encodeURIComponent(values.applicationName)}`,
    service: params.get("service") ?? "",
  };
}

export function resolveSamlLoginContext(values: {
  pathname: string;
  search: string;
  owner?: string;
  applicationName?: string;
}): SamlLoginContext | null {
  if (
    !values.pathname.startsWith("/login/saml/authorize/") ||
    !values.owner ||
    !values.applicationName
  ) {
    return null;
  }

  const params = new URLSearchParams(values.search);
  return {
    owner: values.owner,
    applicationName: values.applicationName,
    relayState: params.get("relayState") ?? "",
    samlRequest: params.get("samlRequest") ?? "",
    username: params.get("username") ?? "",
    loginHint: params.get("login_hint") ?? "",
  };
}
