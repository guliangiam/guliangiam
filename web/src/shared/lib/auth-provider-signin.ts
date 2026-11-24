import { t } from "i18next";
import { AUTH_SAML_REDIRECT } from "@/shared/api/paths";
import type { OAuthQueryContext } from "@/shared/lib/auth-oauth-context";
import { resolveProviderHintUrl } from "@/shared/lib/auth-route-context";
import {
  canStartDirectOAuthProvider,
  type DirectAuthProvider,
  getDirectOAuthProviderAuthUrl,
} from "@/shared/lib/provider-auth";
// Web3 direct-provider removed (Phase 2): wallet-signature login deprecated.
// Web3 identity binding is now achieved via Passkey + deterministic EVM address
// derivation (keccak256(passkey_pubkey || "guliangiam-web3-v1")). See design doc
// Deliverable #2: Liquid Auth + Passkey replacement.
import type { ApplicationItem } from "@/shared/types/domain";

function getSamlRedirectUrl(providerFullName: string, relayState: string): string {
  // providerFullName 格式 "owner/name" → 解析为后端路由的 organizationSlug / applicationSlug
  const [orgSlug, appSlug] = providerFullName.split("/");
  if (!orgSlug || !appSlug) {
    throw new Error(t("auth:signin.saml.missingOwnerOrName"));
  }
  const url = new URL(AUTH_SAML_REDIRECT(orgSlug, appSlug), window.location.origin);
  url.searchParams.set("RelayState", relayState);
  return url.toString();
}

export type ProviderSigninOption = {
  canStartDirectly?: boolean;
  category?: string | null;
  clientId?: string | null;
  clientId2?: string | null;
  customAuthUrl?: string | null;
  customLogo?: string | null;
  disableSsl?: boolean | null;
  domain?: string | null;
  displayName?: string | null;
  enablePkce?: boolean | null;
  appId?: string | null;
  method?: string | null;
  name: string;
  owner?: string | null;
  scopes?: string | null;
  signName?: string | null;
  subType?: string | null;
  type?: string | null;
  url?: string | null;
};

type ApplicationProviderListItem = NonNullable<ApplicationItem["providers"]>[number];
type ApplicationProviderItem = Extract<ApplicationProviderListItem, { provider?: unknown }>;

export type ProviderLaunchPlan =
  | { kind: "saml" }
  | { kind: "oauth" }
  | { kind: "web3" }
  | { kind: "url"; url: string }
  | { kind: "unsupported" };

function encodeRelayStateSegment(value?: string | null) {
  return encodeURIComponent(value ?? "");
}

function buildSamlProviderRelayState(values: {
  application: ApplicationItem | null;
  providerName: string;
  oauthContext: OAuthQueryContext | null;
  callbackOrigin: string;
}) {
  const clientId = values.oauthContext?.clientId ?? "";
  const state = values.oauthContext?.state ?? "";
  const redirectUri = values.oauthContext?.redirectUri ?? "null";
  const callbackUri = `${values.callbackOrigin}/callback/saml`;
  const responseType = values.oauthContext?.responseType ?? "login";
  const applicationName = values.application?.name ?? (clientId ? "" : "app-built-in");
  const scope = values.oauthContext?.scope ?? "read";

  return window.btoa(
    [
      clientId,
      state,
      values.providerName,
      redirectUri,
      callbackUri,
      responseType,
      applicationName,
      scope,
    ]
      .map((item) => encodeRelayStateSegment(item))
      .join("&"),
  );
}

