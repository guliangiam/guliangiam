import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "antd";
import { QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, useLocation, useNavigate } from "react-router-dom";
import type { ReactNode } from "react";
import { useSessionStore } from "@/features/session/session-store";
import { createTestQueryClient } from "@/test-utils/render";
import { useUsersPageController } from "./useUsersPageController";

const { mockGetUsersPaginated, mockUpdateUser, mockImpersonateUser } = vi.hoisted(() => ({
  mockGetUsersPaginated: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockUpdateUser: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockImpersonateUser: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("@/shared/api/iam/user", () => ({
  getUsersPaginated: (...args: unknown[]) => mockGetUsersPaginated(...args),
  deleteUser: vi.fn<() => void>(),
  exportUsersToExcel: vi.fn<() => void>(),
  impersonateUser: (...args: unknown[]) => mockImpersonateUser(...args),
  updateUser: (...args: unknown[]) => mockUpdateUser(...args),
}));

vi.mock("@/shared/api/iam/organization", () => ({
  getOrganizationOptions: vi.fn<() => Promise<unknown[]>>().mockResolvedValue([]),
}));

vi.mock("@/shared/hooks/usePrefetch", () => ({
  usePrefetch: () => ({
    prefetchUserDetail: vi.fn<(organizationSlug: string, username: string) => void>(),
    hoverPrefetch: () => ({}),
  }),
}));

vi.mock("@/shared/hooks/useResourceNames", () => ({
  useOrganizationNames: () => ({ nameMap: new Map() }),
  useApplicationNames: () => ({ nameMap: new Map() }),
}));

function renderController(initialEntries: string[] = ["/"], initialIndex = 0) {
  const queryClient = createTestQueryClient();
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <MemoryRouter initialEntries={initialEntries} initialIndex={initialIndex}>
        <QueryClientProvider client={queryClient}>
          <App component={false}>{children}</App>
        </QueryClientProvider>
      </MemoryRouter>
    );
  }

  return {
    ...renderHook(
      () => ({ ...useUsersPageController(), navigate: useNavigate(), location: useLocation() }),
      {
        wrapper: Wrapper,
      },
    ),
    invalidateSpy,
  };
}

describe("useUsersPageController import", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetUsersPaginated.mockResolvedValue({ items: [], total: 0 });
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

  it("routes the single import entry to /users/import with the active organization filter", async () => {
    useSessionStore.setState({
      account: { owner: "built-in", name: "admin", isAdmin: true },
    });
    const { result } = renderController(["/users?organizationId=org-2"]);

    await waitFor(() => expect(result.current.orgFilter).toBe("org-2"));
    act(() => result.current.handleImport());

    await waitFor(() => expect(result.current.location.pathname).toBe("/users/import"));
    expect(result.current.location.search).toBe("?organizationId=org-2");
  });

  it("omits the organization query when the list has no organization context", async () => {
    useSessionStore.setState({ organization: null });
    const { result } = renderController();

    act(() => result.current.handleImport());

    await waitFor(() => expect(result.current.location.pathname).toBe("/users/import"));
    expect(result.current.location.search).toBe("");
  });

  it("no longer exposes the removed in-page modal import chain", () => {
    const { result } = renderController();

    expect("importModalOpen" in result.current).toBe(false);
    expect("handleImportFile" in result.current).toBe(false);
  });

  it("consumes ?organizationId into the list query and removes it when cleared", async () => {
    useSessionStore.setState({
      account: { owner: "built-in", name: "admin", isAdmin: true },
    });
    const { result } = renderController(["/users?organizationId=org-2"]);

    await waitFor(() =>
      expect(mockGetUsersPaginated).toHaveBeenCalledWith(
        expect.objectContaining({ organizationId: "org-2" }),
      ),
    );
    expect(result.current.orgFilter).toBe("org-2");
    expect(result.current.orgContext).toBe("org-2");

    act(() => result.current.handleOrgChange(""));

    await waitFor(() => expect(result.current.orgContext).toBe(""));
    expect(result.current.orgFilter).toBe("");
    await waitFor(() =>
      expect(mockGetUsersPaginated).toHaveBeenLastCalledWith(
        expect.objectContaining({ organizationId: undefined }),
      ),
    );
  });

  it("follows the URL when browser history changes the organizationId", async () => {
    useSessionStore.setState({
      account: { owner: "built-in", name: "admin", isAdmin: true },
    });
    const { result } = renderController(
      ["/users?organizationId=org-2", "/users?organizationId=org-3"],
      1,
    );

    await waitFor(() => expect(result.current.orgFilter).toBe("org-3"));

    act(() => result.current.handleOrgChange("org-4"));
    await waitFor(() => expect(result.current.orgFilter).toBe("org-4"));

    act(() => result.current.navigate(-1));

    await waitFor(() => expect(result.current.orgFilter).toBe("org-2"));
    expect(result.current.orgContext).toBe("org-2");
  });
});

