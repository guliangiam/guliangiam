import { describe, it, expect, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import {
  useProviderCategories,
  MANAGED_PROVIDER_KINDS,
  CREATABLE_PROVIDER_CATEGORIES,
  CONSOLE_REDIRECT_PROVIDER_CATEGORIES,
  PROVIDER_CONSOLE_REDIRECTS,
  FILTERABLE_PROVIDER_CATEGORIES,
  HISTORICAL_PROVIDER_CATEGORIES,
  PROVIDER_CATEGORIES,
  PROVIDER_FILTER_CATEGORIES,
  PROVIDER_FILTER_TYPES,
  IDVERIFICATION_FILTER_TYPES,
  isManagedProviderKind,
  captchaUITypeToKind,
  captchaKindToUIType,
  categoryCredentialFields,
  categorySupportsUserMapping,
  isAuthSwitchApplicable,
  getCategoryColor,
} from "./useProviderCategories";

// Mock react-i18next
vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key, // Return key as-is (no translation)
    i18n: { language: "en" },
  }),
}));

// ─── Constants ──────────────────────────────────────────────────

describe("PROVIDER_CATEGORIES", () => {
  it("contains all expected categories", () => {
    const values = PROVIDER_CATEGORIES.map((c) => c.value);
    expect(values).toContain("OAuth");
    expect(values).toContain("SAML");
    expect(values).toContain("OIDC");
    expect(values).toContain("Email");
    expect(values).toContain("SMS");
    expect(values).toContain("Storage");
    expect(values).toContain("Payment");
    expect(values).toContain("Notification");
    expect(values).toContain("Captcha");
    expect(values).toContain("MFA");
    expect(values).toContain("FaceId");
    expect(values).toContain("Web3");
    expect(values).toContain("Scan");
    expect(values).toContain("IDVerification");
    expect(values).toContain("Log");
  });

  it("has matching label and value for each category", () => {
    for (const cat of PROVIDER_CATEGORIES) {
      expect(cat.label).toBeTruthy();
      expect(cat.value).toBeTruthy();
    }
  });
});

describe("CREATABLE_PROVIDER_CATEGORIES", () => {
  it("offers Captcha and excludes the remaining dedicated-subsystem categories", () => {
    const values = CREATABLE_PROVIDER_CATEGORIES.map((c) => c.value);
    // Captcha 创建已并入统一表单（分流提交专用端点）。
    expect(values).toContain("Captcha");
    expect(values).not.toContain("FaceId");
    expect(values).not.toContain("Web3");
    expect(values).toContain("OAuth");
  });

  it("keeps Notification creatable: unified rows are the Push MFA provider source", () => {
    // EnablePushMfa 只接受 providers.category = Notification 的统一目录行；
    // 把创建重定向到专用通知台（notification_providers 表）会切断推送 MFA 建档。
    expect(CREATABLE_PROVIDER_CATEGORIES.map((c) => c.value)).toContain("Notification");
    expect(PROVIDER_CONSOLE_REDIRECTS.Notification).toBeUndefined();
  });

  it("excludes Scan and Log: their kinds have no runtime consumer (dead-end creation)", () => {
    const values = CREATABLE_PROVIDER_CATEGORIES.map((c) => c.value);
    expect(values).not.toContain("Scan");
    expect(values).not.toContain("Log");
    // 历史行仍可在列表筛选。
    const filterable = FILTERABLE_PROVIDER_CATEGORIES.map((c) => c.value);
    expect(filterable).toContain("Scan");
    expect(filterable).toContain("Log");
  });
});

describe("HISTORICAL_PROVIDER_CATEGORIES", () => {
  it("contains the categories kept for historical rows only", () => {
    expect(HISTORICAL_PROVIDER_CATEGORIES.map((c) => c.value)).toEqual([
      "Payment",
      "FaceId",
      "Web3",
      "Scan",
      "Log",
    ]);
  });
});

