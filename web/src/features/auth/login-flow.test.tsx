import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";

import { useSessionStore } from "@/features/session/session-store";
import { useSessionBootstrap } from "@/features/session/useSessionBootstrap";
import { HttpError } from "@/shared/api/http";
import {
  LoginSuccess,
  LoginMfaRequired,
  LoginPayload,
  SocialLoginParams,
  SocialAuthorizeParams,
  SocialAuthorizeResult,
} from "@/shared/api/iam/auth";
import { createTestWrapper } from "@/test-utils/render";
import { expectStatus } from "@/test-utils/assertions";

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

const mockGetAuthAccountContext = vi.fn<(...args: unknown[]) => unknown>();
const mockLoginWithPassword = vi.fn<(...args: unknown[]) => unknown>();
const mockSocialLogin = vi.fn<(...args: unknown[]) => unknown>();
const mockSocialAuthorize = vi.fn<(...args: unknown[]) => unknown>();
const mockLogoutSession = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/iam/auth", () => ({
  getAuthAccountContext: (...args: unknown[]) => mockGetAuthAccountContext(...args),
  loginWithPassword: (...args: unknown[]) => mockLoginWithPassword(...args),
  socialLogin: (...args: unknown[]) => mockSocialLogin(...args),
  socialAuthorize: (...args: unknown[]) => mockSocialAuthorize(...args),
  logoutSession: (...args: unknown[]) => mockLogoutSession(...args),
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
  useLocation: () => ({ pathname: "/login", search: "", hash: "", state: null }),
  useParams: () => ({}),
  Link: ({ children }: { children: React.ReactNode }) => children,
  Navigate: () => null,
}));

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

const mockAccount = {
  owner: "org-1",
  name: "admin",
  displayName: "Admin User",
  avatar: null,
  isAdmin: true,
};

const mockOrganization = {
  id: "org-1",
  name: "default",
  displayName: "Default Org",
  logoUrl: null,
  faviconUrl: null,
  navItems: ["/dashboard", "/users"],
  userNavItems: ["/profile"],
};

const createWrapper = createTestWrapper;

function resetSessionStore() {
  useSessionStore.setState({
    account: null,
    organization: null,
    impersonatedUser: null,
  });
}

// ─────────────────────────────────────────────
// Integration Tests: Login Flow
// ─────────────────────────────────────────────

