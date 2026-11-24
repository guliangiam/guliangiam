import { describe, expect, it } from "vitest";
import type { ApplicationDetail } from "@/shared/api/iam/application";
import { applicationDetailSchema } from "@/shared/api/iam/application";
import {
  buildApplicationConfigPayload,
  buildCreateApplicationPayload,
  createDefaultApplicationConfigFormValues,
  detailToConfigFormValues,
  importRequiresClientMaterial,
  importRequiresSecretRotation,
} from "./application-payload";

describe("application detail parsing keeps nested configuration", () => {
  // Regression: the schema used to strip these nested fields, while PATCH replaces
  // the whole array — saving an unrelated field would silently erase them.
  const detail = {
    id: "app-1",
    organizationId: "org-1",
    slug: "demo",
    name: "demo",
    displayName: "Demo",
    category: "",
    appType: "",
    domain: "",
    status: "",
    description: "",
    homepageUrl: "",
    clientId: "",
    tokenEndpointAuthMethod: "none" as const,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    revision: 1,
    signupItems: [
      {
        name: "Email",
        visible: true,
        required: true,
        prompted: false,
        regex: "^.+$",
        options: ["a", "b"],
      },
    ],
    providerItems: [
      {
        provider: "github",
        canSignUp: true,
        canSignIn: true,
        canUnlink: false,
        bindingRule: ["Email"],
        countryCodes: ["+86"],
        signupGroup: "staff",
        rule: "email-domain",
      },
    ],
    samlAttributes: [
      {
        name: "email",
        nameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
        value: "Email",
      },
    ],
  };

  it("retains signupItems.regex/options, providerItems.* and samlAttributes.nameFormat", () => {
    const parsed = applicationDetailSchema.parse(detail);
    expect(parsed.signupItems[0].regex).toBe("^.+$");
    expect(parsed.signupItems[0].options).toEqual(["a", "b"]);
    expect(parsed.providerItems[0].bindingRule).toEqual(["Email"]);
    expect(parsed.providerItems[0].countryCodes).toEqual(["+86"]);
    expect(parsed.providerItems[0].signupGroup).toBe("staff");
    expect(parsed.providerItems[0].rule).toBe("email-domain");
    expect(parsed.samlAttributes[0].nameFormat).toBe(
      "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
    );
  });

  it("uses the larger revision when the form is fresher than the cached detail", () => {
    const parsed = applicationDetailSchema.parse({ ...detail, revision: 7 }) as ApplicationDetail;
    // 保存响应已把表单 revision 推到 8，而 detail 缓存仍是 7。
    const values = {
      ...createDefaultApplicationConfigFormValues(parsed.organizationId),
      ...detailToConfigFormValues(parsed),
      revision: 8,
    };
    expect(buildApplicationConfigPayload((key) => key, values, parsed).revision).toBe(8);

    // 旧草稿里的 revision 不能覆盖更新的服务端快照。
    expect(
      buildApplicationConfigPayload((key) => key, { ...values, revision: 3 }, parsed).revision,
    ).toBe(7);
  });

  it("keeps nested arrays when saving an unrelated field (read-back round trip)", () => {
    const parsed = applicationDetailSchema.parse(detail) as ApplicationDetail;
    const values = {
      ...createDefaultApplicationConfigFormValues(parsed.organizationId),
      ...detailToConfigFormValues(parsed),
      // Only an unrelated scalar changes; nested arrays are untouched.
      displayName: "Renamed",
    };
    const payload = buildApplicationConfigPayload((key) => key, values, parsed);

    expect(payload.displayName).toBe("Renamed");
    expect(payload.signupItems?.[0].regex).toBe("^.+$");
    expect(payload.signupItems?.[0].options).toEqual(["a", "b"]);
    expect(payload.providerItems?.[0].bindingRule).toEqual(["Email"]);
    expect(payload.providerItems?.[0].countryCodes).toEqual(["+86"]);
    expect(payload.providerItems?.[0].signupGroup).toBe("staff");
    expect(payload.samlAttributes?.[0].nameFormat).toBe(
      "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
    );
  });

  it("does not clear either SAML source on an unrelated save", () => {
    const parsed = applicationDetailSchema.parse({
      ...detail,
      samlMetadataUrl: "https://idp.example.com/metadata",
      samlMetadata: "<EntityDescriptor/>",
      certificate: "CERT-PEM",
    }) as ApplicationDetail;
    const values = {
      ...createDefaultApplicationConfigFormValues(parsed.organizationId),
      ...detailToConfigFormValues(parsed),
      displayName: "Renamed",
    };
    const payload = buildApplicationConfigPayload((key) => key, values, parsed);

    // "keep" is the default: both sides are submitted untouched, backend resolves URL-first.
    expect(payload.samlMetadataUrl).toBe("https://idp.example.com/metadata");
    expect(payload.samlMetadata).toBe("<EntityDescriptor/>");
    expect(payload.certificate).toBe("CERT-PEM");
  });

  it("only drops the opposite side after an explicit source switch", () => {
    const parsed = applicationDetailSchema.parse({
      ...detail,
      samlMetadataUrl: "https://idp.example.com/metadata",
      samlMetadata: "<EntityDescriptor/>",
      certificate: "CERT-PEM",
    }) as ApplicationDetail;
    const base = {
      ...createDefaultApplicationConfigFormValues(parsed.organizationId),
      ...detailToConfigFormValues(parsed),
    };

    const usingXml = buildApplicationConfigPayload(
      (key) => key,
      { ...base, samlMetadataSource: "xml" },
      parsed,
    );
    expect(usingXml.samlMetadataUrl).toBe("");
    expect(usingXml.samlMetadata).toBe("<EntityDescriptor/>");
    // Certificate is never erased by switching sources.
    expect(usingXml.certificate).toBe("CERT-PEM");

    const usingUrl = buildApplicationConfigPayload(
      (key) => key,
      { ...base, samlMetadataSource: "url" },
      parsed,
    );
    expect(usingUrl.samlMetadata).toBe("");
    expect(usingUrl.samlMetadataUrl).toBe("https://idp.example.com/metadata");
    expect(usingUrl.certificate).toBe("CERT-PEM");
  });
});

