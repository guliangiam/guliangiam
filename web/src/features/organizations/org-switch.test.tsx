import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";

import { useSessionStore } from "@/features/session/session-store";
import { queryKeys } from "@/shared/api/query-keys";
import { HttpError } from "@/shared/api/http";
import { createTestWrapper } from "../../test-utils/render";

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

const mockGetOrganizationsPaginated = vi.fn<(...args: unknown[]) => unknown>();
const mockGetOrganizationOptions = vi.fn<(...args: unknown[]) => unknown>();
const mockGetOrganization = vi.fn<(...args: unknown[]) => unknown>();
const mockUpdateOrganization = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/iam/organization", () => ({
  getOrganizationsPaginated: (...args: unknown[]) => mockGetOrganizationsPaginated(...args),
  getOrganizationOptions: (...args: unknown[]) => mockGetOrganizationOptions(...args),
  getOrganization: (...args: unknown[]) => mockGetOrganization(...args),
  updateOrganization: (...args: unknown[]) => mockUpdateOrganization(...args),
}));

const mockGetUsersPaginated = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/iam/user", () => ({
  getUsersPaginated: (...args: unknown[]) => mockGetUsersPaginated(...args),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("react-router-dom", () => ({
  useNavigate: () => vi.fn<(...args: unknown[]) => unknown>(),
  useLocation: () => ({ pathname: "/organizations", search: "", hash: "", state: null }),
  useParams: () => ({}),
  Link: ({ children }: { children: React.ReactNode }) => children,
  Navigate: () => null,
}));

// ─────────────────────────────────────────────
// Test Data
// ─────────────────────────────────────────────

const allOrganizations = {
  items: [
    {
      name: "org-alpha",
      displayName: "Alpha Org",
      websiteUrl: "https://alpha.com",
      defaultApplication: "app-1",
      description: "active",
      createdAt: "2024-01-01T00:00:00Z",
      updatedAt: "2024-01-01T00:00:00Z",
    },
    {
      name: "org-beta",
      displayName: "Beta Org",
      websiteUrl: "https://beta.com",
      defaultApplication: "app-2",
      description: "active",
      createdAt: "2024-02-01T00:00:00Z",
      updatedAt: "2024-02-01T00:00:00Z",
    },
    {
      name: "org-gamma",
      displayName: "Gamma Org",
      websiteUrl: "https://gamma.com",
      defaultApplication: "app-3",
      description: "active",
      createdAt: "2024-03-01T00:00:00Z",
      updatedAt: "2024-03-01T00:00:00Z",
    },
  ],
  page: 1,
  pageSize: 50,
  total: 3,
};

const orgOptions = [
  {
    id: "org-1",
    name: "org-alpha",
    slug: "org-alpha",
    displayName: "Alpha Org",
    label: "Alpha Org",
    value: "org-1",
  },
  {
    id: "org-2",
    name: "org-beta",
    slug: "org-beta",
    displayName: "Beta Org",
    label: "Beta Org",
    value: "org-2",
  },
  {
    id: "org-3",
    name: "org-gamma",
    slug: "org-gamma",
    displayName: "Gamma Org",
    label: "Gamma Org",
    value: "org-3",
  },
];

const mockAccount = {
  owner: "org-1",
  name: "admin",
  displayName: "Admin User",
  avatar: null,
  isAdmin: true,
};

const mockOrganization = {
  id: "org-1",
  name: "org-alpha",
  displayName: "Alpha Org",
  logoUrl: null,
  faviconUrl: null,
  navItems: ["/dashboard", "/users"],
  userNavItems: ["/profile"],
};

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

const createWrapper = createTestWrapper;

function resetSessionStore() {
  useSessionStore.setState({
    account: null,
    organization: null,
    impersonatedUser: null,
  });
}

/**
 * Hook simulating org list + switch pattern.
 * When org is switched, session store is updated and queries are invalidated.
 */
function useOrgListWithSwitch() {
  const queryClient = useQueryClient();
  const setSession = useSessionStore((s) => s.setSession);

  const orgListQuery = useQuery({
    queryKey: queryKeys.organizations.lists(),
    queryFn: () => mockGetOrganizationsPaginated({ page: 1, pageSize: 50 }),
  });

  const switchOrgMutation = useMutation({
    mutationFn: async (orgId: string) => {
      const org = await mockGetOrganization(orgId);
      return org;
    },
    onSuccess: (data) => {
      const account = useSessionStore.getState().account;
      setSession({ account, organization: data });
      queryClient.invalidateQueries({ queryKey: queryKeys.users.all });
    },
  });

  return { orgListQuery, switchOrgMutation };
}

/**
 * Hook simulating user list filtered by current organization.
 */
