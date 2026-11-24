import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Form } from "antd";

import { AccountRealNameSection } from "./AccountRealNameSection";
import type { UserDetail, UpdateUserProfilePayload } from "@/shared/api/iam/user-types";
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

const mockUser: UserDetail = {
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
  profile: {
    realName: "John Doe",
    idCardType: "id_card",
    // 规范 9.5：响应不含证件号原文，只有 Set/Masked 对。
    idCardSet: true,
    idCardMasked: "******",
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

function TestWrapper({
  user,
  hasUser = true,
  profileMutationOverrides,
  verifyIdMutationOverrides,
}: {
  user?: UserDetail | undefined;
  hasUser?: boolean;
  profileMutationOverrides?: Record<string, unknown>;
  verifyIdMutationOverrides?: Record<string, unknown>;
}) {
  const [form] = Form.useForm();
  const profileMutation = createMockMutation<void, UpdateUserProfilePayload>(
    profileMutationOverrides,
  );
  const verifyIdMutation = createMockMutation<void, void>(verifyIdMutationOverrides);

  return (
    <AccountRealNameSection
      user={hasUser ? (user ?? mockUser) : undefined}
      realNameForm={form}
      updateProfileMutation={profileMutation}
      verifyIdMutation={verifyIdMutation}
    />
  );
}

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

describe("AccountRealNameSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("renders form fields", () => {
    it("renders realName field label", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("user.realName")).toBeInTheDocument();
    });

    it("renders idCardType field label", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("user.idCardType")).toBeInTheDocument();
    });

    it("renders idCard field label", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      // `idCard` also appears as the selected Select option label, so scope the
      // query to the Form.Item <label> element.
      expect(screen.getByText("user.idCard", { selector: "label" })).toBeInTheDocument();
    });

    it("renders save button", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("account.saveChanges")).toBeInTheDocument();
    });

    it("renders verify identity button when not verified", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("user.verifyIdentity")).toBeInTheDocument();
    });

    it("does not render verify identity button when verified", () => {
      const verifiedUser = { ...mockUser, isVerified: true };
      render(<TestWrapper user={verifiedUser} />, { wrapper: createWrapper() });
      expect(screen.queryByText("user.verifyIdentity")).not.toBeInTheDocument();
    });

    it("renders verified alert when user is verified", () => {
      const verifiedUser = { ...mockUser, isVerified: true };
      render(<TestWrapper user={verifiedUser} />, { wrapper: createWrapper() });
      expect(screen.getByText("verified")).toBeInTheDocument();
    });

    it("does not render verified alert when user is not verified", () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      expect(screen.queryByText("verified")).not.toBeInTheDocument();
    });
  });

  describe("form field population", () => {
    it("populates realName from user profile", async () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      await waitFor(() => {
        const realNameInput = screen.getByRole("textbox", { name: /realName/i });
        expect(realNameInput).toHaveValue("John Doe");
      });
    });

    it("never prefills idCard plaintext; shows the masked placeholder instead", async () => {
      render(<TestWrapper />, { wrapper: createWrapper() });
      await waitFor(() => {
        const idCardInput = screen.getByRole("textbox", { name: /idCard/i });
        // 证件号原文永不回传（规范 9.5）：输入框保持空，占位符展示掩码。
        expect(idCardInput).toHaveValue("");
        expect(idCardInput).toHaveAttribute("placeholder", "******");
      });
    });
  });

  describe("form interaction", () => {
    it("allows typing in realName field", async () => {
      const user = userEvent.setup();
      render(<TestWrapper />, { wrapper: createWrapper() });
      await waitFor(() => {
        expect(screen.getByRole("textbox", { name: /realName/i })).toBeInTheDocument();
      });
      const realNameInput = screen.getByRole("textbox", { name: /realName/i });
      await user.clear(realNameInput);
      await user.type(realNameInput, "Jane Doe");
      expect(realNameInput).toHaveValue("Jane Doe");
    });

    it("allows typing in idCard field", async () => {
      const user = userEvent.setup();
      render(<TestWrapper />, { wrapper: createWrapper() });
      await waitFor(() => {
        expect(screen.getByRole("textbox", { name: /idCard/i })).toBeInTheDocument();
      });
      const idCardInput = screen.getByRole("textbox", { name: /idCard/i });
      await user.clear(idCardInput);
      await user.type(idCardInput, "320101199001011234");
      expect(idCardInput).toHaveValue("320101199001011234");
    });

    it("enables verify button reactively once idCard is typed (FE-A-07 useWatch)", async () => {
      const user = userEvent.setup();
      // 未存储证件号且表单为空 → canVerify 初始 false，按钮 disabled。
      const emptyIdCardUser: UserDetail = {
        ...mockUser,
        profile: { ...mockUser.profile, idCardSet: false, idCardMasked: "" },
      };
      render(<TestWrapper user={emptyIdCardUser} />, { wrapper: createWrapper() });
      const verifyButton = screen.getByRole("button", { name: /verifyIdentity/i });
      await waitFor(() => expect(verifyButton).toBeDisabled());
      // 键入 idCard 后 useWatch 驱动重渲染 → 三字段齐备，按钮可用。
      const idCardInput = screen.getByRole("textbox", { name: /idCard/i });
      await user.type(idCardInput, "320101199001011234");
      await waitFor(() => expect(verifyButton).toBeEnabled());
    });

    it("keeps verify button enabled with an empty field when a number is stored", async () => {
      // idCardSet=true：表单留空 = 保持存储值，验证按钮不因空输入而禁用。
      render(<TestWrapper />, { wrapper: createWrapper() });
      const verifyButton = screen.getByRole("button", { name: /verifyIdentity/i });
      await waitFor(() => expect(verifyButton).toBeEnabled());
    });
  });

  describe("mutation state", () => {
    it("shows loading on save button when mutation is pending", () => {
      render(<TestWrapper profileMutationOverrides={{ isPending: true }} />, {
        wrapper: createWrapper(),
      });
      const saveButton = screen.getByRole("button", { name: /account\.saveChanges/i });
      // antd 6: `loading` adds the `ant-btn-loading` class and blocks clicks, but
      // does NOT emit the `disabled` attribute.
      expect(saveButton).toHaveClass("ant-btn-loading");
    });

    it("shows loading on verify button when verify mutation is pending", () => {
      render(<TestWrapper verifyIdMutationOverrides={{ isPending: true }} />, {
        wrapper: createWrapper(),
      });
      const verifyButton = screen.getByRole("button", { name: /verifyIdentity/i });
      // canVerify 现由 useWatch 驱动：profile 三字段齐备时按钮可用，pending 仅以
      // loading 类表达（antd 6 loading 不发 disabled 属性），与 save 按钮断言一致。
      expect(verifyButton).toHaveClass("ant-btn-loading");
    });
  });

  describe("verified user behavior", () => {
    it("disables form fields when user is verified", () => {
      const verifiedUser = { ...mockUser, isVerified: true };
      render(<TestWrapper user={verifiedUser} />, { wrapper: createWrapper() });
      const realNameInput = screen.getByRole("textbox", { name: /realName/i });
      expect(realNameInput).toBeDisabled();
    });

    it("disables idCard input when user is verified", () => {
      const verifiedUser = { ...mockUser, isVerified: true };
      render(<TestWrapper user={verifiedUser} />, { wrapper: createWrapper() });
      const idCardInput = screen.getByRole("textbox", { name: /idCard/i });
      expect(idCardInput).toBeDisabled();
    });

    it("disables save button when user is verified", () => {
      const verifiedUser = { ...mockUser, isVerified: true };
      render(<TestWrapper user={verifiedUser} />, { wrapper: createWrapper() });
      const saveButton = screen.getByRole("button", { name: /account\.saveChanges/i });
      expect(saveButton).toBeDisabled();
    });
  });

  describe("edge cases", () => {
    it("renders when user is undefined", () => {
      render(<TestWrapper user={undefined} />, { wrapper: createWrapper() });
      expect(screen.getByText("user.realName")).toBeInTheDocument();
    });

    it("renders when user profile is empty", () => {
      const emptyProfileUser = { ...mockUser, profile: {} };
      render(<TestWrapper user={emptyProfileUser} />, { wrapper: createWrapper() });
      expect(screen.getByText("user.realName")).toBeInTheDocument();
    });

    it("renders ID card type select placeholder", () => {
      // The placeholder only renders while the field has no value. Without a user
      // the prefill effect returns early, so the field stays empty.
      render(<TestWrapper hasUser={false} />, { wrapper: createWrapper() });
      expect(screen.getByText("user.selectIdCardType")).toBeInTheDocument();
    });
  });
});
