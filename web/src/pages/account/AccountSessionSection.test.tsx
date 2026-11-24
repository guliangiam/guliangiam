import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";

import { AccountSessionSection } from "./AccountSessionSection";
import type { SessionItem, SessionDetail } from "@/shared/api/iam/session";
import { createTestWrapper } from "../../test-utils/render";

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

const createWrapper = createTestWrapper;

function createMockQuery<TData>(overrides?: Record<string, unknown>) {
  return {
    data: undefined as TData | undefined,
    isLoading: false,
    isError: false,
    isSuccess: false,
    isPending: false,
    error: null,
    refetch: vi.fn<(...args: unknown[]) => unknown>(),
    ...overrides,
  } as unknown as import("@tanstack/react-query").UseQueryResult<TData, Error>;
}

function createMockMutation<TData, TVariables>(overrides?: Record<string, unknown>) {
  return {
    mutate: vi.fn<(...args: unknown[]) => unknown>(),
    isPending: false,
    isSuccess: false,
    isError: false,
    isIdle: true,
    data: undefined,
    error: null,
    reset: vi.fn<(...args: unknown[]) => unknown>(),
    ...overrides,
  } as unknown as import("@tanstack/react-query").UseMutationResult<TData, Error, TVariables>;
}

const mockSessions: SessionItem[] = [
  {
    id: "session-1",
    userId: "user-1",
    username: "admin",
    organizationId: "org-1",
    organizationSlug: "default",
    applicationId: "app-1",
    applicationSlug: "app",
    sessionState: "active",
    ipAddress: "192.168.1.1",
    userAgent: "Chrome/120",
    expiresAt: "2024-12-31T23:59:59Z",
    revokedAt: null,
    createdAt: "2024-01-01T00:00:00Z",
    lastSeenAt: "2024-06-01T00:00:00Z",
    updatedAt: "2024-06-01T00:00:00Z",
  },
  {
    id: "session-2",
    userId: "user-1",
    username: "admin",
    organizationId: "org-1",
    organizationSlug: "default",
    applicationId: "app-1",
    applicationSlug: "app",
    sessionState: "revoked",
    ipAddress: "10.0.0.1",
    userAgent: "Firefox/115",
    expiresAt: "2024-06-01T23:59:59Z",
    revokedAt: "2024-06-01T00:00:00Z",
    createdAt: "2024-01-01T00:00:00Z",
    lastSeenAt: "2024-05-01T00:00:00Z",
    updatedAt: "2024-06-01T00:00:00Z",
  },
];

const mockCurrentSession: SessionDetail = {
  ...mockSessions[0],
  deviceInfo: { os: "Windows", browser: "Chrome" },
  revision: 1,
};

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