describe("PROVIDER_CONSOLE_REDIRECTS", () => {
  it("routes FaceId to its independent create page", () => {
    expect(PROVIDER_CONSOLE_REDIRECTS.FaceId).toBe("/face-id-providers/new");
  });

  it("routes Payment creation to the channel-config console, keeping rows filterable", () => {
    // 支付运行时凭据只存在于 pay_channel_configs（sealed + draft→testing→active）；
    // 统一 providers 行不驱动扣款链，创建入口必须重定向到渠道配置台。
    expect(PROVIDER_CONSOLE_REDIRECTS.Payment).toBe("/payments/channel-configs");
    expect(CREATABLE_PROVIDER_CATEGORIES.map((c) => c.value)).not.toContain("Payment");
    expect(CONSOLE_REDIRECT_PROVIDER_CATEGORIES.map((c) => c.value)).toContain("Payment");
    expect(FILTERABLE_PROVIDER_CATEGORIES.map((c) => c.value)).toContain("Payment");
  });

  it("exposes FaceId in the create category dropdown without making it unified-creatable", () => {
    // FaceId 是 managed kind：必须出现在配置页类别下拉（独立可选），但绝不能进入
    // CREATABLE_PROVIDER_CATEGORIES（统一端点拒绝 managed kind）。
    const redirectValues = CONSOLE_REDIRECT_PROVIDER_CATEGORIES.map((c) => c.value);
    expect(redirectValues).toContain("FaceId");
    expect(CREATABLE_PROVIDER_CATEGORIES.map((c) => c.value)).not.toContain("FaceId");
    const { result } = renderHook(() => useProviderCategories({ category: "OAuth" }));
    expect(result.current.categoryOptions.map((c) => c.value)).toContain("FaceId");
  });

  it("keeps IDVerification independently creatable in the unified form", () => {
    expect(CREATABLE_PROVIDER_CATEGORIES.map((c) => c.value)).toContain("IDVerification");
    expect(PROVIDER_CONSOLE_REDIRECTS.IDVerification).toBeUndefined();
  });
});

describe("FILTERABLE_PROVIDER_CATEGORIES", () => {
  it("keeps historical categories so existing rows stay filterable", () => {
    const values = FILTERABLE_PROVIDER_CATEGORIES.map((c) => c.value);
    expect(values).toContain("Captcha");
    expect(values).toContain("FaceId");
    expect(values).toContain("Web3");
  });

  it("exposes IDVerification in the list filter alongside FaceId (unified visibility)", () => {
    const values = FILTERABLE_PROVIDER_CATEGORIES.map((c) => c.value);
    expect(values).toContain("IDVerification");
    expect(values).toContain("FaceId");
  });

  it("is a subset of PROVIDER_CATEGORIES", () => {
    const allValues = new Set(PROVIDER_CATEGORIES.map((c) => c.value));
    for (const cat of FILTERABLE_PROVIDER_CATEGORIES) {
      expect(allValues.has(cat.value)).toBe(true);
    }
  });
});

describe("PROVIDER_FILTER_CATEGORIES", () => {
  it("aliases the filterable set for the list page", () => {
    expect(PROVIDER_FILTER_CATEGORIES).toEqual(FILTERABLE_PROVIDER_CATEGORIES);
  });
});

describe("PROVIDER_FILTER_TYPES", () => {
  it("contains common provider types", () => {
    const values = PROVIDER_FILTER_TYPES.map((t) => t.value);
    expect(values).toContain("Google");
    expect(values).toContain("GitHub");
    expect(values).toContain("Custom");
  });

  it("keeps the managed LDAP type so historical rows stay filterable", () => {
    expect(PROVIDER_FILTER_TYPES.map((t) => t.value)).toContain("LDAP");
  });
});

describe("IDVERIFICATION_FILTER_TYPES", () => {
  it("derives IDV types from the create-form definitions, not social types", () => {
    const values = IDVERIFICATION_FILTER_TYPES.map((t) => t.value);
    expect(values).toEqual([
      "Jumio",
      "AliyunIdv",
      "TencentIdv",
      "Onfido",
      "Veriff",
      "Persona",
      "Custom",
    ]);
    expect(values).not.toContain("Google");
    expect(values).not.toContain("GitHub");
  });
});

// ─── Managed provider kinds (P2 closure) ────────────────────────

