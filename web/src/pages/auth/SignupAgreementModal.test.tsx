import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SignupAgreementModal } from "./SignupAgreementModal";

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

describe("SignupAgreementModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the terms-and-privacy title when open", () => {
    render(
      <SignupAgreementModal
        open
        onAgree={vi.fn<(...args: unknown[]) => unknown>()}
        onCancel={vi.fn<(...args: unknown[]) => unknown>()}
      />,
    );
    expect(screen.getByText("termsAndPrivacyTitle")).toBeInTheDocument();
  });

  it("renders default terms content when no application terms configured", () => {
    render(
      <SignupAgreementModal
        open
        onAgree={vi.fn<(...args: unknown[]) => unknown>()}
        onCancel={vi.fn<(...args: unknown[]) => unknown>()}
      />,
    );
    expect(screen.getByText("Terms of Use")).toBeInTheDocument();
  });

  it("renders application-provided terms override", () => {
    render(
      <SignupAgreementModal
        open
        termsOfUse="<p>Custom Terms</p>"
        onAgree={vi.fn<(...args: unknown[]) => unknown>()}
        onCancel={vi.fn<(...args: unknown[]) => unknown>()}
      />,
    );
    expect(screen.getByText("Custom Terms")).toBeInTheDocument();
  });

  it("shows privacy policy content when the privacy tab is selected", async () => {
    const user = userEvent.setup();
    render(
      <SignupAgreementModal
        open
        onAgree={vi.fn<(...args: unknown[]) => unknown>()}
        onCancel={vi.fn<(...args: unknown[]) => unknown>()}
      />,
    );
    await user.click(screen.getByText("privacyTitle"));
    expect(screen.getByText("Privacy Policy")).toBeInTheDocument();
  });

  it("renders application-provided privacy policy override", async () => {
    const user = userEvent.setup();
    render(
      <SignupAgreementModal
        open
        privacyPolicy="<p>Custom Privacy</p>"
        onAgree={vi.fn<(...args: unknown[]) => unknown>()}
        onCancel={vi.fn<(...args: unknown[]) => unknown>()}
      />,
    );
    await user.click(screen.getByText("privacyTitle"));
    expect(screen.getByText("Custom Privacy")).toBeInTheDocument();
    expect(screen.queryByText("Privacy Policy")).not.toBeInTheDocument();
  });

  it("falls back to default privacy policy when application value is blank", async () => {
    const user = userEvent.setup();
    render(
      <SignupAgreementModal
        open
        privacyPolicy="   "
        onAgree={vi.fn<(...args: unknown[]) => unknown>()}
        onCancel={vi.fn<(...args: unknown[]) => unknown>()}
      />,
    );
    await user.click(screen.getByText("privacyTitle"));
    expect(screen.getByText("Privacy Policy")).toBeInTheDocument();
  });

  it("calls onAgree when agree-and-continue is clicked", async () => {
    const onAgree = vi.fn<(...args: unknown[]) => unknown>();
    const user = userEvent.setup();
    render(
      <SignupAgreementModal
        open
        onAgree={onAgree}
        onCancel={vi.fn<(...args: unknown[]) => unknown>()}
      />,
    );
    await user.click(screen.getByText("agreeAndContinue"));
    expect(onAgree).toHaveBeenCalled();
  });

  it("calls onCancel when cancel is clicked", async () => {
    const onCancel = vi.fn<(...args: unknown[]) => unknown>();
    const user = userEvent.setup();
    render(
      <SignupAgreementModal
        open
        onAgree={vi.fn<(...args: unknown[]) => unknown>()}
        onCancel={onCancel}
      />,
    );
    await user.click(screen.getByText("cancel"));
    expect(onCancel).toHaveBeenCalled();
  });

  it("does not render content when closed", () => {
    render(
      <SignupAgreementModal
        open={false}
        onAgree={vi.fn<(...args: unknown[]) => unknown>()}
        onCancel={vi.fn<(...args: unknown[]) => unknown>()}
      />,
    );
    expect(screen.queryByText("termsAndPrivacyTitle")).not.toBeInTheDocument();
  });
});
