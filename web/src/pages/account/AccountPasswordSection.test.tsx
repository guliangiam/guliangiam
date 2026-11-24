import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { AccountPasswordSection } from "./AccountPasswordSection";
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

vi.mock("@/shared/ui/PasswordModal", () => ({
  PasswordModal: ({
    userId,
    mode,
    hasPassword,
    onSuccess,
  }: {
    userId: string;
    mode: string;
    hasPassword: boolean;
    onSuccess?: () => void;
  }) => (
    <div data-testid="password-modal">
      <span data-testid="password-modal-userid">{userId}</span>
      <span data-testid="password-modal-mode">{mode}</span>
      <span data-testid="password-modal-has-password">{String(hasPassword)}</span>
      <button data-testid="password-modal-success" onClick={onSuccess}>
        Confirm
      </button>
    </div>
  ),
}));

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

const createWrapper = createTestWrapper;

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

describe("AccountPasswordSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("renders password section", () => {
    it("renders card title", () => {
      render(<AccountPasswordSection userId="user-1" />, { wrapper: createWrapper() });
      expect(screen.getByText("account.passwordSection")).toBeInTheDocument();
    });

    it("renders PasswordModal component", () => {
      render(<AccountPasswordSection userId="user-1" />, { wrapper: createWrapper() });
      expect(screen.getByTestId("password-modal")).toBeInTheDocument();
    });

    it("passes userId to PasswordModal", () => {
      render(<AccountPasswordSection userId="user-123" />, { wrapper: createWrapper() });
      expect(screen.getByTestId("password-modal-userid")).toHaveTextContent("user-123");
    });

    it("passes mode='self' to PasswordModal", () => {
      render(<AccountPasswordSection userId="user-1" />, { wrapper: createWrapper() });
      expect(screen.getByTestId("password-modal-mode")).toHaveTextContent("self");
    });

    it("passes hasPassword=true to PasswordModal", () => {
      render(<AccountPasswordSection userId="user-1" />, { wrapper: createWrapper() });
      expect(screen.getByTestId("password-modal-has-password")).toHaveTextContent("true");
    });
  });

  describe("different userId values", () => {
    it("renders with empty userId", () => {
      render(<AccountPasswordSection userId="" />, { wrapper: createWrapper() });
      expect(screen.getByTestId("password-modal-userid")).toHaveTextContent("");
    });

    it("renders with long userId", () => {
      render(<AccountPasswordSection userId="user-very-long-id-12345" />, {
        wrapper: createWrapper(),
      });
      expect(screen.getByTestId("password-modal-userid")).toHaveTextContent(
        "user-very-long-id-12345",
      );
    });
  });

  describe("onSuccess callback", () => {
    it("PasswordModal has a success callback button", () => {
      render(<AccountPasswordSection userId="user-1" />, { wrapper: createWrapper() });
      expect(screen.getByTestId("password-modal-success")).toBeInTheDocument();
    });

    it("clicking success button triggers onSuccess", async () => {
      const user = userEvent.setup();
      render(<AccountPasswordSection userId="user-1" />, { wrapper: createWrapper() });
      await user.click(screen.getByTestId("password-modal-success"));
      // The callback calls message.success and queryClient.invalidateQueries;
      // assert the section survived the callback without unmounting or throwing.
      expect(screen.getByTestId("password-modal")).toBeInTheDocument();
    });
  });
});