describe("MANAGED_PROVIDER_KINDS", () => {
  it("resolves the UI labels and backend kinds to the same closure", () => {
    for (const kind of MANAGED_PROVIDER_KINDS) {
      expect(isManagedProviderKind(kind)).toBe(true);
    }
    for (const label of [
      "LDAP",
      "reCAPTCHA",
      "reCAPTCHAv3",
      "hCaptcha",
      "Turnstile",
      "GeeTest",
      "AliyunCaptcha",
      "TencentCaptcha",
      "FaceId",
      "AliyunFaceBody",
    ]) {
      expect(isManagedProviderKind(label)).toBe(true);
    }
  });

  it("does not treat non-managed categories or types as managed", () => {
    for (const value of ["OAuth", "GitHub", "MFA", "TOTP", "Scan", "Custom"]) {
      expect(isManagedProviderKind(value)).toBe(false);
    }
  });

  it("is absent from every creatable category and non-Captcha providerType option", () => {
    const creatableCategories = CREATABLE_PROVIDER_CATEGORIES.map((c) => c.value);
    for (const value of creatableCategories) {
      expect(isManagedProviderKind(value)).toBe(false);
    }

    const creatableTypes = CREATABLE_PROVIDER_CATEGORIES.filter(
      // Captcha 是唯一豁免：选中后前端分流提交专用端点，managed kind 合法可选。
      ({ value }) => value !== "Captcha",
    ).flatMap(({ value }) => {
      const { result, unmount } = renderHook(() => useProviderCategories({ category: value }));
      const options = result.current.providerTypeOptions.map((option) => option.value);
      unmount();
      return options;
    });

    expect(creatableTypes.length).toBeGreaterThan(0);
    for (const value of creatableTypes) {
      expect(isManagedProviderKind(value)).toBe(false);
    }
  });

  it("keeps Web3 types available for editing historical rows", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "Web3" }));
    const values = result.current.providerTypeOptions.map((option) => option.value);
    expect(values).toEqual(["MetaMask", "Custom"]);
  });

  it("keeps all eleven captcha types selectable (dedicated-endpoint routing)", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "Captcha" }));
    const values = result.current.providerTypeOptions.map((option) => option.value);
    expect(values).toEqual([
      "Default",
      "reCAPTCHA",
      "reCAPTCHAv3",
      "hCaptcha",
      "Turnstile",
      "GeeTest",
      "AliyunCaptcha",
      "TencentCaptcha",
      "reCAPTCHAEnterprise",
      "NetEaseYidun",
      "BaiduCaptcha",
      "GoCaptcha",
    ]);
  });
});

// ─── Captcha UI value ↔ backend kind mapping ────────────────────

describe("captchaUITypeToKind / captchaKindToUIType", () => {
  const pairs: ReadonlyArray<[string, string]> = [
    ["Default", "default"],
    ["reCAPTCHA", "recaptcha"],
    ["reCAPTCHAv3", "recaptcha_v3"],
    ["reCAPTCHAEnterprise", "recaptcha_enterprise"],
    ["hCaptcha", "hcaptcha"],
    ["Turnstile", "turnstile"],
    ["GeeTest", "geetest"],
    ["AliyunCaptcha", "aliyun_captcha"],
    ["TencentCaptcha", "tencent_captcha"],
    ["NetEaseYidun", "netease_yidun"],
    ["BaiduCaptcha", "baidu_captcha"],
    ["GoCaptcha", "go_captcha"],
  ];

  it("maps every UI value to its backend kind and back", () => {
    for (const [ui, kind] of pairs) {
      expect(captchaUITypeToKind(ui)).toBe(kind);
      expect(captchaKindToUIType(kind)).toBe(ui);
    }
  });

  it("falls back to the input for unknown values", () => {
    expect(captchaUITypeToKind("SomethingNew")).toBe("SomethingNew");
    expect(captchaKindToUIType("something_new")).toBe("something_new");
  });
});

// ─── categoryCredentialFields ───────────────────────────────────

describe("categoryCredentialFields", () => {
  it("hides both credential inputs for Captcha (edited as siteKey/secretKey)", () => {
    expect(categoryCredentialFields("Captcha")).toEqual({
      clientId: false,
      clientSecret: false,
    });
  });

  it("keeps only clientId for Scan (clientSecret is the dynamic App Secret)", () => {
    expect(categoryCredentialFields("Scan")).toEqual({
      clientId: true,
      clientSecret: false,
    });
  });

  it("hides both credential inputs for OAuth/OIDC (edited in the dynamic section)", () => {
    expect(categoryCredentialFields("OAuth")).toEqual({
      clientId: false,
      clientSecret: false,
    });
    expect(categoryCredentialFields("OIDC")).toEqual({
      clientId: false,
      clientSecret: false,
    });
  });

  it("hides both credential inputs for unrelated categories", () => {
    expect(categoryCredentialFields("Email")).toEqual({
      clientId: false,
      clientSecret: false,
    });
    expect(categoryCredentialFields("Unknown")).toEqual({
      clientId: false,
      clientSecret: false,
    });
  });
});

// ─── isAuthSwitchApplicable ─────────────────────────────────────

