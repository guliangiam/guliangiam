import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

import AccountPage from "./AccountPage";
import { useSessionStore } from "@/features/session/session-store";
import { createTestWrapper } from "../../test-utils/render";

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

const mockGetAuthAccountContext = vi.fn<(...args: unknown[]) => unknown>();
const mockGetUser = vi.fn<(...args: unknown[]) => unknown>();
const mockUpdateUserProfile = vi.fn<(...args: unknown[]) => unknown>();
const mockVerifyIdentification = vi.fn<(...args: unknown[]) => unknown>();
const mockInitiateUserMfaSetup = vi.fn<(...args: unknown[]) => unknown>();
const mockReplaceUserMfaMethods = vi.fn<(...args: unknown[]) => unknown>();
const mockUpdateUserPreferredMfa = vi.fn<(...args: unknown[]) => unknown>();
const mockGetProviders = vi.fn<(...args: unknown[]) => unknown>();
const mockGetSessions = vi.fn<(...args: unknown[]) => unknown>();
const mockGetCurrentSession = vi.fn<(...args: unknown[]) => unknown>();
const mockRevokeSession = vi.fn<(...args: unknown[]) => unknown>();
const mockRevokeOtherSessions = vi.fn<(...args: unknown[]) => unknown>();
const mockSocialAuthorize = vi.fn<(...args: unknown[]) => unknown>();
const mockGetIamFaceIdProviders = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/iam/auth", () => ({
  getAuthAccountContext: (...args: unknown[]) => mockGetAuthAccountContext(...args),
  socialAuthorize: (...args: unknown[]) => mockSocialAuthorize(...args),
}));

vi.mock("@/shared/api/iam/face-id-provider", () => ({
  getIamFaceIdProviders: (...args: unknown[]) => mockGetIamFaceIdProviders(...args),
}));

vi.mock("@/shared/api/iam/user", () => ({
  getUser: (...args: unknown[]) => mockGetUser(...args),
  updateUserProfile: (...args: unknown[]) => mockUpdateUserProfile(...args),
  verifyIdentification: (...args: unknown[]) => mockVerifyIdentification(...args),
}));

vi.mock("@/shared/api/iam/auth-selfprofile", () => ({
  getOwnProfile: (...args: unknown[]) => mockGetUser(...args),
  updateOwnProfile: (...args: unknown[]) => mockUpdateUserProfile(...args),
  verifyOwnIdentification: (...args: unknown[]) => mockVerifyIdentification(...args),
  registerOwnFaceId: vi.fn<(...args: unknown[]) => unknown>(),
  revokeOwnFaceId: vi.fn<(...args: unknown[]) => unknown>(),
}));

vi.mock("@/shared/api/iam/auth-selfservice", () => ({
  initiateOwnMfaSetup: (...args: unknown[]) => mockInitiateUserMfaSetup(...args),
  verifyOwnMfaSetup: vi.fn<(...args: unknown[]) => unknown>(),
  enableOwnMfaSetup: vi.fn<(...args: unknown[]) => unknown>(),
  replaceOwnMfaMethods: (...args: unknown[]) => mockReplaceUserMfaMethods(...args),
  updateOwnPreferredMfa: (...args: unknown[]) => mockUpdateUserPreferredMfa(...args),
  deleteOwnIdentity: vi.fn<(...args: unknown[]) => unknown>(),
  listOwnConsents: vi.fn<(...args: unknown[]) => Promise<unknown>>().mockResolvedValue([]),
}));

vi.mock("@/shared/api/iam", () => ({
  initiateUserMfaSetup: (...args: unknown[]) => mockInitiateUserMfaSetup(...args),
  replaceUserMfaMethods: (...args: unknown[]) => mockReplaceUserMfaMethods(...args),
  updateUserPreferredMfa: (...args: unknown[]) => mockUpdateUserPreferredMfa(...args),
  getProviders: (...args: unknown[]) => mockGetProviders(...args),
  getSessions: (...args: unknown[]) => mockGetSessions(...args),
  getCurrentSession: (...args: unknown[]) => mockGetCurrentSession(...args),
  revokeSession: (...args: unknown[]) => mockRevokeSession(...args),
  revokeOtherSessions: (...args: unknown[]) => mockRevokeOtherSessions(...args),
  getOrganization: vi.fn<(...args: unknown[]) => Promise<unknown>>().mockResolvedValue({}),
  updateOrganizationSettings: vi
    .fn<(...args: unknown[]) => Promise<unknown>>()
    .mockResolvedValue({}),
  updateOrganizationTheme: vi.fn<(...args: unknown[]) => Promise<unknown>>().mockResolvedValue({}),
  updateOrganizationSecurityPolicy: vi
    .fn<(...args: unknown[]) => Promise<unknown>>()
    .mockResolvedValue({}),
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
  useLocation: () => ({ pathname: "/account", search: "", hash: "", state: null }),
  useParams: () => ({}),
  Link: ({ children }: { children: React.ReactNode }) => children,
  Navigate: () => null,
}));

vi.mock("@/shared/ui/PasswordModal", () => ({
  PasswordModal: () => <div data-testid="password-modal">Password Modal</div>,
}));

vi.mock("qrcode.react", () => ({
  QRCodeSVG: () => <div data-testid="qrcode-svg" />,
}));

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

const createWrapper = createTestWrapper;

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
  preferredMfaType: "",
  recoveryCodes: [],
  mfaSetupSummary: [],
  profile: { displayName: "Admin User", email: "admin@example.com" },
  credentialSummary: {},
  identities: [],
  mfaMethods: [],
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

