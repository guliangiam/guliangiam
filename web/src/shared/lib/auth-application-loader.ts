import { requestDocumentJson } from "@/shared/api/http";
import { applicationSchema } from "@/shared/types/domain";

import {
  APPLICATIONS_RESOLVE,
  APPLICATION_BY_CLIENT_ID,
  APPLICATION_BY_ID,
} from "@/shared/api/paths";

export type OAuthApplicationContext = {
  clientId: string;
  redirectUri: string;
  responseType: string;
  scope: string;
  state: string;
};

async function getApplicationDetail(owner: string, name: string) {
  const response = await requestDocumentJson(
    `${APPLICATIONS_RESOLVE}?ownerSlug=${encodeURIComponent(owner)}&appSlug=${encodeURIComponent(name)}`,
  );
  return applicationSchema.parse(response);
}

async function getApplicationLogin(values: {
  clientId?: string;
  responseType?: string;
  redirectUri?: string;
  scope?: string;
  state?: string;
  id?: string;
  type: "code" | "cas";
}) {
  if (values.type === "code" && values.clientId) {
    const response = await requestDocumentJson(`${APPLICATION_BY_CLIENT_ID(values.clientId)}`);
    return applicationSchema.parse(response);
  }

  // CAS or fallback: resolve by id (owner/name)
  if (values.id) {
    const parts = values.id.split("/");
    if (parts.length === 2) {
      return getApplicationDetail(parts[0], parts[1]);
    }
    // Try as UUID
    const response = await requestDocumentJson(`${APPLICATION_BY_ID(values.id)}`);
    return applicationSchema.parse(response);
  }

  throw new Error("Either clientId or id must be provided for application lookup");
}

export async function loadApplicationDetail(applicationName: string) {
  return getApplicationDetail("admin", applicationName);
}

export async function loadOwnedApplicationDetail(owner: string, applicationName: string) {
  return getApplicationDetail(owner, applicationName);
}

export async function loadCasApplication(values: { applicationId: string; redirectUri: string }) {
  return getApplicationLogin({
    id: values.applicationId,
    redirectUri: values.redirectUri,
    type: "cas",
  });
}

export async function loadOAuthApplication(oauthContext: OAuthApplicationContext) {
  return getApplicationLogin({
    clientId: oauthContext.clientId,
    redirectUri: oauthContext.redirectUri,
    responseType: oauthContext.responseType,
    scope: oauthContext.scope,
    state: oauthContext.state,
    type: "code",
  });
}

export async function loadResolvedApplication(values: {
  applicationName: string;
  oauthContext?: OAuthApplicationContext | null;
}) {
  if (values.oauthContext) {
    try {
      return await loadOAuthApplication(values.oauthContext);
    } catch {
      return loadApplicationDetail(values.applicationName);
    }
  }

  return loadApplicationDetail(values.applicationName);
}
