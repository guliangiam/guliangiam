import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import SettingsPage from "./SettingsPage";
import { useSessionStore } from "@/features/session/session-store";
import { createTestWrapper } from "../../test-utils/render";

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

const mockGetOrganization = vi.fn<(...args: unknown[]) => unknown>();
const mockUpdateOrganizationSettings = vi.fn<(...args: unknown[]) => unknown>();
const mockUpdateOrganizationTheme = vi.fn<(...args: unknown[]) => unknown>();
const mockUpdateOrganizationSecurityPolicy = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/iam", () => ({
  getOrganization: (...args: unknown[]) => mockGetOrganization(...args),
  updateOrganizationSettings: (...args: unknown[]) => mockUpdateOrganizationSettings(...args),
  updateOrganizationTheme: (...args: unknown[]) => mockUpdateOrganizationTheme(...args),
  updateOrganizationSecurityPolicy: (...args: unknown[]) =>
    mockUpdateOrganizationSecurityPolicy(...args),
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
  useLocation: () => ({ pathname: "/settings", search: "", hash: "", state: null }),
  useParams: () => ({}),
  Link: ({ children }: { children: React.ReactNode }) => children,
  Navigate: () => null,
}));

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

const createWrapper = createTestWrapper;

const mockOrgData = {
  id: "org-1",
  name: "default",
  displayName: "Default Org",
  settings: {
    languages: ["en", "zh"],
    defaultApplicationId: "app-1",
    countryCodes: ["+86", "+1"],
    userTypes: ["normal", "admin"],
    tags: ["tag1"],
    securityPolicySummary: {
      passwordPolicy: {
        minLength: 8,
        requireUppercase: true,
        requireLowercase: true,
        requireDigit: true,
        requireSpecial: false,
      },
      sessionPolicy: {
        idleTimeoutMinutes: 480,
      },
    },
  },
  theme: {
    themeName: "default",
    customCss: "",
  },
  revision: 1,
};

function resetSessionStore() {
  useSessionStore.setState({
    account: null,
    organization: null,
    impersonatedUser: null,
  });
}

function setSessionWithOrg() {
  useSessionStore.setState({
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

      navItems: [],
      userNavItems: [],
    },
    impersonatedUser: null,
  });
}

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

describe("SettingsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetSessionStore();
  });

  describe("no organization state", () => {
    it("renders no org message when orgId is not available", () => {
      render(<SettingsPage />, { wrapper: createWrapper() });
      expect(screen.getByText("settings.noOrg")).toBeInTheDocument();
    });
  });

  describe("renders settings page with org", () => {
    it("renders settings title", async () => {
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.title")).toBeInTheDocument();
      });
    });

    it("renders settings description", async () => {
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.description")).toBeInTheDocument();
      });
    });

    it("renders general tab", async () => {
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.generalTab")).toBeInTheDocument();
      });
    });

    it("renders theme tab", async () => {
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.themeTab")).toBeInTheDocument();
      });
    });

    it("renders security tab", async () => {
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.securityTab")).toBeInTheDocument();
      });
    });
  });

  describe("general settings tab", () => {
    it("renders languages field", async () => {
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.languages")).toBeInTheDocument();
      });
    });

    it("renders default application ID field", async () => {
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.defaultApplicationId")).toBeInTheDocument();
      });
    });

    it("renders country codes field", async () => {
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.countryCodes")).toBeInTheDocument();
      });
    });

    it("renders user types field", async () => {
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.userTypes")).toBeInTheDocument();
      });
    });

    it("renders tags field", async () => {
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.tags")).toBeInTheDocument();
      });
    });

    it("renders save button", async () => {
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getAllByText("settings.save").length).toBeGreaterThan(0);
      });
    });
  });

  describe("loading state", () => {
    it("renders skeleton when loading", () => {
      setSessionWithOrg();
      mockGetOrganization.mockReturnValue(new Promise(() => {}));

      render(<SettingsPage />, { wrapper: createWrapper() });

      const skeleton = document.querySelector(".ant-skeleton");
      expect(skeleton).toBeInTheDocument();
    });
  });

  describe("API integration", () => {
    it("calls getOrganization with orgId", async () => {
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(mockGetOrganization).toHaveBeenCalledWith("org-1");
      });
    });
  });

  describe("tab switching", () => {
    it("can switch to theme tab", async () => {
      const user = userEvent.setup();
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.themeTab")).toBeInTheDocument();
      });

      await user.click(screen.getByText("settings.themeTab"));

      await waitFor(() => {
        expect(screen.getByText("settings.themeName")).toBeInTheDocument();
      });
    });

    it("can switch to security tab", async () => {
      const user = userEvent.setup();
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.securityTab")).toBeInTheDocument();
      });

      await user.click(screen.getByText("settings.securityTab"));

      await waitFor(() => {
        expect(screen.getByText("settings.passwordMinLength")).toBeInTheDocument();
      });
    });
  });

  describe("theme settings tab content", () => {
    it("renders theme name field", async () => {
      const user = userEvent.setup();
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.themeTab")).toBeInTheDocument();
      });

      await user.click(screen.getByText("settings.themeTab"));

      await waitFor(() => {
        expect(screen.getByText("settings.themeName")).toBeInTheDocument();
      });
    });

    it("renders custom CSS field", async () => {
      const user = userEvent.setup();
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.themeTab")).toBeInTheDocument();
      });

      await user.click(screen.getByText("settings.themeTab"));

      await waitFor(() => {
        expect(screen.getByText("settings.customCss")).toBeInTheDocument();
      });
    });
  });

  describe("security policy tab content", () => {
    it("renders password min length field", async () => {
      const user = userEvent.setup();
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.securityTab")).toBeInTheDocument();
      });

      await user.click(screen.getByText("settings.securityTab"));

      await waitFor(() => {
        expect(screen.getByText("settings.passwordMinLength")).toBeInTheDocument();
      });
    });

    it("renders password require uppercase field", async () => {
      const user = userEvent.setup();
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.securityTab")).toBeInTheDocument();
      });

      await user.click(screen.getByText("settings.securityTab"));

      await waitFor(() => {
        expect(screen.getByText("settings.passwordRequireUppercase")).toBeInTheDocument();
      });
    });

    it("renders session idle timeout field", async () => {
      const user = userEvent.setup();
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue(mockOrgData);

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.securityTab")).toBeInTheDocument();
      });

      await user.click(screen.getByText("settings.securityTab"));

      await waitFor(() => {
        expect(screen.getByText("settings.sessionIdleTimeout")).toBeInTheDocument();
      });
    });
  });

  describe("edge cases", () => {
    it("handles organization with empty settings", async () => {
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue({
        id: "org-1",
        name: "default",
        displayName: "Default Org",
        settings: {},
        theme: {},
        revision: 1,
      });

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.title")).toBeInTheDocument();
      });
    });

    it("handles organization with null theme", async () => {
      setSessionWithOrg();
      mockGetOrganization.mockResolvedValue({
        id: "org-1",
        name: "default",
        displayName: "Default Org",
        settings: {},
        theme: null,
        revision: 1,
      });

      render(<SettingsPage />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByText("settings.title")).toBeInTheDocument();
      });
    });
  });
});
