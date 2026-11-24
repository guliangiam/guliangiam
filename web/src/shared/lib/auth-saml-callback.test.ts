import { beforeEach, describe, expect, it, vi } from "vitest";
import { requestDocumentJson } from "@/shared/api/http";
import {
  createSamlAcsPayload,
  getSamlCallbackError,
  resolveSamlCallbackContext,
  submitSamlAcs,
} from "./auth-saml-callback";

vi.mock("@/shared/api/http", () => ({
  requestDocumentJson: vi.fn<(...args: unknown[]) => Promise<unknown>>((...args: unknown[]) =>
    Promise.resolve(args),
  ),
}));

vi.mock("i18next", () => ({
  t: (key: string) => key,
}));

function buildAcsRelayState(organizationSlug: string, providerId: string, nonce: string): string {
  const encoded = window
    .btoa(JSON.stringify({ o: organizationSlug, p: providerId, s: nonce }))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
  return `v1.1793000000.${"a".repeat(64)}.${encoded}`;
}

describe("auth-saml-callback", () => {
  beforeEach(() => {
    vi.mocked(requestDocumentJson).mockClear();
  });

  it("resolves the signed RelayState into an ACS payload", async () => {
    const relayState = buildAcsRelayState("acme", "provider-1", "nonce-1");
    const search = new URLSearchParams({
      SAMLResponse: "saml-response",
      RelayState: relayState,
    }).toString();

    const context = await resolveSamlCallbackContext(`?${search}`);
    expect(getSamlCallbackError(context)).toBeNull();
    expect(createSamlAcsPayload(context)).toEqual({
      organizationSlug: "acme",
      providerId: "provider-1",
      samlResponse: "saml-response",
      relayState,
    });
  });

  it("rejects a callback missing SAMLResponse", async () => {
    const relayState = buildAcsRelayState("acme", "provider-1", "nonce-1");
    const context = await resolveSamlCallbackContext(
      `?${new URLSearchParams({ RelayState: relayState }).toString()}`,
    );

    expect(getSamlCallbackError(context)).toBe("saml.missingCallbackParams");
    expect(createSamlAcsPayload(context)).toBeNull();
  });

  it("rejects a legacy RelayState without organization/provider binding", async () => {
    const legacy = window.btoa(
      ["client-id", "state-1", "okta", "null"].map(encodeURIComponent).join("&"),
    );
    const context = await resolveSamlCallbackContext(
      `?${new URLSearchParams({ SAMLResponse: "saml-response", RelayState: legacy }).toString()}`,
    );

    expect(context.relayContext).not.toBeNull();
    expect(getSamlCallbackError(context)).toBe("saml.missingCallbackParams");
  });

  it("posts the form payload to the new ACS route", async () => {
    await submitSamlAcs({
      organizationSlug: "acme",
      providerId: "provider-1",
      samlResponse: "saml-response",
      relayState: "signed-state",
    });

    expect(requestDocumentJson).toHaveBeenCalledWith("/api/v1/auth/saml/acs/acme/provider-1", {
      method: "POST",
      body: "SAMLResponse=saml-response&RelayState=signed-state",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
    });
  });
});
