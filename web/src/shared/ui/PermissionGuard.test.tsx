import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";

import { PermissionGuard } from "./PermissionGuard";
import { createTestWrapper } from "../../test-utils/render";

// Mock react-i18next
vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

// Mock usePermissions
const mockHasAction = vi.fn<(...args: unknown[]) => unknown>();
const mockHasResource = vi.fn<(...args: unknown[]) => unknown>();
let mockIsLoading = false;
vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    permissions: { menus: [], actions: [], resources: [] },
    isLoading: mockIsLoading,
    hasMenu: vi.fn<(...args: unknown[]) => unknown>(),
    hasAction: mockHasAction,
    hasResource: mockHasResource,
  }),
}));

const createWrapper = createTestWrapper;

describe("PermissionGuard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsLoading = false;
    mockHasAction.mockReturnValue(false);
    mockHasResource.mockReturnValue(false);
  });

  describe("renders children when permission check passes", () => {
    it("renders children when action permission is granted", () => {
      mockHasAction.mockReturnValue(true);
      const wrapper = createWrapper();
      render(
        <PermissionGuard action="user:create">
          <div data-testid="child">Create User</div>
        </PermissionGuard>,
        { wrapper },
      );
      expect(screen.getByTestId("child")).toBeInTheDocument();
    });

    // 回归：/organizations/:organizationName/domains 曾因权限集缺 b2b 而整页 403。
    it("renders the domains page content when b2b:read is granted", () => {
      mockHasAction.mockImplementation((action) => action === "b2b:read");
      const wrapper = createWrapper();
      render(
        <PermissionGuard action="b2b:read">
          <div data-testid="child">Org Domains</div>
        </PermissionGuard>,
        { wrapper },
      );
      expect(screen.getByTestId("child")).toBeInTheDocument();
      expect(screen.queryByText("403")).not.toBeInTheDocument();
    });

    it("renders children when resource permission is granted", () => {
      mockHasResource.mockReturnValue(true);
      const wrapper = createWrapper();
      render(
        <PermissionGuard resource="users">
          <div data-testid="child">User Panel</div>
        </PermissionGuard>,
        { wrapper },
      );
      expect(screen.getByTestId("child")).toBeInTheDocument();
    });

    it("renders children when no action or resource is specified", () => {
      const wrapper = createWrapper();
      render(
        <PermissionGuard>
          <div data-testid="child">Always Visible</div>
        </PermissionGuard>,
        { wrapper },
      );
      expect(screen.getByTestId("child")).toBeInTheDocument();
    });

    it("renders children when both action and resource are granted", () => {
      mockHasAction.mockReturnValue(true);
      mockHasResource.mockReturnValue(true);
      const wrapper = createWrapper();
      render(
        <PermissionGuard action="user:create" resource="users">
          <div data-testid="child">Create User</div>
        </PermissionGuard>,
        { wrapper },
      );
      expect(screen.getByTestId("child")).toBeInTheDocument();
    });
  });

  describe("renders fallback when permission check fails", () => {
    it("renders default 403 result when action permission is denied", () => {
      mockHasAction.mockReturnValue(false);
      const wrapper = createWrapper();
      render(
        <PermissionGuard action="user:delete">
          <div data-testid="child">Delete User</div>
        </PermissionGuard>,
        { wrapper },
      );
      expect(screen.queryByTestId("child")).not.toBeInTheDocument();
      expect(screen.getByRole("alert")).toBeInTheDocument();
      expect(screen.getByText("403")).toBeInTheDocument();
      expect(screen.getByText("common:forbidden")).toBeInTheDocument();
    });

    it("renders default 403 result when resource permission is denied", () => {
      mockHasResource.mockReturnValue(false);
      const wrapper = createWrapper();
      render(
        <PermissionGuard resource="admin-panel">
          <div data-testid="child">Admin Panel</div>
        </PermissionGuard>,
        { wrapper },
      );
      expect(screen.queryByTestId("child")).not.toBeInTheDocument();
      expect(screen.getByText("403")).toBeInTheDocument();
    });

    it("renders custom fallback when action permission is denied", () => {
      mockHasAction.mockReturnValue(false);
      const wrapper = createWrapper();
      render(
        <PermissionGuard
          action="user:delete"
          fallback={<div data-testid="fallback">No Access</div>}
        >
          <div data-testid="child">Delete User</div>
        </PermissionGuard>,
        { wrapper },
      );
      expect(screen.queryByTestId("child")).not.toBeInTheDocument();
      expect(screen.getByTestId("fallback")).toBeInTheDocument();
      expect(screen.getByText("No Access")).toBeInTheDocument();
    });

    it("renders custom fallback when resource permission is denied", () => {
      mockHasResource.mockReturnValue(false);
      const wrapper = createWrapper();
      render(
        <PermissionGuard
          resource="billing"
          fallback={<div data-testid="fallback">Access Denied</div>}
        >
          <div data-testid="child">Billing</div>
        </PermissionGuard>,
        { wrapper },
      );
      expect(screen.queryByTestId("child")).not.toBeInTheDocument();
      expect(screen.getByTestId("fallback")).toBeInTheDocument();
    });

    it("renders 403 when action is granted but resource is denied", () => {
      mockHasAction.mockReturnValue(true);
      mockHasResource.mockReturnValue(false);
      const wrapper = createWrapper();
      render(
        <PermissionGuard action="user:create" resource="admin-panel">
          <div data-testid="child">Content</div>
        </PermissionGuard>,
        { wrapper },
      );
      expect(screen.queryByTestId("child")).not.toBeInTheDocument();
      expect(screen.getByText("403")).toBeInTheDocument();
    });
  });

  describe("permission loading state", () => {
    it("renders Skeleton instead of children or 403 while permissions are loading", () => {
      mockIsLoading = true;
      mockHasAction.mockReturnValue(true);
      const wrapper = createWrapper();
      const { container } = render(
        <PermissionGuard action="user:create">
          <div data-testid="child">Content</div>
        </PermissionGuard>,
        { wrapper },
      );

      expect(container.querySelector(".ant-skeleton")).toBeInTheDocument();
      expect(screen.queryByTestId("child")).not.toBeInTheDocument();
      expect(screen.queryByText("403")).not.toBeInTheDocument();
    });
  });

  describe("edge cases", () => {
    it("handles empty action string as no restriction", () => {
      // Empty string is falsy, so no permission check is performed
      const wrapper = createWrapper();
      render(
        <PermissionGuard action="">
          <div data-testid="child">Content</div>
        </PermissionGuard>,
        { wrapper },
      );
      expect(screen.getByTestId("child")).toBeInTheDocument();
    });

    it("handles empty resource string as no restriction", () => {
      const wrapper = createWrapper();
      render(
        <PermissionGuard resource="">
          <div data-testid="child">Content</div>
        </PermissionGuard>,
        { wrapper },
      );
      expect(screen.getByTestId("child")).toBeInTheDocument();
    });

    it("renders null fallback correctly", () => {
      mockHasAction.mockReturnValue(false);
      const wrapper = createWrapper();
      render(
        <PermissionGuard action="user:delete" fallback={null}>
          <div data-testid="child">Delete User</div>
        </PermissionGuard>,
        { wrapper },
      );
      expect(screen.queryByTestId("child")).not.toBeInTheDocument();
    });
  });
});
