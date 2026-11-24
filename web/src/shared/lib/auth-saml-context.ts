import type { OAuthAuthorizeContext } from "@/shared/lib/auth-finishers";

export type SamlRelayContext = {
  application: string;
  clientId: string;
  providerName: string;
  redirectUri: string;
  responseType: string;
  scope: string;
  state: string;
  // SP browser ACS context (server-signed RelayState payload). Empty for the
  // legacy console RelayState shape.
  organizationSlug: string;
  providerId: string;
};

function decodeRelayStateValue(value?: string) {
  if (!value) {
    return "";
  }

  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

// Backend-issued RelayState uses `v1.<issuedAtUnix>.<hexHMAC>.<original>`.
// Stripping the envelope is purely presentational — the browser cannot verify
// the HMAC and must not pretend to. The backend validates it during login.
const RELAY_STATE_ENVELOPE_PATTERN = /^v1\.\d+\.[0-9a-f]+\./;

export function stripRelayStateEnvelope(value: string): string {
  const match = RELAY_STATE_ENVELOPE_PATTERN.exec(value);
  if (!match) {
    return value;
  }
  return value.slice(match[0].length);
}

// Go emits RawURLEncoding for the SP ACS payload; window.atob only accepts the
// standard alphabet and, depending on the browser, requires padding.
function decodeBase64Payload(value: string): string | null {
  const normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  const padded = normalized + "=".repeat((4 - (normalized.length % 4)) % 4);
  try {
    return window.atob(padded);
  } catch {
    return null;
  }
}

function parseAcsRelayPayload(decoded: string): SamlRelayContext | null {
  if (!decoded.trim().startsWith("{")) {
    return null;
  }

  let payload: unknown;
  try {
    payload = JSON.parse(decoded);
  } catch {
    return null;
  }
  if (!payload || typeof payload !== "object") {
    return null;
  }

  const record = payload as Record<string, unknown>;
  const organizationSlug =
    typeof record.o === "string"
      ? record.o
      : typeof record.organizationSlug === "string"
        ? record.organizationSlug
        : "";
  const providerId =
    typeof record.p === "string"
      ? record.p
      : typeof record.providerId === "string"
        ? record.providerId
        : "";
  const state =
    typeof record.s === "string" ? record.s : typeof record.state === "string" ? record.state : "";

  if (!organizationSlug || !providerId || !state) {
    return null;
  }

  return {
    application: "",
    clientId: "",
    providerName: providerId,
    redirectUri: "",
    responseType: "login",
    scope: "",
    state,
    organizationSlug,
    providerId,
  };
}

function parseLegacyRelayPayload(decoded: string): SamlRelayContext | null {
  const segments = decoded.split("&").map((item) => decodeRelayStateValue(item));
  const clientId = segments[0] ?? "";
  const redirectUri = segments[3] ?? "";
  const responseType =
    segments[5] ?? (redirectUri && redirectUri !== "null" && clientId ? "code" : "login");
  const application = segments[6] ?? (clientId ? "" : "app-built-in");

  if (!segments[2]) {
    return null;
  }

  return {
    application,
    clientId,
    providerName: segments[2],
    redirectUri,
    responseType,
    scope: segments[7] ?? "read",
    state: segments[1] ?? "",
    organizationSlug: "",
    providerId: "",
  };
}

export function parseSamlRelayState(relayState: string): SamlRelayContext | null {
  const decoded = decodeBase64Payload(stripRelayStateEnvelope(relayState));
  if (!decoded) {
    return null;
  }
  return parseAcsRelayPayload(decoded) ?? parseLegacyRelayPayload(decoded);
}

export function getSamlCallbackResponseType(context: SamlRelayContext | null) {
  if (!context) {
    return "login";
  }

  if (context.responseType) {
    return context.responseType;
  }

  if (!context.redirectUri || context.redirectUri === "null") {
    return "login";
  }

  return context.clientId ? "code" : "login";
}

export function toOAuthContextFromSamlRelay(values: {
  relayContext: SamlRelayContext | null;
  responseType: string;
}): OAuthAuthorizeContext | null {
  const { relayContext, responseType } = values;

  if (!relayContext?.clientId || !relayContext.redirectUri || relayContext.redirectUri === "null") {
    return null;
  }

  if (responseType === "login") {
    return null;
  }

  return {
    clientId: relayContext.clientId,
    redirectUri: relayContext.redirectUri,
    responseType,
    scope: relayContext.scope || "read",
    state: relayContext.state,
  };
}
