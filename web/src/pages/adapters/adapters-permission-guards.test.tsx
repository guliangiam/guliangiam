import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";

import { createTestWrapper } from "../../test-utils/render";
import { useSessionStore } from "@/features/session/session-store";
import AdaptersPage from "./AdaptersPage";
import AdapterCreatePage from "./AdapterCreatePage";
import AdapterDetailPage from "./AdapterDetailPage";

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

const mockHasAction = vi.fn<(action: string) => boolean>();
const mockHasResource = vi.fn<(resource: string) => boolean>();

const mockGetAdaptersPaginated = vi.fn<() => Promise<unknown>>();
const mockGetAdapter = vi.fn<() => Promise<unknown>>();

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    isLoading: false,
    hasAction: mockHasAction,
    hasResource: mockHasResource,
  }),
}));

vi.mock("@/shared/api/iam", () => ({
  getIamAdaptersPaginated: () => mockGetAdaptersPaginated(),
  deleteIamAdapter: vi.fn<() => Promise<unknown>>(),
  createIamAdapter: vi.fn<() => Promise<unknown>>(),
  getIamAdapter: () => mockGetAdapter(),
  patchIamAdapter: vi.fn<() => Promise<unknown>>(),
  getOrganizationOptions: vi.fn<() => Promise<unknown>>(),
}));

vi.mock("@/shared/ui/OrganizationSelect", () => ({
  OrganizationSelect: () => <input data-testid="organization-select" />,
}));

// The page-level guard and action guards are what this suite locks; the
// form shell's own behaviour is covered by ResourceFormPage.test.tsx.
vi.mock("@/shared/ui/ResourceFormPage", () => ({
  ResourceFormPage: ({
    title,
    extra,
    submitPermission,
  }: {
    title: ReactNode;
    extra?: ReactNode;
    submitPermission?: { permission: string; resource: string };
  }) => (
    <div data-testid="resource-form">
      <h1>{title}</h1>
      <span data-testid="submit-permission">{JSON.stringify(submitPermission)}</span>
      {extra}
    </div>
  ),
}));

vi.mock("react-router-dom", () => ({
  Link: ({ children }: { children: ReactNode }) => <span>{children}</span>,
  useNavigate: () => vi.fn<() => void>(),
  useParams: () => ({ id: "adapter-1" }),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
    i18n: { language: "en" },
  }),
  Trans: ({ children }: { children: ReactNode }) => children,
}));

// ─────────────────────────────────────────────
// Fixtures
// ─────────────────────────────────────────────

const now = "2026-11-03T00:00:00Z";

const adapter = {
  id: "adapter-1",
  organizationId: "org-1",
  name: "adapter-postgres",
  displayName: "Postgres Adapter",
  type: "database",
  server: "localhost",
  database: "casbin",
  useSameDb: true,
  isEnabled: true,
  createdAt: now,
  updatedAt: now,
  revision: 1,
};

const adapterDetail = {
  ...adapter,
  policy: "p, alice, data1, read",
  dbPasswordSet: true,
};

function setSession() {
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
}

// ─────────────────────────────────────────────
// Adapters list
// ─────────────────────────────────────────────

describe("AdaptersPage permission guards", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setSession();
    mockHasResource.mockReturnValue(true);
    mockGetAdaptersPaginated.mockResolvedValue({
      items: [adapter],
      total: 1,
      page: 1,
      pageSize: 20,
    });
  });

  it("renders 403 without adapter:read", () => {
    mockHasAction.mockReturnValue(false);
    render(<AdaptersPage />, { wrapper: createTestWrapper() });

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.queryByText(adapter.name)).not.toBeInTheDocument();
  });

  it("shows the list but no write or delete actions with read only", async () => {
    mockHasAction.mockImplementation((action) => action === "adapter:read");
    render(<AdaptersPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText(adapter.name)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "New Adapter" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "edit" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "delete" })).not.toBeInTheDocument();
  });

  it("shows create and edit but not delete with read + write", async () => {
    mockHasAction.mockImplementation(
      (action) => action === "adapter:read" || action === "adapter:write",
    );
    render(<AdaptersPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByRole("button", { name: "New Adapter" })).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "edit" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "delete" })).not.toBeInTheDocument();
  });

  it("shows row and bulk delete with read + delete", async () => {
    const user = userEvent.setup();
    mockHasAction.mockImplementation(
      (action) => action === "adapter:read" || action === "adapter:delete",
    );
    render(<AdaptersPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByRole("button", { name: "delete" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "New Adapter" })).not.toBeInTheDocument();

    await user.click(screen.getAllByRole("checkbox").at(-1)!);
    expect(await screen.findByRole("button", { name: "batchDelete" })).toBeInTheDocument();
  });

  it("keeps bulk delete hidden when the adapters resource is missing", async () => {
    const user = userEvent.setup();
    mockHasAction.mockImplementation(
      (action) => action === "adapter:read" || action === "adapter:delete",
    );
    mockHasResource.mockReturnValue(false);
    render(<AdaptersPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByRole("button", { name: "delete" })).toBeInTheDocument();
    await user.click(screen.getAllByRole("checkbox").at(-1)!);
    expect(screen.queryByRole("button", { name: "batchDelete" })).not.toBeInTheDocument();
  });
});

// ─────────────────────────────────────────────
// Adapter create / detail
// ─────────────────────────────────────────────

describe("adapter form permission wiring", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setSession();
    mockHasResource.mockReturnValue(true);
    mockGetAdapter.mockResolvedValue(adapterDetail);
  });

  it("passes adapter:write to the create form's submit guard slot", () => {
    render(<AdapterCreatePage />, { wrapper: createTestWrapper() });

    expect(screen.getByTestId("submit-permission")).toHaveTextContent(
      JSON.stringify({ permission: "adapter:write", resource: "adapters" }),
    );
  });

  it("renders 403 without adapter:read on the detail page", () => {
    mockHasAction.mockReturnValue(false);
    render(<AdapterDetailPage />, { wrapper: createTestWrapper() });

    expect(screen.getByText("403")).toBeInTheDocument();
    expect(screen.queryByTestId("resource-form")).not.toBeInTheDocument();
  });

  it("shows adapter:write submission but hides delete with read + write", async () => {
    mockHasAction.mockImplementation(
      (action) => action === "adapter:read" || action === "adapter:write",
    );
    render(<AdapterDetailPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByTestId("resource-form")).toBeInTheDocument();
    expect(screen.getByTestId("submit-permission")).toHaveTextContent(
      JSON.stringify({ permission: "adapter:write", resource: "adapters" }),
    );
    expect(screen.queryByRole("button", { name: "delete" })).not.toBeInTheDocument();
  });

  it("shows delete with adapter:delete", async () => {
    mockHasAction.mockImplementation(
      (action) =>
        action === "adapter:read" || action === "adapter:write" || action === "adapter:delete",
    );
    render(<AdapterDetailPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByTestId("resource-form")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "delete" })).toBeInTheDocument();
  });
});
