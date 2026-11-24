import { describe, it, expect, vi, beforeEach, afterAll } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";

import { useAccountPage } from "./useAccountPage";

import { createTestWrapper } from "../../test-utils/render";

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

const mockGetAuthAccountContext = vi.fn<(...args: unknown[]) => unknown>();
const mockGetOwnProfile = vi.fn<(...args: unknown[]) => unknown>();
const mockUpdateOwnProfile = vi.fn<(...args: unknown[]) => unknown>();
const mockVerifyOwnIdentification = vi.fn<(...args: unknown[]) => unknown>();
const mockInitiateOwnMfaSetup = vi.fn<(...args: unknown[]) => unknown>();
const mockReplaceOwnMfaMethods = vi.fn<(...args: unknown[]) => unknown>();
const mockUpdateOwnPreferredMfa = vi.fn<(...args: unknown[]) => unknown>();
const mockGetProviders = vi.fn<(...args: unknown[]) => unknown>();
const mockGetSessions = vi.fn<(...args: unknown[]) => unknown>();
const mockGetCurrentSession = vi.fn<(...args: unknown[]) => unknown>();
const mockRevokeSession = vi.fn<(...args: unknown[]) => unknown>();
const mockRevokeOtherSessions = vi.fn<(...args: unknown[]) => unknown>();
const mockRegisterOwnFaceId = vi.fn<(...args: unknown[]) => unknown>();
const mockRevokeOwnFaceId = vi.fn<(...args: unknown[]) => unknown>();
const mockDeleteOwnIdentity = vi.fn<(...args: unknown[]) => unknown>();
const mockSocialAuthorize = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/iam/auth", () => ({
  getAuthAccountContext: (...args: unknown[]) => mockGetAuthAccountContext(...args),
  socialAuthorize: (...args: unknown[]) => mockSocialAuthorize(...args),
}));

const sdk = vi.hoisted(() => {
  class CloudAuthMetaError extends Error {
    reason: string;
    constructor(reason: string) {
      super(`cloudauth: ${reason}`);
      this.name = "CloudAuthMetaError";
      this.reason = reason;
    }
  }
  return {
    collectMetaInfo: vi.fn<() => Promise<string>>(),
    buildIdvReturnUrl: (path: string) => `https://console.test${path}`,
    CloudAuthMetaError,
  };
});

vi.mock("@/shared/lib/cloudauth-sdk", () => sdk);

vi.mock("@/shared/api/iam/auth-selfprofile", () => ({
  getOwnProfile: (...args: unknown[]) => mockGetOwnProfile(...args),
  updateOwnProfile: (...args: unknown[]) => mockUpdateOwnProfile(...args),
  verifyOwnIdentification: (...args: unknown[]) => mockVerifyOwnIdentification(...args),
  registerOwnFaceId: (...args: unknown[]) => mockRegisterOwnFaceId(...args),
  revokeOwnFaceId: (...args: unknown[]) => mockRevokeOwnFaceId(...args),
}));

vi.mock("@/shared/api/iam/auth-selfservice", () => ({
  initiateOwnMfaSetup: (...args: unknown[]) => mockInitiateOwnMfaSetup(...args),
  verifyOwnMfaSetup: vi.fn<(...args: unknown[]) => unknown>(),
  enableOwnMfaSetup: vi.fn<(...args: unknown[]) => unknown>(),
  replaceOwnMfaMethods: (...args: unknown[]) => mockReplaceOwnMfaMethods(...args),
  updateOwnPreferredMfa: (...args: unknown[]) => mockUpdateOwnPreferredMfa(...args),
  deleteOwnIdentity: (...args: unknown[]) => mockDeleteOwnIdentity(...args),
  listOwnConsents: vi.fn<(...args: unknown[]) => Promise<unknown>>().mockResolvedValue([]),
}));

vi.mock("@/shared/api/iam", () => ({
  getProviders: (...args: unknown[]) => mockGetProviders(...args),
  getSessions: (...args: unknown[]) => mockGetSessions(...args),
  getCurrentSession: (...args: unknown[]) => mockGetCurrentSession(...args),
  revokeSession: (...args: unknown[]) => mockRevokeSession(...args),
  revokeOtherSessions: (...args: unknown[]) => mockRevokeOtherSessions(...args),
}));

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

const originalLocation = window.location;