function resetSessionStore() {
  useSessionStore.setState({
    account: null,
    organization: null,
    impersonatedUser: null,
  });
}

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

describe("AccountPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetSessionStore();
    // Every section is mounted at once now (anchor-scroll shell), so the
    // previously lazy-gated queries run on first paint: give each a defined
    // result to avoid "Query data cannot be undefined".
    mockGetProviders.mockResolvedValue([]);
    mockGetSessions.mockResolvedValue([]);
    mockGetCurrentSession.mockResolvedValue(null);
    mockGetIamFaceIdProviders.mockResolvedValue([]);
    mockSocialAuthorize.mockResolvedValue({ redirectUrl: "https://provider.example/authorize" });
  });

  describe("renders account page sections", () => {
    it("renders profile tab", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetUser.mockResolvedValue(mockUserDetail);

      render(<AccountPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("account.profile")).toBeInTheDocument();
      });
    });

    it("renders real name tab", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetUser.mockResolvedValue(mockUserDetail);

      render(<AccountPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("realName")).toBeInTheDocument();
      });
    });

    it("renders 3rd-party logins tab", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetUser.mockResolvedValue(mockUserDetail);

      render(<AccountPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByRole("heading", { name: "user.thirdPartyLogin" })).toBeInTheDocument();
      });
    });

    it("renders security tab", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetUser.mockResolvedValue(mockUserDetail);

      render(<AccountPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("account.security")).toBeInTheDocument();
      });
    });

    it("renders sessions tab", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetUser.mockResolvedValue(mockUserDetail);

      render(<AccountPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        // The nav rail label and the sessions FormSection card title share the
        // same key, so assert on the plural accessor.
        expect(screen.getAllByText("account.sessions").length).toBeGreaterThan(0);
      });
    });
  });

  describe("loading state", () => {
    it("renders loading card when account query is loading", () => {
      mockGetAuthAccountContext.mockReturnValue(new Promise(() => {}));

      render(<AccountPage />, { wrapper: createWrapper() });

      // Should render a loading card
      const loadingCard = document.querySelector(".ant-card-loading");
      expect(loadingCard || document.querySelector(".ant-skeleton")).toBeTruthy();
    });
  });

  describe("profile section content", () => {
    it("renders username in profile section", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetUser.mockResolvedValue(mockUserDetail);

      render(<AccountPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("admin")).toBeInTheDocument();
      });
    });

    it("renders organization in profile section", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetUser.mockResolvedValue(mockUserDetail);

      render(<AccountPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("Default Org")).toBeInTheDocument();
      });
    });

    it("renders save changes button in profile section", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetUser.mockResolvedValue(mockUserDetail);

      render(<AccountPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        // Both profile and real-name sections mount at once and each carries a
        // save button, so assert on the plural accessor.
        expect(screen.getAllByText("account.saveChanges").length).toBeGreaterThan(0);
      });
    });
  });

  describe("security section content", () => {
    // All sections are mounted at once (anchor-scroll shell), so the security
    // pane content is present without activating a tab.
    async function renderOnSecurityTab() {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetUser.mockResolvedValue(mockUserDetail);

      render(<AccountPage />, { wrapper: createWrapper() });
      await screen.findByText("account.passwordSection");
    }

    it("renders password section card", async () => {
      await renderOnSecurityTab();

      await waitFor(() => {
        expect(screen.getByText("account.passwordSection")).toBeInTheDocument();
      });
    });

    it("renders MFA section card", async () => {
      await renderOnSecurityTab();

      await waitFor(() => {
        expect(screen.getByText("account.mfaSection")).toBeInTheDocument();
      });
    });

    it("renders no MFA enabled alert when no methods", async () => {
      await renderOnSecurityTab();

      await waitFor(() => {
        expect(screen.getByText("account.noMfaEnabled")).toBeInTheDocument();
      });
    });

    it("renders Face ID real-name guidance when the user is not verified", async () => {
      await renderOnSecurityTab();

      await waitFor(() => {
        expect(screen.getByText("user.faceIdRealNameRequired")).toBeInTheDocument();
      });
      // 未实名时不暴露注册入口
      expect(screen.queryByText("user.faceIdRegisterNew")).not.toBeInTheDocument();
    });

    it("renders Face ID register entry when the user is verified", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetUser.mockResolvedValue({ ...mockUserDetail, isVerified: true });

      render(<AccountPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getAllByText("user.faceIdRegisterNew").length).toBeGreaterThan(0);
      });
    });
  });

  describe("consents section content", () => {
    it("renders the read-only consents section", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetUser.mockResolvedValue(mockUserDetail);

      render(<AccountPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("user.tabs.consents")).toBeInTheDocument();
      });
      // 自助撤销端点尚不存在：只读段不得出现撤销入口
      expect(screen.queryByText("user.consents.revoke")).not.toBeInTheDocument();
    });
  });

  describe("API integration", () => {
    it("calls getAuthAccountContext on mount", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetUser.mockResolvedValue(mockUserDetail);

      render(<AccountPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(mockGetAuthAccountContext).toHaveBeenCalled();
      });
    });

    it("loads own profile via the session-bound self endpoint", async () => {
      mockGetAuthAccountContext.mockResolvedValue(mockAccount);
      mockGetUser.mockResolvedValue(mockUserDetail);

      render(<AccountPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(mockGetUser).toHaveBeenCalledWith();
      });
    });
  });
});
