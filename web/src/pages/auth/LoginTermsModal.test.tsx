import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LoginTermsModal } from "./LoginTermsModal";

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

vi.mock("@/shared/lib/dom-purify", () => ({
  sanitizeHtml: (html: string) => html,
  sanitizeCss: (css: string) => css,
}));

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

describe("LoginTermsModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("renders when open", () => {
    it("renders modal title", () => {
      render(<LoginTermsModal open={true} onCancel={vi.fn<(...args: unknown[]) => unknown>()} />);
      expect(screen.getByText("termsTitle")).toBeInTheDocument();
    });

    it("renders no terms configured message when termsOfUse is not provided", () => {
      render(<LoginTermsModal open={true} onCancel={vi.fn<(...args: unknown[]) => unknown>()} />);
      expect(screen.getByText("noTermsConfigured")).toBeInTheDocument();
    });

    it("renders terms content when termsOfUse is provided", () => {
      render(
        <LoginTermsModal
          open={true}
          termsOfUse="<p>Terms and Conditions</p>"
          onCancel={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByText("Terms and Conditions")).toBeInTheDocument();
    });

    it("renders terms content with HTML", () => {
      render(
        <LoginTermsModal
          open={true}
          termsOfUse="<h1>Terms</h1><p>Please read carefully.</p>"
          onCancel={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByText("Terms")).toBeInTheDocument();
      expect(screen.getByText("Please read carefully.")).toBeInTheDocument();
    });

    it("renders no terms configured when termsOfUse is null", () => {
      render(
        <LoginTermsModal
          open={true}
          termsOfUse={null}
          onCancel={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByText("noTermsConfigured")).toBeInTheDocument();
    });

    it("renders no terms configured when termsOfUse is empty string", () => {
      render(
        <LoginTermsModal
          open={true}
          termsOfUse=""
          onCancel={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByText("noTermsConfigured")).toBeInTheDocument();
    });
  });

  describe("closed state", () => {
    it("does not render modal content when closed", () => {
      render(<LoginTermsModal open={false} onCancel={vi.fn<(...args: unknown[]) => unknown>()} />);
      expect(screen.queryByText("termsTitle")).not.toBeInTheDocument();
    });
  });

  describe("cancel interaction", () => {
    it("calls onCancel when modal is closed", async () => {
      const onCancel = vi.fn<(...args: unknown[]) => unknown>();
      const user = userEvent.setup();
      render(<LoginTermsModal open={true} onCancel={onCancel} />);
      // The modal should have a close button
      const closeBtn = document.querySelector(".ant-modal-close");
      expect(closeBtn).not.toBeNull();
      await user.click(closeBtn as HTMLElement);
      expect(onCancel).toHaveBeenCalled();
    });
  });

  describe("edge cases", () => {
    it("handles very long terms content", () => {
      const longContent = "<p>" + "A".repeat(1000) + "</p>";
      render(
        <LoginTermsModal
          open={true}
          termsOfUse={longContent}
          onCancel={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByText("A".repeat(1000))).toBeInTheDocument();
    });

    it("handles terms with special characters", () => {
      render(
        <LoginTermsModal
          open={true}
          termsOfUse="<p>Terms &amp; Conditions</p>"
          onCancel={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      // The HTML entity should be preserved by sanitizeHtml
      expect(screen.getByText("Terms & Conditions")).toBeInTheDocument();
    });
  });
});