function stubLocation() {
  Object.defineProperty(window, "location", {
    value: { ...originalLocation, href: "" },
    writable: true,
    configurable: true,
  });
}

afterAll(() => {
  Object.defineProperty(window, "location", {
    value: originalLocation,
    writable: true,
    configurable: true,
  });
});

const mockAccount = {
  account: {
    id: "user-1",
    owner: "org-1",
    name: "admin",
    displayName: "Admin User",
    avatar: null,
    isAdmin: true,
  },
  organization: {
    id: "org-1",
    name: "default",
    displayName: "Default Org",
    logoUrl: null,
    faviconUrl: null,
    navItems: [],
    userNavItems: [],
  },
};

const mockUserDetail = {
  id: "user-1",
  organizationId: "org-1",
  username: "admin",
  displayName: "Admin User",
  avatarUrl: "",
  email: "admin@example.com",
  phone: "",
  userType: "normal",
  status: "active",
  isAdmin: true,
  isForbidden: false,
  isDeleted: false,
  isVerified: false,
  tag: "",
  score: 100,
  balance: 0,
  lastSigninAt: null,
  createdAt: "2024-01-01T00:00:00Z",
  externalId: "",
  signupApplicationId: "",
  registerType: "",
  registerSource: "",
  preferredMfaType: "totp",
  recoveryCodes: [],
  mfaSetupSummary: [],
  profile: {},
  credentialSummary: {},
  identities: [],
  mfaMethods: [{ methodType: "totp", enabled: true, verified: true }],
  webauthnCredentialCount: 0,
  roleBindings: [],
  groupMemberships: [],
  consentSummary: {},
  balanceCredit: 0,
  balanceCurrency: "USD",
  ipWhitelist: "",
  addresses: [],
  managedAccounts: [],
  faceIds: [],
  cart: [],
  mfaItems: [],
  mfaAccounts: [],
  applicationScopes: [],
  custom1: "",
  custom2: "",
  custom3: "",
  custom4: "",
  custom5: "",
  custom6: "",
  custom7: "",
  custom8: "",
  custom9: "",
  custom10: "",
  revision: 1,
};

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

