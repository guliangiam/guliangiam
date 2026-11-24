import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";

import { useSessionStore } from "@/features/session/session-store";
import { createTestWrapper } from "../../test-utils/render";
import AIGatewayProvidersPage from "./providers/AIGatewayProvidersPage";
import AIGatewayModelsPage from "./models/AIGatewayModelsPage";
import AIGatewayAliasesPage from "./aliases/AIGatewayAliasesPage";
import AIGatewayRoutesPage from "./routes/AIGatewayRoutesPage";
import AIGatewayAPIKeysPage from "./api-keys/AIGatewayAPIKeysPage";
import AIGatewayQuotasPage from "./quotas/AIGatewayQuotasPage";
import AIGatewayPricesPage from "./prices/AIGatewayPricesPage";
import AIGatewayProviderDetailPage from "./providers/AIGatewayProviderDetailPage";

const mockHasAction = vi.fn<(action: string) => boolean>();
const mockHasResource = vi.fn<(resource: string) => boolean>();

const mockListProviders = vi.fn<(params?: unknown) => Promise<unknown>>();
const mockListModels = vi.fn<(params?: unknown) => Promise<unknown>>();
const mockListAliases = vi.fn<(params?: unknown) => Promise<unknown>>();
const mockListRoutes = vi.fn<(params?: unknown) => Promise<unknown>>();
const mockListAPIKeys = vi.fn<(params?: unknown) => Promise<unknown>>();
const mockListQuotas = vi.fn<(params?: unknown) => Promise<unknown>>();
const mockListPrices = vi.fn<(params?: unknown) => Promise<unknown>>();
const mockGetProvider = vi.fn<() => Promise<unknown>>();
const mockGetProviderHealth = vi.fn<() => Promise<unknown>>();

/** Mirrors the backend ListResponse envelope the list pages now consume. */
function paged<T>(items: T[], page = 1, pageSize = 20, total = items.length) {
  return { items, page: { page, pageSize, total } };
}

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    isLoading: false,
    hasAction: mockHasAction,
    hasResource: mockHasResource,
  }),
}));

