import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { createTestWrapper } from "@/test-utils/render";
import { useSessionStore } from "@/features/session/session-store";
import OrganizationsPage from "./OrganizationsPage";

const { mockGetOrganizationsPaginated, mockDeleteOrganization, mockGetApplicationOptions } =
  vi.hoisted(() => ({
    mockGetOrganizationsPaginated: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    mockDeleteOrganization: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    mockGetApplicationOptions: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  }));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
}));

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    permissions: {
      menus: [],
      actions: ["organizations:write"],
      resources: ["organizations"],
    },
    isLoading: false,
    hasMenu: () => true,
    hasAction: () => true,
    hasResource: () => true,
    refetch: vi.fn<(...args: unknown[]) => unknown>(),
  }),
}));

vi.mock("@/shared/api/iam/organization", () => ({
  getOrganizationsPaginated: (...args: unknown[]) => mockGetOrganizationsPaginated(...args),
  deleteOrganization: (...args: unknown[]) => mockDeleteOrganization(...args),
}));

vi.mock("@/shared/api/iam/application", () => ({
  getApplicationOptions: (...args: unknown[]) => mockGetApplicationOptions(...args),
}));

const organizations = [
  {
    id: "org-1",
    name: "alpha",
    slug: "alpha",
    displayName: "Alpha",
    status: "active",
    passwordType: "argon2id",
    balance: 12.5,
    currency: "USD",
    createdAt: "2026-01-01T00:00:00Z",
  },
  {
    id: "org-2",
    name: "beta",
    slug: "beta",
    displayName: "Beta",
    status: "disabled",
    passwordType: "argon2id",
    balance: 0,
    currency: "USD",
    createdAt: "2026-01-02T00:00:00Z",
  },
];

function renderPage() {
  return render(
    <MemoryRouter initialEntries={["/organizations"]}>
      <OrganizationsPage />
    </MemoryRouter>,
    { wrapper: createTestWrapper() },
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  useSessionStore.setState({
    account: { owner: "built-in", name: "admin", isAdmin: true },
    organization: null,
    impersonatedUser: null,
  });
  mockGetOrganizationsPaginated.mockResolvedValue({ items: organizations, total: 2 });
  mockDeleteOrganization.mockResolvedValue(undefined);
  mockGetApplicationOptions.mockResolvedValue([]);
});

describe("OrganizationsPage list", () => {
  it("loads the first page and passes the pagination parameters through", async () => {
    renderPage();

    expect(await screen.findByText("Alpha")).toBeInTheDocument();
    expect(screen.getByText("Beta")).toBeInTheDocument();
    expect(mockGetOrganizationsPaginated).toHaveBeenCalledTimes(1);
    const params = mockGetOrganizationsPaginated.mock.calls[0]![0] as Record<string, unknown>;
    expect(params).toMatchObject({ page: 1, pageSize: 50 });
    expect(params.search).toBeUndefined();
    expect(params.status).toBeUndefined();
  });

  it("requests the next page through the server pagination control", async () => {
    mockGetOrganizationsPaginated.mockResolvedValue({ items: organizations, total: 120 });
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Alpha");

    const page2 = await screen.findByRole("button", { name: "Page 2" });
    await user.click(page2);

    await waitFor(() =>
      expect(mockGetOrganizationsPaginated).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 2, pageSize: 50 }),
      ),
    );
  });

  it("passes the selected status filter through to the query", async () => {
    const user = userEvent.setup();
    const { container } = renderPage();
    await screen.findByText("Alpha");

    const filterSelect = container.querySelector(".gl-resource-card__toolbar .ant-select");
    expect(filterSelect).not.toBeNull();
    await user.click(filterSelect as HTMLElement);
    await user.click(await screen.findByText("enabled"));

    await waitFor(() =>
      expect(mockGetOrganizationsPaginated).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 1, status: "active" }),
      ),
    );
  });

  it("renders the empty state when no organization exists", async () => {
    mockGetOrganizationsPaginated.mockResolvedValue({ items: [], total: 0 });
    renderPage();

    expect(await screen.findByText("noData")).toBeInTheDocument();
  });

  it("renders the load-failed state when the query fails", async () => {
    mockGetOrganizationsPaginated.mockRejectedValue(new Error("boom"));
    renderPage();

    expect((await screen.findAllByText("loadFailed")).length).toBeGreaterThan(0);
  });
});