describe("isAuthSwitchApplicable", () => {
  it("keeps OAuth/OIDC-only switches off SAML", () => {
    for (const field of ["enablePkce", "enableProxy", "httpHeaders"] as const) {
      expect(isAuthSwitchApplicable(field, "OAuth", "GitHub")).toBe(true);
      expect(isAuthSwitchApplicable(field, "OIDC", "Keycloak")).toBe(true);
      expect(isAuthSwitchApplicable(field, "SAML", "SAML 2.0")).toBe(false);
      expect(isAuthSwitchApplicable(field, "Email", "SMTP")).toBe(false);
    }
  });

  it("keeps OAuth/OIDC-only switches off SAML and non-identity categories", () => {
    for (const field of ["enablePkce", "enableProxy", "httpHeaders", "customLogoutUrl"] as const) {
      expect(isAuthSwitchApplicable(field, "OAuth", "GitHub")).toBe(true);
      expect(isAuthSwitchApplicable(field, "OIDC", "Keycloak")).toBe(true);
      expect(isAuthSwitchApplicable(field, "SAML", "SAML 2.0")).toBe(false);
      expect(isAuthSwitchApplicable(field, "Email", "SMTP")).toBe(false);
    }
  });

  it("keeps emailRegex for every category that logs in through LoginWithProvider", () => {
    expect(isAuthSwitchApplicable("emailRegex", "OAuth", "GitHub")).toBe(true);
    expect(isAuthSwitchApplicable("emailRegex", "OIDC", "Keycloak")).toBe(true);
    expect(isAuthSwitchApplicable("emailRegex", "SAML", "SAML 2.0")).toBe(true);
    expect(isAuthSwitchApplicable("emailRegex", "SMS", "TwilioSMS")).toBe(false);
  });

  it("shows requireMessageAuthenticator only for the RADIUS MFA provider", () => {
    expect(isAuthSwitchApplicable("requireMessageAuthenticator", "MFA", "RADIUS")).toBe(true);
    expect(isAuthSwitchApplicable("requireMessageAuthenticator", "MFA", "radius")).toBe(true);
    expect(isAuthSwitchApplicable("requireMessageAuthenticator", "MFA", "TOTP")).toBe(false);
    expect(isAuthSwitchApplicable("requireMessageAuthenticator", "SAML", "SAML 2.0")).toBe(false);
  });

  it("shows SAML switches only for the SAML category", () => {
    for (const field of ["enableSignAuthnRequest", "cert"] as const) {
      expect(isAuthSwitchApplicable(field, "SAML", "SAML 2.0")).toBe(true);
      expect(isAuthSwitchApplicable(field, "OAuth", "GitHub")).toBe(false);
      expect(isAuthSwitchApplicable(field, "OIDC", "Keycloak")).toBe(false);
    }
  });

  it("keeps state (cross-category gate) and unconfirmed customLogo visible everywhere", () => {
    for (const category of ["OAuth", "SAML", "Email", "Storage", "Unknown"]) {
      expect(isAuthSwitchApplicable("state", category)).toBe(true);
      expect(isAuthSwitchApplicable("customLogo", category)).toBe(true);
    }
  });
});

// ─── categorySupportsUserMapping ────────────────────────────────

describe("categorySupportsUserMapping", () => {
  it("returns true for OAuth", () => {
    expect(categorySupportsUserMapping("OAuth")).toBe(true);
  });

  it("returns true for SAML", () => {
    expect(categorySupportsUserMapping("SAML")).toBe(true);
  });

  it("returns true for OIDC", () => {
    expect(categorySupportsUserMapping("OIDC")).toBe(true);
  });

  it("returns false for Email", () => {
    expect(categorySupportsUserMapping("Email")).toBe(false);
  });

  it("returns false for Storage", () => {
    expect(categorySupportsUserMapping("Storage")).toBe(false);
  });

  it("returns false for unknown category", () => {
    expect(categorySupportsUserMapping("Unknown")).toBe(false);
  });
});

// ─── getCategoryColor ───────────────────────────────────────────