export function buildProviderSigninOptions(values: {
  providerItems: ApplicationProviderItem[];
  pathname: string;
  currentHref: string;
  origin: string;
  hash?: string;
  search?: string;
  oauthContext: OAuthQueryContext | null;
  applicationClientId?: string | null;
  applicationName?: string | null;
}) {
  return values.providerItems.reduce<ProviderSigninOption[]>((items, providerItem) => {
    const provider = providerItem.provider as DirectAuthProvider | undefined;
    if (!provider?.name) {
      return items;
    }

    items.push({
      appId:
        typeof (provider as Record<string, unknown>).appId === "string"
          ? ((provider as Record<string, unknown>).appId as string)
          : undefined,
      canStartDirectly:
        providerItem.provider?.category === "SAML" ||
        (providerItem.provider?.category === "OAuth" && canStartDirectOAuthProvider(provider)),
      category: providerItem.provider?.category,
      clientId: provider.clientId,
      clientId2: provider.clientId2,
      customAuthUrl:
        typeof (provider as Record<string, unknown>).customAuthUrl === "string"
          ? ((provider as Record<string, unknown>).customAuthUrl as string)
          : undefined,
      customLogo:
        typeof (provider as Record<string, unknown>).customLogo === "string"
          ? ((provider as Record<string, unknown>).customLogo as string)
          : undefined,
      disableSsl: provider.disableSsl,
      domain: provider.domain,
      displayName: providerItem.provider?.displayName,
      enablePkce:
        typeof (provider as Record<string, unknown>).enablePkce === "boolean"
          ? ((provider as Record<string, unknown>).enablePkce as boolean)
          : undefined,
      method: provider.method,
      name: provider.name,
      owner: providerItem.provider?.owner,
      scopes: provider.scopes,
      signName: provider.signName,
      subType: provider.subType,
      type: provider.type,
      url: resolveProviderHintUrl({
        providerName: provider.name,
        pathname: values.pathname,
        currentHref: values.currentHref,
        origin: values.origin,
        hash: values.hash,
        search: values.search,
        oauthContext: values.oauthContext,
        applicationClientId: values.applicationClientId,
        applicationName: values.applicationName,
      }),
    });

    return items;
  }, []);
}

export function resolveProviderLaunchPlan(provider: ProviderSigninOption): ProviderLaunchPlan {
  if (provider.category === "SAML") {
    return { kind: "saml" };
  }

  if (provider.category === "OAuth" && !provider.url && provider.canStartDirectly) {
    return { kind: "oauth" };
  }

  if (provider.category === "Web3") {
    // DEPRECATED (Phase 2): wallet-sign Web3 login removed.
    // Web3 identity binding now goes through Passkey (WebAuthn) registration,
    // which automatically derives and stores an EVM address.
    return { kind: "unsupported" };
  }

  if (provider.url) {
    return { kind: "url", url: provider.url };
  }

  return { kind: "unsupported" };
}

export async function runSamlProviderSignin(values: {
  application: ApplicationItem | null;
  provider: Pick<ProviderSigninOption, "owner" | "name">;
  oauthContext: OAuthQueryContext | null;
  getSamlRedirectUrlImpl?: typeof getSamlRedirectUrl;
  windowObject?: Window;
  documentObject?: Document;
  callbackOrigin?: string;
}) {
  if (!values.application) {
    throw new Error(t("auth:signin.saml.missingConfig"));
  }

  if (!values.provider.owner || !values.provider.name) {
    throw new Error(t("auth:signin.saml.missingOwnerOrName"));
  }

  const windowObject = values.windowObject ?? window;
  const getSamlRedirectUrlImpl = values.getSamlRedirectUrlImpl ?? getSamlRedirectUrl;
  const relayState = buildSamlProviderRelayState({
    application: values.application,
    providerName: values.provider.name,
    oauthContext: values.oauthContext,
    callbackOrigin: values.callbackOrigin ?? windowObject.location.origin,
  });
  const redirectUrl = getSamlRedirectUrlImpl(
    `${values.provider.owner}/${values.provider.name}`,
    relayState,
  );

  // 后端返回 303 redirect → 让浏览器直接跟随，前端不拦截
  windowObject.location.assign(redirectUrl);
}

export async function startDirectProviderSignin(values: {
  application: ApplicationItem | null;
  provider: DirectAuthProvider;
  method?: string;
  replace?: boolean;
  code?: string;
  unsupportedMessage?: string;
  missingUrlMessage?: string;
  getDirectOAuthProviderAuthUrlImpl?: typeof getDirectOAuthProviderAuthUrl;
  windowObject?: Window;
}) {
  // DEPRECATED (Phase 2): Web3 direct-provider path removed.
  if (!canStartDirectOAuthProvider(values.provider)) {
    throw new Error(values.unsupportedMessage ?? t("auth:signin.saml.providerUnsupported"));
  }

  const getDirectOAuthProviderAuthUrlImpl =
    values.getDirectOAuthProviderAuthUrlImpl ?? getDirectOAuthProviderAuthUrl;
  const authUrl = await getDirectOAuthProviderAuthUrlImpl(
    values.application,
    values.provider,
    values.method,
    values.code,
  );
  if (!authUrl) {
    throw new Error(values.missingUrlMessage ?? t("auth:signin.saml.providerMissingUrl"));
  }

  const windowObject = values.windowObject ?? window;
  if (values.replace) {
    windowObject.location.replace(authUrl);
    return;
  }

  windowObject.location.assign(authUrl);
}
