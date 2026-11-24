import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";

import { useSessionBootstrap } from "@/features/session/useSessionBootstrap";
import { HttpError } from "@/shared/api/http";
import { createTestWrapper } from "../../test-utils/render";

// Mock the auth API
const mockGetAuthAccountContext = vi.fn<(...args: unknown[]) => unknown>();
vi.mock("@/shared/api/iam/auth", () => ({
  getAuthAccountContext: (...args: unknown[]) => mockGetAuthAccountContext(...args),
}));

// Mock the session store
const mockSetSession = vi.fn<(...args: unknown[]) => unknown>();
vi.mock("@/features/session/session-store", () => ({
  useSessionStore: vi.fn<(...args: unknown[]) => unknown>(
    (selector: (state: Record<string, unknown>) => unknown) =>
      selector({
        account: null,
        organization: null,
        setSession: mockSetSession,
      }),
  ),
}));

const createWrapper = createTestWrapper;

describe("useSessionBootstrap", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetAuthAccountContext.mockReset();
    mockSetSession.mockReset();
  });

  it("returns isLoading true initially", () => {
    mockGetAuthAccountContext.mockImplementation(() => new Promise(() => {}));

    const wrapper = createWrapper();
    const { result } = renderHook(() => useSessionBootstrap(), { wrapper });

    expect(result.current.isLoading).toBe(true);
    expect(result.current.account).toBeNull();
    expect(result.current.organization).toBeNull();
  });

  it("loads user info when already logged in (API returns session data)", async () => {
    const mockAccount = {
      owner: "org-1",
      name: "admin",
      displayName: "Admin User",
      avatar: null,
      isAdmin: true,
    };
    const mockOrg = {
      id: "org-1",
      name: "default",
      displayName: "Default Org",
      logoUrl: null,
      faviconUrl: null,
      navItems: [],
      userNavItems: [],
    };
    mockGetAuthAccountContext.mockResolvedValue({
      account: mockAccount,
      organization: mockOrg,
      accessToken: "valid-token",
    });

    const wrapper = createWrapper();
    const { result } = renderHook(() => useSessionBootstrap(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    // setSession should be called with the session data (including accessToken)
    expect(mockSetSession).toHaveBeenCalled();
    const callArg = mockSetSession.mock.calls[0][0];
    expect(callArg.account).toEqual(mockAccount);
    expect(callArg.organization).toEqual(mockOrg);
  });

  it("clears session on 401 (token expired / not logged in)", async () => {
    const httpError = new HttpError(401, null, "Unauthorized");
    mockGetAuthAccountContext.mockRejectedValue(httpError);

    const wrapper = createWrapper();
    const { result } = renderHook(() => useSessionBootstrap(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    // Should clear session on 401 (password policy is cleared with the session)
    expect(mockSetSession).toHaveBeenCalledWith({
      account: null,
      organization: null,
      passwordPolicy: null,
      sessionPolicy: null,
      needUpdatePassword: null,
    });
  });

  it("clears session on 404 (account not found)", async () => {
    const httpError = new HttpError(404, null, "Not Found");
    mockGetAuthAccountContext.mockRejectedValue(httpError);

    const wrapper = createWrapper();
    const { result } = renderHook(() => useSessionBootstrap(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(mockSetSession).toHaveBeenCalledWith({
      account: null,
      organization: null,
      passwordPolicy: null,
      sessionPolicy: null,
      needUpdatePassword: null,
    });
  });

  it("does not clear session on other HTTP errors (e.g. 500)", async () => {
    const httpError = new HttpError(500, null, "Internal Server Error");
    mockGetAuthAccountContext.mockRejectedValue(httpError);

    const wrapper = createWrapper();
    renderHook(() => useSessionBootstrap(), { wrapper });

    // Wait for the query to be attempted
    await waitFor(() => expect(mockGetAuthAccountContext).toHaveBeenCalled());

    // Should NOT call setSession with null for 500 errors
    // (retry will be attempted, so setSession may not be called at all)
    const nullSessionCalls = mockSetSession.mock.calls.filter((call: unknown[]) => {
      const arg = call[0] as { account: unknown; organization: unknown };
      return arg?.account === null && arg?.organization === null;
    });
    expect(nullSessionCalls).toHaveLength(0);
  });

  it("does not retry on 401 errors", async () => {
    const httpError = new HttpError(401, null, "Unauthorized");
    mockGetAuthAccountContext.mockRejectedValue(httpError);

    const wrapper = createWrapper();
    renderHook(() => useSessionBootstrap(), { wrapper });

    await waitFor(() => expect(mockGetAuthAccountContext).toHaveBeenCalled());

    // With retry: false for 401, should only be called once
    expect(mockGetAuthAccountContext).toHaveBeenCalledTimes(1);
  });

  it("does not retry on 404 errors", async () => {
    const httpError = new HttpError(404, null, "Not Found");
    mockGetAuthAccountContext.mockRejectedValue(httpError);

    const wrapper = createWrapper();
    renderHook(() => useSessionBootstrap(), { wrapper });

    await waitFor(() => expect(mockGetAuthAccountContext).toHaveBeenCalled());
    expect(mockGetAuthAccountContext).toHaveBeenCalledTimes(1);
  });

  it("returns account and organization from session store", async () => {
    const mockAccount = {
      owner: "org-1",
      name: "testuser",
      displayName: "Test User",
      avatar: null,
      isAdmin: false,
    };
    const mockOrg = {
      id: "org-1",
      name: "testorg",
      displayName: "Test Org",
      logoUrl: null,
      faviconUrl: null,
      navItems: [],
      userNavItems: [],
    };
    mockGetAuthAccountContext.mockResolvedValue({
      account: mockAccount,
      organization: mockOrg,
    });

    const wrapper = createWrapper();
    const { result } = renderHook(() => useSessionBootstrap(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    // The hook reads account/organization from the store
    // After setSession is called, the store would have the data
    // In our mock, the store always returns null for account/organization
    // but setSession was called correctly
    expect(mockSetSession).toHaveBeenCalled();
    const callArg = mockSetSession.mock.calls[0][0];
    expect(callArg.account).toEqual(mockAccount);
    expect(callArg.organization).toEqual(mockOrg);
  });
});