describe("useAccountPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("initial state", () => {
    it("returns userId from account data", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.userId).toBe("user-1");
      });
    });

    it("returns empty MFA methods initially", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue({ ...mockUserDetail, mfaMethods: [] });

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.mfaMethods).toEqual([]);
      });
    });
  });

  describe("account query", () => {
    it("fetches account context on mount", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);

      renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(mockGetAuthAccountContext).toHaveBeenCalled();
      });
    });

    it("fetches own profile when userId is available", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);

      renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(mockGetOwnProfile).toHaveBeenCalledWith();
      });
    });

    it("returns account data", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.account).toEqual(mockAccount);
      });
    });
  });

  describe("MFA data", () => {
    it("returns MFA methods from user data", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.mfaMethods).toHaveLength(1);
        expect(result.current.mfaMethods[0].methodType).toBe("totp");
      });
    });

    it("returns preferred MFA from user data", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.preferredMfa).toBe("totp");
      });
    });
  });

  describe("mutations", () => {
    it("provides updateProfileMutation", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.updateProfileMutation).toBeDefined();
        expect(result.current.updateProfileMutation.mutate).toBeDefined();
      });
    });

    it("provides mfaSetupMutation", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.mfaSetupMutation).toBeDefined();
      });
    });

    it("provides mfaDisableMutation", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.mfaDisableMutation).toBeDefined();
      });
    });

    it("provides setPreferredMfaMutation", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.setPreferredMfaMutation).toBeDefined();
      });
    });

    it("provides revokeSessionMutation", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.revokeSessionMutation).toBeDefined();
      });
    });

    it("provides revokeOthersMutation", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.revokeOthersMutation).toBeDefined();
      });
    });

    it("provides verifyIdMutation", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.verifyIdMutation).toBeDefined();
      });
    });
  });

  describe("verifyId mutation", () => {
    it("sends the collected metaInfo and the self-service return path", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);
      sdk.collectMetaInfo.mockResolvedValue(JSON.stringify({ deviceType: "h5" }));
      mockVerifyOwnIdentification.mockResolvedValue({ status: "pending" });

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });
      await waitFor(() => {
        expect(result.current.userId).toBe("user-1");
      });

      await act(async () => {
        result.current.verifyIdMutation.mutate();
      });

      // 后端对 metaInfo / returnUrl 双 fail-closed，缺失即 400：两者必须随请求发出。
      await waitFor(() =>
        expect(mockVerifyOwnIdentification).toHaveBeenCalledWith({
          metaInfo: JSON.stringify({ deviceType: "h5" }),
          returnUrl: "https://console.test/account",
        }),
      );
    });

    it("keeps the request local when metaInfo collection fails", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);
      sdk.collectMetaInfo.mockRejectedValue(new sdk.CloudAuthMetaError("load"));

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });
      await waitFor(() => {
        expect(result.current.userId).toBe("user-1");
      });

      await act(async () => {
        result.current.verifyIdMutation.mutate();
      });

      await waitFor(() => {
        expect(result.current.verifyIdMutation.isError).toBe(true);
      });
      expect(mockVerifyOwnIdentification).not.toHaveBeenCalled();
      // 采集状态必须回落，否则按钮永久停在「采集中」。
      expect(result.current.idvCollecting).toBe(false);
    });

    it("stores the pending verification so the redirect back resumes polling", async () => {
      localStorage.removeItem("idv:pending:user-1");
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);
      sdk.collectMetaInfo.mockResolvedValue("{}");
      mockVerifyOwnIdentification.mockResolvedValue({
        status: "pending",
        data: { verificationId: "v-9", redirectUrl: "https://verify.test/certify/v-9" },
      });

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });
      await waitFor(() => {
        expect(result.current.userId).toBe("user-1");
      });

      await act(async () => {
        result.current.verifyIdMutation.mutate();
      });

      // 自服务是本人操作：跳转厂商页前落盘标记，回跳后才有可恢复的轮询入口。
      await waitFor(() => {
        expect(localStorage.getItem("idv:pending:user-1")).toBe("v-9");
      });
    });
  });

  describe("updateProfile mutation", () => {
    it("calls updateOwnProfile with the form values (partial merge)", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);
      mockUpdateOwnProfile.mockResolvedValue(undefined);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.userId).toBe("user-1");
      });

      await act(async () => {
        result.current.updateProfileMutation.mutate({
          displayName: "New Name",
          bio: "hello",
        });
      });

      await waitFor(() => {
        expect(mockUpdateOwnProfile).toHaveBeenCalledWith({
          displayName: "New Name",
          bio: "hello",
        });
      });
    });

    it("throws error when userId is not available", async () => {
      mockGetAuthAccountContext.mockResolvedValue({
        ...mockAccount,
        account: { ...mockAccount.account, id: undefined },
      });
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.accountQuery.isLoading).toBe(false);
      });

      // userId should be undefined, mutation should throw
      await act(async () => {
        result.current.updateProfileMutation.mutate({ displayName: "Test" });
      });

      // The mutation should have an error since userId is not available
      await waitFor(() => {
        expect(result.current.updateProfileMutation.isError).toBe(true);
      });
    });
  });

  describe("MFA setup mutation", () => {
    it("calls initiateOwnMfaSetup with mfaType (session-bound)", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);
      mockInitiateOwnMfaSetup.mockResolvedValue({ secret: "abc", url: "otpauth://..." });

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.userId).toBe("user-1");
      });

      await act(async () => {
        result.current.mfaSetupMutation.mutate("totp");
      });

      await waitFor(() => {
        expect(mockInitiateOwnMfaSetup).toHaveBeenCalledWith({ mfaType: "totp" });
      });
    });
  });

  describe("MFA disable mutation", () => {
    it("sends the user revision and drops the client-supplied verified flag", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue({
        ...mockUserDetail,
        revision: 3,
        mfaMethods: [
          { methodType: "totp", enabled: true, verified: true },
          { methodType: "sms", enabled: true, verified: true, displayName: "SMS" },
        ],
      });
      mockReplaceOwnMfaMethods.mockResolvedValue(undefined);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.userId).toBe("user-1");
      });
      await waitFor(() => {
        expect(result.current.mfaMethods).toHaveLength(2);
      });

      await act(async () => {
        result.current.mfaDisableMutation.mutate("totp");
      });

      await waitFor(() => {
        expect(mockReplaceOwnMfaMethods).toHaveBeenCalledTimes(1);
      });
      expect(mockReplaceOwnMfaMethods.mock.calls[0]).toEqual([
        3,
        [{ methodType: "sms", enabled: true, displayName: "SMS" }],
      ]);
    });
  });

  describe("session revoke mutation", () => {
    it("calls revokeSession with sessionId", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);
      mockRevokeSession.mockResolvedValue(undefined);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.userId).toBe("user-1");
      });

      await act(async () => {
        result.current.revokeSessionMutation.mutate("session-123");
      });

      await waitFor(() => {
        expect(mockRevokeSession).toHaveBeenCalledWith("session-123");
      });
    });
  });

  describe("revoke other sessions mutation", () => {
    it("calls revokeOtherSessions", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);
      mockRevokeOtherSessions.mockResolvedValue({ status: "revoked", revokedCount: 3 });

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.userId).toBe("user-1");
      });

      await act(async () => {
        result.current.revokeOthersMutation.mutate();
      });

      await waitFor(() => {
        expect(mockRevokeOtherSessions).toHaveBeenCalled();
      });
    });
  });

  describe("faceId mutations", () => {
    it("registerFaceIdMutation calls registerOwnFaceId with file, providerId and deviceInfo", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);
      mockRegisterOwnFaceId.mockResolvedValue({
        id: "face-1",
        imageUrl: "",
        createdAt: "2024-01-01T00:00:00Z",
      });

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.userId).toBe("user-1");
      });

      const file = new File(["bytes"], "face.png", { type: "image/png" });
      await act(async () => {
        result.current.registerFaceIdMutation.mutate({ file, providerId: "face-provider-1" });
      });

      await waitFor(() => {
        expect(mockRegisterOwnFaceId).toHaveBeenCalledWith({
          file,
          providerId: "face-provider-1",
          deviceInfo: navigator.userAgent,
        });
      });
    });

    it("registerFaceIdMutation omits providerId when not selected", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);
      mockRegisterOwnFaceId.mockResolvedValue({
        id: "face-1",
        imageUrl: "",
        createdAt: "2024-01-01T00:00:00Z",
      });

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.userId).toBe("user-1");
      });

      const file = new File(["bytes"], "face.png", { type: "image/png" });
      await act(async () => {
        result.current.registerFaceIdMutation.mutate({ file });
      });

      await waitFor(() => {
        expect(mockRegisterOwnFaceId).toHaveBeenCalledWith({
          file,
          providerId: undefined,
          deviceInfo: navigator.userAgent,
        });
      });
    });

    it("revokeFaceIdMutation calls revokeOwnFaceId without arguments", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);
      mockRevokeOwnFaceId.mockResolvedValue(undefined);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.userId).toBe("user-1");
      });

      await act(async () => {
        result.current.revokeFaceIdMutation.mutate();
      });

      await waitFor(() => {
        expect(mockRevokeOwnFaceId).toHaveBeenCalledWith();
      });
    });
  });

  describe("identity unlink mutation", () => {
    it("deleteIdentityMutation calls deleteOwnIdentity with the identity id", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);
      mockDeleteOwnIdentity.mockResolvedValue(undefined);

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.userId).toBe("user-1");
      });

      await act(async () => {
        result.current.deleteIdentityMutation.mutate("identity-1");
      });

      await waitFor(() => {
        expect(mockDeleteOwnIdentity).toHaveBeenCalledWith("identity-1");
      });
    });
  });

  describe("linkIdentity", () => {
    it("calls socialAuthorize with the organization slug and link state, then redirects", async () => {
      stubLocation();
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);
      mockSocialAuthorize.mockResolvedValue({ redirectUrl: "https://provider.example/authorize" });

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.userId).toBe("user-1");
      });

      await act(async () => {
        await result.current.linkIdentity("provider-google");
      });

      expect(mockSocialAuthorize).toHaveBeenCalledWith({
        providerId: "provider-google",
        organizationSlug: "default",
        state: "link:user-1:provider-google",
      });
      expect(window.location.href).toBe("https://provider.example/authorize");
    });

    it("surfaces an error message when socialAuthorize fails", async () => {
      stubLocation();
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetOwnProfile.mockResolvedValue(mockUserDetail);
      mockSocialAuthorize.mockRejectedValue(new Error("boom"));

      const { result } = renderHook(() => useAccountPage(), { wrapper: createWrapper() });

      await waitFor(() => {
        expect(result.current.userId).toBe("user-1");
      });

      await act(async () => {
        await result.current.linkIdentity("provider-google");
      });

      expect(window.location.href).toBe("");
    });
  });
});
