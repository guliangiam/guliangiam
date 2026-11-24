import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";

import { usePermissions } from "./usePermissions";
import { iamGet } from "@/shared/api/iam/client";
import { useSessionStore } from "@/features/session/session-store";
import { createTestWrapper } from "../../test-utils/render";

// Mock the API client
vi.mock("@/shared/api/iam/client", () => ({
  iamGet: vi.fn<(...args: unknown[]) => unknown>(),
}));

type UserPermissionSet = {
  menus: string[];
  actions: string[];
  resources: string[];
};

const mockIamGet = vi.mocked(iamGet);

const createWrapper = createTestWrapper;

describe("usePermissions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useSessionStore.setState({ account: null });
  });

  it("returns empty permissions and isLoading true initially", () => {
    mockIamGet.mockImplementation(() => new Promise(() => {})); // never resolves

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    expect(result.current.isLoading).toBe(true);
    expect(result.current.permissions.menus).toEqual([]);
    expect(result.current.permissions.actions).toEqual([]);
    expect(result.current.permissions.resources).toEqual([]);
  });

  it("returns permissions when API resolves successfully", async () => {
    const mockPerms: UserPermissionSet = {
      menus: ["/dashboard", "/users"],
      actions: ["user:create", "user:delete"],
      resources: ["users", "organizations"],
    };
    mockIamGet.mockResolvedValue(mockPerms);

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.permissions.menus).toEqual(["/dashboard", "/users"]);
    expect(result.current.permissions.actions).toEqual(["user:create", "user:delete"]);
    expect(result.current.permissions.resources).toEqual(["users", "organizations"]);
  });

  it("hasMenu returns true when menu path exists in permissions", async () => {
    mockIamGet.mockResolvedValue({
      menus: ["/dashboard", "/users", "/settings"],
      actions: [],
      resources: [],
    });

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.hasMenu("/users")).toBe(true);
  });

  it("hasMenu returns false when menu path does not exist", async () => {
    mockIamGet.mockResolvedValue({
      menus: ["/dashboard", "/users"],
      actions: [],
      resources: [],
    });

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.hasMenu("/admin")).toBe(false);
  });

  it("hasAction returns true when action exists in permissions", async () => {
    mockIamGet.mockResolvedValue({
      menus: [],
      actions: ["user:create", "user:delete"],
      resources: [],
    });

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.hasAction("user:create")).toBe(true);
  });

  // Transitional alias: canonical route key is `users:write`, but
  // /auth/permissions still emits the legacy singular `user:write`.
  it("hasAction accepts the legacy singular alias for canonical users:* keys", async () => {
    mockIamGet.mockResolvedValue({
      menus: [],
      actions: ["user:write"],
      resources: ["users"],
    });

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.hasAction("users:write")).toBe(true);
    expect(result.current.hasAction("users:read")).toBe(false);
  });

  it("canonical plural users:* actions hit in-tree singular guards", async () => {
    mockIamGet.mockResolvedValue({
      menus: [],
      actions: ["users:read", "users:write"],
      resources: ["users"],
    });

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.hasAction("user:read")).toBe(true);
    expect(result.current.hasAction("user:write")).toBe(true);
  });

  // organizations 迁移过渡别名的双向证明：后端/路由已改复数，树外守卫若仍
  // 写单数 `org:write` 也不能静默隐藏按钮。PermissionGuard 读取的是 hasAction。
  it("organizations:write grant hits both plural and legacy singular guards", async () => {
    mockIamGet.mockResolvedValue({
      menus: [],
      actions: ["organizations:read", "organizations:write"],
      resources: ["organizations"],
    });

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.hasAction("organizations:write")).toBe(true);
    expect(result.current.hasAction("org:write")).toBe(true);
    expect(result.current.hasAction("organizations:read")).toBe(true);
    expect(result.current.hasAction("org:read")).toBe(true);
  });

  it("legacy org:write grant also hits the plural guard during transition", async () => {
    mockIamGet.mockResolvedValue({
      menus: [],
      actions: ["org:read", "org:write"],
      resources: ["organizations"],
    });

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.hasAction("organizations:write")).toBe(true);
    expect(result.current.hasAction("org:write")).toBe(true);
    // 没有 org:delete → organizations:delete 别名组：授予仅 write 的角色不应
    // 命中 delete；组织删除走独立权限点 organizations:delete（见 usePermissions.ts）。
    expect(result.current.hasAction("org:delete")).toBe(false);
    expect(result.current.hasAction("organizations:delete")).toBe(false);
  });

  it("hasAction returns false when action does not exist", async () => {
    mockIamGet.mockResolvedValue({
      menus: [],
      actions: ["user:create"],
      resources: [],
    });

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.hasAction("user:delete")).toBe(false);
  });

  it("hasResource returns true when resource exists in permissions", async () => {
    mockIamGet.mockResolvedValue({
      menus: [],
      actions: [],
      resources: ["users", "organizations"],
    });

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.hasResource("users")).toBe(true);
  });

  it("hasResource returns false when resource does not exist", async () => {
    mockIamGet.mockResolvedValue({
      menus: [],
      actions: [],
      resources: ["users"],
    });

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.hasResource("billing")).toBe(false);
  });

  it("returns empty permission set when API returns null/undefined data", async () => {
    mockIamGet.mockResolvedValue(null);

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.hasMenu("/any")).toBe(false);
    expect(result.current.hasAction("any")).toBe(false);
    expect(result.current.hasResource("any")).toBe(false);
  });

  it("uses staleTime of 5 minutes for caching", async () => {
    mockIamGet.mockResolvedValue({
      menus: ["/dashboard"],
      actions: [],
      resources: [],
    });

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    // After first load, the query should be cached (staleTime: 5min)
    // Calling again should not trigger a new request within staleTime
    expect(mockIamGet).toHaveBeenCalledTimes(1);
  });

  it("handles full admin permissions correctly", async () => {
    mockIamGet.mockResolvedValue({
      menus: ["/dashboard", "/users", "/organizations", "/applications", "/settings"],
      actions: ["user:create", "user:read", "user:update", "user:delete", "org:manage"],
      resources: ["users", "organizations", "applications", "tokens"],
    });

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.hasMenu("/users")).toBe(true);
    expect(result.current.hasAction("user:delete")).toBe(true);
    expect(result.current.hasResource("tokens")).toBe(true);
  });

  // 回归：管理员兜底集曾漏掉 b2b，导致 /organizations/:organizationName/domains
  // 在权限接口失败时整页 403。
  it("admin fallback set covers the b2b domains capability", async () => {
    useSessionStore.setState({
      account: {
        id: "user-1",
        owner: "org-1",
        name: "admin",
        displayName: "Admin User",
        avatar: null,
        isAdmin: true,
      },
    });
    mockIamGet.mockRejectedValue(new Error("permissions unavailable"));

    const wrapper = createWrapper();
    const { result } = renderHook(() => usePermissions(), { wrapper });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.hasAction("b2b:read")).toBe(true);
    expect(result.current.hasAction("b2b:write")).toBe(true);
    expect(result.current.hasResource("domains")).toBe(true);
  });
});
