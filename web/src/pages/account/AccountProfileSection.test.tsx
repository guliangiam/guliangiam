import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Form } from "antd";

import { AccountProfileSection } from "./AccountProfileSection";
import type { UserDetail, UpdateUserProfilePayload } from "@/shared/api/iam/user-types";
import type { AuthAccountContext } from "@/shared/api/iam/auth";
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

const mockAccount: AuthAccountContext = {
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

const mockUser: UserDetail = {
  id: "user-1",
  organizationId: "org-1",
  username: "admin",
  displayName: "Admin User",
  avatarUrl: "",
  email: "admin@example.com",
  phone: "13800138000",
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
  profile: {
    displayName: "Admin User",
    email: "admin@example.com",
    phone: "+8613800138000",
    countryCode: "CN",
    region: "CN",
    location: "Beijing",
    gender: "male",
    birthday: "1990-01-01",
    bio: "Hello world",
    language: "en",
  },
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

function createMockMutation(overrides?: Record<string, unknown>) {
  return {
    mutate: vi.fn<(...args: unknown[]) => unknown>(),
    mutateAsync: vi.fn<(...args: unknown[]) => Promise<unknown>>(async () => undefined),
    isPending: false,
    isSuccess: false,
    isError: false,
    isIdle: true,
    data: undefined,
    error: null,
    reset: vi.fn<(...args: unknown[]) => unknown>(),
    ...overrides,
  } as unknown as import("@tanstack/react-query").UseMutationResult<
    void,
    Error,
    UpdateUserProfilePayload
  >;
}

// ─────────────────────────────────────────────
// Test Component with Form
// ─────────────────────────────────────────────

function TestWrapper({
  account,
  hasAccount = true,
  user = mockUser,
  mutationOverrides,
}: {
  account?: AuthAccountContext | undefined;
  hasAccount?: boolean;
  user?: UserDetail | undefined;
  mutationOverrides?: Record<string, unknown>;
}) {
  const [form] = Form.useForm();
  const mutation = createMockMutation(mutationOverrides);

  return (
    <AccountProfileSection
      account={hasAccount ? (account ?? mockAccount) : undefined}
      user={user}
      profileForm={form}
      updateProfileMutation={mutation}
    />
  );
}

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

describe("AccountProfileSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("renders form fields", () => {
    it("renders username display", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("admin")).toBeInTheDocument();
    });

    it("renders organization display", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("Default Org")).toBeInTheDocument();
    });

    it("renders displayName field label", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("account.displayName")).toBeInTheDocument();
    });

    it("renders email field label", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("account.email")).toBeInTheDocument();
    });

    it("renders phone field label", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("account.phone")).toBeInTheDocument();
    });

    it("renders countryCode field label", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("auth:countryCode")).toBeInTheDocument();
    });

    it("renders region field label", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("region")).toBeInTheDocument();
    });

    it("renders location field label", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("account.location")).toBeInTheDocument();
    });

    it("renders gender field label", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("user.gender")).toBeInTheDocument();
    });

    it("renders birthday field label", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("user.birthday")).toBeInTheDocument();
    });

    it("renders bio field label", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("user.bio")).toBeInTheDocument();
    });

    it("renders language field label", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("language")).toBeInTheDocument();
    });

    it("renders save button", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("account.saveChanges")).toBeInTheDocument();
    });

    it("renders N/A when account is undefined", () => {
      render(<TestWrapper hasAccount={false} />, { wrapper: createWrapper() });
      const naElements = screen.getAllByText("N/A");
      expect(naElements.length).toBeGreaterThanOrEqual(1);
    });

    it("renders N/A when account name is missing", () => {
      // `name` is genuinely absent (undefined), not an empty string: the
      // component uses `??`, so only null/undefined fall back to "N/A".
      const noNameAccount = {
        ...mockAccount,
        account: { ...mockAccount.account, name: undefined },
      } as typeof mockAccount;
      render(<TestWrapper account={noNameAccount} />, { wrapper: createWrapper() });
      expect(screen.getAllByText("N/A").length).toBeGreaterThanOrEqual(1);
    });

    it("renders organization owner when org displayName is null", () => {
      const nullOrgName: AuthAccountContext = {
        ...mockAccount,
        organization: { ...mockAccount.organization, displayName: null },
      };
      render(<TestWrapper account={nullOrgName} />, { wrapper: createWrapper() });
      expect(screen.getByText("org-1")).toBeInTheDocument();
    });
  });

  describe("form field population from user data", () => {
    it("populates displayName from user profile", async () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      await waitFor(() => {
        const displayNameInput = screen.getByRole("textbox", { name: /account\.displayName/i });
        expect(displayNameInput).toHaveValue("Admin User");
      });
    });

    it("populates email from user profile", async () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      await waitFor(() => {
        const emailInput = screen.getByRole("textbox", { name: /account\.email/i });
        expect(emailInput).toHaveValue("admin@example.com");
      });
    });

    it("populates phone from user profile", async () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      await waitFor(() => {
        const phoneInput = screen.getByRole("textbox", { name: /account\.phone/i });
        expect(phoneInput).toHaveValue("13800138000");
      });
    });

    it("populates countryCode as an ISO country selector", async () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      await waitFor(() => {
        expect(screen.getByText("People's Republic of China (+86)")).toBeInTheDocument();
      });
      expect(screen.getByRole("combobox", { name: /countryCode/i })).toBeInTheDocument();
    });

    it("populates region from user profile", async () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      await waitFor(() => {
        const regionInput = screen.getByRole("textbox", { name: /region/i });
        expect(regionInput).toHaveValue("CN");
      });
    });

    it("populates location from user profile", async () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      await waitFor(() => {
        const locationInput = screen.getByRole("textbox", { name: /location/i });
        expect(locationInput).toHaveValue("Beijing");
      });
    });

    it("populates bio from user profile", async () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      await waitFor(() => {
        const bioInput = screen.getByRole("textbox", { name: /bio/i });
        expect(bioInput).toHaveValue("Hello world");
      });
    });

    it("populates birthday from user profile", async () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      await waitFor(() => {
        const birthdayInput = screen.getByRole("textbox", { name: /birthday/i });
        expect(birthdayInput).toHaveValue("1990-01-01");
      });
    });
  });

  describe("form validation", () => {
    it("displayName is required", async () => {
      const user = userEvent.setup();
      render(<TestWrapper />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByRole("textbox", { name: /account\.displayName/i })).toBeInTheDocument();
      });

      const displayNameInput = screen.getByRole("textbox", { name: /account\.displayName/i });
      await user.clear(displayNameInput);
      await user.tab();

      await waitFor(() => {
        expect(screen.getByText("account.displayNameRequired")).toBeInTheDocument();
      });
    });
  });

  describe("user interaction", () => {
    it("allows typing in displayName field", async () => {
      const user = userEvent.setup();
      render(<TestWrapper />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByRole("textbox", { name: /account\.displayName/i })).toBeInTheDocument();
      });

      const displayNameInput = screen.getByRole("textbox", { name: /account\.displayName/i });
      await user.clear(displayNameInput);
      await user.type(displayNameInput, "New Name");

      expect(displayNameInput).toHaveValue("New Name");
    });

    it("allows typing in email field", async () => {
      const user = userEvent.setup();
      render(<TestWrapper />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByRole("textbox", { name: /account\.email/i })).toBeInTheDocument();
      });

      const emailInput = screen.getByRole("textbox", { name: /account\.email/i });
      await user.clear(emailInput);
      await user.type(emailInput, "new@example.com");

      expect(emailInput).toHaveValue("new@example.com");
    });

    it("allows typing in phone field", async () => {
      const user = userEvent.setup();
      render(<TestWrapper />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByRole("textbox", { name: /account\.phone/i })).toBeInTheDocument();
      });

      const phoneInput = screen.getByRole("textbox", { name: /account\.phone/i });
      await user.clear(phoneInput);
      await user.type(phoneInput, "13900139000");

      expect(phoneInput).toHaveValue("13900139000");
    });

    it("allows typing in region field", async () => {
      const user = userEvent.setup();
      render(<TestWrapper />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByRole("textbox", { name: /region/i })).toBeInTheDocument();
      });

      const regionInput = screen.getByRole("textbox", { name: /region/i });
      await user.clear(regionInput);
      await user.type(regionInput, "US");

      expect(regionInput).toHaveValue("US");
    });

    it("allows typing in location field", async () => {
      const user = userEvent.setup();
      render(<TestWrapper />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByRole("textbox", { name: /location/i })).toBeInTheDocument();
      });

      const locationInput = screen.getByRole("textbox", { name: /location/i });
      await user.clear(locationInput);
      await user.type(locationInput, "Shanghai");

      expect(locationInput).toHaveValue("Shanghai");
    });

    it("allows typing in bio textarea", async () => {
      const user = userEvent.setup();
      render(<TestWrapper />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByRole("textbox", { name: /bio/i })).toBeInTheDocument();
      });

      const bioInput = screen.getByRole("textbox", { name: /bio/i });
      await user.clear(bioInput);
      await user.type(bioInput, "Updated bio text");

      expect(bioInput).toHaveValue("Updated bio text");
    });
  });

  describe("mutation state", () => {
    it("shows loading state on save button when mutation is pending", () => {
      render(<TestWrapper mutationOverrides={{ isPending: true }} />, { wrapper: createWrapper() });
      const saveButton = screen.getByRole("button", { name: /account\.saveChanges/i });
      // antd 6: `loading` adds the `ant-btn-loading` class and blocks clicks, but
      // does NOT emit the `disabled` attribute.
      expect(saveButton).toHaveClass("ant-btn-loading");
    });

    it("save button is enabled when mutation is not pending", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      const saveButton = screen.getByRole("button", { name: /account\.saveChanges/i });
      expect(saveButton).not.toBeDisabled();
    });
  });

  describe("phone normalization", () => {
    it("submits E.164 phone with ISO country code", async () => {
      const user = userEvent.setup();
      const mutateAsync = vi.fn<(...args: unknown[]) => Promise<unknown>>(async () => undefined);
      render(<TestWrapper mutationOverrides={{ mutateAsync }} />, { wrapper: createWrapper() });

      await waitFor(() => {
        expect(screen.getByRole("textbox", { name: /account\.phone/i })).toHaveValue("13800138000");
      });
      await user.click(screen.getByRole("button", { name: /account\.saveChanges/i }));

      await waitFor(() => {
        expect(mutateAsync).toHaveBeenCalled();
      });
      const payload = mutateAsync.mock.calls.at(-1)?.[0] as UpdateUserProfilePayload;
      expect(payload.phone).toBe("+8613800138000");
      expect(payload.countryCode).toBe("CN");
    });
  });

  describe("edge cases", () => {
    it("renders when user is undefined", () => {
      render(<TestWrapper user={undefined} />, { wrapper: createWrapper() });
      expect(screen.getByText("admin")).toBeInTheDocument();
    });

    it("renders when user profile is empty", () => {
      const userWithEmptyProfile: UserDetail = {
        ...mockUser,
        profile: {},
      };
      render(<TestWrapper user={userWithEmptyProfile} />, { wrapper: createWrapper() });
      expect(screen.getByText("admin")).toBeInTheDocument();
    });

    it("renders when user profile has null values", () => {
      const userWithNulls: UserDetail = {
        ...mockUser,
        profile: {
          displayName: null,
          email: null,
          phone: null,
        },
      };
      render(<TestWrapper user={userWithNulls} />, { wrapper: createWrapper() });
      expect(screen.getByText("admin")).toBeInTheDocument();
    });

    it("renders the derived calling code next to the local phone number", async () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      await waitFor(() => {
        expect(screen.getByRole("textbox", { name: /account\.phone/i })).toHaveValue("13800138000");
      });
      expect(screen.getAllByText("+86").length).toBeGreaterThan(0);
      expect(screen.queryByPlaceholderText("+86")).not.toBeInTheDocument();
    });

    it("renders birthday placeholder YYYY-MM-DD", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByPlaceholderText("YYYY-MM-DD")).toBeInTheDocument();
    });
  });
});