describe("importRequiresClientMaterial", () => {
  it("blocks methods whose key or certificate is not carried by the export", () => {
    for (const method of [
      "private_key_jwt",
      "tls_client_auth",
      "self_signed_tls_client_auth",
      "TLS_CLIENT_AUTH",
    ]) {
      expect(importRequiresClientMaterial({ export: { tokenEndpointAuthMethod: method } })).toBe(
        true,
      );
    }
  });

  it("allows public and secret clients", () => {
    for (const method of ["none", "client_secret_basic", "client_secret_post", "", undefined]) {
      expect(importRequiresClientMaterial({ export: { tokenEndpointAuthMethod: method } })).toBe(
        false,
      );
    }
  });

  it("ignores documents without an export section", () => {
    expect(importRequiresClientMaterial(null)).toBe(false);
    expect(importRequiresClientMaterial({})).toBe(false);
    expect(importRequiresClientMaterial("not-an-export")).toBe(false);
    expect(importRequiresClientMaterial({ export: { tokenEndpointAuthMethod: 42 } })).toBe(false);
  });
});

describe("importRequiresSecretRotation", () => {
  it("warns for secret-based methods", () => {
    for (const method of ["client_secret_basic", "client_secret_post", "CLIENT_SECRET_POST"]) {
      expect(importRequiresSecretRotation({ export: { tokenEndpointAuthMethod: method } })).toBe(
        true,
      );
    }
  });

  it("does not warn for public or material-requiring methods", () => {
    for (const method of ["none", "private_key_jwt", "tls_client_auth", "", undefined]) {
      expect(importRequiresSecretRotation({ export: { tokenEndpointAuthMethod: method } })).toBe(
        false,
      );
    }
  });

  it("ignores documents without an export section", () => {
    expect(importRequiresSecretRotation(null)).toBe(false);
    expect(importRequiresSecretRotation({})).toBe(false);
    expect(importRequiresSecretRotation({ export: { tokenEndpointAuthMethod: 42 } })).toBe(false);
  });
});

