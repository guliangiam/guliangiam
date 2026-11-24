import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { FormInstance } from "antd";
import type { ReactNode } from "react";
import type { QueryClient } from "@tanstack/react-query";
import { useSessionStore } from "@/features/session/session-store";
import { HttpError } from "@/shared/api/http";
import { createTestQueryClient, createTestWrapper } from "@/test-utils/render";
import ProviderDetailPage from "./ProviderDetailPage";

const {
  mockGetProvider,
  mockCreateProvider,
  mockUpdateProvider,
  mockEnableProvider,
  mockDeleteProvider,
  mockTestProviderConnection,
  mockGetOrganizationOptions,
  mockHasAction,
  mockHasResource,
  mockMessage,
  mockNavigate,
  routeState,
} = vi.hoisted(() => ({
  mockGetProvider: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockCreateProvider: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockUpdateProvider: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockEnableProvider: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockDeleteProvider: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockTestProviderConnection: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockGetOrganizationOptions: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockHasAction: vi.fn<(action: string) => boolean>(),
  mockHasResource: vi.fn<(resource: string) => boolean>(),
  mockMessage: {
    success: vi.fn<(...args: unknown[]) => void>(),
    error: vi.fn<(...args: unknown[]) => void>(),
    warning: vi.fn<(...args: unknown[]) => void>(),
    info: vi.fn<(...args: unknown[]) => void>(),
  },
  mockNavigate: vi.fn<(to: string) => void>(),
  routeState: { providerId: "provider-1", search: "" },
}));

vi.mock("@/shared/api/iam/provider-extended", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/iam/provider-extended")>();
  return {
    ...actual,
    getProvider: (...args: unknown[]) => mockGetProvider(...args),
    createProvider: (...args: unknown[]) => mockCreateProvider(...args),
    updateProvider: (...args: unknown[]) => mockUpdateProvider(...args),
    enableProvider: (...args: unknown[]) => mockEnableProvider(...args),
    deleteProvider: (...args: unknown[]) => mockDeleteProvider(...args),
    testProviderConnection: (...args: unknown[]) => mockTestProviderConnection(...args),
  };
});

vi.mock("@/shared/api/iam/organization", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/iam/organization")>();
  return {
    ...actual,
    getOrganizationOptions: (...args: unknown[]) => mockGetOrganizationOptions(...args),
  };
});

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    hasAction: mockHasAction,
    hasResource: mockHasResource,
    isLoading: false,
  }),
}));

