import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";

import { useProviderList } from "./useProviderList";
import { createTestWrapper } from "../../test-utils/render";
// Mock API
const mockGetProvidersPaginated = vi.fn<(...args: unknown[]) => unknown>();
vi.mock("@/shared/api/iam/provider-extended", () => ({
  getProvidersPaginated: (...args: unknown[]) => mockGetProvidersPaginated(...args),
  deleteProvider: vi.fn<(...args: unknown[]) => Promise<unknown>>().mockResolvedValue(undefined),
}));

// Mock session store
vi.mock("@/features/session/session-store", () => ({
  useSessionStore: vi.fn<(...args: unknown[]) => unknown>(
    (selector: (state: Record<string, unknown>) => unknown) =>
      selector({
        organization: { id: "org-1", name: "default" },
      }),
  ),
}));

// Mock antd App
vi.mock("antd", () => ({
  App: {
    useApp: () => ({
      message: {
        success: vi.fn<(...args: unknown[]) => unknown>(),
        error: vi.fn<(...args: unknown[]) => unknown>(),
      },
    }),
  },
}));

// Mock react-i18next
vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
}));

// Mock react-router-dom
vi.mock("react-router-dom", () => ({
  useNavigate: () => vi.fn<(...args: unknown[]) => unknown>(),
}));

const createWrapper = () => createTestWrapper({ withApp: false });

describe("useProviderList", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetProvidersPaginated.mockResolvedValue({
      items: [
        { id: "prov-1", name: "GitHub OAuth", category: "OAuth", providerType: "GitHub" },
        { id: "prov-2", name: "SAML Provider", category: "SAML", providerType: "SAML 2.0" },
      ],
      page: 1,
      pageSize: 50,
      total: 2,
    });
  });

  it("initializes with default pagination", () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => useProviderList(), { wrapper });

    expect(result.current.pagination.page).toBe(1);
    expect(result.current.pagination.pageSize).toBe(50);
    expect(result.current.pagination.total).toBe(0);
  });

  it("initializes with empty search", () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => useProviderList(), { wrapper });

    expect(result.current.search).toBe("");
  });

  it("initializes with empty activeFilters", () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => useProviderList(), { wrapper });

    expect(result.current.activeFilters).toEqual({});
  });

  it("initializes with empty selectedRowKeys", () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => useProviderList(), { wrapper });

    expect(result.current.selectedRowKeys).toEqual([]);
  });

  it("setSearch updates search and resets page to 1", async () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => useProviderList(), { wrapper });

    // First set page to 2
    await act(async () => {
      result.current.setPagination((prev) => ({ ...prev, page: 2 }));
    });

    expect(result.current.pagination.page).toBe(2);

    // Then set search - should reset page to 1
    await act(async () => {
      result.current.setSearch("github");
    });

    expect(result.current.search).toBe("github");
    expect(result.current.pagination.page).toBe(1);
  });

  it("setOrgFilter updates orgFilter and resets page to 1", async () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => useProviderList(), { wrapper });

    await act(async () => {
      result.current.setPagination((prev) => ({ ...prev, page: 3 }));
    });

    await act(async () => {
      result.current.setOrgFilter("org-2");
    });

    expect(result.current.orgFilter).toBe("org-2");
    expect(result.current.pagination.page).toBe(1);
  });

  it("setActiveFilters updates filters and resets page to 1", async () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => useProviderList(), { wrapper });

    await act(async () => {
      result.current.setPagination((prev) => ({ ...prev, page: 5 }));
    });

    await act(async () => {
      result.current.setActiveFilters({ category: "OAuth", type: "GitHub" });
    });

    expect(result.current.activeFilters).toEqual({ category: "OAuth", type: "GitHub" });
    expect(result.current.pagination.page).toBe(1);
  });

  it("setSelectedRowKeys updates selected keys", async () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => useProviderList(), { wrapper });

    await act(async () => {
      result.current.setSelectedRowKeys(["prov-1", "prov-2"]);
    });

    expect(result.current.selectedRowKeys).toEqual(["prov-1", "prov-2"]);
  });

  it("setPagination updates pagination state", async () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => useProviderList(), { wrapper });

    await act(async () => {
      result.current.setPagination((prev) => ({ ...prev, page: 3, pageSize: 20 }));
    });

    expect(result.current.pagination.page).toBe(3);
    expect(result.current.pagination.pageSize).toBe(20);
  });

  it("fetches providers on mount", async () => {
    const wrapper = createWrapper();
    renderHook(() => useProviderList(), { wrapper });

    await waitFor(() => expect(mockGetProvidersPaginated).toHaveBeenCalled());
  });

  it("uses initialOrgId when provided", async () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => useProviderList({ initialOrgId: "org-initial" }), {
      wrapper,
    });

    expect(result.current.orgFilter).toBe("org-initial");
  });
});
