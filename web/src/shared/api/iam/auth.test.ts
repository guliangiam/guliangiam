import { describe, it, expect, vi, beforeEach } from "vitest";
import { expectStatus } from "@/test-utils/assertions";
import {
  getAuthAccountContext,
  updateOwnAccount,
  loginWithPassword,
  logoutSession,
  socialAuthorize,
  getCaptchaStatus,
  verifyCode,
  ssoLogout,
} from "./auth";

// Mock the HTTP module
const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();
vi.mock("@/shared/api/http", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/http")>();
  return {
    ...actual,
    requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
  };
});

describe("Auth API", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // ── getAuthAccountContext ──

  describe("getAuthAccountContext", () => {
    it("calls correct endpoint and returns mapped context", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        account: {
          id: "user-1",
          owner: "org-1",
          name: "admin",
          displayName: "Admin User",
          avatar: "https://avatar.example.com/admin.png",
          isAdmin: true,
        },
        organization: {
          id: "org-1",
          name: "default",
          displayName: "Default Org",
          navItems: ["/dashboard"],
          userNavItems: [],
        },
        accessToken: "valid-token",
      });

      const result = await getAuthAccountContext();
      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/auth/account");
      expect(result.account.name).toBe("admin");
      expect(result.account.isAdmin).toBe(true);
      expect(result.organization.id).toBe("org-1");
      expect(result.accessToken).toBe("valid-token");
    });

    it("PATCHes /api/v1/auth/account for self-service profile and contact binding", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        account: {
          id: "user-1",
          owner: "org-1",
          name: "guest-1",
          displayName: "Guest",
          avatar: "",
          isAdmin: false,
        },
        organization: {
          id: "org-1",
          name: "default",
          displayName: "Default",
          navItems: [],
          userNavItems: [],
        },
        accessToken: "token",
      });

      await updateOwnAccount({ phone: "13800000000", verificationCode: "123456" });

      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/auth/account", {
        method: "PATCH",
        body: JSON.stringify({ phone: "13800000000", verificationCode: "123456" }),
      });
    });

    it("handles null displayName and avatar", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        account: {
          owner: "org-1",
          name: "testuser",
          isAdmin: false,
        },
        organization: {
          id: "org-1",
          name: "default",
          navItems: [],
          userNavItems: [],
        },
      });

      const result = await getAuthAccountContext();
      expect(result.account.displayName).toBeNull();
      expect(result.account.avatar).toBeNull();
    });
  });

  // ── loginWithPassword ──

  describe("loginWithPassword", () => {
    it("returns success status on successful login", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        status: "success",
        account: {
          owner: "org-1",
          name: "admin",
          displayName: "Admin",
          isAdmin: true,
        },
        organization: {
          id: "org-1",
          name: "default",
          navItems: [],
          userNavItems: [],
        },
      });

      const result = await loginWithPassword({
        organizationSlug: "default",
        username: "admin",
        password: "password123",
        applicationId: "app-1",
      });

      expectStatus(result, "success");
      expect(result.account.name).toBe("admin");
    });

    it("returns mfa_required status when MFA is needed", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        status: "mfa_required",
        challengeId: "challenge-1",
        requiredMfa: ["totp"],
        availableMfaMethods: [
          { id: "mfa-1", methodType: "totp", displayName: "Authenticator App" },
        ],
      });

      const result = await loginWithPassword({
        organizationSlug: "default",
        username: "admin",
        password: "password123",
        applicationId: "app-1",
      });

      expectStatus(result, "mfa_required");
      expect(result.challengeId).toBe("challenge-1");
      expect(result.requiredMfa).toEqual(["totp"]);
      expect(result.availableMfaMethods).toHaveLength(1);
    });

    it("returns required_mfa_setup status when MFA setup is needed", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        status: "required_mfa_setup",
        challengeId: "challenge-setup-1",
        requiredMfa: ["totp"],
      });

      const result = await loginWithPassword({
        organizationSlug: "default",
        username: "admin",
        password: "password123",
        applicationId: "app-1",
      });

      expectStatus(result, "required_mfa_setup");
      expect(result.challengeId).toBe("challenge-setup-1");
      expect(result.requiredMfa).toEqual(["totp"]);
    });

    it("sends POST request with JSON body", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        status: "success",
        account: { owner: "org-1", name: "admin", isAdmin: true },
        organization: { id: "org-1", name: "default", navItems: [], userNavItems: [] },
      });

      const payload = {
        organizationSlug: "default",
        username: "admin",
        password: "password123",
        applicationId: "app-1",
      };

      await loginWithPassword(payload);

      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify(payload),
        headers: { "Content-Type": "application/json" },
      });
    });
  });

  // ── logoutSession ──

  describe("logoutSession", () => {
    it("calls correct endpoint and returns logged_out status", async () => {
      mockRequestDocumentJson.mockResolvedValue({ status: "logged_out" });

      const result = await logoutSession();
      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/auth/logout", {
        method: "POST",
      });
      expect(result.status).toBe("logged_out");
    });
  });

  // ── socialAuthorize ──

  describe("socialAuthorize", () => {
    it("calls correct endpoint and returns redirect URL", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        redirectUrl: "https://github.com/login/oauth/authorize?client_id=xxx",
      });

      const result = await socialAuthorize({
        providerId: "provider-github",
        applicationId: "app-1",
      });

      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/auth/social/authorize", {
        method: "POST",
        body: JSON.stringify({ providerId: "provider-github", applicationId: "app-1" }),
        headers: { "Content-Type": "application/json" },
      });
      expect(result.redirectUrl).toContain("github.com");
    });
  });

  // ── getCaptchaStatus ──

  describe("getCaptchaStatus", () => {
    it("calls correct endpoint with organizationId", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        required: true,
        providerType: "reCAPTCHA",
        siteKey: "6LeIxAcTAAAAAJ",
      });

      const result = await getCaptchaStatus("org-1");
      expect(mockRequestDocumentJson).toHaveBeenCalled();
      const calledUrl = mockRequestDocumentJson.mock.calls[0][0] as string;
      expect(calledUrl).toContain("/api/v1/auth/captcha-status");
      expect(calledUrl).toContain("organizationId=org-1");
      expect(result.required).toBe(true);
    });
  });

  // ── verifyCode ──

  describe("verifyCode", () => {
    it("sends POST request with correct payload", async () => {
      mockRequestDocumentJson.mockResolvedValue(undefined);

      await verifyCode({ dest: "user@example.com", code: "123456", type: "email" });

      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/auth/verify-code", {
        method: "POST",
        body: JSON.stringify({ dest: "user@example.com", code: "123456", type: "email" }),
      });
    });
  });

  // ── ssoLogout ──

  describe("ssoLogout", () => {
    it("calls correct endpoint and returns logout URLs", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        logoutUrls: ["https://idp.example.com/slo", "https://okta.example.com/logout"],
      });

      const result = await ssoLogout();
      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/auth/sso-logout", {
        method: "POST",
      });
      expect(result.logoutUrls).toHaveLength(2);
    });
  });
});
