import { act, render, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { App, Form } from "antd";
import { QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import type { ReactNode } from "react";
import { useSessionStore } from "@/features/session/session-store";
import { createTestQueryClient } from "@/test-utils/render";
import { HttpError } from "@/shared/api/http";
import { queryKeys } from "@/shared/api/query-keys";
import {
  createDefaultApplicationConfigFormValues,
  validateApplicationConfigValues,
} from "./application-payload";
import type { ApplicationConfigFormValues } from "./tabs/types";
import { useApplicationDetailPageController } from "./useApplicationDetailPageController";

const {
  mockCreateApplication,
  mockGetApplication,
  mockUpdateApplication,
  mockUpdateApplicationProviderBindings,
  mockGetApplicationClientCertificates,
  mockRouteParams,
  mockMessage,
} = vi.hoisted(() => ({
  mockCreateApplication: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockGetApplication: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockUpdateApplication: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockUpdateApplicationProviderBindings: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockGetApplicationClientCertificates: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockRouteParams: { applicationId: "new" },
  mockMessage: {
    success: vi.fn<(content: unknown) => void>(),
    error: vi.fn<(content: unknown) => void>(),
    warning: vi.fn<(content: unknown) => void>(),
    info: vi.fn<(content: unknown) => void>(),
  },
}));

vi.mock("antd", async (importOriginal) => {
  const actual = await importOriginal<typeof import("antd")>();
  const AppWithMock = Object.assign(actual.App, {
    useApp: () => ({ message: mockMessage, notification: {}, modal: {} }),
  });
  return { ...actual, App: AppWithMock };
});

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("react-router-dom", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router-dom")>();
  return {
    ...actual,
    useParams: () => mockRouteParams,
    useNavigate: () => vi.fn<(path: string) => void>(),
  };
});

vi.mock("@/shared/api/iam/application", () => ({
  createApplication: (...args: unknown[]) => mockCreateApplication(...args),
  getApplication: (...args: unknown[]) => mockGetApplication(...args),
  updateApplication: (...args: unknown[]) => mockUpdateApplication(...args),
  getApplicationClientCertificates: (...args: unknown[]) =>
    mockGetApplicationClientCertificates(...args),
  rotateApplicationClientSecret: vi.fn<() => Promise<unknown>>(),
  deleteApplication: vi.fn<() => void>(),
  exportApplicationConfig: vi.fn<() => void>(),
  getApplicationDeleteSummary: vi.fn<() => void>(),
  getApplications: vi.fn<() => Promise<unknown>>(async () => []),
  importApplicationConfig: vi.fn<() => void>(),
  updateApplicationProviderBindings: (...args: unknown[]) =>
    mockUpdateApplicationProviderBindings(...args),
}));

vi.mock("@/shared/api/iam/organization", () => ({
  getOrganizationOptions: vi.fn<() => Promise<unknown>>(async () => []),
}));

vi.mock("@/shared/api/iam/group", () => ({
  getGroups: vi.fn<() => Promise<unknown>>(async () => []),
}));

vi.mock("@/shared/api/iam/provider-extended", () => ({
  getProviders: vi.fn<() => Promise<unknown>>(async () => []),
}));

vi.mock("@/shared/hooks/useResourceNames", () => ({
  useOrganizationNames: () => ({ nameMap: new Map() }),
}));

vi.mock("@/shared/hooks/useUnsavedChangesGuard", () => ({
  useUnsavedChangesGuard: () => undefined,
}));

function renderController() {
  const queryClient = createTestQueryClient();
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <MemoryRouter initialEntries={["/applications/new"]}>
        <QueryClientProvider client={queryClient}>
          <App component={false}>{children}</App>
        </QueryClientProvider>
      </MemoryRouter>
    );
  }

  return {
    ...renderHook(() => useApplicationDetailPageController(), { wrapper: Wrapper }),
    invalidateSpy,
  };
}

const HELLO_PEM = "-----BEGIN CERTIFICATE-----\naGVsbG8=\n-----END CERTIFICATE-----";
const HELLO_FINGERPRINT = "LPJNul-wow4m6DsqxbninhsWHlwfp0JecwQzYpOLmCQ";