describe("AccountSessionSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("renders session section", () => {
    it("renders revoke other sessions button", () => {
      render(
        <AccountSessionSection
          sessionsQuery={createMockQuery<SessionItem[]>({ data: [] })}
          currentSessionQuery={createMockQuery<SessionDetail>()}
          revokeSessionMutation={createMockMutation<void, string>()}
          revokeOthersMutation={createMockMutation<
            { status: string; revokedCount: number },
            void
          >()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("account.revokeOtherSessions")).toBeInTheDocument();
    });

    it("renders session table columns", () => {
      render(
        <AccountSessionSection
          sessionsQuery={createMockQuery<SessionItem[]>({ data: mockSessions })}
          currentSessionQuery={createMockQuery<SessionDetail>()}
          revokeSessionMutation={createMockMutation<void, string>()}
          revokeOthersMutation={createMockMutation<
            { status: string; revokedCount: number },
            void
          >()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("account.ipAddress")).toBeInTheDocument();
      expect(screen.getByText("account.userAgent")).toBeInTheDocument();
      expect(screen.getByText("account.lastSeen")).toBeInTheDocument();
      expect(screen.getByText("account.sessionState")).toBeInTheDocument();
      expect(screen.getByText("account.actions")).toBeInTheDocument();
    });

    it("renders session data in table", () => {
      render(
        <AccountSessionSection
          sessionsQuery={createMockQuery<SessionItem[]>({ data: mockSessions })}
          currentSessionQuery={createMockQuery<SessionDetail>()}
          revokeSessionMutation={createMockMutation<void, string>()}
          revokeOthersMutation={createMockMutation<
            { status: string; revokedCount: number },
            void
          >()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("192.168.1.1")).toBeInTheDocument();
      expect(screen.getByText("10.0.0.1")).toBeInTheDocument();
    });

    it("renders session state tags", () => {
      render(
        <AccountSessionSection
          sessionsQuery={createMockQuery<SessionItem[]>({ data: mockSessions })}
          currentSessionQuery={createMockQuery<SessionDetail>()}
          revokeSessionMutation={createMockMutation<void, string>()}
          revokeOthersMutation={createMockMutation<
            { status: string; revokedCount: number },
            void
          >()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("active")).toBeInTheDocument();
      expect(screen.getByText("revoked")).toBeInTheDocument();
    });
  });

  describe("current session card", () => {
    it("renders current device card when data is available", () => {
      render(
        <AccountSessionSection
          sessionsQuery={createMockQuery<SessionItem[]>({ data: [] })}
          currentSessionQuery={createMockQuery<SessionDetail>({ data: mockCurrentSession })}
          revokeSessionMutation={createMockMutation<void, string>()}
          revokeOthersMutation={createMockMutation<
            { status: string; revokedCount: number },
            void
          >()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("account.currentDevice")).toBeInTheDocument();
    });

    it("renders current session IP address", () => {
      render(
        <AccountSessionSection
          sessionsQuery={createMockQuery<SessionItem[]>({ data: [] })}
          currentSessionQuery={createMockQuery<SessionDetail>({ data: mockCurrentSession })}
          revokeSessionMutation={createMockMutation<void, string>()}
          revokeOthersMutation={createMockMutation<
            { status: string; revokedCount: number },
            void
          >()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("192.168.1.1")).toBeInTheDocument();
    });

    it("renders current session user agent", () => {
      render(
        <AccountSessionSection
          sessionsQuery={createMockQuery<SessionItem[]>({ data: [] })}
          currentSessionQuery={createMockQuery<SessionDetail>({ data: mockCurrentSession })}
          revokeSessionMutation={createMockMutation<void, string>()}
          revokeOthersMutation={createMockMutation<
            { status: string; revokedCount: number },
            void
          >()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("Chrome/120")).toBeInTheDocument();
    });

    it("does not render current device card when no data", () => {
      render(
        <AccountSessionSection
          sessionsQuery={createMockQuery<SessionItem[]>({ data: [] })}
          currentSessionQuery={createMockQuery<SessionDetail>()}
          revokeSessionMutation={createMockMutation<void, string>()}
          revokeOthersMutation={createMockMutation<
            { status: string; revokedCount: number },
            void
          >()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.queryByText("account.currentDevice")).not.toBeInTheDocument();
    });
  });

  describe("session actions", () => {
    it("renders revoke button for active sessions", () => {
      render(
        <AccountSessionSection
          sessionsQuery={createMockQuery<SessionItem[]>({ data: [mockSessions[0]] })}
          currentSessionQuery={createMockQuery<SessionDetail>()}
          revokeSessionMutation={createMockMutation<void, string>()}
          revokeOthersMutation={createMockMutation<
            { status: string; revokedCount: number },
            void
          >()}
        />,
        { wrapper: createWrapper() },
      );
      const revokeButtons = screen.getAllByText("account.revoke");
      expect(revokeButtons.length).toBeGreaterThanOrEqual(1);
    });

    it("renders revoked tag for revoked sessions", () => {
      render(
        <AccountSessionSection
          sessionsQuery={createMockQuery<SessionItem[]>({ data: [mockSessions[1]] })}
          currentSessionQuery={createMockQuery<SessionDetail>()}
          revokeSessionMutation={createMockMutation<void, string>()}
          revokeOthersMutation={createMockMutation<
            { status: string; revokedCount: number },
            void
          >()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("account.revoked")).toBeInTheDocument();
    });
  });

  describe("loading state", () => {
    it("shows loading state for sessions query", () => {
      render(
        <AccountSessionSection
          sessionsQuery={createMockQuery<SessionItem[]>({ isLoading: true })}
          currentSessionQuery={createMockQuery<SessionDetail>()}
          revokeSessionMutation={createMockMutation<void, string>()}
          revokeOthersMutation={createMockMutation<
            { status: string; revokedCount: number },
            void
          >()}
        />,
        { wrapper: createWrapper() },
      );
      // Table should be in loading state
      expect(screen.getByText("account.revokeOtherSessions")).toBeInTheDocument();
    });

    it("shows loading on revoke others button when mutation is pending", () => {
      render(
        <AccountSessionSection
          sessionsQuery={createMockQuery<SessionItem[]>({ data: [] })}
          currentSessionQuery={createMockQuery<SessionDetail>()}
          revokeSessionMutation={createMockMutation<void, string>()}
          revokeOthersMutation={createMockMutation<{ status: string; revokedCount: number }>({
            isPending: true,
          })}
        />,
        { wrapper: createWrapper() },
      );
      const revokeButton = screen.getByRole("button", { name: /account\.revokeOtherSessions/i });
      // antd 6: `loading` adds the `ant-btn-loading` class and blocks clicks, but
      // does NOT emit the `disabled` attribute.
      expect(revokeButton).toHaveClass("ant-btn-loading");
    });
  });

  describe("edge cases", () => {
    it("handles empty sessions list", () => {
      render(
        <AccountSessionSection
          sessionsQuery={createMockQuery<SessionItem[]>({ data: [] })}
          currentSessionQuery={createMockQuery<SessionDetail>()}
          revokeSessionMutation={createMockMutation<void, string>()}
          revokeOthersMutation={createMockMutation<
            { status: string; revokedCount: number },
            void
          >()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("account.revokeOtherSessions")).toBeInTheDocument();
    });

    it("handles undefined sessions data", () => {
      render(
        <AccountSessionSection
          sessionsQuery={createMockQuery<SessionItem[]>()}
          currentSessionQuery={createMockQuery<SessionDetail>()}
          revokeSessionMutation={createMockMutation<void, string>()}
          revokeOthersMutation={createMockMutation<
            { status: string; revokedCount: number },
            void
          >()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("account.revokeOtherSessions")).toBeInTheDocument();
    });

    it("renders dash for missing IP address", () => {
      const sessionNoIp: SessionDetail = {
        ...mockCurrentSession,
        ipAddress: "",
      };
      render(
        <AccountSessionSection
          sessionsQuery={createMockQuery<SessionItem[]>({ data: [] })}
          currentSessionQuery={createMockQuery<SessionDetail>({ data: sessionNoIp })}
          revokeSessionMutation={createMockMutation<void, string>()}
          revokeOthersMutation={createMockMutation<
            { status: string; revokedCount: number },
            void
          >()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("account.currentDevice")).toBeInTheDocument();
    });
  });
});