describe("getCategoryColor", () => {
  it("returns correct color for OAuth", () => {
    expect(getCategoryColor("OAuth")).toBe("blue");
  });

  it("returns correct color for SAML", () => {
    expect(getCategoryColor("SAML")).toBe("purple");
  });

  it("returns correct color for OIDC", () => {
    expect(getCategoryColor("OIDC")).toBe("cyan");
  });

  it("returns correct color for Email", () => {
    expect(getCategoryColor("Email")).toBe("green");
  });

  it("returns correct color for SMS", () => {
    expect(getCategoryColor("SMS")).toBe("orange");
  });

  it("returns correct color for Storage", () => {
    expect(getCategoryColor("Storage")).toBe("geekblue");
  });

  it("returns correct color for Payment", () => {
    expect(getCategoryColor("Payment")).toBe("gold");
  });

  it("returns correct color for MFA", () => {
    expect(getCategoryColor("MFA")).toBe("red");
  });

  it("returns correct color for FaceId", () => {
    expect(getCategoryColor("FaceId")).toBe("volcano");
  });

  it("returns correct color for Log", () => {
    expect(getCategoryColor("Log")).toBe("default");
  });

  it("returns 'default' for unknown category", () => {
    expect(getCategoryColor("Unknown")).toBe("default");
  });
});

// ─── useProviderCategories hook ─────────────────────────────────

describe("useProviderCategories", () => {
  it("returns categoryOptions matching CREATABLE plus console-redirect categories", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "OAuth" }));
    expect(result.current.categoryOptions).toEqual([
      ...CREATABLE_PROVIDER_CATEGORIES,
      ...CONSOLE_REDIRECT_PROVIDER_CATEGORIES,
    ]);
  });

  // 回归：调用方把 categoryOptions 放进 effect 依赖，若每次 render 都换新引用，
  // 预置类别 effect 会反复重跑并把用户刚选中的 providerType 清成 undefined。
  it("keeps categoryOptions identity stable across re-renders", () => {
    const { result, rerender } = renderHook(({ category }) => useProviderCategories({ category }), {
      initialProps: { category: "OAuth" },
    });
    const first = result.current.categoryOptions;
    rerender({ category: "OAuth" });
    rerender({ category: "Captcha" });
    expect(result.current.categoryOptions).toBe(first);
  });

  it("returns providerTypeOptions for OAuth category", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "OAuth" }));
    const options = result.current.providerTypeOptions;
    expect(options.length).toBeGreaterThan(0);
    // Should include GitHub and Custom
    const values = options.map((o) => o.value);
    expect(values).toContain("GitHub");
    expect(values).toContain("Custom");
  });

  it("returns providerTypeOptions for SAML category", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "SAML" }));
    const values = result.current.providerTypeOptions.map((o) => o.value);
    expect(values).toContain("SAML 2.0");
    expect(values).toContain("Keycloak");
  });

  it("returns default Custom type for unknown category", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "Unknown" }));
    const values = result.current.providerTypeOptions.map((o) => o.value);
    expect(values).toEqual(["Custom"]);
  });

  it("returns credentialFields hiding both inputs for Captcha", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "Captcha" }));
    expect(result.current.credentialFields).toEqual({ clientId: false, clientSecret: false });
  });

  it("returns credentialFields keeping clientId only for Scan", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "Scan" }));
    expect(result.current.credentialFields).toEqual({ clientId: true, clientSecret: false });
  });

  it("returns credentialFields hiding both inputs for OAuth", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "OAuth" }));
    expect(result.current.credentialFields).toEqual({ clientId: false, clientSecret: false });
  });

  it("returns showUserMapping true for OAuth category", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "OAuth" }));
    expect(result.current.showUserMapping).toBe(true);
  });

  it("returns showUserMapping true for SAML category", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "SAML" }));
    expect(result.current.showUserMapping).toBe(true);
  });

  it("returns showUserMapping false for Email category", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "Email" }));
    expect(result.current.showUserMapping).toBe(false);
  });

  it("returns correct categoryColor", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "OAuth" }));
    expect(result.current.categoryColor).toBe("blue");
  });

  it("returns providerTypeOptions for Email category", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "Email" }));
    const values = result.current.providerTypeOptions.map((o) => o.value);
    expect(values).toContain("SMTP");
    expect(values).toContain("SendGrid");
  });

  it("returns providerTypeOptions for Storage category", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "Storage" }));
    const values = result.current.providerTypeOptions.map((o) => o.value);
    expect(values).toContain("AWS S3");
    expect(values).toContain("MinIO");
  });

  it("returns providerTypeOptions for Payment category", () => {
    const { result } = renderHook(() => useProviderCategories({ category: "Payment" }));
    const values = result.current.providerTypeOptions.map((o) => o.value);
    expect(values).toContain("Stripe");
    expect(values).toContain("Alipay");
  });
});
