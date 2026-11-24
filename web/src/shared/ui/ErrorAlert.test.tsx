import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ErrorAlert } from "./ErrorAlert";

describe("ErrorAlert", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("renders with title", () => {
    it("renders error alert with title", () => {
      render(<ErrorAlert title="Error occurred" />);
      expect(screen.getByText("Error occurred")).toBeInTheDocument();
    });

    it("renders error alert with description", () => {
      render(<ErrorAlert title="Error" description="Something went wrong" />);
      expect(screen.getByText("Error")).toBeInTheDocument();
      expect(screen.getByText("Something went wrong")).toBeInTheDocument();
    });

    it("renders with ReactNode description", () => {
      render(
        <ErrorAlert title="Error" description={<span data-testid="custom-desc">Custom</span>} />,
      );
      expect(screen.getByText("Error")).toBeInTheDocument();
      expect(screen.getByTestId("custom-desc")).toBeInTheDocument();
    });
  });

  describe("returns null when no content", () => {
    it("returns null when title and description are both missing", () => {
      const { container } = render(<ErrorAlert />);
      expect(container.innerHTML).toBe("");
    });

    it("returns null when title and description are both undefined", () => {
      const { container } = render(<ErrorAlert title={undefined} description={undefined} />);
      expect(container.innerHTML).toBe("");
    });
  });

  describe("variant styling", () => {
    it("renders auth variant as error type", () => {
      render(<ErrorAlert title="Auth Error" variant="auth" />);
      const alert = document.querySelector(".ant-alert-error");
      expect(alert).toBeInTheDocument();
    });

    it("renders network variant as warning type", () => {
      render(<ErrorAlert title="Network Error" variant="network" />);
      const alert = document.querySelector(".ant-alert-warning");
      expect(alert).toBeInTheDocument();
    });

    it("renders mfa variant as info type", () => {
      render(<ErrorAlert title="MFA Info" variant="mfa" />);
      const alert = document.querySelector(".ant-alert-info");
      expect(alert).toBeInTheDocument();
    });

    it("renders account variant as error type", () => {
      render(<ErrorAlert title="Account Error" variant="account" />);
      const alert = document.querySelector(".ant-alert-error");
      expect(alert).toBeInTheDocument();
    });

    it("renders generic variant as error type by default", () => {
      render(<ErrorAlert title="Generic Error" variant="generic" />);
      const alert = document.querySelector(".ant-alert-error");
      expect(alert).toBeInTheDocument();
    });

    it("renders default variant as generic error", () => {
      render(<ErrorAlert title="Default Error" />);
      const alert = document.querySelector(".ant-alert-error");
      expect(alert).toBeInTheDocument();
    });
  });

  describe("closable behavior", () => {
    it("is not closable by default", () => {
      render(<ErrorAlert title="Error" />);
      const closeIcon = document.querySelector(".ant-alert-close-icon");
      expect(closeIcon).not.toBeInTheDocument();
    });

    it("is closable when closable prop is true", () => {
      render(<ErrorAlert title="Error" closable />);
      const closeIcon = document.querySelector(".ant-alert-close-icon");
      expect(closeIcon).toBeInTheDocument();
    });

    it("calls onClose when closed", async () => {
      const onClose = vi.fn<(...args: unknown[]) => unknown>();
      const user = userEvent.setup();
      render(<ErrorAlert title="Error" closable onClose={onClose} />);
      const closeBtn = document.querySelector(".ant-alert-close-icon");
      expect(closeBtn).not.toBeNull();
      await user.click(closeBtn as HTMLElement);
      expect(onClose).toHaveBeenCalled();
    });
  });

  describe("showIcon", () => {
    it("always shows icon", () => {
      render(<ErrorAlert title="Error" />);
      const icon = document.querySelector(".ant-alert-icon");
      expect(icon).toBeInTheDocument();
    });
  });

  describe("edge cases", () => {
    it("renders with only description and no title", () => {
      render(<ErrorAlert description="Just a description" />);
      expect(screen.getByText("Just a description")).toBeInTheDocument();
    });

    it("renders with empty string title", () => {
      render(<ErrorAlert title="" description="Description" />);
      expect(screen.getByText("Description")).toBeInTheDocument();
    });
  });
});