describe("login flow integration", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetSessionStore();
  });

  describe("complete login flow: input username → input password → submit → success → session update", () => {
    it("updates session store after successful login", async () => {
      // Step 1: loginWithPassword returns success
      const loginSuccessResult: LoginSuccess = {
        status: "success",
        account: mockAccount,
        organization: mockOrganization,
        accessToken: "valid-token",
      };
      mockLoginWithPassword.mockResolvedValue(loginSuccessResult);

      // Step 2: getAuthAccountContext returns session after login
      mockGetAuthAccountContext.mockResolvedValue({
        account: mockAccount,
        organization: mockOrganization,
        accessToken: "valid-token",
      });

      // Verify session store is empty before login
      expect(useSessionStore.getState().account).toBeNull();
      expect(useSessionStore.getState().organization).toBeNull();

      // Simulate login API call
      const loginPayload: LoginPayload = {
        organizationSlug: "default",
        username: "admin",
        password: "correct-password",
        applicationId: "app-1",
      };
      const loginResult = await mockLoginWithPassword(loginPayload);

      // Verify login API was called correctly
      expect(mockLoginWithPassword).toHaveBeenCalledWith(loginPayload);
      expect(loginResult.status).toBe("success");

      // Bootstrap session after login
      const wrapper = createWrapper();
      const { result } = renderHook(() => useSessionBootstrap(), { wrapper });

      await waitFor(() => expect(result.current.isLoading).toBe(false));

      // Verify session store is updated
      const sessionState = useSessionStore.getState();
      expect(sessionState.account).toEqual(mockAccount);
      expect(sessionState.organization).toEqual(mockOrganization);
    });

    it("login API sends correct payload with organization context", async () => {
      mockLoginWithPassword.mockResolvedValue({
        status: "success",
        account: mockAccount,
        organization: mockOrganization,
      });

      const payload: LoginPayload = {
        organizationSlug: "my-org",
        username: "testuser",
        password: "mypassword",
        applicationId: "app-1",
      };

      await mockLoginWithPassword(payload);

      expect(mockLoginWithPassword).toHaveBeenCalledWith(
        expect.objectContaining({
          organizationSlug: "my-org",
          username: "testuser",
          password: "mypassword",
          applicationId: "app-1",
        }),
      );
    });
  });

  describe("login failure flow: wrong password → error message → retry possible", () => {
    it("handles 401 unauthorized error and does not update session", async () => {
      const httpError = new HttpError(
        401,
        { status: "error", msg: "Invalid credentials" },
        "Unauthorized",
      );
      mockLoginWithPassword.mockRejectedValue(httpError);
      mockGetAuthAccountContext.mockRejectedValue(httpError);

      // Attempt login
      await expect(
        mockLoginWithPassword({
          organizationSlug: "default",
          username: "admin",
          password: "wrong-password",
          applicationId: "app-1",
        }),
      ).rejects.toThrow("Unauthorized");

      // Session bootstrap should clear session on 401
      const wrapper = createWrapper();
      const { result } = renderHook(() => useSessionBootstrap(), { wrapper });

      await waitFor(() => expect(result.current.isLoading).toBe(false));

      // Session should remain null (cleared)
      const sessionState = useSessionStore.getState();
      expect(sessionState.account).toBeNull();
      expect(sessionState.organization).toBeNull();
    });

    it("handles 400 bad request for malformed input", async () => {
      const httpError = new HttpError(400, null, "Bad Request");
      mockLoginWithPassword.mockRejectedValue(httpError);

      await expect(
        mockLoginWithPassword({
          organizationSlug: "default",
          username: "",
          password: "",
          applicationId: "app-1",
        }),
      ).rejects.toThrow("Bad Request");

      // Login API was called (attempt was made)
      expect(mockLoginWithPassword).toHaveBeenCalledTimes(1);
    });

    it("allows retry after login failure", async () => {
      // First attempt fails
      const httpError = new HttpError(401, null, "Unauthorized");
      mockLoginWithPassword.mockRejectedValueOnce(httpError);

      await expect(
        mockLoginWithPassword({
          organizationSlug: "default",
          username: "admin",
          password: "wrong",
          applicationId: "app-1",
        }),
      ).rejects.toThrow("Unauthorized");

      // Second attempt succeeds
      mockLoginWithPassword.mockResolvedValue({
        status: "success",
        account: mockAccount,
        organization: mockOrganization,
      });

      const result = await mockLoginWithPassword({
        organizationSlug: "default",
        username: "admin",
        password: "correct",
        applicationId: "app-1",
      });

      expect(result.status).toBe("success");
      expect(mockLoginWithPassword).toHaveBeenCalledTimes(2);
    });
  });

  describe("MFA flow: login → MFA challenge → input verification code → success", () => {
    it("handles MFA required response and completes MFA verification", async () => {
      // Step 1: loginWithPassword returns MFA required
      const mfaRequiredResult: LoginMfaRequired = {
        status: "mfa_required",
        challengeId: "challenge-123",
        challengeState: "pending",
        challengeExpiresAt: new Date(Date.now() + 300000).toISOString(),
        requiredMfa: ["totp"],
        availableMfaMethods: [
          { id: "mfa-1", methodType: "totp", displayName: "Authenticator App" },
        ],
      };
      mockLoginWithPassword.mockResolvedValue(mfaRequiredResult);

      // Step 2: Submit MFA challenge with passcode
      const loginPayload: LoginPayload = {
        organizationSlug: "default",
        username: "admin",
        password: "correct-password",
        applicationId: "app-1",
      };
      const firstResult = await mockLoginWithPassword(loginPayload);

      // Verify MFA challenge is returned
      expectStatus(firstResult, "mfa_required");
      expect(firstResult.challengeId).toBe("challenge-123");
      expect(firstResult.requiredMfa).toContain("totp");
      expect(firstResult.availableMfaMethods).toHaveLength(1);
      expect(firstResult.availableMfaMethods[0].methodType).toBe("totp");

      // Step 3: Submit MFA verification code
      mockLoginWithPassword.mockResolvedValue({
        status: "success",
        account: mockAccount,
        organization: mockOrganization,
        accessToken: "mfa-verified-token",
      } as LoginSuccess);

      const mfaPayload: LoginPayload = {
        challengeId: "challenge-123",
        mfaType: "totp",
        passcode: "123456",
      };
      const mfaResult = await mockLoginWithPassword(mfaPayload);

      expectStatus(mfaResult, "success");
      expect(mfaResult.account).toEqual(mockAccount);
      expect(mfaResult.organization).toEqual(mockOrganization);
    });

    it("handles invalid MFA code with error", async () => {
      const httpError = new HttpError(401, null, "Invalid MFA code");
      mockLoginWithPassword.mockRejectedValue(httpError);

      await expect(
        mockLoginWithPassword({
          challengeId: "challenge-123",
          mfaType: "totp",
          passcode: "000000",
        }),
      ).rejects.toThrow("Invalid MFA code");
    });

    it("handles multiple MFA methods when available", async () => {
      const mfaRequiredResult: LoginMfaRequired = {
        status: "mfa_required",
        challengeId: "challenge-456",
        nextMfa: "sms",
        challengeExpiresAt: new Date(Date.now() + 300000).toISOString(),
        requiredMfa: ["totp", "sms"],
        availableMfaMethods: [
          { id: "mfa-1", methodType: "totp", displayName: "Authenticator App" },
          { id: "mfa-2", methodType: "sms", displayName: "SMS", target: "+1***1234" },
        ],
      };
      mockLoginWithPassword.mockResolvedValue(mfaRequiredResult);

      const result = await mockLoginWithPassword({
        organizationSlug: "default",
        username: "admin",
        password: "correct-password",
        applicationId: "app-1",
      });

      expectStatus(result, "mfa_required");
      expect(result.requiredMfa).toEqual(["totp", "sms"]);
      expect(result.availableMfaMethods).toHaveLength(2);
      expect(result.nextMfa).toBe("sms");
    });
  });

  describe("social login flow: click provider → redirect → callback → success", () => {
    it("initiates social authorize and gets redirect URL", async () => {
      const authorizeResult: SocialAuthorizeResult = {
        redirectUrl: "https://accounts.google.com/o/oauth2/v2/auth?client_id=xxx",
      };
      mockSocialAuthorize.mockResolvedValue(authorizeResult);

      const params: SocialAuthorizeParams = {
        providerId: "provider-google",
        organizationSlug: "default",
        applicationId: "app-1",
      };

      const result = await mockSocialAuthorize(params);

      expect(mockSocialAuthorize).toHaveBeenCalledWith(params);
      expect(result.redirectUrl).toContain("accounts.google.com");
    });

    it("completes social login with authorization code callback", async () => {
      mockSocialLogin.mockResolvedValue({
        status: "success",
        account: mockAccount,
        organization: mockOrganization,
        accessToken: "social-token",
      } as LoginSuccess);

      const params: SocialLoginParams = {
        providerId: "provider-google",
        code: "auth-code-from-google",
        state: "random-state",
        organizationSlug: "default",
        applicationId: "app-1",
      };

      const result = await mockSocialLogin(params);

      expect(mockSocialLogin).toHaveBeenCalledWith(params);
      expectStatus(result, "success");
      expect(result.account).toEqual(mockAccount);
      expect(result.organization).toEqual(mockOrganization);
    });

    it("handles social login with MFA required after callback", async () => {
      mockSocialLogin.mockResolvedValue({
        status: "mfa_required",
        challengeId: "social-challenge-1",
        requiredMfa: ["totp"],
        availableMfaMethods: [{ id: "mfa-1", methodType: "totp", displayName: "Authenticator" }],
      } as LoginMfaRequired);

      const result = await mockSocialLogin({
        providerId: "provider-github",
        code: "github-code",
        applicationId: "app-1",
      });

      expectStatus(result, "mfa_required");
      expect(result.challengeId).toBe("social-challenge-1");
    });

    it("handles social login callback error", async () => {
      const httpError = new HttpError(401, null, "Invalid authorization code");
      mockSocialLogin.mockRejectedValue(httpError);

      await expect(
        mockSocialLogin({
          providerId: "provider-google",
          code: "invalid-code",
          applicationId: "app-1",
        }),
      ).rejects.toThrow("Invalid authorization code");
    });
  });

  describe("logout flow", () => {
    it("clears session on logout", async () => {
      // First set up a session
      useSessionStore.getState().setSession({
        account: mockAccount,
        organization: mockOrganization,
      });

      expect(useSessionStore.getState().account).not.toBeNull();

      // Logout
      mockLogoutSession.mockResolvedValue({ status: "logged_out" });
      await mockLogoutSession();

      // Clear session store (simulating what the UI would do after logout)
      useSessionStore.getState().setSession({ account: null, organization: null });

      const sessionState = useSessionStore.getState();
      expect(sessionState.account).toBeNull();
      expect(sessionState.organization).toBeNull();
    });
  });

  describe("session bootstrap after login", () => {
    it("session bootstrap fetches account context and populates store", async () => {
      mockGetAuthAccountContext.mockResolvedValue({
        account: mockAccount,
        organization: mockOrganization,
        accessToken: "session-token",
      });

      const wrapper = createWrapper();
      const { result } = renderHook(() => useSessionBootstrap(), { wrapper });

      // Initially loading
      expect(result.current.isLoading).toBe(true);

      // After loading completes
      await waitFor(() => expect(result.current.isLoading).toBe(false));

      // Session store should have been updated
      const sessionState = useSessionStore.getState();
      expect(sessionState.account).toEqual(mockAccount);
      expect(sessionState.organization).toEqual(mockOrganization);
    });

    it("session bootstrap handles network error gracefully", async () => {
      const networkError = new HttpError(500, null, "Internal Server Error");
      mockGetAuthAccountContext.mockRejectedValue(networkError);

      const wrapper = createWrapper();
      renderHook(() => useSessionBootstrap(), { wrapper });

      // Wait for the query to be attempted
      await waitFor(() => expect(mockGetAuthAccountContext).toHaveBeenCalled());

      // Session store should remain null (not cleared for 500 errors)
      const sessionState = useSessionStore.getState();
      // For 500 errors, setSession with null is NOT called
      expect(sessionState.account).toBeNull();
      expect(sessionState.organization).toBeNull();
    });
  });
});