function createApplicationDetail(overrides: Record<string, unknown> = {}) {
  return {
    id: "app-1",
    organizationId: "org-1",
    slug: "existing-app",
    name: "existing-app",
    owner: "admin",
    displayName: "Existing App",
    category: "",
    appType: "",
    description: "",
    homepageUrl: "",
    domain: "",
    status: "active",
    clientId: "gl_cid_existing",
    tokenEndpointAuthMethod: "none",
    clientSecretSet: false,
    clientSecretMasked: "",
    clientJwks: { kids: [], keys: [] },
    redirectUris: [],
    grantTypes: ["authorization_code"],
    customScopes: [],
    providerIds: [],
    logo: "",
    revision: 3,
    ...overrides,
  };
}

describe("validateApplicationConfigValues", () => {
  const tr = (key: string) => key;

  function apply(values: ReturnType<typeof createDefaultApplicationConfigFormValues>) {
    return validateApplicationConfigValues(tr, values);
  }

  it("blocks a secret submitted while the public client method is selected", () => {
    const values = createDefaultApplicationConfigFormValues("org-1");
    values.tokenEndpointAuthMethod = "none";
    values.clientSecret = "should-not-be-sent";
    expect(apply(values)).toContain("application.validation.secretNotAllowedForMethod");
  });

  it("defaults a new application to the only redeemable public-client combination", () => {
    expect(createDefaultApplicationConfigFormValues("org-1").grantTypes).toEqual([
      "authorization_code",
    ]);
  });

  it("requires the authorization_code grant for the public client method", () => {
    const values = createDefaultApplicationConfigFormValues("org-1");
    values.tokenEndpointAuthMethod = "none";
    values.grantTypes = ["refresh_token"];
    expect(apply(values)).toContain("application.validation.noneRequiresAuthorizationCode");
  });

  it("forbids the client_credentials grant for the public client method", () => {
    const values = createDefaultApplicationConfigFormValues("org-1");
    values.tokenEndpointAuthMethod = "none";
    values.grantTypes = ["authorization_code", "client_credentials"];
    expect(apply(values)).toContain("application.validation.noneForbidsClientCredentials");
  });

  it("keeps a legacy public-client application editable while the combination is untouched", () => {
    const values = createDefaultApplicationConfigFormValues("org-1");
    values.tokenEndpointAuthMethod = "none";
    values.grantTypes = ["client_credentials"];
    expect(
      validateApplicationConfigValues(tr, values, {
        persistedAuthMethod: "none",
        persistedGrantTypes: ["client_credentials"],
      }),
    ).toEqual([]);
  });

  it("re-arms the public-client rule when the save touches the grant types", () => {
    const values = createDefaultApplicationConfigFormValues("org-1");
    values.tokenEndpointAuthMethod = "none";
    values.grantTypes = ["client_credentials"];
    const errors = validateApplicationConfigValues(tr, values, {
      persistedAuthMethod: "none",
      persistedGrantTypes: ["client_credentials", "authorization_code"],
    });
    expect(errors).toContain("application.validation.noneRequiresAuthorizationCode");
    expect(errors).toContain("application.validation.noneForbidsClientCredentials");
  });

  it("allows a secret for client_secret_post and client_secret_basic", () => {
    for (const method of ["client_secret_post", "client_secret_basic"] as const) {
      const values = createDefaultApplicationConfigFormValues("org-1");
      values.tokenEndpointAuthMethod = method;
      values.clientSecret = "secret";
      expect(apply(values)).toEqual([]);
    }
  });

  it("requires a JWKS when switching into private_key_jwt", () => {
    const values = createDefaultApplicationConfigFormValues("org-1");
    values.tokenEndpointAuthMethod = "private_key_jwt";
    expect(apply(values)).toContain("application.validation.jwksRequired");
    expect(validateApplicationConfigValues(tr, values, { requireClientJwks: true })).toContain(
      "application.validation.jwksRequired",
    );
  });

  it("allows an empty JWKS textarea when the client already uses private_key_jwt", () => {
    const values = createDefaultApplicationConfigFormValues("org-1");
    values.tokenEndpointAuthMethod = "private_key_jwt";
    expect(validateApplicationConfigValues(tr, values, { requireClientJwks: false })).toEqual([]);
  });

  it("rejects private key material inside a JWKS", () => {
    const values = createDefaultApplicationConfigFormValues("org-1");
    values.tokenEndpointAuthMethod = "private_key_jwt";
    values.jwks = JSON.stringify({
      keys: [{ kty: "RSA", kid: "k1", n: "n", e: "AQAB", d: "leak" }],
    });
    expect(apply(values)).toContain("application.validation.jwksKeyPrivate");
  });

  it("rejects a JWKS key without kid or with an unsupported kty", () => {
    const values = createDefaultApplicationConfigFormValues("org-1");
    values.tokenEndpointAuthMethod = "private_key_jwt";
    values.jwks = JSON.stringify({ keys: [{ kty: "oct", n: "n", e: "AQAB" }] });
    const errors = apply(values);
    expect(errors).toContain("application.validation.jwksKidRequired");
    expect(errors).toContain("application.validation.jwksKeyType");
  });

  it("accepts a public RSA JWKS for private_key_jwt", () => {
    const values = createDefaultApplicationConfigFormValues("org-1");
    values.tokenEndpointAuthMethod = "private_key_jwt";
    values.jwks = JSON.stringify({
      keys: [{ kty: "RSA", kid: "k1", alg: "RS256", n: "n", e: "AQAB" }],
    });
    expect(apply(values)).toEqual([]);
  });

  it("requires a certificate body for mTLS methods", () => {
    const values = createDefaultApplicationConfigFormValues("org-1");
    values.tokenEndpointAuthMethod = "tls_client_auth";
    values.mtlsCertificates = [{ certificate: "  " }];
    expect(apply(values)).toContain("application.validation.certificateRequired");

    values.mtlsCertificates = [{ certificate: HELLO_PEM }];
    expect(apply(values)).toEqual([]);
  });

  it("requires at least one certificate when switching into an mTLS method", () => {
    for (const method of ["tls_client_auth", "self_signed_tls_client_auth"] as const) {
      const values = createDefaultApplicationConfigFormValues("org-1");
      values.tokenEndpointAuthMethod = method;
      expect(apply(values)).toContain("application.validation.mtlsCertificateRequired");
    }
  });

  it("accepts an empty certificate editor only for an unchanged mTLS client with registered certificates", () => {
    const values = createDefaultApplicationConfigFormValues("org-1");
    values.tokenEndpointAuthMethod = "self_signed_tls_client_auth";
    expect(
      validateApplicationConfigValues(tr, values, {
        persistedAuthMethod: "self_signed_tls_client_auth",
        existingCertificateCount: 1,
      }),
    ).toEqual([]);
    // Switching between the two mTLS variants needs the material again.
    expect(
      validateApplicationConfigValues(tr, values, {
        persistedAuthMethod: "tls_client_auth",
        existingCertificateCount: 1,
      }),
    ).toContain("application.validation.mtlsCertificateRequired");
    // Same method but nothing registered: still unusable at the token endpoint.
    expect(
      validateApplicationConfigValues(tr, values, {
        persistedAuthMethod: "self_signed_tls_client_auth",
        existingCertificateCount: 0,
      }),
    ).toContain("application.validation.mtlsCertificateRequired");
  });

  it("blocks JWKS and certificates on methods that do not use them", () => {
    const values = createDefaultApplicationConfigFormValues("org-1");
    values.tokenEndpointAuthMethod = "client_secret_basic";
    values.jwks = JSON.stringify({ keys: [{ kty: "RSA", kid: "k1", n: "n", e: "AQAB" }] });
    values.mtlsCertificates = [{ certificate: "pem" }];
    const errors = apply(values);
    expect(errors).toContain("application.validation.jwksNotAllowedForMethod");
    expect(errors).toContain("application.validation.fingerprintsNotAllowedForMethod");
  });
});

