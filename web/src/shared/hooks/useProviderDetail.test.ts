import { describe, it, expect } from "vitest";
import {
  createDefaultProvider,
  detailToFormValues,
  buildCreatePayload,
  buildUpdatePayload,
  httpHeadersToRows,
  rowsToHttpHeaders,
  type ProviderFormValues,
} from "./useProviderDetail";
import type { ProviderDetail } from "@/shared/api/iam/provider-extended";

// ─── createDefaultProvider ──────────────────────────────────────

describe("createDefaultProvider", () => {
  it("returns default form values with given organizationId", () => {
    const result = createDefaultProvider("org-1");
    expect(result.organizationId).toBe("org-1");
    expect(result.name).toBe("");
    expect(result.displayName).toBe("");
    expect(result.category).toBe("OAuth");
    expect(result.providerType).toBe("Custom");
    expect(result.enableSignUp).toBe(false);
    expect(result.enablePkce).toBe(false);
    expect(result.scopes).toEqual([]);
  });

  it("sets all string fields to empty string by default", () => {
    const result = createDefaultProvider("org-1");
    expect(result.subType).toBe("");
    expect(result.method).toBe("");
    expect(result.clientId).toBe("");
    expect(result.clientSecret).toBe("");
    expect(result.authUrl).toBe("");
    expect(result.tokenUrl).toBe("");
    expect(result.host).toBe("");
    expect(result.userMapping).toBe("");
  });

  it("sets port to 0 by default", () => {
    const result = createDefaultProvider("org-1");
    expect(result.port).toBe(0);
  });

  it("defaults idvSceneId to empty string (IDV config echoed separately)", () => {
    const result = createDefaultProvider("org-1");
    expect(result.idvSceneId).toBe("");
  });
});

// ─── detailToFormValues ─────────────────────────────────────────

describe("detailToFormValues", () => {
  const baseDetail: ProviderDetail = {
    id: "provider-1",
    createdAt: "2024-01-01T00:00:00Z",
    updatedAt: "2024-01-01T00:00:00Z",
    organizationId: "org-1",
    name: "my-provider",
    displayName: "My Provider",
    category: "OAuth",
    providerType: "GitHub",
    subType: "",
    method: "",
    clientId: "client-123",
    clientSecret: "secret-should-be-blanked",
    clientId2: "client2-123",
    clientSecret2: "secret2-should-be-blanked",
    authUrl: "https://github.com/login/oauth/authorize",
    tokenUrl: "https://github.com/login/oauth/access_token",
    userInfoUrl: "https://api.github.com/user",
    callbackUrl: "https://app.example.com/callback",
    signingAlg: "",
    scopes: ["read:user", "user:email"],
    idpEntityId: "",
    ssoUrl: "",
    sloUrl: "",
    metadata: "",
    issuerUrl: "",
    region: "",
    host: "",
    port: 0,
    databaseType: "",
    template: "",
    customApi: "",
    customHeader: "",
    customBody: "",
    admin: "",
    password: "password-should-be-blanked",
    appId: "",
    contentType: "",
    sender: "",
    receiver: "",
    providerUrl: "",
    userMapping: { id: "id", name: "displayName" },
    enableSignUp: true,
    enablePkce: false,
    emailRegex: "",
    revision: 1,
  };

  it("maps basic fields correctly", () => {
    const result = detailToFormValues(baseDetail);
    expect(result.organizationId).toBe("org-1");
    expect(result.name).toBe("my-provider");
    expect(result.displayName).toBe("My Provider");
    expect(result.category).toBe("OAuth");
    expect(result.providerType).toBe("GitHub");
  });

  it("blanks clientSecret for security", () => {
    const result = detailToFormValues(baseDetail);
    expect(result.clientSecret).toBe("");
  });

  it("blanks clientSecret2 for security", () => {
    const result = detailToFormValues(baseDetail);
    expect(result.clientSecret2).toBe("");
  });

  it("blanks password for security", () => {
    const result = detailToFormValues(baseDetail);
    expect(result.password).toBe("");
  });

  it("preserves clientId", () => {
    const result = detailToFormValues(baseDetail);
    expect(result.clientId).toBe("client-123");
  });

  it("preserves scopes", () => {
    const result = detailToFormValues(baseDetail);
    expect(result.scopes).toEqual(["read:user", "user:email"]);
  });

  it("stringifies userMapping", () => {
    const result = detailToFormValues(baseDetail);
    expect(result.userMapping).toBe(JSON.stringify({ id: "id", name: "displayName" }));
  });

  it("handles null userMapping", () => {
    const detail = { ...baseDetail, userMapping: null } as unknown as ProviderDetail;
    const result = detailToFormValues(detail);
    expect(result.userMapping).toBe("");
  });

  it("preserves enableSignUp", () => {
    const result = detailToFormValues(baseDetail);
    expect(result.enableSignUp).toBe(true);
  });

  it("defaults enableSignUp to false when undefined", () => {
    const detail = { ...baseDetail, enableSignUp: undefined } as unknown as ProviderDetail;
    const result = detailToFormValues(detail);
    expect(result.enableSignUp).toBe(false);
  });

  it("defaults enablePkce to false when undefined", () => {
    const detail = { ...baseDetail, enablePkce: undefined } as unknown as ProviderDetail;
    const result = detailToFormValues(detail);
    expect(result.enablePkce).toBe(false);
  });

  it("defaults emailRegex to empty string when undefined", () => {
    const detail = { ...baseDetail, emailRegex: undefined } as unknown as ProviderDetail;
    const result = detailToFormValues(detail);
    expect(result.emailRegex).toBe("");
  });

  it("blanks signinHtml, customCss, customJs", () => {
    const result = detailToFormValues(baseDetail);
    expect(result.signinHtml).toBe("");
    expect(result.customCss).toBe("");
    expect(result.customJs).toBe("");
  });

  it("leaves idvSceneId empty (populated from the config endpoint, not the detail)", () => {
    const result = detailToFormValues(baseDetail);
    expect(result.idvSceneId).toBe("");
  });
});

