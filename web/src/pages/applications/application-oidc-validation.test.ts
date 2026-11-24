import { describe, expect, it } from "vitest";
import type { ApplicationConfigFormValues } from "./tabs/types";
import {
  CUSTOM_SCOPE_PRESETS,
  customScopePresetDescriptionKey,
  customScopePresetNameKey,
  evaluateOidcIssues,
  findCustomScopePreset,
  isValidClientJwksUri,
  sameStringSet,
} from "./application-oidc-validation";

function values(overrides: Partial<ApplicationConfigFormValues> = {}): ApplicationConfigFormValues {
  return {
    displayName: "App",
    status: "active",
    tokenEndpointAuthMethod: "none",
    grantTypes: ["authorization_code"],
    ...overrides,
  };
}

function codes(
  input: ApplicationConfigFormValues,
  options?: Parameters<typeof evaluateOidcIssues>[1],
) {
  return evaluateOidcIssues(input, options).map((issue) => issue.code);
}

describe("evaluateOidcIssues", () => {
  it("requires authorization_code for a public client", () => {
    expect(codes(values({ grantTypes: ["client_credentials"] }))).toEqual([
      "noneRequiresAuthorizationCode",
      "noneForbidsClientCredentials",
    ]);
  });

  it("accepts a persisted public-client combination that is not touched", () => {
    const input = values({ grantTypes: ["implicit"] });
    expect(
      codes(input, {
        persistedAuthMethod: "none",
        persistedGrantTypes: ["implicit"],
      }),
    ).toEqual([]);
  });

  it("rejects material the selected method does not use", () => {
    expect(
      codes(
        values({
          tokenEndpointAuthMethod: "none",
          clientSecret: "secret",
          jwks: '{"keys":[]}',
          mtlsCertificates: [{ certificate: "pem" }],
        }),
      ),
    ).toEqual([
      "secretNotAllowedForMethod",
      "jwksNotAllowedForMethod",
      "fingerprintsNotAllowedForMethod",
    ]);
  });

  it("requires a JWKS for private_key_jwt unless the stored keys are kept", () => {
    const input = values({ tokenEndpointAuthMethod: "private_key_jwt" });
    expect(codes(input)).toContain("jwksRequired");
    expect(codes(input, { requireClientJwks: false })).not.toContain("jwksRequired");
  });

  it("requires at least one certificate for mTLS clients", () => {
    expect(codes(values({ tokenEndpointAuthMethod: "tls_client_auth" }))).toContain(
      "mtlsCertificateRequired",
    );
    expect(
      codes(values({ tokenEndpointAuthMethod: "tls_client_auth" }), {
        persistedAuthMethod: "tls_client_auth",
        existingCertificateCount: 1,
      }),
    ).toEqual([]);
  });

  it("flags empty certificate rows", () => {
    expect(
      codes(
        values({
          tokenEndpointAuthMethod: "tls_client_auth",
          mtlsCertificates: [{ certificate: "" }],
        }),
      ),
    ).toContain("certificateRequired");
  });
});

describe("sameStringSet", () => {
  it("compares sets ignoring order", () => {
    expect(sameStringSet(["a", "b"], ["b", "a"])).toBe(true);
    expect(sameStringSet(["a"], ["a", "b"])).toBe(false);
  });
});

describe("custom scope presets", () => {
  it("exposes the protocol scopes and read/write", () => {
    const scopes = CUSTOM_SCOPE_PRESETS.map((preset) => preset.scope);
    expect(scopes).toEqual([
      "openid",
      "profile",
      "email",
      "phone",
      "address",
      "offline_access",
      "read",
      "write",
    ]);
  });

  it("resolves presets and builds i18n keys", () => {
    expect(findCustomScopePreset("offline_access")).toBe("offline_access");
    expect(findCustomScopePreset("custom:scope")).toBeUndefined();
    expect(customScopePresetNameKey("openid")).toBe(
      "application.customScopePresets.openid.displayName",
    );
    expect(customScopePresetDescriptionKey("email")).toBe(
      "application.customScopePresets.email.description",
    );
  });
});

describe("jwksUri", () => {
  const uri = "https://client.example.com/jwks.json";

  it("satisfies the private_key_jwt material requirement on its own", () => {
    expect(codes(values({ tokenEndpointAuthMethod: "private_key_jwt", jwksUri: uri }))).toEqual([]);
  });

  it("still requires material when neither inline jwks nor uri is present", () => {
    expect(codes(values({ tokenEndpointAuthMethod: "private_key_jwt" }))).toEqual(["jwksRequired"]);
  });

  it("rejects a uri for methods that do not use it", () => {
    expect(codes(values({ tokenEndpointAuthMethod: "none", jwksUri: uri }))).toEqual([
      "jwksUriNotAllowedForMethod",
    ]);
  });

  it("rejects non-HTTPS, userinfo, fragment, malformed and overlong uris", () => {
    const bad = [
      "http://client.example.com/jwks.json",
      "https://user:pass@client.example.com/jwks.json",
      "https://client.example.com/jwks.json#fragment",
      "not a url",
      `https://${"a".repeat(2048)}.example.com`,
    ];
    for (const candidate of bad) {
      expect(isValidClientJwksUri(candidate)).toBe(false);
      expect(
        codes(values({ tokenEndpointAuthMethod: "private_key_jwt", jwksUri: candidate })),
      ).toContain("jwksUriInvalid");
    }
    expect(isValidClientJwksUri("")).toBe(true);
    expect(isValidClientJwksUri(uri)).toBe(true);
  });

  it("keeps the edit-mode exemption when only a stored uri exists", () => {
    expect(
      codes(values({ tokenEndpointAuthMethod: "private_key_jwt", jwksUri: uri }), {
        requireClientJwks: false,
        persistedAuthMethod: "private_key_jwt",
      }),
    ).toEqual([]);
  });
});
