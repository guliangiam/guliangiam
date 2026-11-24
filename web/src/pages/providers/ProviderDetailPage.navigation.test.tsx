import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "antd";
import { QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { useSessionStore } from "@/features/session/session-store";
import { createTestQueryClient } from "@/test-utils/render";
import ProviderDetailPage from "./ProviderDetailPage";

const {
  mockGetProvider,
  mockUpdateProvider,
  mockGetOrganizationOptions,
  mockHasAction,
  mockHasResource,
  mockMessage,
} = vi.hoisted(() => ({
  mockGetProvider: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockUpdateProvider: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockGetOrganizationOptions: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockHasAction: vi.fn<(action: string) => boolean>(),
  mockHasResource: vi.fn<(resource: string) => boolean>(),
  mockMessage: {
    success: vi.fn<(...args: unknown[]) => void>(),
    error: vi.fn<(...args: unknown[]) => void>(),
    warning: vi.fn<(...args: unknown[]) => void>(),
    info: vi.fn<(...args: unknown[]) => void>(),
  },
}));

vi.mock("@/shared/api/iam/provider-extended", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/iam/provider-extended")>();
  return {
    ...actual,
    getProvider: (...args: unknown[]) => mockGetProvider(...args),
    createProvider: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    updateProvider: (...args: unknown[]) => mockUpdateProvider(...args),
    enableProvider: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    deleteProvider: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    testProviderConnection: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
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

function providerDetail() {
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
  };
}

function renderInRouter() {
  const queryClient = createTestQueryClient();
  const router = createMemoryRouter(
    [
      { path: "/providers/:providerId", element: <ProviderDetailPage /> },
      { path: "/providers", element: <div>providers-list</div> },
    ],
    { initialEntries: ["/providers/provider-1"] },
  );
  render(
    <App component={false}>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </App>,
  );
  return null;
}

describe("ProviderDetailPage unsaved-changes prompt", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    mockHasAction.mockReturnValue(true);
    mockHasResource.mockReturnValue(true);
    mockGetProvider.mockResolvedValue(providerDetail());
    mockUpdateProvider.mockResolvedValue(undefined);
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

  it("prompts exactly once from the route guard and registers no beforeunload handler", async () => {
    const user = userEvent.setup();
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    const addEventListenerSpy = vi.spyOn(window, "addEventListener");

    renderInRouter();

    const displayName = await screen.findByLabelText("displayName");
    await waitFor(() => expect(displayName).toHaveValue("GitHub"));
    await user.type(displayName, " Edited");
    await user.click(screen.getByRole("button", { name: "backToList" }));

    await waitFor(() => expect(confirmSpy).toHaveBeenCalledTimes(1));
    expect(confirmSpy).toHaveBeenCalledWith("unsavedChanges.defaultMessage");
    // 页面级（浏览器原生 beforeunload）提示已移除：只保留路由级这一套来源。
    expect(addEventListenerSpy.mock.calls.filter(([type]) => type === "beforeunload")).toHaveLength(
      0,
    );
    // 选择“留下”后仍停留在详情页。
    expect(screen.getByLabelText("displayName")).toBeInTheDocument();
    expect(screen.queryByText("providers-list")).not.toBeInTheDocument();

    confirmSpy.mockRestore();
    addEventListenerSpy.mockRestore();
  });
});