describe("useApplicationDetailPageController create flow", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockRouteParams.applicationId = "new";
    mockGetApplicationClientCertificates.mockResolvedValue([]);
    useSessionStore.setState({
      account: null,
      organization: {
        id: "org-1",
        name: "built-in",
        displayName: "Built-in",
        navItems: [],
        userNavItems: [],
      },
      impersonatedUser: null,
    });
  });

  it("invalidates the list and shows an identifiable message when POST succeeds but the follow-up GET fails", async () => {
    // POST 201 已落库；紧随其后的 GET 500 只影响扩展配置回填，不能掩盖
    // “记录已创建、列表仍是旧数据”的可见性缺口，也不能退化成通用 500 文案。
    mockCreateApplication.mockResolvedValue({ id: "app-1", status: "created" });
    mockGetApplication.mockRejectedValue(
      new HttpError(500, { error: { code: "internal", message: "boom" } }),
    );

    const values = createDefaultApplicationConfigFormValues("org-1");
    values.slug = "l4-create";
    values.displayName = "L4 Create";

    const { result, invalidateSpy } = renderController();

    await act(async () => {
      await expect(
        result.current.createApplicationMutation.mutateAsync({ values, clientCertificates: [] }),
      ).rejects.toMatchObject({
        name: "ApplicationCreatedConfigError",
        message: "application.createdButConfigFailed",
      });
    });

    expect(mockCreateApplication).toHaveBeenCalledTimes(1);
    expect(mockUpdateApplication).not.toHaveBeenCalled();
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.applications.paginated() });
    expect(mockMessage.error).toHaveBeenCalledWith("application.createdButConfigFailed");
  });

  it("calls createApplication once when two submissions are triggered concurrently", async () => {
    mockCreateApplication.mockResolvedValue({ id: "app-1", status: "created" });
    mockGetApplication.mockRejectedValue(
      new HttpError(500, { error: { code: "internal", message: "boom" } }),
    );

    const values = createDefaultApplicationConfigFormValues("org-1");
    values.slug = "l4-double";
    values.displayName = "L4 Double";

    const { result } = renderController();

    await act(async () => {
      const input = { values, clientCertificates: [] };
      const first = result.current.createApplicationMutation.mutateAsync(input);
      const second = result.current.createApplicationMutation.mutateAsync(input);
      await Promise.allSettled([first, second]);
    });

    expect(mockCreateApplication).toHaveBeenCalledTimes(1);
  });

  it("treats a 409 as self-duplicate after this session already created the slug", async () => {
    const values = createDefaultApplicationConfigFormValues("org-1");
    values.slug = "l4-self";
    values.displayName = "L4 Self";

    mockCreateApplication.mockResolvedValueOnce({ id: "app-1", status: "created" });
    mockGetApplication.mockRejectedValueOnce(
      new HttpError(500, { error: { code: "internal", message: "boom" } }),
    );

    const { result, invalidateSpy } = renderController();

    // 第一次：POST 201，但详情加载失败——记录已存在。
    await act(async () => {
      await expect(
        result.current.createApplicationMutation.mutateAsync({ values, clientCertificates: [] }),
      ).rejects.toMatchObject({ name: "ApplicationCreatedConfigError" });
    });

    // 第二次：用户原样重试，POST 命中自己刚创建的 slug → 409。
    const duplicate = new HttpError(409, {
      error: { code: "already_exists", message: "application slug already exists in organization" },
    });
    duplicate.code = "already_exists";
    mockCreateApplication.mockRejectedValueOnce(duplicate);

    await act(async () => {
      await expect(
        result.current.createApplicationMutation.mutateAsync({ values, clientCertificates: [] }),
      ).rejects.toMatchObject({
        name: "SelfCreatedDuplicateError",
        message: "application.duplicateOwnCreated",
      });
    });

    expect(mockCreateApplication).toHaveBeenCalledTimes(2);
    expect(mockMessage.warning).toHaveBeenCalledWith("application.duplicateOwnCreated");
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.applications.paginated() });
  });

  it("sends mTLS certificates inside the create request instead of a follow-up call", async () => {
    mockCreateApplication.mockResolvedValue({ id: "app-1", status: "created" });
    mockGetApplication.mockResolvedValue(
      createApplicationDetail({ tokenEndpointAuthMethod: "tls_client_auth" }),
    );

    const values = createDefaultApplicationConfigFormValues("org-1");
    values.slug = "l4-mtls";
    values.displayName = "L4 mTLS";
    values.tokenEndpointAuthMethod = "tls_client_auth";
    values.mtlsCertificates = [{ certificate: HELLO_PEM }];
    const clientCertificates = [
      { certificate: HELLO_PEM, fingerprintSha256: HELLO_FINGERPRINT, selfSigned: false },
    ];

    const { result } = renderController();

    await act(async () => {
      await result.current.createApplicationMutation.mutateAsync({ values, clientCertificates });
    });

    expect(mockCreateApplication).toHaveBeenCalledTimes(1);
    const postPayload = mockCreateApplication.mock.calls[0][0] as Record<string, unknown>;
    expect(postPayload.tokenEndpointAuthMethod).toBe("tls_client_auth");
    expect(postPayload.clientCertificates).toEqual(clientCertificates);
    // The follow-up configuration PATCH must not repeat the certificate write.
    const patchPayload = mockUpdateApplication.mock.calls[0][1] as Record<string, unknown>;
    expect(patchPayload).not.toHaveProperty("clientCertificates");
  });
});