vi.mock("@/shared/api/aigateway", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/aigateway")>();
  return {
    ...actual,
    listProviders: (params?: unknown) => mockListProviders(params),
    getProvider: () => mockGetProvider(),
    getProviderHealth: () => mockGetProviderHealth(),
    createProvider: vi.fn<() => Promise<unknown>>(),
    updateProvider: vi.fn<() => Promise<unknown>>(),
    deleteProvider: vi.fn<() => Promise<unknown>>(),
    testProvider: vi.fn<() => Promise<unknown>>(),
    listModels: (params?: unknown) => mockListModels(params),
    createModel: vi.fn<() => Promise<unknown>>(),
    updateModel: vi.fn<() => Promise<unknown>>(),
    deleteModel: vi.fn<() => Promise<unknown>>(),
    listAliases: (params?: unknown) => mockListAliases(params),
    createAlias: vi.fn<() => Promise<unknown>>(),
    updateAlias: vi.fn<() => Promise<unknown>>(),
    deleteAlias: vi.fn<() => Promise<unknown>>(),
    listRoutes: (params?: unknown) => mockListRoutes(params),
    createRoute: vi.fn<() => Promise<unknown>>(),
    updateRoute: vi.fn<() => Promise<unknown>>(),
    deleteRoute: vi.fn<() => Promise<unknown>>(),
    listAPIKeys: (params?: unknown) => mockListAPIKeys(params),
    createAPIKey: vi.fn<() => Promise<unknown>>(),
    updateAPIKey: vi.fn<() => Promise<unknown>>(),
    deleteAPIKey: vi.fn<() => Promise<unknown>>(),
    revokeAPIKey: vi.fn<() => Promise<unknown>>(),
    listQuotas: (params?: unknown) => mockListQuotas(params),
    createQuota: vi.fn<() => Promise<unknown>>(),
    updateQuota: vi.fn<() => Promise<unknown>>(),
    deleteQuota: vi.fn<() => Promise<unknown>>(),
    listPrices: (params?: unknown) => mockListPrices(params),
    createPrice: vi.fn<() => Promise<unknown>>(),
    updatePrice: vi.fn<() => Promise<unknown>>(),
    deletePrice: vi.fn<() => Promise<unknown>>(),
  };
});

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("react-router-dom", () => ({
  useParams: () => ({ id: "provider-1" }),
  useNavigate: () => vi.fn<() => void>(),
  Link: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("@/shared/ui/ResourceFormPage", () => ({
  ResourceFormPage: ({
    title,
    extra,
    children,
  }: {
    title: React.ReactNode;
    extra?: React.ReactNode;
    children?: React.ReactNode;
  }) => (
    <div data-testid="provider-detail">
      <h1>{title}</h1>
      {extra}
      {children}
    </div>
  ),
}));

const now = "2026-11-03T00:00:00Z";

const provider = {
  id: "provider-1",
  organizationId: "org-1",
  name: "provider-a",
  providerType: "openai",
  baseUrl: "https://api.example.com",
  status: "active",
  apiKeyFingerprint: "sk-…abcd",
  supportedProtocols: ["openai"],
  requestTimeoutMs: 30000,
  maxRetries: 2,
  priority: 0,
  weight: 100,
  createdAt: now,
  updatedAt: now,
  revision: 1,
};

const model = {
  id: "model-1",
  organizationId: "org-1",
  providerId: provider.id,
  providerModelName: "gpt-4o",
  status: "active",
  maxContextTokens: 128000,
  maxOutputTokens: 4096,
  createdAt: now,
  updatedAt: now,
  revision: 1,
};

const alias = {
  id: "alias-1",
  organizationId: "org-1",
  alias: "alias-a",
  status: "active",
  createdAt: now,
  updatedAt: now,
  revision: 1,
};

const route = {
  id: "route-1",
  organizationId: "org-1",
  aliasId: alias.id,
  aliasName: alias.alias,
  modelId: model.id,
  modelName: model.providerModelName,
  priority: 0,
  weight: 100,
  status: "active",
  createdAt: now,
  updatedAt: now,
  revision: 1,
};

const apiKey = {
  id: "key-1",
  organizationId: "org-1",
  name: "key-a",
  keyPrefix: "sk-live-abcd",
  allowedAliases: [],
  status: "active",
  createdAt: now,
  updatedAt: now,
  revision: 1,
};

const quota = {
  id: "quota-1",
  organizationId: "org-1",
  scope: "organization",
  scopeRef: "quota-scope-a",
  limitType: "rpm",
  limitValue: 60000,
  resetCycle: "monthly",
  status: "active",
  createdAt: now,
  updatedAt: now,
  revision: 1,
};

const price = {
  id: "price-1",
  organizationId: "org-1",
  modelId: model.id,
  versionNo: 2,
  inputPricePer1M: 2.5,
  outputPricePer1M: 10,
  currency: "USD",
  effectiveFrom: now,
  createdAt: now,
  updatedAt: now,
};

type PageCase = {
  name: string;
  renderPage: () => ReactElement;
  rowText: string;
  createLabel: string;
  writeAction: string;
  deleteAction: string;
  resource: string;
  /** 连通性测试按钮（aigateway-provider:write） */
  hasConnectivityTest?: boolean;
  /** API Key Revoke 按钮（aigateway-api-key:write） */
  hasRevoke?: boolean;
};

const pageCases: PageCase[] = [
  {
    name: "providers",
    renderPage: () => <AIGatewayProvidersPage />,
    rowText: provider.name,
    createLabel: "aigateway.createProvider",
    writeAction: "aigateway-provider:write",
    deleteAction: "aigateway-provider:delete",
    resource: "aigateway-providers",
    hasConnectivityTest: true,
  },
  {
    name: "models",
    renderPage: () => <AIGatewayModelsPage />,
    rowText: model.providerModelName,
    createLabel: "aigateway.createModel",
    writeAction: "aigateway-model:write",
    deleteAction: "aigateway-model:delete",
    resource: "aigateway-models",
  },
  {
    name: "aliases",
    renderPage: () => <AIGatewayAliasesPage />,
    rowText: `\`${alias.alias}\``,
    createLabel: "aigateway.createAlias",
    writeAction: "aigateway-alias:write",
    deleteAction: "aigateway-alias:delete",
    resource: "aigateway-aliases",
  },
  {
    name: "routes",
    renderPage: () => <AIGatewayRoutesPage />,
    rowText: route.aliasName,
    createLabel: "aigateway.createRoute",
    writeAction: "aigateway-route:write",
    deleteAction: "aigateway-route:delete",
    resource: "aigateway-routes",
  },
  {
    name: "api-keys",
    renderPage: () => <AIGatewayAPIKeysPage />,
    rowText: apiKey.name,
    createLabel: "aigateway.createAPIKey",
    writeAction: "aigateway-api-key:write",
    deleteAction: "aigateway-api-key:delete",
    resource: "aigateway-api-keys",
    hasRevoke: true,
  },
  {
    name: "quotas",
    renderPage: () => <AIGatewayQuotasPage />,
    rowText: quota.scopeRef,
    createLabel: "aigateway.createQuota",
    writeAction: "aigateway-quota:write",
    deleteAction: "aigateway-quota:delete",
    resource: "aigateway-quotas",
  },
  {
    name: "prices",
    renderPage: () => <AIGatewayPricesPage />,
    rowText: "model-1…",
    createLabel: "aigateway.createPrice",
    writeAction: "aigateway-price:write",
    deleteAction: "aigateway-price:delete",
    resource: "aigateway-prices",
  },
];

async function renderPageAndSelect(page: PageCase) {
  const user = userEvent.setup();
  const view = render(page.renderPage(), { wrapper: createTestWrapper() });

  expect(await screen.findByText(page.rowText)).toBeInTheDocument();

  const rowCheckbox = screen.getAllByRole("checkbox").at(-1);
  expect(rowCheckbox).toBeDefined();
  await user.click(rowCheckbox!);
  expect(await screen.findByRole("toolbar")).toBeInTheDocument();

  return view;
}

describe.each(pageCases)("$name permission guards", (page) => {
  beforeEach(() => {
    vi.clearAllMocks();
    useSessionStore.setState({
      account: { owner: "built-in", name: "admin", isAdmin: false },
      organization: {
        id: "org-1",
        name: "built-in",
        displayName: "Built-in",
        navItems: [],
        userNavItems: [],
      },
      impersonatedUser: null,
    });
    mockHasResource.mockReturnValue(true);
    mockListProviders.mockResolvedValue(paged([provider]));
    mockListModels.mockResolvedValue(paged([model]));
    mockListAliases.mockResolvedValue(paged([alias]));
    mockListRoutes.mockResolvedValue(paged([route]));
    mockListAPIKeys.mockResolvedValue(paged([apiKey]));
    mockListQuotas.mockResolvedValue(paged([quota]));
    mockListPrices.mockResolvedValue(paged([price]));
  });

  it("hides create, edit and delete without write/delete actions", async () => {
    mockHasAction.mockReturnValue(false);
    const view = await renderPageAndSelect(page);

    expect(screen.queryByText(page.createLabel)).not.toBeInTheDocument();
    expect(screen.queryAllByLabelText("edit")).toHaveLength(0);
    expect(screen.queryAllByLabelText("delete")).toHaveLength(0);
    expect(screen.queryAllByLabelText("api")).toHaveLength(0);
    expect(screen.queryAllByLabelText("stop")).toHaveLength(0);
    expect(screen.queryByRole("button", { name: "batchDelete" })).not.toBeInTheDocument();
    expect(view.container.querySelector(".ant-skeleton")).not.toBeInTheDocument();
  });

  it("shows write actions but hides delete and batch delete with write only", async () => {
    mockHasAction.mockImplementation((action) => action === page.writeAction);
    mockHasResource.mockImplementation((resource) => resource === page.resource);
    await renderPageAndSelect(page);

    expect(screen.getByText(page.createLabel)).toBeInTheDocument();
    expect(screen.queryAllByLabelText("edit").length).toBeGreaterThan(0);
    expect(screen.queryAllByLabelText("api")).toHaveLength(page.hasConnectivityTest ? 1 : 0);
    expect(screen.queryAllByLabelText("stop")).toHaveLength(page.hasRevoke ? 1 : 0);
    expect(screen.queryAllByLabelText("delete")).toHaveLength(0);
    expect(screen.queryByRole("button", { name: "batchDelete" })).not.toBeInTheDocument();
  });

  it("shows row delete and batch delete with write + delete actions", async () => {
    mockHasAction.mockImplementation(
      (action) => action === page.writeAction || action === page.deleteAction,
    );
    mockHasResource.mockImplementation((resource) => resource === page.resource);
    await renderPageAndSelect(page);

    expect(screen.queryAllByLabelText("delete").length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "batchDelete" })).toBeInTheDocument();
  });

  it("keeps batch delete hidden when the frontend resource is missing", async () => {
    mockHasAction.mockImplementation((action) => action === page.deleteAction);
    mockHasResource.mockReturnValue(false);
    await renderPageAndSelect(page);

    expect(screen.queryAllByLabelText("delete").length).toBeGreaterThan(0);
    expect(screen.queryByRole("button", { name: "batchDelete" })).not.toBeInTheDocument();
  });
});

describe("providers batch enable/disable guard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useSessionStore.setState({
      account: { owner: "built-in", name: "admin", isAdmin: false },
      organization: {
        id: "org-1",
        name: "built-in",
        displayName: "Built-in",
        navItems: [],
        userNavItems: [],
      },
      impersonatedUser: null,
    });
    mockHasResource.mockReturnValue(true);
    mockListProviders.mockResolvedValue(paged([provider]));
  });

  it("hides bulk enable/disable without aigateway-provider:write", async () => {
    mockHasAction.mockReturnValue(false);
    await renderPageAndSelect(pageCases[0]);

    expect(screen.queryByRole("button", { name: "action.enable" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "action.disable" })).not.toBeInTheDocument();
  });

  it("shows bulk enable/disable with aigateway-provider:write", async () => {
    mockHasAction.mockImplementation((action) => action === "aigateway-provider:write");
    await renderPageAndSelect(pageCases[0]);

    expect(screen.getByRole("button", { name: "action.enable" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "action.disable" })).toBeInTheDocument();
  });
});