describe("useUsersPageController bulk status", () => {
  const rows = Array.from({ length: 12 }, (_, i) => ({
    id: `u${i + 1}`,
    username: `user${i + 1}`,
    status: "disabled",
    revision: 1,
    organizationId: "org-1",
  }));
  const ids = rows.map((row) => row.id);

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetUsersPaginated.mockResolvedValue({ items: rows, total: rows.length });
    mockUpdateUser.mockResolvedValue({});
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

  async function renderWithRows() {
    const rendered = renderController(["/users"]);
    await waitFor(() => expect(rendered.result.current.crud.query.data?.length).toBe(12));
    return rendered;
  }

  // 原实现 Promise.allSettled 全量并发无上限；百条选中会瞬时百并发触发限流。
  it("caps concurrent status updates at the batch size of 10", async () => {
    let active = 0;
    let maxActive = 0;
    mockUpdateUser.mockImplementation(async () => {
      active += 1;
      maxActive = Math.max(maxActive, active);
      await new Promise((resolve) => setTimeout(resolve, 5));
      active -= 1;
      return {};
    });
    const { result } = await renderWithRows();

    await act(async () => {
      await result.current.bulkStatusMutation.mutateAsync({ status: "active", ids });
    });

    expect(mockUpdateUser).toHaveBeenCalledTimes(12);
    expect(maxActive).toBeLessThanOrEqual(10);
    expect(mockUpdateUser).toHaveBeenCalledWith("u1", { status: "active", revision: 1 }, "org-1");
  });

  it("keeps only the failed rows selected after a partial failure", async () => {
    mockUpdateUser.mockImplementation(async (userId: unknown) => {
      if (userId === "u3" || userId === "u7") throw new Error("boom");
      return {};
    });
    const { result } = await renderWithRows();
    act(() => result.current.crud.setSelectedKeys(ids));

    await act(async () => {
      await result.current.bulkStatusMutation.mutateAsync({ status: "active", ids });
    });

    await waitFor(() => expect(result.current.crud.selectedKeys).toEqual(["u3", "u7"]));
  });

  it("clears the selection when every row succeeds", async () => {
    const { result } = await renderWithRows();
    act(() => result.current.crud.setSelectedKeys(ids));

    await act(async () => {
      await result.current.bulkStatusMutation.mutateAsync({ status: "active", ids });
    });

    await waitFor(() => expect(result.current.crud.selectedKeys).toEqual([]));
  });

  it("skips rows already in the target status", async () => {
    mockGetUsersPaginated.mockResolvedValue({
      items: rows.map((row) => ({ ...row, status: "active" })),
      total: rows.length,
    });
    const { result } = await renderWithRows();

    await act(async () => {
      await result.current.bulkStatusMutation.mutateAsync({ status: "active", ids });
    });

    expect(mockUpdateUser).not.toHaveBeenCalled();
  });
});

describe("useUsersPageController impersonation", () => {
  beforeEach(() => {
    vi.clearAllMocks();
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
      impersonationToken: null,
    });
    mockGetUsersPaginated.mockResolvedValue({
      items: [
        {
          id: "user-1",
          username: "bob",
          displayName: "Bob",
          organizationId: "org-1",
          status: "active",
          revision: 0,
        },
      ],
      total: 1,
    });
  });

  it("keeps the imp token in memory so subsequent requests run as the target", async () => {
    mockImpersonateUser.mockResolvedValue({ token: "imp_test_token" });
    const { result } = renderController();
    await waitFor(() => expect(result.current.crud.query.data?.length).toBe(1));

    await act(async () => {
      await result.current.impersonateMutation.mutateAsync("user-1");
    });

    const state = useSessionStore.getState();
    expect(state.impersonatedUser?.id).toBe("user-1");
    expect(state.impersonationToken).toBe("imp_test_token");
    expect(result.current.location.pathname).toBe("/");
  });

  it("does not store a token when impersonation fails", async () => {
    mockImpersonateUser.mockRejectedValue(new Error("forbidden"));
    const { result } = renderController();
    await waitFor(() => expect(result.current.crud.query.data?.length).toBe(1));

    await act(async () => {
      await result.current.impersonateMutation.mutateAsync("user-1").catch(() => undefined);
    });

    const state = useSessionStore.getState();
    expect(state.impersonatedUser).toBeNull();
    expect(state.impersonationToken).toBeNull();
  });
});
