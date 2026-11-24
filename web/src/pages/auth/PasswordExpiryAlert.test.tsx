import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import PasswordExpiryAlert from "./PasswordExpiryAlert";

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (key === "passwordExpiry.expiringSoonMessage" && params?.days !== undefined) {
        return `Expires in ${params.days} days`;
      }
      return key;
    },
    i18n: { language: "en" },
  }),
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

describe("PasswordExpiryAlert", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("returns null when no expiry", () => {
    it("returns null when password is not expired and not expiring soon", () => {
      const { container } = render(
        <PasswordExpiryAlert
          isExpired={false}
          isExpiringSoon={false}
          daysRemaining={30}
          forceChange={false}
          onChangePassword={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(container.innerHTML).toBe("");
    });
  });

  describe("expired state", () => {
    it("renders error alert when password is expired", () => {
      render(
        <PasswordExpiryAlert
          isExpired={true}
          isExpiringSoon={false}
          daysRemaining={0}
          forceChange={false}
          onChangePassword={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByText("passwordExpiry.expiredMessage")).toBeInTheDocument();
    });

    it("renders change now button when not force change", () => {
      render(
        <PasswordExpiryAlert
          isExpired={true}
          isExpiringSoon={false}
          daysRemaining={0}
          forceChange={false}
          onChangePassword={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByText("passwordExpiry.changeNow")).toBeInTheDocument();
    });

    it("renders force change modal when forceChange is true", () => {
      render(
        <PasswordExpiryAlert
          isExpired={true}
          isExpiringSoon={false}
          daysRemaining={0}
          forceChange={true}
          onChangePassword={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByText("passwordExpiry.forceChangeTitle")).toBeInTheDocument();
    });

    it("renders form fields in force change modal", () => {
      render(
        <PasswordExpiryAlert
          isExpired={true}
          isExpiringSoon={false}
          daysRemaining={0}
          forceChange={true}
          onChangePassword={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByText("passwordExpiry.oldPassword")).toBeInTheDocument();
      expect(screen.getByText("passwordExpiry.newPassword")).toBeInTheDocument();
      expect(screen.getByText("passwordExpiry.confirmPassword")).toBeInTheDocument();
    });

    it("renders change password button in force change modal", () => {
      render(
        <PasswordExpiryAlert
          isExpired={true}
          isExpiringSoon={false}
          daysRemaining={0}
          forceChange={true}
          onChangePassword={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByText("passwordExpiry.changePassword")).toBeInTheDocument();
    });
  });

  describe("expiring soon state", () => {
    it("renders warning alert when password is expiring soon", () => {
      render(
        <PasswordExpiryAlert
          isExpired={false}
          isExpiringSoon={true}
          daysRemaining={7}
          forceChange={false}
          onChangePassword={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByText("Expires in 7 days")).toBeInTheDocument();
    });

    it("renders change now button for expiring soon", () => {
      render(
        <PasswordExpiryAlert
          isExpired={false}
          isExpiringSoon={true}
          daysRemaining={5}
          forceChange={false}
          onChangePassword={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByText("passwordExpiry.changeNow")).toBeInTheDocument();
    });

    it("renders warning type alert for expiring soon", () => {
      render(
        <PasswordExpiryAlert
          isExpired={false}
          isExpiringSoon={true}
          daysRemaining={3}
          forceChange={false}
          onChangePassword={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      const alert = document.querySelector(".ant-alert-warning");
      expect(alert).toBeInTheDocument();
    });
  });

  describe("change password modal interaction", () => {
    it("opens modal when clicking change now button", async () => {
      const user = userEvent.setup();
      render(
        <PasswordExpiryAlert
          isExpired={true}
          isExpiringSoon={false}
          daysRemaining={0}
          forceChange={false}
          onChangePassword={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      await user.click(screen.getByText("passwordExpiry.changeNow"));

      await waitFor(() => {
        expect(screen.getByText("passwordExpiry.changePasswordTitle")).toBeInTheDocument();
      });
    });
  });

  describe("edge cases", () => {
    it("handles both expired and expiring soon flags", () => {
      render(
        <PasswordExpiryAlert
          isExpired={true}
          isExpiringSoon={true}
          daysRemaining={0}
          forceChange={false}
          onChangePassword={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByText("passwordExpiry.expiredMessage")).toBeInTheDocument();
    });

    it("handles daysRemaining = 0", () => {
      render(
        <PasswordExpiryAlert
          isExpired={false}
          isExpiringSoon={true}
          daysRemaining={0}
          forceChange={false}
          onChangePassword={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByText("Expires in 0 days")).toBeInTheDocument();
    });

    it("handles daysRemaining = 1", () => {
      render(
        <PasswordExpiryAlert
          isExpired={false}
          isExpiringSoon={true}
          daysRemaining={1}
          forceChange={false}
          onChangePassword={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByText("Expires in 1 days")).toBeInTheDocument();
    });
  });
});