describe("providers server pagination", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useSessionStore.setState({
      account: { owner: "built-in", name: "admin", isAdmin: false },
      organization: {
        id: "org-1",
        name: "built-in",
        displayName: "Built-in",
        navItems: [],
        userNavItems: [],
      },
      impersonatedUser: null,
    });
    mockHasAction.mockReturnValue(false);
    mockHasResource.mockReturnValue(true);
  });

  it("sends the backend default and refetches when the page changes", async () => {
    const providerTwo = { ...provider, id: "provider-2", name: "provider-b" };
    mockListProviders.mockImplementation((params?: unknown) => {
      const page = (params as { page?: number } | undefined)?.page ?? 1;
      return Promise.resolve(paged(page === 2 ? [providerTwo] : [provider], page, 20, 41));
    });

    render(<AIGatewayProvidersPage />, { wrapper: createTestWrapper() });

    await waitFor(() =>
      expect(mockListProviders).toHaveBeenCalledWith({ page: 1, pageSize: 20, search: "" }),
    );
    expect((await screen.findAllByText(provider.name)).length).toBeGreaterThan(0);

    fireEvent.click(screen.getByLabelText("Page 2"));

    await waitFor(() =>
      expect(mockListProviders).toHaveBeenCalledWith({ page: 2, pageSize: 20, search: "" }),
    );
    expect((await screen.findAllByText(providerTwo.name)).length).toBeGreaterThan(0);
  });
});