vi.mock("react-router-dom", () => ({
  useNavigate: () => mockNavigate,
  useParams: () => ({ providerId: routeState.providerId }),
  useSearchParams: () => [new URLSearchParams(routeState.search)],
  Link: ({ to, children }: { to: string; children: ReactNode }) => <a href={to}>{children}</a>,
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("antd", async (importOriginal) => {
  const actual = await importOriginal<typeof import("antd")>();
  const AppWithMock = Object.assign(actual.App, {
    useApp: () => ({ message: mockMessage, notification: {}, modal: {} }),
  });
  return { ...actual, App: AppWithMock };
});

vi.mock("@/shared/ui/ResourceFormPage", async () => {
  const { Button, Form } = await import("antd");
  const { isHttpError } = await import("@/shared/api/http");
  const { applyServerFieldErrors, buildServerFieldResolver } =
    await import("@/shared/lib/server-error");
  const t = ((key: string) => key) as unknown as import("i18next").TFunction;
  return {
    FORM_ITEM_FULL_WIDTH_CLASS: "gl-form-item-full",
    ResourceFormPage: (props: {
      title?: string;
      children?: ReactNode;
      extra?: ReactNode;
      form?: FormInstance;
      onFinish?: (values: unknown) => void | Promise<void>;
      onCancel?: () => void;
      saving?: boolean;
      fieldAliases?: Record<string, string>;
      fieldLabels?: Record<string, string>;
    }) => {
      const { title, children, extra, form, onFinish, onCancel, saving } = props;
      return (
        <div data-testid="provider-form">
          <h1>{title}</h1>
          <div data-testid="provider-form-extra">{extra}</div>
          <Form form={form} layout="vertical">
            {children}
          </Form>
          <Button
            type="primary"
            loading={saving}
            onClick={async () => {
              // Client ID lives in the dynamic provider section; supply it here
              // so validation matches a user who filled the required credential.
              if (form && !form.getFieldValue("clientId")) {
                form.setFieldValue("clientId", "client-id-1");
              }
              const values = await form!.validateFields();
              // 与真实壳同契约：提交拒绝时把 422/400 字段错误落到表单项。
              try {
                await onFinish?.({ ...form!.getFieldsValue(true), ...values });
              } catch (cause) {
                if (form && isHttpError(cause)) {
                  applyServerFieldErrors(
                    form,
                    cause,
                    buildServerFieldResolver(form, {
                      aliases: props.fieldAliases,
                      labels: props.fieldLabels,
                    }),
                    t,
                  );
                }
              }
            }}
          >
            save-provider
          </Button>
          <Button onClick={() => onCancel?.()}>cancel-provider</Button>
        </div>
      );
    },
  };
});

function providerDetail(overrides: Record<string, unknown> = {}) {
  return {
    id: "provider-1",
    organizationId: "org-1",
    name: "github",
    displayName: "GitHub",
    category: "OAuth",
    providerType: "GitHub",
    subType: "",
    method: "",
    clientId: "client-existing",
    clientId2: "",
    authUrl: "https://github.com/login/oauth/authorize",
    tokenUrl: "https://github.com/login/oauth/access_token",
    userInfoUrl: "https://api.github.com/user",
    callbackUrl: "",
    signingAlg: "",
    scopes: ["user:email"],
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
    appId: "",
    contentType: "",
    sender: "",
    receiver: "",
    providerUrl: "",
    userMapping: { id: "sub" },
    enableSignUp: false,
    enablePkce: false,
    emailRegex: "",
    enableProxy: false,
    httpHeaders: { "X-Existing": "1" },
    requireMessageAuthenticator: false,
    enableSignAuthnRequest: false,
    customLogoutUrl: "",
    customLogo: "",
    cert: "",
    state: "",
    status: "active",
    revision: 7,
    ...overrides,
  };
}

function renderPage() {
  const queryClient: QueryClient = createTestQueryClient();
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  const Wrapper = createTestWrapper({ queryClient });
  const view = render(<ProviderDetailPage />, { wrapper: Wrapper });
  return { ...view, invalidateSpy };
}

async function confirmPopconfirm(user: ReturnType<typeof userEvent.setup>) {
  const confirmButton = await waitFor(() => {
    const button = document.querySelector<HTMLButtonElement>(".ant-popconfirm .ant-btn-primary");
    if (!button) throw new Error("popconfirm is not open");
    return button;
  });
  await user.click(confirmButton);
}

describe("ProviderDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    routeState.providerId = "provider-1";
    mockHasAction.mockReturnValue(true);
    mockHasResource.mockReturnValue(true);
    mockGetOrganizationOptions.mockResolvedValue([
      { id: "org-1", name: "built-in", slug: "built-in", displayName: "Built-in" },
    ]);
    useSessionStore.setState({
      account: { owner: "built-in", name: "admin", isAdmin: true },
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

  it("blocks the page when the user cannot read providers", () => {
    mockHasAction.mockReturnValue(false);

    renderPage();

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(mockHasAction).toHaveBeenCalledWith("provider:read");
    expect(mockGetProvider).not.toHaveBeenCalled();
  });

  it("requires provider:write on the new-provider route", () => {
    routeState.providerId = "new";
    mockHasAction.mockReturnValue(false);

    renderPage();

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(mockHasAction).toHaveBeenCalledWith("provider:write");
  });

  it("shows a skeleton while the provider detail is loading", () => {
    mockGetProvider.mockReturnValue(new Promise(() => {}));

    const { container } = renderPage();

    expect(container.querySelector(".ant-skeleton")).not.toBeNull();
    expect(screen.queryByTestId("provider-form")).not.toBeInTheDocument();
  });

  it("shows the load error and refetches when retry is clicked", async () => {
    const user = userEvent.setup();
    mockGetProvider.mockRejectedValue(new Error("backend unavailable"));

    renderPage();

    expect(await screen.findByText("loadFailed")).toBeInTheDocument();
    expect(mockGetProvider).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: "retry" }));
    await waitFor(() => expect(mockGetProvider).toHaveBeenCalledTimes(2));
  });

  it("maps a 403 detail response to the forbidden result", async () => {
    mockGetProvider.mockRejectedValue(new HttpError(403, null, "Forbidden"));

    renderPage();

    expect(await screen.findByText("403")).toBeInTheDocument();
  });

  it("maps a 404 detail response to the not-found result", async () => {
    mockGetProvider.mockRejectedValue(new HttpError(404, null, "Not found"));

    renderPage();

    expect(await screen.findByText("provider.notFound")).toBeInTheDocument();
  });

  it("shows not-found when the detail payload is empty", async () => {
    mockGetProvider.mockResolvedValue(null);

    renderPage();

    expect(await screen.findByText("provider.notFound")).toBeInTheDocument();
  });

  it("renders the form once the provider detail arrives", async () => {
    mockGetProvider.mockResolvedValue(providerDetail());

    renderPage();

    expect(await screen.findByTestId("provider-form")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "GitHub" })).toBeInTheDocument();
    expect(screen.queryByText("403")).not.toBeInTheDocument();
    expect(screen.queryByText("provider.notFound")).not.toBeInTheDocument();
    expect(mockGetOrganizationOptions).toHaveBeenCalled();
  });

  it("renders one main heading and five section navigation links", async () => {
    mockGetProvider.mockResolvedValue(providerDetail());

    renderPage();
    await screen.findByTestId("provider-form");

    const mainHeadings = screen.getAllByRole("heading", { level: 1 });
    expect(mainHeadings).toHaveLength(1);
    expect(mainHeadings[0]).toHaveTextContent("GitHub");

    const nav = screen.getByRole("navigation", { name: "sectionNav.sectionsAriaLabel" });
    expect(
      within(nav)
        .getAllByRole("link")
        .map((link) => link.textContent),
    ).toEqual([
      "provider.basicInfo",
      "provider.credentials",
      "provider.dynamicFields",
      "provider.userMapping",
      "provider.authSwitches",
    ]);
  });

  it("highlights the clicked section link and keeps its section anchor in the DOM", async () => {
    const user = userEvent.setup();
    mockGetProvider.mockResolvedValue(providerDetail());

    renderPage();
    await screen.findByTestId("provider-form");

    const nav = screen.getByRole("navigation", { name: "sectionNav.sectionsAriaLabel" });
    const basic = within(nav).getByRole("link", { name: "provider.basicInfo" });
    const authSwitches = within(nav).getByRole("link", { name: "provider.authSwitches" });
    expect(basic).toHaveAttribute("aria-current", "location");

    await user.click(authSwitches);

    expect(authSwitches).toHaveAttribute("aria-current", "location");
    expect(basic).not.toHaveAttribute("aria-current");
    const target = document.getElementById("provider-section-auth-switches");
    expect(target).not.toBeNull();
    expect(
      within(target as HTMLElement).getByRole("heading", {
        level: 3,
        name: "provider.authSwitches",
      }),
    ).toBeInTheDocument();
  });

  it("renders the user-mapping section title exactly once", async () => {
    mockGetProvider.mockResolvedValue(providerDetail());

    renderPage();
    await screen.findByTestId("provider-form");

    expect(screen.getAllByRole("heading", { level: 3, name: "provider.userMapping" })).toHaveLength(
      1,
    );
  });

  it("submits the same update payload key set for the same detail", async () => {
    const user = userEvent.setup();
    mockGetProvider.mockResolvedValue(providerDetail());
    mockUpdateProvider.mockResolvedValue(undefined);

    renderPage();
    await screen.findByTestId("provider-form");
    const displayName = screen.getByLabelText("displayName");
    await waitFor(() => expect(displayName).toHaveValue("GitHub"));
    await user.clear(displayName);
    await user.type(displayName, "GitHub Renamed");
    await user.click(screen.getByRole("button", { name: "save-provider" }));

    await waitFor(() => expect(mockUpdateProvider).toHaveBeenCalledTimes(1));
    const [, payload] = mockUpdateProvider.mock.calls[0] as [string, Record<string, unknown>];
    expect(Object.keys(payload).sort()).toEqual(
      [
        "admin",
        "agentId",
        "appId",
        "authUrl",
        "callbackUrl",
        "category",
        "cert",
        "clientId",
        "clientId2",
        "clientSecret",
        "clientSecret2",
        "contentType",
        "customApi",
        "customBody",
        "customHeader",
        "customLogoutUrl",
        "customLogo",
        "databaseType",
        "displayName",
        "emailRegex",
        "enablePkce",
        "enableProxy",
        "enableSignAuthnRequest",
        "host",
        "httpHeaders",
        "idpEntityId",
        "issuerUrl",
        "metadata",
        "method",
        "password",
        "port",
        "providerType",
        "providerUrl",
        "receiver",
        "region",
        "requireMessageAuthenticator",
        "revision",
        "scopes",
        "sender",
        "signingAlg",
        "sloUrl",
        "ssoUrl",
        "state",
        "subType",
        "template",
        "tokenUrl",
        "userInfoUrl",
        "userMapping",
      ].sort(),
    );
  });

  it("creates a provider with mapped headers, parsed user mapping and no empty secret", async () => {
    const user = userEvent.setup();
    routeState.providerId = "new";
    mockCreateProvider.mockResolvedValue({ id: "provider-new" });
    mockGetProvider.mockResolvedValue(null);

    const { invalidateSpy } = renderPage();
    await screen.findByTestId("provider-form");

    await user.type(screen.getByLabelText("name"), "github");
    await user.type(screen.getByLabelText("displayName"), "GitHub");
    await user.type(screen.getAllByPlaceholderText("e.g. sub")[0], "sub");
    await user.click(screen.getByRole("button", { name: /addHeader/ }));
    await user.type(screen.getByPlaceholderText("key"), "X-Test");
    await user.type(screen.getByPlaceholderText("value"), "abc");
    await user.click(screen.getByRole("button", { name: "save-provider" }));

    await waitFor(() => expect(mockCreateProvider).toHaveBeenCalledTimes(1));
    const payload = mockCreateProvider.mock.calls[0][0] as Record<string, unknown>;
    expect(payload).toMatchObject({
      organizationId: "org-1",
      name: "github",
      displayName: "GitHub",
      category: "OAuth",
      providerType: "Custom",
      clientId: "client-id-1",
      httpHeaders: { "X-Test": "abc" },
      userMapping: { id: "sub" },
    });
    expect(payload.clientSecret).toBeUndefined();

    await waitFor(() => expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["providers"] }));
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["provider", "provider-new"],
    });
    expect(mockMessage.success).toHaveBeenCalledWith("provider.providerCreated");
    expect(mockNavigate).toHaveBeenCalledWith("/providers/provider-new");
  });

  it("updates a provider with the current revision", async () => {
    const user = userEvent.setup();
    mockGetProvider.mockResolvedValue(providerDetail());
    mockUpdateProvider.mockResolvedValue(undefined);

    const { invalidateSpy } = renderPage();
    await screen.findByTestId("provider-form");
    const displayName = screen.getByLabelText("displayName");
    await waitFor(() => expect(displayName).toHaveValue("GitHub"));
    await user.clear(displayName);
    await user.type(displayName, "GitHub Edited");
    await user.click(screen.getByRole("button", { name: "save-provider" }));

    await waitFor(() => expect(mockUpdateProvider).toHaveBeenCalledTimes(1));
    const [providerId, payload] = mockUpdateProvider.mock.calls[0] as [
      string,
      Record<string, unknown>,
    ];
    expect(providerId).toBe("provider-1");
    expect(payload).toMatchObject({
      displayName: "GitHub Edited",
      revision: 7,
      userMapping: { id: "sub" },
      httpHeaders: { "X-Existing": "1" },
    });
    expect(payload.clientSecret).toBeUndefined();
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["provider", "provider-1"],
    });
    expect(mockMessage.success).toHaveBeenCalledWith("provider.providerSaved");
    expect(mockNavigate).toHaveBeenCalledWith("/providers/provider-1");
  });

  it("marks providerType red when the server rejects the derived kind", async () => {
    const user = userEvent.setup();
    mockGetProvider.mockResolvedValue(providerDetail());
    const rejection = new HttpError(422, {}, "unsupported identity provider kind: alibabasms");
    rejection.details = { field: "kind", value: "alibabasms", allowed: ["alibaba_sms"] };
    mockUpdateProvider.mockRejectedValue(rejection);

    renderPage();
    await screen.findByTestId("provider-form");
    await user.click(screen.getByRole("button", { name: "save-provider" }));

    await waitFor(() => expect(mockUpdateProvider).toHaveBeenCalledTimes(1));
    // kind 不是表单项；别名必须把它归位到 providerType，而不是只剩通用 toast。
    const typeItem = screen.getByLabelText("type").closest(".ant-form-item");
    await waitFor(() => expect(typeItem).toHaveClass("ant-form-item-has-error"));
  });

  it("locks type and category when editing a legacy FaceId provider", async () => {
    mockGetProvider.mockResolvedValue(
      providerDetail({ category: "FaceId", providerType: "FaceId" }),
    );

    renderPage();
    await screen.findByTestId("provider-form");

    // 历史 FaceId 行由专用子系统托管：编辑态锁定类型/类别，避免选中仅剩的
    // Custom 触发 managed-kind 422（表现为「类型 自定义」）。
    const categorySelect = screen.getByLabelText("category").closest(".ant-select");
    const typeSelect = screen.getByLabelText("type").closest(".ant-select");
    await waitFor(() => expect(categorySelect).toHaveClass("ant-select-disabled"));
    expect(typeSelect).toHaveClass("ant-select-disabled");
    expect(screen.getByText("provider.faceIdManagedNote")).toBeInTheDocument();
  });

  it("enables a provider after confirmation and invalidates the caches", async () => {
    const user = userEvent.setup();
    mockGetProvider.mockResolvedValue(providerDetail({ status: "draft" }));
    mockEnableProvider.mockResolvedValue(undefined);

    const { invalidateSpy } = renderPage();
    await screen.findByTestId("provider-form");
    const extra = screen.getByTestId("provider-form-extra");

    await user.click(within(extra).getByRole("button", { name: "enable" }));
    await confirmPopconfirm(user);

    await waitFor(() => expect(mockEnableProvider).toHaveBeenCalledWith("provider-1", "org-1"));
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["provider", "provider-1"],
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["providers"] });
    expect(mockMessage.success).toHaveBeenCalledWith("provider.providerEnabled");
  });

  it("deletes a provider after confirmation and returns to the list", async () => {
    const user = userEvent.setup();
    mockGetProvider.mockResolvedValue(providerDetail());
    mockDeleteProvider.mockResolvedValue(undefined);

    const { invalidateSpy } = renderPage();
    await screen.findByTestId("provider-form");
    const extra = screen.getByTestId("provider-form-extra");

    await user.click(within(extra).getByRole("button", { name: "delete" }));
    await confirmPopconfirm(user);

    await waitFor(() => expect(mockDeleteProvider).toHaveBeenCalledWith("provider-1", "org-1"));
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["providers"] });
    expect(mockMessage.success).toHaveBeenCalledWith("provider.providerDeleted");
    expect(mockNavigate).toHaveBeenCalledWith("/providers");
  });

  it("runs the connection probe and reports success", async () => {
    const user = userEvent.setup();
    mockGetProvider.mockResolvedValue(providerDetail());
    mockTestProviderConnection.mockResolvedValue({ connected: true });

    renderPage();
    await screen.findByTestId("provider-form");

    await user.click(screen.getByRole("button", { name: /provider\.testConnection/ }));

    await waitFor(() =>
      expect(mockTestProviderConnection).toHaveBeenCalledWith("provider-1", "org-1"),
    );
    expect(mockMessage.success).toHaveBeenCalledWith(
      expect.objectContaining({ content: "provider.testConnectionSuccess" }),
    );
  });

  it("renders exactly one enableSignAuthnRequest control and writes it to the payload", async () => {
    const user = userEvent.setup();
    mockGetProvider.mockResolvedValue(
      providerDetail({ category: "SAML", providerType: "SAML 2.0", enableSignAuthnRequest: false }),
    );
    mockUpdateProvider.mockResolvedValue(undefined);

    renderPage();
    await screen.findByTestId("provider-form");

    // The legacy SAML dynamic control ("Sign Authn Request") is gone; only the
    // auth-switch section control remains, and it must reach the payload.
    expect(screen.queryByText("Sign Authn Request")).not.toBeInTheDocument();
    const switches = screen.getAllByLabelText("provider.enableSignAuthnRequest");
    expect(switches).toHaveLength(1);
    await user.click(switches[0]);
    await user.click(screen.getByRole("button", { name: "save-provider" }));

    await waitFor(() => expect(mockUpdateProvider).toHaveBeenCalledTimes(1));
    const [, payload] = mockUpdateProvider.mock.calls[0] as [string, Record<string, unknown>];
    expect(payload.enableSignAuthnRequest).toBe(true);
  });

  it.each([
    ["Email", "SMTP"],
    ["SMS", "TwilioSMS"],
    ["Storage", "AWS S3"],
    ["Notification", "DingTalk"],
    ["Scan", "WeChatScan"],
    ["Log", "Elasticsearch"],
    ["Audit", "Syslog"],
  ])("renders providerType only in basic info for %s providers", async (category, providerType) => {
    mockGetProvider.mockResolvedValue(providerDetail({ category, providerType }));

    renderPage();
    await screen.findByTestId("provider-form");

    expect(screen.getAllByLabelText("type")).toHaveLength(1);
    // The category component must not render its own provider-type control.
    expect(screen.queryByText("providerType")).not.toBeInTheDocument();
  });

  it("keeps a single clientId editor for Captcha and persists it", async () => {
    const user = userEvent.setup();
    mockGetProvider.mockResolvedValue(
      providerDetail({ category: "Captcha", providerType: "reCAPTCHA", clientId: "site-key-old" }),
    );
    mockUpdateProvider.mockResolvedValue(undefined);

    renderPage();
    await screen.findByTestId("provider-form");

    expect(screen.queryByLabelText("clientId")).not.toBeInTheDocument();
    const siteKey = screen.getByLabelText("siteKey");
    expect(siteKey).toHaveValue("site-key-old");
    await user.clear(siteKey);
    await user.type(siteKey, "site-key-new");
    await user.click(screen.getByRole("button", { name: "save-provider" }));

    await waitFor(() => expect(mockUpdateProvider).toHaveBeenCalledTimes(1));
    const [, payload] = mockUpdateProvider.mock.calls[0] as [string, Record<string, unknown>];
    expect(payload.clientId).toBe("site-key-new");
  });

  it("keeps the Scan clientId editor and the dynamic App Secret without duplicates", async () => {
    const user = userEvent.setup();
    mockGetProvider.mockResolvedValue(
      providerDetail({
        category: "Scan",
        providerType: "WeChatScan",
        clientId: "scan-client",
        clientSecret: "",
      }),
    );
    mockUpdateProvider.mockResolvedValue(undefined);

    renderPage();
    await screen.findByTestId("provider-form");

    expect(screen.getByLabelText("clientId")).toHaveValue("scan-client");
    expect(screen.queryByLabelText("clientSecret")).not.toBeInTheDocument();
    const appSecret = screen.getByLabelText("appSecret");
    await user.type(appSecret, "app-secret-new");
    await user.click(screen.getByRole("button", { name: "save-provider" }));

    await waitFor(() => expect(mockUpdateProvider).toHaveBeenCalledTimes(1));
    const [, payload] = mockUpdateProvider.mock.calls[0] as [string, Record<string, unknown>];
    expect(payload.clientId).toBe("scan-client");
    expect(payload.clientSecret).toBe("app-secret-new");
  });

  it("shows the stored client-secret indicator without echoing the secret", async () => {
    mockGetProvider.mockResolvedValue(
      providerDetail({ clientSecretSet: true, clientSecretMasked: "****abcd" }),
    );

    renderPage();
    await screen.findByTestId("provider-form");

    // OAuth 的密钥在动态区编辑：指示条回显脱敏尾段，输入框保持空白。
    expect(screen.getByText("provider.savedCredentialsTitle")).toBeInTheDocument();
    expect(screen.getByText("clientSecret: ****abcd")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("leaveEmptyToKeepCurrent")).toHaveValue("");
  });

  it("lists saved dynamic-section credentials with masked tails", async () => {
    mockGetProvider.mockResolvedValue(
      providerDetail({
        category: "Email",
        providerType: "SMTP",
        passwordSet: true,
        passwordMasked: "****ijkl",
        appSecretSet: true,
        appSecretMasked: "****mnop",
      }),
    );

    renderPage();
    await screen.findByTestId("provider-form");

    expect(screen.getByText("provider.savedCredentialsTitle")).toBeInTheDocument();
    expect(screen.getByText("password: ****ijkl · appSecret: ****mnop")).toBeInTheDocument();
  });

  it("hides the saved-credentials strip when nothing is stored", async () => {
    mockGetProvider.mockResolvedValue(providerDetail());

    renderPage();
    await screen.findByTestId("provider-form");

    expect(screen.queryByText("provider.savedCredentialsTitle")).not.toBeInTheDocument();
  });

  it("hides inapplicable auth switches without dropping their stored values", async () => {
    const user = userEvent.setup();
    mockGetProvider.mockResolvedValue(
      providerDetail({
        category: "SAML",
        providerType: "SAML 2.0",
        enablePkce: true,
        enableProxy: true,
        httpHeaders: { "X-Saml": "1" },
        requireMessageAuthenticator: true,
        cert: "-----BEGIN CERTIFICATE-----",
        customLogo: "https://cdn.example.com/logo.svg",
      }),
    );
    mockUpdateProvider.mockResolvedValue(undefined);

    renderPage();
    await screen.findByTestId("provider-form");

    // OAuth/OIDC-only and RADIUS-only controls are not rendered for SAML.
    expect(screen.queryByLabelText("provider.enablePkce")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("provider.enableProxy")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("provider.httpHeaders")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("provider.requireMessageAuthenticator")).not.toBeInTheDocument();
    // SAML switches and unconfirmed-type switches stay visible.
    expect(screen.getByLabelText("provider.enableSignAuthnRequest")).toBeInTheDocument();
    expect(screen.getByLabelText("provider.cert")).toHaveValue("-----BEGIN CERTIFICATE-----");
    expect(screen.getByLabelText("provider.customLogo")).toHaveValue(
      "https://cdn.example.com/logo.svg",
    );
    expect(screen.getByLabelText("provider.state")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "save-provider" }));

    await waitFor(() => expect(mockUpdateProvider).toHaveBeenCalledTimes(1));
    const [, payload] = mockUpdateProvider.mock.calls[0] as [string, Record<string, unknown>];
    expect(payload.enablePkce).toBe(true);
    expect(payload.enableProxy).toBe(true);
    expect(payload.httpHeaders).toEqual({ "X-Saml": "1" });
    expect(payload.requireMessageAuthenticator).toBe(true);
  });

  it("renders requireMessageAuthenticator for the RADIUS MFA provider", async () => {
    mockGetProvider.mockResolvedValue(providerDetail({ category: "MFA", providerType: "RADIUS" }));

    renderPage();
    await screen.findByTestId("provider-form");

    expect(screen.getByLabelText("provider.requireMessageAuthenticator")).toBeInTheDocument();
    expect(screen.queryByLabelText("provider.enableSignAuthnRequest")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("provider.enablePkce")).not.toBeInTheDocument();
  });
});