// ─── httpHeaders adapter ────────────────────────────────────────

describe("httpHeadersToRows / rowsToHttpHeaders", () => {
  it("echoes an API string map as ordered table rows", () => {
    expect(httpHeadersToRows({ "X-IdP-Tenant": "acme", "X-Trace": "1" })).toEqual([
      { key: "X-IdP-Tenant", value: "acme" },
      { key: "X-Trace", value: "1" },
    ]);
  });

  it("returns an empty table for missing headers", () => {
    expect(httpHeadersToRows(undefined)).toEqual([]);
    expect(httpHeadersToRows(null)).toEqual([]);
  });

  it("submits table rows as the API string map", () => {
    expect(rowsToHttpHeaders([{ key: "X-IdP-Tenant", value: "acme" }])).toEqual({
      "X-IdP-Tenant": "acme",
    });
  });

  it("round-trips an API map through rows and back", () => {
    const headers = { "X-IdP-Tenant": "acme", "X-Trace": "1" };
    expect(rowsToHttpHeaders(httpHeadersToRows(headers))).toEqual(headers);
  });

  it("trims keys, drops blank rows, and omits an empty table", () => {
    expect(
      rowsToHttpHeaders([
        { key: " X-Trace ", value: "1" },
        { key: "", value: "ignored" },
      ]),
    ).toEqual({ "X-Trace": "1" });
    expect(rowsToHttpHeaders([])).toBeUndefined();
    expect(rowsToHttpHeaders([{ key: "  ", value: "ignored" }])).toBeUndefined();
  });
});

// ─── buildCreatePayload ─────────────────────────────────────────

describe("buildCreatePayload", () => {
  const baseValues: ProviderFormValues = {
    organizationId: "org-1",
    name: "new-provider",
    displayName: "New Provider",
    category: "OAuth",
    providerType: "Google",
    subType: "",
    method: "",
    clientId: "my-client-id",
    clientSecret: "my-client-secret",
    clientId2: "",
    clientSecret2: "",
    authUrl: "https://accounts.google.com/o/oauth2/v2/auth",
    tokenUrl: "https://oauth2.googleapis.com/token",
    userInfoUrl: "",
    callbackUrl: "",
    signingAlg: "",
    scopes: ["openid", "profile"],
    idpEntityId: "",
    ssoUrl: "",
    sloUrl: "",
    metadata: "",
    issuerUrl: "",
    region: "",
    host: "",
    port: 0,
    databaseType: "",
    template: "",
    customApi: "",
    customHeader: "",
    customBody: "",
    admin: "",
    password: "",
    appId: "",
    contentType: "",
    sender: "",
    receiver: "",
    providerUrl: "",
    userMapping: "",
    enableSignUp: false,
    enablePkce: false,
    emailRegex: "",
    signinHtml: "",
    customCss: "",
    customJs: "",
  };

  it("includes organizationId, name, displayName, category, providerType", () => {
    const result = buildCreatePayload(baseValues);
    expect(result.organizationId).toBe("org-1");
    expect(result.name).toBe("new-provider");
    expect(result.displayName).toBe("New Provider");
    expect(result.category).toBe("OAuth");
    expect(result.providerType).toBe("Google");
  });

  it("includes clientId when non-empty", () => {
    const result = buildCreatePayload(baseValues);
    expect(result.clientId).toBe("my-client-id");
  });

  it("converts empty clientId to undefined", () => {
    const values = { ...baseValues, clientId: "" };
    const result = buildCreatePayload(values);
    expect(result.clientId).toBeUndefined();
  });

  it("includes clientSecret when non-empty", () => {
    const result = buildCreatePayload(baseValues);
    expect(result.clientSecret).toBe("my-client-secret");
  });

  it("converts empty clientSecret to undefined", () => {
    const values = { ...baseValues, clientSecret: "" };
    const result = buildCreatePayload(values);
    expect(result.clientSecret).toBeUndefined();
  });

  it("keeps an empty clientSecret out of the serialized create payload", () => {
    const payload = buildCreatePayload({ ...baseValues, clientSecret: "" });
    // `undefined` keys disappear when the payload is serialized for the wire,
    // so the backend sees "no change" rather than an empty credential.
    expect(JSON.parse(JSON.stringify(payload))).not.toHaveProperty("clientSecret");
  });

  it("parses the userMapping JSON string into the API object", () => {
    const payload = buildCreatePayload({
      ...baseValues,
      userMapping: JSON.stringify({ id: "id", name: "displayName" }),
    });
    expect(payload.userMapping).toEqual({ id: "id", name: "displayName" });
  });

  it("drops an unparseable userMapping instead of sending invalid JSON", () => {
    expect(
      buildCreatePayload({ ...baseValues, userMapping: "not-json" }).userMapping,
    ).toBeUndefined();
  });

  it("includes scopes array", () => {
    const result = buildCreatePayload(baseValues);
    expect(result.scopes).toEqual(["openid", "profile"]);
  });

  it("converts empty string fields to undefined where applicable", () => {
    const result = buildCreatePayload(baseValues);
    // Fields that use `|| undefined` pattern in extractCommonFields
    expect(result.region).toBeUndefined();
    expect(result.host).toBeUndefined();
    // subType and method are passed through without the || undefined pattern
    expect(result.subType).toBe("");
    expect(result.method).toBe("");
  });
});

