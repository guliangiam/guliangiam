import { useMemo, useState } from "react";
import { App } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useLocation, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { loadResolvedApplication } from "@/shared/lib/auth-application-loader";
import { parseOAuthQueryContext } from "@/shared/lib/auth-oauth-context";
import { resolveAuthApplicationName } from "@/shared/lib/auth-route-context";
import { requestDocumentJson } from "@/shared/api/http";
import type { ApplicationItem } from "@/shared/types/domain";

import { AUTH_CONSENTS_GRANT } from "@/shared/api/paths";
import { localizedError } from "@/shared/lib/server-error";

type ConsentGrantResponse = {
  code?: { code?: string };
  [key: string]: unknown;
};

async function grantConsent(payload: {
  owner: string;
  application: string;
  grantedScopes: string[];
  clientId: string;
  provider: string;
  signinMethod: string;
  responseType: string;
  redirectUri: string;
  scope: string;
  state: string;
  nonce?: string;
  challenge?: string;
  resource?: string;
}): Promise<ConsentGrantResponse | undefined> {
  return requestDocumentJson<ConsentGrantResponse>(AUTH_CONSENTS_GRANT, {
    method: "POST",
    body: JSON.stringify({
      applicationId: payload.application,
      grantedScopes: payload.grantedScopes,
    }),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

type ConsentScopeItem = {
  scope: string;
  displayName: string;
  description: string;
};

type CustomScope = {
  scope?: string | null;
  displayName?: string | null;
  description?: string | null;
};

function getConsentScopes(
  application: ApplicationItem | null,
  scopeValue: string,
  t: (key: string) => string,
) {
  if (!scopeValue) {
    return [] as ConsentScopeItem[];
  }

  const customScopes = Array.isArray(
    (application as { customScopes?: unknown } | null)?.customScopes,
  )
    ? (((application as { customScopes?: unknown }).customScopes as unknown[]) ?? [])
    : [];
  const customScopeMap = new Map<string, CustomScope>();
  for (const item of customScopes) {
    if (!item || typeof item !== "object") {
      continue;
    }

    const scopeItem = item as CustomScope;
    if (!scopeItem.scope) {
      continue;
    }
    customScopeMap.set(scopeItem.scope, scopeItem);
  }

  return scopeValue
    .split(" ")
    .map((scope) => scope.trim())
    .filter(Boolean)
    .map((scope) => {
      const matched = customScopeMap.get(scope);
      if (matched) {
        return {
          scope,
          displayName: matched.displayName || matched.scope || scope,
          description: matched.description || "",
        } satisfies ConsentScopeItem;
      }

      return {
        scope,
        displayName: scope,
        description: t("auth:consentNoScopeDesc"),
      } satisfies ConsentScopeItem;
    });
}

function buildRedirectUrl(redirectUri: string, values: Record<string, string>) {
  const redirectUrl = new URL(redirectUri);
  for (const [key, value] of Object.entries(values)) {
    if (!value) {
      continue;
    }
    redirectUrl.searchParams.set(key, value);
  }
  return redirectUrl.toString();
}

type ConsentPageBaseState = {
  applicationName: string;
};

type ConsentPageLoadingState = ConsentPageBaseState & {
  status: "loading";
};

type ConsentPageErrorState = ConsentPageBaseState & {
  status: "context-error" | "session-error" | "application-error";
  errorTitle: string;
};

export type ConsentPageReadyState = ConsentPageBaseState & {
  status: "ready";
  application: ApplicationItem;
  consentScopes: ConsentScopeItem[];
  canGrant: boolean;
  isSubmitting: boolean;
  handleGrant: () => Promise<void>;
  handleDeny: () => void;
};

export type ConsentPageControllerState =
  | ConsentPageLoadingState
  | ConsentPageErrorState
  | ConsentPageReadyState;

export function useConsentPageController(): ConsentPageControllerState {
  const { message } = App.useApp();
  const location = useLocation();
  const params = useParams<{ applicationName?: string }>();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const { t } = useTranslation();

  const applicationName = resolveAuthApplicationName({
    routeApplicationName: params.applicationName,
    search: location.search,
  });

  const oauthContext = useMemo(() => parseOAuthQueryContext(location.search), [location.search]);

  const applicationQuery = useQuery({
    queryKey: [
      "auth",
      "consent",
      "application",
      applicationName,
      oauthContext?.clientId,
      oauthContext?.redirectUri,
    ],
    enabled: Boolean(applicationName),
    queryFn: () => loadResolvedApplication({ applicationName, oauthContext }),
  });

  const consentScopes = useMemo(
    () => getConsentScopes(applicationQuery.data ?? null, oauthContext?.scope ?? "", t as never),
    [applicationQuery.data, oauthContext?.scope, t],
  );

  const canGrant = consentScopes.length > 0;

  async function handleGrant() {
    if (!applicationQuery.data || !oauthContext) {
      return;
    }

    setIsSubmitting(true);
    try {
      const response = await grantConsent({
        owner: applicationQuery.data.owner || "admin",
        application: `${applicationQuery.data.owner || "admin"}/${applicationQuery.data.name}`,
        grantedScopes: consentScopes.map((item) => item.scope),
        clientId: oauthContext.clientId,
        provider: "",
        signinMethod: "",
        responseType: oauthContext.responseType || "code",
        redirectUri: oauthContext.redirectUri,
        scope: oauthContext.scope,
        state: oauthContext.state,
        nonce: oauthContext.nonce,
        challenge: oauthContext.codeChallenge,
        resource: oauthContext.resource,
      });

      const consentResponse = response;
      const codeObj = consentResponse?.code;
      const code = typeof codeObj?.code === "string" ? codeObj.code : "";
      if (!code) {
        message.error(t("consent.grantedButNoCode"));
        return;
      }

      window.location.assign(
        buildRedirectUrl(oauthContext.redirectUri, {
          code,
          state: oauthContext.state,
        }),
      );
    } catch (error) {
      message.error(localizedError(error, t, "consent.grantFailed"));
    } finally {
      setIsSubmitting(false);
    }
  }

  function handleDeny() {
    if (!oauthContext) {
      return;
    }

    window.location.assign(
      buildRedirectUrl(oauthContext.redirectUri, {
        error: "access_denied",
        error_description: "User denied consent",
        state: oauthContext.state,
      }),
    );
  }

  if (!oauthContext) {
    return {
      applicationName,
      status: "context-error",
      errorTitle: t("consent.missingOAuthContext"),
    };
  }

  if (applicationQuery.isLoading) {
    return {
      applicationName,
      status: "loading",
    };
  }

  if (applicationQuery.isError || !applicationQuery.data) {
    return {
      applicationName,
      status: "application-error",
      errorTitle: t("consent.invalidApplication"),
    };
  }

  return {
    applicationName,
    status: "ready",
    application: applicationQuery.data,
    consentScopes,
    canGrant,
    isSubmitting,
    handleGrant,
    handleDeny,
  };
}