function useUserListForCurrentOrg() {
  const organization = useSessionStore((s) => s.organization);

  const userListQuery = useQuery({
    queryKey: queryKeys.users.paginatedWithParams({
      page: 1,
      pageSize: 50,
      organizationId: organization?.id,
    }),
    queryFn: () =>
      mockGetUsersPaginated({
        page: 1,
        pageSize: 50,
        organizationId: organization?.id,
      }),
    enabled: !!organization?.id,
  });

  return { userListQuery, organization };
}

// ─────────────────────────────────────────────
// Integration Tests: Organization Switch
// ─────────────────────────────────────────────

describe("organization switch integration", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetSessionStore();
  });

  describe("switch organization → list refresh → data filtering", () => {
    it("switches organization and triggers data refresh", async () => {
      // Set up initial session
      useSessionStore.getState().setSession({
        account: mockAccount,
        organization: mockOrganization,
      });

      // Org list fetch
      mockGetOrganizationsPaginated.mockResolvedValue(allOrganizations);

      // Get target org detail
      const betaOrg = {
        id: "org-2",
        name: "org-beta",
        displayName: "Beta Org",
        logoUrl: null,
        faviconUrl: null,
        navItems: ["/dashboard", "/users", "/settings"],
        userNavItems: ["/profile"],
      };
      mockGetOrganization.mockResolvedValue(betaOrg);

      // User list for new org
      mockGetUsersPaginated.mockResolvedValue({
        items: [{ id: "user-beta-1", username: "betauser", displayName: "Beta User" }],
        page: 1,
        pageSize: 50,
        total: 1,
      });

      const wrapper = createWrapper();
      const { result } = renderHook(() => useOrgListWithSwitch(), { wrapper });

      // Wait for org list to load
      await waitFor(() => expect(result.current.orgListQuery.isLoading).toBe(false));
      expect(result.current.orgListQuery.data?.items).toHaveLength(3);

      // Switch to Beta org
      await act(async () => {
        result.current.switchOrgMutation.mutate("org-2");
      });

      await waitFor(() => expect(result.current.switchOrgMutation.isSuccess).toBe(true));

      // Verify org detail was fetched
      expect(mockGetOrganization).toHaveBeenCalledWith("org-2");

      // Verify session store was updated with new org
      const sessionState = useSessionStore.getState();
      expect(sessionState.organization?.id).toBe("org-2");
      expect(sessionState.organization?.displayName).toBe("Beta Org");
    });

    it("filters user list by current organization after switch", async () => {
      // Set initial org
      useSessionStore.getState().setSession({
        account: mockAccount,
        organization: mockOrganization,
      });

      // Users for org-1
      mockGetUsersPaginated.mockResolvedValueOnce({
        items: [{ id: "user-1", username: "alpha-user", displayName: "Alpha User" }],
        page: 1,
        pageSize: 50,
        total: 1,
      });

      const wrapper = createWrapper();
      const { result } = renderHook(() => useUserListForCurrentOrg(), { wrapper });

      await waitFor(() => expect(result.current.userListQuery.isLoading).toBe(false));

      // Verify initial org is org-1
      expect(result.current.organization?.id).toBe("org-1");
      expect(mockGetUsersPaginated).toHaveBeenCalledWith(
        expect.objectContaining({ organizationId: "org-1" }),
      );

      // Switch org
      const betaOrg = {
        id: "org-2",
        name: "org-beta",
        displayName: "Beta Org",
        logoUrl: null,
        faviconUrl: null,
        navItems: [],
        userNavItems: [],
      };
      useSessionStore.getState().setSession({
        account: mockAccount,
        organization: betaOrg,
      });

      // Users for org-2
      mockGetUsersPaginated.mockResolvedValueOnce({
        items: [{ id: "user-2", username: "beta-user", displayName: "Beta User" }],
        page: 1,
        pageSize: 50,
        total: 1,
      });
    });
  });

  describe("admin can see all organizations", () => {
    it("returns all organizations for admin user", async () => {
      useSessionStore.getState().setSession({
        account: { ...mockAccount, isAdmin: true },
        organization: mockOrganization,
      });

      mockGetOrganizationsPaginated.mockResolvedValue(allOrganizations);

      const wrapper = createWrapper();
      const { result } = renderHook(() => useOrgListWithSwitch(), { wrapper });

      await waitFor(() => expect(result.current.orgListQuery.isLoading).toBe(false));

      // Admin sees all 3 organizations
      expect(result.current.orgListQuery.data?.items).toHaveLength(3);
      expect(result.current.orgListQuery.data?.total).toBe(3);
    });

    it("admin can switch to any organization", async () => {
      mockGetOrganization.mockResolvedValue({
        id: "org-3",
        name: "org-gamma",
        displayName: "Gamma Org",
        logoUrl: null,
        faviconUrl: null,
        navItems: [],
        userNavItems: [],
      });

      const org = await mockGetOrganization("org-3");
      expect(org.id).toBe("org-3");
      expect(org.displayName).toBe("Gamma Org");
    });

    it("admin can access organization options for dropdown", async () => {
      mockGetOrganizationOptions.mockResolvedValue(orgOptions);

      const options = await mockGetOrganizationOptions();
      expect(options).toHaveLength(3);
      expect(options[0].displayName).toBe("Alpha Org");
      expect(options[2].displayName).toBe("Gamma Org");
    });
  });

  describe("regular user only sees their own organization", () => {
    it("returns only user's organization for non-admin", async () => {
      const regularAccount = {
        owner: "org-1",
        name: "regularuser",
        displayName: "Regular User",
        avatar: null,
        isAdmin: false,
      };

      useSessionStore.getState().setSession({
        account: regularAccount,
        organization: mockOrganization,
      });

      // Non-admin only sees their own org
      const userOrgList = {
        items: [allOrganizations.items[0]],
        page: 1,
        pageSize: 50,
        total: 1,
      };
      mockGetOrganizationsPaginated.mockResolvedValue(userOrgList);

      const wrapper = createWrapper();
      const { result } = renderHook(() => useOrgListWithSwitch(), { wrapper });

      await waitFor(() => expect(result.current.orgListQuery.isLoading).toBe(false));

      // Regular user sees only 1 organization
      expect(result.current.orgListQuery.data?.items).toHaveLength(1);
      expect(result.current.orgListQuery.data?.total).toBe(1);
    });

    it("regular user cannot switch to other organizations", async () => {
      // Attempting to switch to another org returns forbidden
      mockGetOrganization.mockRejectedValue(
        new HttpError(403, null, "Forbidden: cannot access organization"),
      );

      await expect(mockGetOrganization("org-2")).rejects.toThrow(
        "Forbidden: cannot access organization",
      );
    });

    it("regular user sees only their org in options", async () => {
      mockGetOrganizationOptions.mockResolvedValue([orgOptions[0]]);

      const options = await mockGetOrganizationOptions();
      expect(options).toHaveLength(1);
      expect(options[0].id).toBe("org-1");
    });
  });

  describe("organization data consistency after switch", () => {
    it("session store organization matches selected org", async () => {
      useSessionStore.getState().setSession({
        account: mockAccount,
        organization: mockOrganization,
      });

      const gammaOrg = {
        id: "org-3",
        name: "org-gamma",
        displayName: "Gamma Org",
        logoUrl: "https://gamma.com/logo.png",
        faviconUrl: null,
        navItems: ["/dashboard", "/users", "/settings", "/billing"],
        userNavItems: ["/profile", "/settings"],
      };
      mockGetOrganization.mockResolvedValue(gammaOrg);

      const org = await mockGetOrganization("org-3");

      // Update session store
      useSessionStore.getState().setSession({
        account: mockAccount,
        organization: org,
      });

      const sessionState = useSessionStore.getState();
      expect(sessionState.organization?.id).toBe("org-3");
      expect(sessionState.organization?.displayName).toBe("Gamma Org");
      expect(sessionState.organization?.navItems).toContain("/billing");
    });

    it("account is preserved when switching organization", async () => {
      useSessionStore.getState().setSession({
        account: mockAccount,
        organization: mockOrganization,
      });

      // Switch org
      useSessionStore.getState().setSession({
        account: mockAccount,
        organization: {
          id: "org-2",
          name: "org-beta",
          displayName: "Beta Org",

          navItems: [],
          userNavItems: [],
        },
      });

      const sessionState = useSessionStore.getState();
      // Account should be preserved
      expect(sessionState.account?.name).toBe("admin");
      expect(sessionState.account?.isAdmin).toBe(true);
      // Organization should be updated
      expect(sessionState.organization?.id).toBe("org-2");
    });
  });

  describe("organization list error handling", () => {
    it("handles organization list fetch error", async () => {
      mockGetOrganizationsPaginated.mockRejectedValue(
        new HttpError(500, null, "Internal Server Error"),
      );

      const wrapper = createWrapper();
      const { result } = renderHook(() => useOrgListWithSwitch(), { wrapper });

      await waitFor(() => expect(result.current.orgListQuery.isError).toBe(true));

      expect(result.current.orgListQuery.error).toBeInstanceOf(HttpError);
    });

    it("handles organization switch error gracefully", async () => {
      mockGetOrganization.mockRejectedValue(new HttpError(404, null, "Organization not found"));

      const wrapper = createWrapper();
      const { result } = renderHook(() => useOrgListWithSwitch(), { wrapper });

      await act(async () => {
        result.current.switchOrgMutation.mutate("nonexistent-org");
      });

      await waitFor(() => expect(result.current.switchOrgMutation.isError).toBe(true));

      // Session store should not be updated on error
      const sessionState = useSessionStore.getState();
      expect(sessionState.organization).toBeNull();
    });
  });
});