// ─── buildUpdatePayload ─────────────────────────────────────────

describe("buildUpdatePayload", () => {
  const baseValues: ProviderFormValues = {
    organizationId: "org-1",
    name: "existing-provider",
    displayName: "Updated Provider",
    category: "SAML",
    providerType: "SAML 2.0",
    subType: "",
    method: "",
    clientId: "client-id",
    clientSecret: "new-secret",
    clientId2: "",
    clientSecret2: "",
    authUrl: "",
    tokenUrl: "",
    userInfoUrl: "",
    callbackUrl: "",
    signingAlg: "RSA-SHA256",
    scopes: [],
    idpEntityId: "idp-entity",
    ssoUrl: "https://idp.example.com/sso",
    sloUrl: "",
    metadata: "",
    issuerUrl: "",
    region: "",
    host: "",
    port: 0,
    databaseType: "",
    template: "",
    customApi: "",
    customHeader: "",
    customBody: "",
    admin: "",
    password: "",
    appId: "",
    contentType: "",
    sender: "",
    receiver: "",
    providerUrl: "",
    userMapping: "",
    enableSignUp: false,
    enablePkce: false,
    emailRegex: "",
    signinHtml: "",
    customCss: "",
    customJs: "",
  };

  it("includes displayName, category, providerType", () => {
    const result = buildUpdatePayload(baseValues, 3);
    expect(result.displayName).toBe("Updated Provider");
    expect(result.category).toBe("SAML");
    expect(result.providerType).toBe("SAML 2.0");
  });

  it("includes revision", () => {
    const result = buildUpdatePayload(baseValues, 3);
    expect(result.revision).toBe(3);
  });

  it("keeps empty secrets out of the serialized update payload but always carries revision", () => {
    const payload = buildUpdatePayload(
      { ...baseValues, clientSecret: "", clientSecret2: "", password: "" },
      9,
    );
    const wire = JSON.parse(JSON.stringify(payload));
    expect(wire).not.toHaveProperty("clientSecret");
    expect(wire).not.toHaveProperty("clientSecret2");
    expect(wire).not.toHaveProperty("password");
    expect(wire.revision).toBe(9);
  });

  it("does not include organizationId or name (update only)", () => {
    const result = buildUpdatePayload(baseValues, 1);
    expect("organizationId" in result).toBe(false);
    expect("name" in result).toBe(false);
  });

  it("includes signingAlg when non-empty", () => {
    const result = buildUpdatePayload(baseValues, 1);
    expect(result.signingAlg).toBe("RSA-SHA256");
  });

  it("includes idpEntityId when non-empty", () => {
    const result = buildUpdatePayload(baseValues, 1);
    expect(result.idpEntityId).toBe("idp-entity");
  });

  it("includes ssoUrl when non-empty", () => {
    const result = buildUpdatePayload(baseValues, 1);
    expect(result.ssoUrl).toBe("https://idp.example.com/sso");
  });
});
