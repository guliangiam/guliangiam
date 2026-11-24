import { t } from "i18next";
import { requestDocumentJson } from "@/shared/api/http";
import { AUTH_SAML_SP_ACS } from "@/shared/api/paths";
import { parseSamlRelayState, type SamlRelayContext } from "@/shared/lib/auth-saml-context";

export type SamlCallbackContext = {
  relayState: string;
  samlResponse: string;
  relayContext: SamlRelayContext | null;
};

export type SamlAcsPayload = {
  organizationSlug: string;
  providerId: string;
  samlResponse: string;
  relayState: string;
};

export async function resolveSamlCallbackContext(search: string): Promise<SamlCallbackContext> {
  const params = new URLSearchParams(search);
  const rawRelayState = params.get("relayState") ?? params.get("RelayState") ?? "";
  const samlResponse = params.get("samlResponse") ?? params.get("SAMLResponse") ?? "";

  // RelayState is signed and verified by the backend. The browser only carries
  // it: parsing here builds the request, it is not a security check.
  const relayState = rawRelayState;

  return {
    relayState,
    samlResponse,
    relayContext: parseSamlRelayState(relayState),
  };
}

export function getSamlCallbackError(context: SamlCallbackContext) {
  if (!context.relayState || !context.samlResponse || !context.relayContext) {
    return t("saml.missingCallbackParams");
  }

  // The browser ACS route needs the organization and provider identifiers from
  // the signed RelayState payload. A legacy console RelayState does not carry
  // them, so it cannot be forwarded to the SP consumer.
  if (!context.relayContext.organizationSlug || !context.relayContext.providerId) {
    return t("saml.missingCallbackParams");
  }

  return null;
}

export function createSamlAcsPayload(context: SamlCallbackContext): SamlAcsPayload | null {
  const organizationSlug = context.relayContext?.organizationSlug ?? "";
  const providerId = context.relayContext?.providerId ?? "";
  if (!organizationSlug || !providerId || !context.samlResponse || !context.relayState) {
    return null;
  }

  return {
    organizationSlug,
    providerId,
    samlResponse: context.samlResponse,
    relayState: context.relayState,
  };
}

/**
 * POST the IdP form fields to the SP browser ACS as
 * application/x-www-form-urlencoded.
 *
 * The ACS answers 303 with a session Set-Cookie; fetch follows the redirect and
 * the cookie is stored, so the caller only needs to refresh the session query
 * and navigate to the console.
 */
export async function submitSamlAcs(payload: SamlAcsPayload): Promise<void> {
  const body = new URLSearchParams({
    SAMLResponse: payload.samlResponse,
    RelayState: payload.relayState,
  });

  await requestDocumentJson(AUTH_SAML_SP_ACS(payload.organizationSlug, payload.providerId), {
    method: "POST",
    body: body.toString(),
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
    },
  });
}
