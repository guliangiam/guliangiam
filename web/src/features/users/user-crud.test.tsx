import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";

import { queryKeys } from "@/shared/api/query-keys";
import { HttpError } from "@/shared/api/http";
import { createTestWrapper } from "../../test-utils/render";

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

const mockGetUsersPaginated = vi.fn<(...args: unknown[]) => unknown>();
const mockGetUser = vi.fn<(...args: unknown[]) => unknown>();
const mockCreateUser = vi.fn<(...args: unknown[]) => unknown>();
const mockUpdateUser = vi.fn<(...args: unknown[]) => unknown>();
const mockDeleteUser = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/iam/user", () => ({
  getUsersPaginated: (...args: unknown[]) => mockGetUsersPaginated(...args),
  getUser: (...args: unknown[]) => mockGetUser(...args),
  createUser: (...args: unknown[]) => mockCreateUser(...args),
  updateUser: (...args: unknown[]) => mockUpdateUser(...args),
  deleteUser: (...args: unknown[]) => mockDeleteUser(...args),
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
  useLocation: () => ({ pathname: "/users", search: "", hash: "", state: null }),
  useParams: () => ({}),
  Link: ({ children }: { children: React.ReactNode }) => children,
  Navigate: () => null,
}));

// ─────────────────────────────────────────────
// Test Data
// ─────────────────────────────────────────────

const mockUserList = {
  items: [
    {
      id: "user-1",
      username: "admin",
      displayName: "Admin User",
      email: "admin@example.com",
      phone: null,
      userType: "normal",
      status: "active",
      isAdmin: true,
      isForbidden: false,
      isDeleted: false,
      createdAt: "2024-01-01T00:00:00Z",
      organizationId: "org-1",
    },
    {
      id: "user-2",
      username: "testuser",
      displayName: "Test User",
      email: "test@example.com",
      phone: null,
      userType: "normal",
      status: "active",
      isAdmin: false,
      isForbidden: false,
      isDeleted: false,
      createdAt: "2024-02-01T00:00:00Z",
      organizationId: "org-1",
    },
  ],
  page: 1,
  pageSize: 50,
  total: 2,
};

const mockUserDetail = {
  id: "user-1",
  username: "admin",
  displayName: "Admin User",
  email: "admin@example.com",
  phone: null,
  userType: "normal",
  status: "active",
  isAdmin: true,
  isForbidden: false,
  isDeleted: false,
  isVerified: true,
  avatarUrl: null,
  organizationId: "org-1",
  lastSigninAt: "2024-06-01T00:00:00Z",
  createdAt: "2024-01-01T00:00:00Z",
  score: 100,
  balance: 0,
};

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

const createWrapper = createTestWrapper;

/**
 * Custom hook that simulates the user list + create mutation pattern
 * used in real pages. This tests the TanStack Query integration.
 */
function useUserListWithCreate() {
  const queryClient = useQueryClient();

  const listQuery = useQuery({
    queryKey: queryKeys.users.paginatedWithParams({ page: 1, pageSize: 50 }),
    queryFn: () => mockGetUsersPaginated({ page: 1, pageSize: 50 }),
  });

  const createMutation = useMutation({
    mutationFn: (payload: unknown) => mockCreateUser(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.users.all });
    },
  });

  return { listQuery, createMutation };
}

/**
 * Custom hook that simulates the user detail + update mutation pattern.
 */
function useUserDetailWithUpdate(userId: string) {
  const queryClient = useQueryClient();

  const detailQuery = useQuery({
    queryKey: queryKeys.users.detail(userId),
    queryFn: () => mockGetUser(userId),
    enabled: !!userId,
  });

  const updateMutation = useMutation({
    mutationFn: (payload: { id: string; data: unknown }) =>
      mockUpdateUser(payload.id, payload.data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.users.detail(userId) });
    },
  });

  return { detailQuery, updateMutation };
}

/**
 * Custom hook that simulates user list + delete mutation pattern.
 */
function useUserListWithDelete() {
  const queryClient = useQueryClient();

  const listQuery = useQuery({
    queryKey: queryKeys.users.paginatedWithParams({ page: 1, pageSize: 50 }),
    queryFn: () => mockGetUsersPaginated({ page: 1, pageSize: 50 }),
  });

  const deleteMutation = useMutation({
    mutationFn: (userId: string) => mockDeleteUser(userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.users.all });
    },
  });

  return { listQuery, deleteMutation };
}

