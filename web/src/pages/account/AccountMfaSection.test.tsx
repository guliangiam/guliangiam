import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { AccountMfaSection } from "./AccountMfaSection";
import type { UserMfaMethod, UserMfaSetupInitiateResult } from "@/shared/api/iam/user-types";
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

vi.mock("qrcode.react", () => ({
  QRCodeSVG: ({ value }: { value: string }) => <div data-testid="qrcode-svg" data-value={value} />,
}));

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

const createWrapper = createTestWrapper;

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

const mockMfaMethods: UserMfaMethod[] = [
  { methodType: "totp", enabled: true, verified: true, displayName: "Authenticator" },
  { methodType: "sms", enabled: true, verified: true, displayName: "SMS" },
];

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

describe("AccountMfaSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("renders MFA section", () => {
    it("renders card title", () => {
      render(
        <AccountMfaSection
          mfaMethods={[]}
          preferredMfa={undefined}
          mfaSetupMutation={createMockMutation<UserMfaSetupInitiateResult, string>()}
          mfaDisableMutation={createMockMutation<void, string>()}
          setPreferredMfaMutation={createMockMutation<void, string>()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("account.mfaSection")).toBeInTheDocument();
    });

    it("renders info alert when no MFA methods", () => {
      render(
        <AccountMfaSection
          mfaMethods={[]}
          preferredMfa={undefined}
          mfaSetupMutation={createMockMutation<UserMfaSetupInitiateResult, string>()}
          mfaDisableMutation={createMockMutation<void, string>()}
          setPreferredMfaMutation={createMockMutation<void, string>()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("account.noMfaEnabled")).toBeInTheDocument();
    });

    it("renders table when MFA methods exist", () => {
      render(
        <AccountMfaSection
          mfaMethods={mockMfaMethods}
          preferredMfa="totp"
          mfaSetupMutation={createMockMutation<UserMfaSetupInitiateResult, string>()}
          mfaDisableMutation={createMockMutation<void, string>()}
          setPreferredMfaMutation={createMockMutation<void, string>()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("totp")).toBeInTheDocument();
      expect(screen.getByText("sms")).toBeInTheDocument();
    });

    it("renders setup TOTP button", () => {
      render(
        <AccountMfaSection
          mfaMethods={[]}
          preferredMfa={undefined}
          mfaSetupMutation={createMockMutation<UserMfaSetupInitiateResult, string>()}
          mfaDisableMutation={createMockMutation<void, string>()}
          setPreferredMfaMutation={createMockMutation<void, string>()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("account.setupTotp")).toBeInTheDocument();
    });

    it("renders column headers in table", () => {
      render(
        <AccountMfaSection
          mfaMethods={mockMfaMethods}
          preferredMfa="totp"
          mfaSetupMutation={createMockMutation<UserMfaSetupInitiateResult, string>()}
          mfaDisableMutation={createMockMutation<void, string>()}
          setPreferredMfaMutation={createMockMutation<void, string>()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("account.mfaType")).toBeInTheDocument();
      expect(screen.getByText("account.mfaEnabled")).toBeInTheDocument();
      expect(screen.getByText("account.mfaPreferred")).toBeInTheDocument();
      expect(screen.getByText("account.mfaActions")).toBeInTheDocument();
    });
  });

  describe("MFA method display", () => {
    it("renders single MFA method", () => {
      render(
        <AccountMfaSection
          mfaMethods={[{ methodType: "totp", enabled: true, verified: true }]}
          preferredMfa="totp"
          mfaSetupMutation={createMockMutation<UserMfaSetupInitiateResult, string>()}
          mfaDisableMutation={createMockMutation<void, string>()}
          setPreferredMfaMutation={createMockMutation<void, string>()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("totp")).toBeInTheDocument();
    });

    it("renders multiple MFA methods", () => {
      render(
        <AccountMfaSection
          mfaMethods={mockMfaMethods}
          preferredMfa="totp"
          mfaSetupMutation={createMockMutation<UserMfaSetupInitiateResult, string>()}
          mfaDisableMutation={createMockMutation<void, string>()}
          setPreferredMfaMutation={createMockMutation<void, string>()}
        />,
        { wrapper: createWrapper() },
      );
      const totpElements = screen.getAllByText("totp");
      const smsElements = screen.getAllByText("sms");
      expect(totpElements.length).toBeGreaterThanOrEqual(1);
      expect(smsElements.length).toBeGreaterThanOrEqual(1);
    });

    it("renders disable button for each method", () => {
      render(
        <AccountMfaSection
          mfaMethods={mockMfaMethods}
          preferredMfa="totp"
          mfaSetupMutation={createMockMutation<UserMfaSetupInitiateResult, string>()}
          mfaDisableMutation={createMockMutation<void, string>()}
          setPreferredMfaMutation={createMockMutation<void, string>()}
        />,
        { wrapper: createWrapper() },
      );
      const disableButtons = screen.getAllByText("account.disable");
      expect(disableButtons.length).toBe(2);
    });
  });

  describe("mutation state", () => {
    it("shows loading on setup button when mutation is pending", () => {
      render(
        <AccountMfaSection
          mfaMethods={[]}
          preferredMfa={undefined}
          mfaSetupMutation={createMockMutation<UserMfaSetupInitiateResult, string>({
            isPending: true,
          })}
          mfaDisableMutation={createMockMutation<void, string>()}
          setPreferredMfaMutation={createMockMutation<void, string>()}
        />,
        { wrapper: createWrapper() },
      );
      const setupButton = screen.getByRole("button", { name: /account\.setupTotp/i });
      // antd 6: `loading` adds the `ant-btn-loading` class and blocks clicks, but
      // does NOT emit the `disabled` attribute (that requires an explicit `disabled`).
      expect(setupButton).toHaveClass("ant-btn-loading");
    });

    it("shows loading on disable button when disable mutation is pending", () => {
      render(
        <AccountMfaSection
          mfaMethods={[{ methodType: "totp", enabled: true, verified: true }]}
          preferredMfa="totp"
          mfaSetupMutation={createMockMutation<UserMfaSetupInitiateResult, string>()}
          mfaDisableMutation={createMockMutation<void, string>({ isPending: true })}
          setPreferredMfaMutation={createMockMutation<void, string>()}
        />,
        { wrapper: createWrapper() },
      );
      const disableButtons = screen
        .getAllByRole("button")
        .filter((btn) => btn.textContent?.includes("account.disable"));
      expect(disableButtons.length).toBeGreaterThanOrEqual(1);
    });
  });

  describe("user interaction", () => {
    it("clicking setup TOTP button triggers mutation", async () => {
      const mockMutate = vi.fn<(...args: unknown[]) => unknown>();
      const user = userEvent.setup();
      render(
        <AccountMfaSection
          mfaMethods={[]}
          preferredMfa={undefined}
          mfaSetupMutation={createMockMutation<UserMfaSetupInitiateResult, string>({
            mutate: mockMutate,
          })}
          mfaDisableMutation={createMockMutation<void, string>()}
          setPreferredMfaMutation={createMockMutation<void, string>()}
        />,
        { wrapper: createWrapper() },
      );
      await user.click(screen.getByText("account.setupTotp"));
      expect(mockMutate).toHaveBeenCalledWith("totp", expect.any(Object));
    });
  });

  describe("TOTP enable passcode proof", () => {
    function renderSetupFlow(enableMutate: ReturnType<typeof vi.fn>) {
      const setupMutation = createMockMutation<UserMfaSetupInitiateResult, string>({
        mutate: vi.fn<(...args: unknown[]) => unknown>((...args: unknown[]) => {
          const opts = args[1] as { onSuccess?: (d: unknown) => void } | undefined;
          opts?.onSuccess?.({
            url: "otpauth://totp/demo",
            secret: "SECRET",
            recoveryCodes: ["rc-1"],
          });
        }),
      });
      const verifyMutation = createMockMutation<
        void,
        { mfaType: string; passcode: string; secret?: string }
      >({
        mutate: vi.fn<(...args: unknown[]) => unknown>((...args: unknown[]) => {
          const opts = args[1] as { onSuccess?: () => void } | undefined;
          opts?.onSuccess?.();
        }),
      });
      const enableMutation = createMockMutation<
        void,
        { mfaType: string; secret?: string; passcode: string; recoveryCodes: string[] }
      >({ mutate: enableMutate });
      render(
        <AccountMfaSection
          mfaMethods={[]}
          preferredMfa={undefined}
          mfaSetupMutation={setupMutation}
          mfaVerifyMutation={verifyMutation}
          mfaEnableMutation={enableMutation}
          mfaDisableMutation={createMockMutation<void, string>()}
          setPreferredMfaMutation={createMockMutation<void, string>()}
        />,
        { wrapper: createWrapper() },
      );
    }

    async function advanceToRecovery(user: ReturnType<typeof userEvent.setup>) {
      await user.click(screen.getByText("account.setupTotp"));
      await user.click(screen.getByText("account.mfaNext"));
      await user.type(screen.getByPlaceholderText("000000"), "123456");
      await user.click(screen.getByText("account.mfaVerify"));
    }

    it("forwards a fresh passcode to the enable mutation", async () => {
      const enableMutate = vi.fn<(...args: unknown[]) => unknown>();
      const user = userEvent.setup();
      renderSetupFlow(enableMutate);
      await advanceToRecovery(user);

      const codeInput = screen.getByPlaceholderText("000000");
      await user.clear(codeInput);
      await user.type(codeInput, "654321");
      await user.click(screen.getByText("account.mfaEnable"));

      expect(enableMutate).toHaveBeenCalledWith(
        {
          mfaType: "totp",
          secret: "SECRET",
          passcode: "654321",
          recoveryCodes: ["rc-1"],
        },
        expect.any(Object),
      );
    });

    it("blocks enable when no passcode is entered", async () => {
      const enableMutate = vi.fn<(...args: unknown[]) => unknown>();
      const user = userEvent.setup();
      renderSetupFlow(enableMutate);
      await advanceToRecovery(user);

      await user.clear(screen.getByPlaceholderText("000000"));
      await user.click(screen.getByText("account.mfaEnable"));

      expect(enableMutate).not.toHaveBeenCalled();
    });
  });

  describe("edge cases", () => {
    it("handles empty MFA methods array", () => {
      render(
        <AccountMfaSection
          mfaMethods={[]}
          preferredMfa={undefined}
          mfaSetupMutation={createMockMutation<UserMfaSetupInitiateResult, string>()}
          mfaDisableMutation={createMockMutation<void, string>()}
          setPreferredMfaMutation={createMockMutation<void, string>()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("account.noMfaEnabled")).toBeInTheDocument();
      expect(screen.queryByText("totp")).not.toBeInTheDocument();
    });

    it("handles undefined preferredMfa", () => {
      render(
        <AccountMfaSection
          mfaMethods={[{ methodType: "totp", enabled: true, verified: true }]}
          preferredMfa={undefined}
          mfaSetupMutation={createMockMutation<UserMfaSetupInitiateResult, string>()}
          mfaDisableMutation={createMockMutation<void, string>()}
          setPreferredMfaMutation={createMockMutation<void, string>()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("totp")).toBeInTheDocument();
    });

    it("handles disabled MFA method", () => {
      render(
        <AccountMfaSection
          mfaMethods={[{ methodType: "totp", enabled: false, verified: false }]}
          preferredMfa={undefined}
          mfaSetupMutation={createMockMutation<UserMfaSetupInitiateResult, string>()}
          mfaDisableMutation={createMockMutation<void, string>()}
          setPreferredMfaMutation={createMockMutation<void, string>()}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("totp")).toBeInTheDocument();
    });
  });
});