describe("provider detail permission guards", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useSessionStore.setState({
      account: { owner: "built-in", name: "admin", isAdmin: false },
      organization: null,
      impersonatedUser: null,
    });
    mockHasResource.mockReturnValue(true);
    mockGetProvider.mockResolvedValue(provider);
    mockGetProviderHealth.mockResolvedValue({
      id: "health-1",
      providerId: provider.id,
      state: "healthy",
      consecutiveFailures: 0,
      updatedAt: now,
    });
    mockListModels.mockResolvedValue(paged([model]));
  });

  async function renderDetail() {
    render(<AIGatewayProviderDetailPage />, { wrapper: createTestWrapper() });
    expect(await screen.findByTestId("provider-detail")).toBeInTheDocument();
    expect((await screen.findAllByText(provider.name)).length).toBeGreaterThan(0);
  }

  it("hides test, edit and delete without write/delete actions", async () => {
    mockHasAction.mockReturnValue(false);
    await renderDetail();

    expect(screen.queryByText("aigateway.testConnectivity")).not.toBeInTheDocument();
    expect(screen.queryByText("edit")).not.toBeInTheDocument();
    expect(screen.queryByText("delete")).not.toBeInTheDocument();
  });

  it("shows test and edit but hides delete with write only", async () => {
    mockHasAction.mockImplementation((action) => action === "aigateway-provider:write");
    await renderDetail();

    expect(screen.getByText("aigateway.testConnectivity")).toBeInTheDocument();
    expect(screen.getByText("edit")).toBeInTheDocument();
    expect(screen.queryByText("delete")).not.toBeInTheDocument();
  });

  it("shows delete with aigateway-provider:delete", async () => {
    mockHasAction.mockImplementation(
      (action) => action === "aigateway-provider:write" || action === "aigateway-provider:delete",
    );
    await renderDetail();

    expect(screen.getByText("aigateway.testConnectivity")).toBeInTheDocument();
    expect(screen.getByText("edit")).toBeInTheDocument();
    expect(screen.getByText("delete")).toBeInTheDocument();
  });
});
