import { describe, expect, it } from "vitest";
import { parseSamlRelayState, stripRelayStateEnvelope } from "./auth-saml-context";

function buildRelayState(segments: string[]): string {
  return window.btoa(segments.map((item) => encodeURIComponent(item)).join("&"));
}

function buildAcsRelayState(organizationSlug: string, providerId: string, nonce: string): string {
  const encoded = window
    .btoa(JSON.stringify({ o: organizationSlug, p: providerId, s: nonce }))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
  return `v1.1793000000.${"d".repeat(64)}.${encoded}`;
}

const RAW_STATE = buildRelayState([
  "client-id",
  "state-123",
  "okta",
  "https://sp.example.com/callback",
  "https://console.example.com/callback/saml",
  "code",
  "demo-app",
  "openid",
]);

describe("stripRelayStateEnvelope", () => {
  it("returns the original value when no envelope is present", () => {
    expect(stripRelayStateEnvelope(RAW_STATE)).toBe(RAW_STATE);
  });

  it("strips a backend-issued v1 envelope", () => {
    const signed = `v1.1793000000.${"a".repeat(64)}.${RAW_STATE}`;
    expect(stripRelayStateEnvelope(signed)).toBe(RAW_STATE);
  });

  it("does not strip an unknown version prefix", () => {
    const forged = `v2.1793000000.${"a".repeat(64)}.${RAW_STATE}`;
    expect(stripRelayStateEnvelope(forged)).toBe(forged);
  });

  it("returns an empty value unchanged", () => {
    expect(stripRelayStateEnvelope("")).toBe("");
  });
});

describe("parseSamlRelayState", () => {
  it("parses an unsigned relay state", () => {
    const context = parseSamlRelayState(RAW_STATE);
    expect(context).not.toBeNull();
    expect(context?.clientId).toBe("client-id");
    expect(context?.providerName).toBe("okta");
    expect(context?.application).toBe("demo-app");
    expect(context?.responseType).toBe("code");
  });

  it("parses the original value inside a signed envelope", () => {
    const signed = `v1.1793000000.${"b".repeat(64)}.${RAW_STATE}`;
    const context = parseSamlRelayState(signed);
    expect(context).not.toBeNull();
    expect(context?.providerName).toBe("okta");
    expect(context?.state).toBe("state-123");
  });

  it("returns null for a forged envelope whose payload is not base64", () => {
    expect(parseSamlRelayState(`v2.1793000000.${"c".repeat(64)}.not-base64`)).toBeNull();
  });

  it("returns null for a non-base64 value", () => {
    expect(parseSamlRelayState("not-base64")).toBeNull();
  });

  it("parses the server-issued SP ACS payload", () => {
    const context = parseSamlRelayState(buildAcsRelayState("acme", "provider-1", "nonce-1"));

    expect(context).not.toBeNull();
    expect(context?.organizationSlug).toBe("acme");
    expect(context?.providerId).toBe("provider-1");
    expect(context?.state).toBe("nonce-1");
    expect(context?.responseType).toBe("login");
  });

  it("keeps the legacy context free of ACS identifiers", () => {
    const context = parseSamlRelayState(RAW_STATE);

    expect(context?.organizationSlug).toBe("");
    expect(context?.providerId).toBe("");
  });
});