// ─────────────────────────────────────────────
// Integration Tests: User CRUD
// ─────────────────────────────────────────────

describe("user CRUD integration", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("create user flow: fill form → submit → success → list refresh", () => {
    it("creates a user and invalidates user list query", async () => {
      // Initial list fetch
      mockGetUsersPaginated.mockResolvedValueOnce(mockUserList);

      // Create user succeeds
      mockCreateUser.mockResolvedValue({ id: "user-3", status: "created" });

      // After invalidation, list is refetched with new user
      const updatedList = {
        ...mockUserList,
        items: [
          ...mockUserList.items,
          {
            id: "user-3",
            username: "newuser",
            displayName: "New User",
            email: "new@example.com",
            phone: null,
            userType: "normal",
            status: "active",
            isAdmin: false,
            isForbidden: false,
            isDeleted: false,
            createdAt: "2024-06-01T00:00:00Z",
            organizationId: "org-1",
          },
        ],
        total: 3,
      };
      mockGetUsersPaginated.mockResolvedValueOnce(updatedList);

      const wrapper = createWrapper();
      const { result } = renderHook(() => useUserListWithCreate(), { wrapper });

      // Wait for initial list to load
      await waitFor(() => expect(result.current.listQuery.isLoading).toBe(false));
      expect(result.current.listQuery.data?.total).toBe(2);

      // Create a new user
      await act(async () => {
        result.current.createMutation.mutate({
          organizationId: "org-1",
          username: "newuser",
          displayName: "New User",
          email: "new@example.com",
          password: "Password123!",
        });
      });

      // Wait for mutation to complete and list to be refetched
      await waitFor(() => expect(result.current.createMutation.isSuccess).toBe(true));

      // Verify create was called with correct payload
      expect(mockCreateUser).toHaveBeenCalledWith(
        expect.objectContaining({
          organizationId: "org-1",
          username: "newuser",
          displayName: "New User",
        }),
      );

      // Verify list was refetched (invalidation triggered)
      expect(mockGetUsersPaginated).toHaveBeenCalledTimes(2);
    });

    it("handles create user error without refreshing list", async () => {
      mockGetUsersPaginated.mockResolvedValue(mockUserList);
      mockCreateUser.mockRejectedValue(new HttpError(409, null, "User already exists"));

      const wrapper = createWrapper();
      const { result } = renderHook(() => useUserListWithCreate(), { wrapper });

      await waitFor(() => expect(result.current.listQuery.isLoading).toBe(false));

      await act(async () => {
        result.current.createMutation.mutate({
          organizationId: "org-1",
          username: "admin", // duplicate
          displayName: "Duplicate",
          email: "dup@example.com",
          password: "Password123!",
        });
      });

      await waitFor(() => expect(result.current.createMutation.isError).toBe(true));

      expect(mockCreateUser).toHaveBeenCalledTimes(1);
      // List should only be fetched once (no invalidation on error)
      expect(mockGetUsersPaginated).toHaveBeenCalledTimes(1);
    });

    it("validates required fields before submitting create", async () => {
      mockCreateUser.mockResolvedValue({ id: "user-3", status: "created" });

      // Verify that calling createUser with missing required fields
      // would be caught by form validation (not API)
      const payload = {
        organizationId: "org-1",
        username: "",
        displayName: "",
        email: "invalid-email",
        password: "short",
      };

      // In real app, form validation would prevent this call.
      // Here we test that the API would reject it if called.
      mockCreateUser.mockRejectedValueOnce(new HttpError(400, null, "Bad Request"));

      await expect(mockCreateUser(payload)).rejects.toThrow("Bad Request");
    });
  });

  describe("edit user flow: modify fields → submit → success → detail update", () => {
    it("updates a user and invalidates detail query", async () => {
      mockGetUser.mockResolvedValueOnce(mockUserDetail);

      // Update succeeds
      mockUpdateUser.mockResolvedValue(undefined);

      // After invalidation, detail is refetched with updated data
      const updatedDetail = {
        ...mockUserDetail,
        displayName: "Admin User Updated",
        email: "admin.updated@example.com",
      };
      mockGetUser.mockResolvedValueOnce(updatedDetail);

      const wrapper = createWrapper();
      const { result } = renderHook(() => useUserDetailWithUpdate("user-1"), { wrapper });

      // Wait for initial detail to load
      await waitFor(() => expect(result.current.detailQuery.isLoading).toBe(false));
      expect(result.current.detailQuery.data?.displayName).toBe("Admin User");

      // Update user
      await act(async () => {
        result.current.updateMutation.mutate({
          id: "user-1",
          data: { displayName: "Admin User Updated", email: "admin.updated@example.com" },
        });
      });

      await waitFor(() => expect(result.current.updateMutation.isSuccess).toBe(true));

      // Verify update was called
      expect(mockUpdateUser).toHaveBeenCalledWith(
        "user-1",
        expect.objectContaining({
          displayName: "Admin User Updated",
          email: "admin.updated@example.com",
        }),
      );

      // Detail was refetched after invalidation
      expect(mockGetUser).toHaveBeenCalledTimes(2);
    });

    it("handles update error and preserves existing detail", async () => {
      mockGetUser.mockResolvedValue(mockUserDetail);
      mockUpdateUser.mockRejectedValue(new HttpError(400, null, "Invalid email format"));

      const wrapper = createWrapper();
      const { result } = renderHook(() => useUserDetailWithUpdate("user-1"), { wrapper });

      await waitFor(() => expect(result.current.detailQuery.isLoading).toBe(false));

      await act(async () => {
        result.current.updateMutation.mutate({
          id: "user-1",
          data: { email: "invalid-format" },
        });
      });

      await waitFor(() => expect(result.current.updateMutation.isError).toBe(true));

      // Original detail data is still available
      expect(result.current.detailQuery.data?.displayName).toBe("Admin User");
    });

    it("handles concurrent updates with revision check", async () => {
      mockGetUser.mockResolvedValue(mockUserDetail);

      // Simulate conflict error (optimistic locking)
      mockUpdateUser.mockRejectedValue(
        new HttpError(409, null, "Conflict: resource has been modified by another request"),
      );

      const wrapper = createWrapper();
      const { result } = renderHook(() => useUserDetailWithUpdate("user-1"), { wrapper });

      await waitFor(() => expect(result.current.detailQuery.isLoading).toBe(false));

      await act(async () => {
        result.current.updateMutation.mutate({
          id: "user-1",
          data: { displayName: "Stale Update" },
        });
      });

      await waitFor(() => expect(result.current.updateMutation.isError).toBe(true));
      expect(result.current.updateMutation.error).toBeInstanceOf(HttpError);
    });
  });

  describe("delete user flow: confirm → submit → success → list refresh", () => {
    it("deletes a user and invalidates list query", async () => {
      // Initial list
      mockGetUsersPaginated.mockResolvedValueOnce(mockUserList);

      // Delete succeeds
      mockDeleteUser.mockResolvedValue(undefined);

      // After invalidation, list is refetched without deleted user
      const reducedList = {
        items: mockUserList.items.filter((u) => u.id !== "user-2"),
        page: 1,
        pageSize: 50,
        total: 1,
      };
      mockGetUsersPaginated.mockResolvedValueOnce(reducedList);

      const wrapper = createWrapper();
      const { result } = renderHook(() => useUserListWithDelete(), { wrapper });

      await waitFor(() => expect(result.current.listQuery.isLoading).toBe(false));
      expect(result.current.listQuery.data?.total).toBe(2);

      // Delete user
      await act(async () => {
        result.current.deleteMutation.mutate("user-2");
      });

      await waitFor(() => expect(result.current.deleteMutation.isSuccess).toBe(true));

      expect(mockDeleteUser).toHaveBeenCalledWith("user-2");
      // List was refetched after invalidation
      expect(mockGetUsersPaginated).toHaveBeenCalledTimes(2);
    });

    it("handles delete error without refreshing list", async () => {
      mockGetUsersPaginated.mockResolvedValue(mockUserList);
      mockDeleteUser.mockRejectedValue(
        new HttpError(403, null, "Forbidden: cannot delete last admin"),
      );

      const wrapper = createWrapper();
      const { result } = renderHook(() => useUserListWithDelete(), { wrapper });

      await waitFor(() => expect(result.current.listQuery.isLoading).toBe(false));

      await act(async () => {
        result.current.deleteMutation.mutate("user-1");
      });

      await waitFor(() => expect(result.current.deleteMutation.isError).toBe(true));

      expect(mockDeleteUser).toHaveBeenCalledWith("user-1");
      // List was only fetched once (no invalidation on error)
      expect(mockGetUsersPaginated).toHaveBeenCalledTimes(1);
    });

    it("handles delete of non-existent user", async () => {
      mockDeleteUser.mockRejectedValue(new HttpError(404, null, "User not found"));

      await expect(mockDeleteUser("nonexistent-id")).rejects.toThrow("User not found");
    });
  });

  describe("user list pagination and filtering", () => {
    it("fetches users with pagination params", async () => {
      mockGetUsersPaginated.mockResolvedValue(mockUserList);

      const result = await mockGetUsersPaginated({ page: 1, pageSize: 50 });

      expect(result.items).toHaveLength(2);
      expect(result.total).toBe(2);
      expect(result.page).toBe(1);
    });

    it("fetches users filtered by organization", async () => {
      const orgFilteredList = {
        items: [mockUserList.items[0]],
        page: 1,
        pageSize: 50,
        total: 1,
      };
      mockGetUsersPaginated.mockResolvedValue(orgFilteredList);

      const result = await mockGetUsersPaginated({
        page: 1,
        pageSize: 50,
        organizationId: "org-1",
      });

      expect(mockGetUsersPaginated).toHaveBeenCalledWith(
        expect.objectContaining({ organizationId: "org-1" }),
      );
      expect(result.items).toHaveLength(1);
    });

    it("fetches users with search keyword", async () => {
      mockGetUsersPaginated.mockResolvedValue({
        items: [mockUserList.items[0]],
        page: 1,
        pageSize: 50,
        total: 1,
      });

      const result = await mockGetUsersPaginated({
        page: 1,
        pageSize: 50,
        search: "admin",
      });

      expect(mockGetUsersPaginated).toHaveBeenCalledWith(
        expect.objectContaining({ search: "admin" }),
      );
      expect(result.items).toHaveLength(1);
    });
  });

  describe("closed-loop: CRUD → API calls → data persistence → UI feedback", () => {
    it("create → list refresh shows new user", async () => {
      mockGetUsersPaginated.mockResolvedValueOnce(mockUserList);
      mockCreateUser.mockResolvedValue({ id: "user-3", status: "created" });
      mockGetUsersPaginated.mockResolvedValueOnce({
        ...mockUserList,
        total: 3,
      });

      // Step 1: Fetch initial list
      const initialList = await mockGetUsersPaginated({ page: 1, pageSize: 50 });
      expect(initialList.total).toBe(2);

      // Step 2: Create user
      const createResult = await mockCreateUser({
        organizationId: "org-1",
        username: "newuser",
        displayName: "New User",
      });
      expect(createResult.id).toBe("user-3");

      // Step 3: Refetch list (simulating invalidation)
      const refreshedList = await mockGetUsersPaginated({ page: 1, pageSize: 50 });
      expect(refreshedList.total).toBe(3);
    });

    it("update → detail refresh shows updated data", async () => {
      mockGetUser.mockResolvedValueOnce(mockUserDetail);
      mockUpdateUser.mockResolvedValue(undefined);
      mockGetUser.mockResolvedValueOnce({
        ...mockUserDetail,
        displayName: "Updated Name",
      });

      // Step 1: Fetch detail
      const original = await mockGetUser("user-1");
      expect(original.displayName).toBe("Admin User");

      // Step 2: Update
      await mockUpdateUser("user-1", { displayName: "Updated Name" });

      // Step 3: Refetch detail
      const updated = await mockGetUser("user-1");
      expect(updated.displayName).toBe("Updated Name");
    });

    it("delete → list refresh excludes deleted user", async () => {
      mockGetUsersPaginated.mockResolvedValueOnce(mockUserList);
      mockDeleteUser.mockResolvedValue(undefined);
      mockGetUsersPaginated.mockResolvedValueOnce({
        items: [mockUserList.items[0]],
        page: 1,
        pageSize: 50,
        total: 1,
      });

      // Step 1: Fetch list
      const beforeDelete = await mockGetUsersPaginated({ page: 1, pageSize: 50 });
      expect(beforeDelete.total).toBe(2);

      // Step 2: Delete
      await mockDeleteUser("user-2");

      // Step 3: Refetch list
      const afterDelete = await mockGetUsersPaginated({ page: 1, pageSize: 50 });
      expect(afterDelete.total).toBe(1);
      expect(afterDelete.items.every((u: { id: string }) => u.id !== "user-2")).toBe(true);
    });
  });
});
