import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";

import { useSessionStore } from "@/features/session/session-store";
import { createTestWrapper } from "../../test-utils/render";
import SessionsPage from "./SessionsPage";

const mockHasAction = vi.fn<(action: string) => boolean>();
const mockHasResource = vi.fn<(resource: string) => boolean>();
const mockGetSessionsPaginated = vi.fn<(...args: unknown[]) => unknown>();
const mockGetCurrentSession = vi.fn<(...args: unknown[]) => unknown>();
const mockRevokeOtherSessions = vi.fn<(...args: unknown[]) => unknown>();
const mockRevokeSession = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    isLoading: false,
    hasAction: mockHasAction,
    hasResource: mockHasResource,
  }),
}));

vi.mock("@/shared/api/iam/session", () => ({
  getSessionsPaginated: (...args: unknown[]) => mockGetSessionsPaginated(...args),
  getCurrentSession: (...args: unknown[]) => mockGetCurrentSession(...args),
  revokeOtherSessions: (...args: unknown[]) => mockRevokeOtherSessions(...args),
  revokeSession: (...args: unknown[]) => mockRevokeSession(...args),
}));

vi.mock("@/shared/hooks/useResourceNames", () => ({
  useOrganizationNames: () => ({ nameMap: new Map() }),
}));

vi.mock("@/features/navigation/AuditSectionNav", () => ({
  AuditSectionNav: () => <div data-testid="audit-section-nav" />,
}));

vi.mock("react-router-dom", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router-dom")>();
  return {
    ...actual,
    Link: ({ children }: { children?: React.ReactNode }) => children ?? null,
  };
});

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

const otherSession = {
  id: "session-2",
  userId: "user-2",
  username: "alice",
  organizationId: "org-1",
  organizationSlug: "built-in",
  applicationId: "app-1",
  applicationSlug: "console",
  sessionState: "active",
  ipAddress: "192.168.1.1",
  userAgent: "Mozilla/5.0",
  expiresAt: "2026-12-01T00:00:00Z",
  revokedAt: null,
  createdAt: "2026-11-01T00:00:00Z",
  lastSeenAt: "2026-11-02T00:00:00Z",
  updatedAt: "2026-11-02T00:00:00Z",
};

const currentSession = {
  ...otherSession,
  id: "session-1",
  userId: "user-1",
  username: "bob",
  applicationSlug: "dashboard",
  deviceInfo: {},
  revision: 1,
};

describe("SessionsPage permission guards", () => {
  beforeEach(() => {
    vi.clearAllMocks();
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
    mockGetSessionsPaginated.mockResolvedValue({
      items: [otherSession],
      total: 1,
      page: 1,
      pageSize: 20,
    });
    mockGetCurrentSession.mockResolvedValue(currentSession);
  });

  it("keeps the session write actions out of the DOM without session:write", async () => {
    mockHasAction.mockReturnValue(false);
    mockHasResource.mockReturnValue(false);

    render(<SessionsPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText("alice")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "logoutOtherDevices" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "revoke" })).not.toBeInTheDocument();
  });

  it("renders the session write actions with session:write on sessions", async () => {
    mockHasAction.mockImplementation((action) => action === "session:write");
    mockHasResource.mockImplementation((resource) => resource === "sessions");

    render(<SessionsPage />, { wrapper: createTestWrapper() });

    expect(await screen.findByText("alice")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "logoutOtherDevices" }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole("button", { name: "revoke" }).length).toBeGreaterThan(0);
  });
});