describe("UI customization normalization", () => {
  const base = { id: "app-1", organizationId: "org-1", slug: "demo" } as ApplicationDetail;

  it("maps legacy orgChoiceMode vocabulary to the canonical one", () => {
    expect(detailToConfigFormValues({ ...base, orgChoiceMode: "None" }).orgChoiceMode).toBe("");
    expect(detailToConfigFormValues({ ...base, orgChoiceMode: "Select" }).orgChoiceMode).toBe(
      "direct",
    );
    expect(detailToConfigFormValues({ ...base, orgChoiceMode: "Input" }).orgChoiceMode).toBe(
      "search",
    );
    expect(detailToConfigFormValues({ ...base, orgChoiceMode: "search" }).orgChoiceMode).toBe(
      "search",
    );
    expect(detailToConfigFormValues({ ...base, orgChoiceMode: "bogus" }).orgChoiceMode).toBe("");
  });

  it("normalizes legacy formOffset 0/empty to center (2)", () => {
    expect(detailToConfigFormValues({ ...base, formOffset: "0" }).formOffset).toBe("2");
    expect(detailToConfigFormValues({ ...base, formOffset: "" }).formOffset).toBe("2");
    expect(detailToConfigFormValues({ ...base, formOffset: "4" }).formOffset).toBe("4");
    expect(createDefaultApplicationConfigFormValues().formOffset).toBe("2");
  });

  it("uses the shared signup defaults as the single source of truth", () => {
    const defaults = createDefaultApplicationConfigFormValues();
    expect(defaults.signupItems.map((item) => item.name)).toEqual([
      "Username",
      "Email",
      "Phone",
      "Password",
      "Confirm password",
    ]);
  });
});

describe("jwksUri round trip", () => {
  const uri = "https://client.example.com/jwks.json";
  const base = {
    id: "app-1",
    organizationId: "org-1",
    slug: "demo",
    name: "demo",
    displayName: "Demo",
    category: "",
    appType: "",
    domain: "",
    status: "active",
    description: "",
    homepageUrl: "",
    clientId: "gl_cid_demo",
    tokenEndpointAuthMethod: "private_key_jwt" as const,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    revision: 1,
  };
  const parsedWithUri = applicationDetailSchema.parse({
    ...base,
    jwksUri: uri,
  }) as ApplicationDetail;
  const formValues = {
    ...createDefaultApplicationConfigFormValues("org-1"),
    ...detailToConfigFormValues(parsedWithUri),
  };

  it("echoes the stored uri into form values", () => {
    expect(detailToConfigFormValues(parsedWithUri).jwksUri).toBe(uri);
    expect(
      detailToConfigFormValues(applicationDetailSchema.parse(base) as ApplicationDetail).jwksUri,
    ).toBe("");
  });

  it("omits jwksUri from the patch when unchanged", () => {
    const payload = buildApplicationConfigPayload((key) => key, formValues, parsedWithUri);
    expect("jwksUri" in payload).toBe(false);
  });

  it("sends an empty string to clear the stored uri", () => {
    const payload = buildApplicationConfigPayload(
      (key) => key,
      { ...formValues, jwksUri: "" },
      parsedWithUri,
    );
    expect(payload.jwksUri).toBe("");
  });

  it("clears the stored uri when the method leaves private_key_jwt", () => {
    const payload = buildApplicationConfigPayload(
      (key) => key,
      { ...formValues, tokenEndpointAuthMethod: "client_secret_basic", jwksUri: "" },
      parsedWithUri,
    );
    expect(payload.jwksUri).toBe("");
  });

  it("creates with the trimmed uri and keeps it through the supported-keys filter", () => {
    const payload = buildCreateApplicationPayload((key) => key, {
      ...createDefaultApplicationConfigFormValues("org-1"),
      slug: "demo",
      displayName: "Demo",
      tokenEndpointAuthMethod: "private_key_jwt",
      jwksUri: `  ${uri}  `,
    });
    expect(payload.jwksUri).toBe(uri);
  });
});