describe("useApplicationDetailPageController edit flow", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockRouteParams.applicationId = "app-1";
    mockGetApplicationClientCertificates.mockResolvedValue([]);
    useSessionStore.setState({
      account: null,
      organization: {
        id: "org-1",
        name: "built-in",
        displayName: "Built-in",
        navItems: [],
        userNavItems: [],
      },
      impersonatedUser: null,
    });
  });

  it("sends certificates built from the editor inside the configuration PATCH", async () => {
    mockGetApplication.mockResolvedValue(
      createApplicationDetail({
        tokenEndpointAuthMethod: "self_signed_tls_client_auth",
        grantTypes: ["authorization_code"],
      }),
    );
    mockUpdateApplication.mockResolvedValue(undefined);

    const values = createDefaultApplicationConfigFormValues("org-1");
    values.slug = "existing-app";
    values.displayName = "Existing App";
    values.tokenEndpointAuthMethod = "self_signed_tls_client_auth";
    values.mtlsCertificates = [{ certificate: HELLO_PEM }];

    const { result } = renderController();
    await waitFor(() => expect(result.current.detail).not.toBeNull());

    await act(async () => {
      await result.current.handleSave(values);
    });

    expect(mockUpdateApplication).toHaveBeenCalledTimes(1);
    const [id, payload] = mockUpdateApplication.mock.calls[0] as [string, Record<string, unknown>];
    expect(id).toBe("app-1");
    expect(payload.clientCertificates).toEqual([
      { certificate: HELLO_PEM, fingerprintSha256: HELLO_FINGERPRINT, selfSigned: true },
    ]);
  });

  // FE-B-01：lazy 分区未挂载时壳只回传已注册字段；保存不得把未挂载分区的
  // 既有配置（定制 CSS/SAML/IP 白名单/登录方式/token 属性）主动清空。
  it("preserves unmounted lazy-section config when the shell submits partial values", async () => {
    mockGetApplication.mockResolvedValue(
      createApplicationDetail({
        formCss: ".login-content { color: red; }",
        formCssMobile: ".login-content { color: blue; }",
        headerHtml: "<header>kept</header>",
        ipWhitelist: "203.0.113.0/24",
        ipBlacklist: "198.51.100.7",
        samlMetadata: "<EntityDescriptor/>",
        signinMethods: ["Password", "FaceID"],
        tokenAttributes: [{ name: "email", sourceField: "profile.email" }],
      }),
    );
    mockUpdateApplication.mockResolvedValue(undefined);

    const { result } = renderController();
    await waitFor(() => expect(result.current.detail).not.toBeNull());

    // 模拟冷缓存下只有 Basic 分区挂载：validateFields() 只返回这些字段。
    const partialValues = {
      slug: "existing-app",
      displayName: "Renamed App",
      status: "active",
    } as ApplicationConfigFormValues;

    await act(async () => {
      await result.current.handleSave(partialValues);
    });

    expect(mockUpdateApplication).toHaveBeenCalledTimes(1);
    const [, payload] = mockUpdateApplication.mock.calls[0] as [string, Record<string, unknown>];
    expect(payload.displayName).toBe("Renamed App");
    expect(payload.formCss).toBe(".login-content { color: red; }");
    expect(payload.formCssMobile).toBe(".login-content { color: blue; }");
    expect(payload.headerHtml).toBe("<header>kept</header>");
    expect(payload.ipWhitelist).toBe("203.0.113.0/24");
    expect(payload.ipBlacklist).toBe("198.51.100.7");
    expect(payload.samlMetadata).toBe("<EntityDescriptor/>");
    expect(payload.signinMethods).toEqual(["Password", "FaceID"]);
    expect(payload.tokenAttributes).toEqual([{ name: "email", sourceField: "profile.email" }]);
  });

  it("refuses to switch into an mTLS method without certificates", async () => {
    mockGetApplication.mockResolvedValue(createApplicationDetail());
    mockUpdateApplication.mockResolvedValue(undefined);

    const values = createDefaultApplicationConfigFormValues("org-1");
    values.slug = "existing-app";
    values.displayName = "Existing App";
    values.tokenEndpointAuthMethod = "tls_client_auth";

    const { result } = renderController();
    await waitFor(() => expect(result.current.detail).not.toBeNull());

    await act(async () => {
      await expect(result.current.handleSave(values)).rejects.toThrow(
        "application.validation.mtlsCertificateRequired",
      );
    });

    expect(mockUpdateApplication).not.toHaveBeenCalled();
  });

  it("keeps registered certificates when an unchanged mTLS client saves with an empty editor", async () => {
    mockGetApplication.mockResolvedValue(
      createApplicationDetail({
        tokenEndpointAuthMethod: "tls_client_auth",
        grantTypes: ["authorization_code"],
      }),
    );
    mockGetApplicationClientCertificates.mockResolvedValue([
      {
        fingerprintSha256: HELLO_FINGERPRINT,
        notAfter: null,
        selfSigned: false,
        status: "active",
      },
    ]);
    mockUpdateApplication.mockResolvedValue(undefined);

    const values = createDefaultApplicationConfigFormValues("org-1");
    values.slug = "existing-app";
    values.displayName = "Existing App";
    values.tokenEndpointAuthMethod = "tls_client_auth";
    values.mtlsCertificates = [];

    const { result } = renderController();
    await waitFor(() => expect(result.current.existingClientCertificates).toHaveLength(1));

    await act(async () => {
      await result.current.handleSave(values);
    });

    expect(mockUpdateApplication).toHaveBeenCalledTimes(1);
    const payload = mockUpdateApplication.mock.calls[0][1] as Record<string, unknown>;
    expect(payload).not.toHaveProperty("clientCertificates");
  });

  it("derives changed provider bindings from provider items after the configuration PATCH", async () => {
    mockGetApplication.mockResolvedValue(createApplicationDetail({ providerIds: ["provider-a"] }));
    mockUpdateApplication.mockResolvedValue(undefined);
    mockUpdateApplicationProviderBindings.mockResolvedValue(undefined);

    const values = createDefaultApplicationConfigFormValues("org-1");
    values.slug = "existing-app";
    values.displayName = "Existing App";
    values.providerItems = [
      { provider: "provider-b", canSignUp: false, canSignIn: true, canUnlink: false },
    ];

    const { result } = renderController();
    await waitFor(() => expect(result.current.detail).not.toBeNull());

    await act(async () => {
      await result.current.handleSave(values);
    });

    expect(mockUpdateApplication).toHaveBeenCalledTimes(1);
    expect(mockUpdateApplicationProviderBindings).toHaveBeenCalledWith(
      "app-1",
      ["provider-b"],
      "org-1",
    );
    expect(mockUpdateApplication.mock.invocationCallOrder[0]).toBeLessThan(
      mockUpdateApplicationProviderBindings.mock.invocationCallOrder[0],
    );
  });

  it("derives the registry from provider items even when a stale providerIds value remains", async () => {
    mockGetApplication.mockResolvedValue(createApplicationDetail({ providerIds: ["provider-a"] }));
    mockUpdateApplication.mockResolvedValue(undefined);
    mockUpdateApplicationProviderBindings.mockResolvedValue(undefined);

    const values = createDefaultApplicationConfigFormValues("org-1");
    values.slug = "existing-app";
    values.displayName = "Existing App";
    values.providerIds = ["provider-a"];
    values.providerItems = [
      { provider: "provider-b", canSignUp: false, canSignIn: true, canUnlink: false },
      { provider: "provider-c", canSignUp: true, canSignIn: true, canUnlink: true },
    ];

    const { result } = renderController();
    await waitFor(() => expect(result.current.detail).not.toBeNull());

    await act(async () => {
      await result.current.handleSave(values);
    });

    expect(mockUpdateApplicationProviderBindings).toHaveBeenCalledWith(
      "app-1",
      ["provider-b", "provider-c"],
      "org-1",
    );
  });

  it("keeps every provider item field in the payload while deriving the registry", async () => {
    mockGetApplication.mockResolvedValue(createApplicationDetail({ providerIds: [] }));
    mockUpdateApplication.mockResolvedValue(undefined);
    mockUpdateApplicationProviderBindings.mockResolvedValue(undefined);

    const items = [
      {
        provider: "provider-b",
        canSignUp: true,
        canSignIn: true,
        canUnlink: false,
        alertType: "warning",
        prompt: "Use SSO",
        bindingRule: ["Email"],
        countryCodes: ["+86"],
        prompted: true,
        signupGroup: "staff",
        rule: "email-domain",
      },
    ];

    const values = createDefaultApplicationConfigFormValues("org-1");
    values.slug = "existing-app";
    values.displayName = "Existing App";
    values.providerItems = items;

    const { result } = renderController();
    await waitFor(() => expect(result.current.detail).not.toBeNull());

    await act(async () => {
      await result.current.handleSave(values);
    });

    const payload = mockUpdateApplication.mock.calls[0][1] as Record<string, unknown>;
    expect(payload.providerItems).toEqual(items);
    expect(mockUpdateApplicationProviderBindings).toHaveBeenCalledWith(
      "app-1",
      ["provider-b"],
      "org-1",
    );
  });

  it("skips the bindings write when the provider items match the registry", async () => {
    mockGetApplication.mockResolvedValue(createApplicationDetail({ providerIds: ["provider-a"] }));
    mockUpdateApplication.mockResolvedValue(undefined);

    const values = createDefaultApplicationConfigFormValues("org-1");
    values.slug = "existing-app";
    values.displayName = "Existing App";
    values.providerItems = [
      { provider: "provider-a", canSignUp: false, canSignIn: true, canUnlink: false },
    ];

    const { result } = renderController();
    await waitFor(() => expect(result.current.detail).not.toBeNull());

    await act(async () => {
      await result.current.handleSave(values);
    });

    expect(mockUpdateApplication).toHaveBeenCalledTimes(1);
    expect(mockUpdateApplicationProviderBindings).not.toHaveBeenCalled();
  });

  it("reports a bindings-only failure without discarding the saved configuration", async () => {
    mockGetApplication.mockResolvedValue(createApplicationDetail({ providerIds: ["provider-a"] }));
    mockUpdateApplication.mockResolvedValue(undefined);
    mockUpdateApplicationProviderBindings.mockRejectedValue(
      new HttpError(500, { error: { code: "internal", message: "boom" } }),
    );

    const values = createDefaultApplicationConfigFormValues("org-1");
    values.slug = "existing-app";
    values.displayName = "Existing App";
    values.providerItems = [
      { provider: "provider-b", canSignUp: false, canSignIn: true, canUnlink: false },
    ];

    const { result, invalidateSpy } = renderController();
    await waitFor(() => expect(result.current.detail).not.toBeNull());

    await act(async () => {
      await expect(result.current.handleSave(values)).rejects.toThrow(
        "application.bindingsPartialFailure",
      );
    });

    await waitFor(() =>
      expect(mockMessage.warning).toHaveBeenCalledWith("application.bindingsPartialFailure"),
    );
    expect(mockMessage.success).not.toHaveBeenCalled();
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: queryKeys.applications.detail("app-1"),
    });
  });

  it("marks the offending form field red when the PATCH names it in the message", async () => {
    mockGetApplication.mockResolvedValue(createApplicationDetail());
    const rejected = new HttpError(422, undefined, "displayName must not be blank");
    rejected.code = "unprocessable_entity";
    mockUpdateApplication.mockRejectedValue(rejected);

    const { result } = renderController();
    await waitFor(() => expect(result.current.detail).not.toBeNull());

    const { configForm } = result.current;
    function Probe() {
      return (
        <Form form={configForm}>
          <Form.Item name="displayName">
            <input />
          </Form.Item>
          <Form.Item name="slug">
            <input />
          </Form.Item>
        </Form>
      );
    }
    render(<Probe />);

    const values = createDefaultApplicationConfigFormValues("org-1");
    values.slug = "existing-app";
    values.displayName = "";

    await act(async () => {
      await expect(result.current.handleSave(values)).rejects.toThrow(
        "displayName must not be blank",
      );
    });

    // The toast alone is not enough: the named field must carry the inline
    // red error so the user can see which control to fix.
    const entry = configForm.getFieldsError().find((item) => String(item.name) === "displayName");
    expect(entry?.errors).toEqual(["common:serverError.invalidField"]);
    const untouched = configForm.getFieldsError().find((item) => String(item.name) === "slug");
    expect(untouched?.errors).toEqual([]);
  });
});
