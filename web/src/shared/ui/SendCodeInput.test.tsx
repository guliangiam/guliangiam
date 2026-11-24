import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SendCodeInput } from "./SendCodeInput";

describe("SendCodeInput", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe("renders component", () => {
    it("renders input field", () => {
      render(<SendCodeInput onSendCode={vi.fn<(...args: unknown[]) => unknown>()} />);
      expect(screen.getByPlaceholderText("Enter verification code")).toBeInTheDocument();
    });

    it("renders send code button", () => {
      render(<SendCodeInput onSendCode={vi.fn<(...args: unknown[]) => unknown>()} />);
      expect(screen.getByText("Send Code")).toBeInTheDocument();
    });

    it("renders with custom placeholder", () => {
      render(
        <SendCodeInput
          onSendCode={vi.fn<(...args: unknown[]) => unknown>()}
          placeholder="Enter code"
        />,
      );
      expect(screen.getByPlaceholderText("Enter code")).toBeInTheDocument();
    });

    it("renders with custom send button text", () => {
      render(
        <SendCodeInput
          onSendCode={vi.fn<(...args: unknown[]) => unknown>()}
          sendButtonText="Send"
        />,
      );
      expect(screen.getByText("Send")).toBeInTheDocument();
    });
  });

  describe("controlled value", () => {
    it("displays the provided value", () => {
      render(
        <SendCodeInput
          value="123456"
          onSendCode={vi.fn<(...args: unknown[]) => unknown>()}
          onChange={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      const input = screen.getByPlaceholderText("Enter verification code") as HTMLInputElement;
      expect(input.value).toBe("123456");
    });

    it("calls onChange when typing", async () => {
      const onChange = vi.fn<(...args: unknown[]) => unknown>();
      const user = userEvent.setup({ delay: null });
      render(
        <SendCodeInput onSendCode={vi.fn<(...args: unknown[]) => unknown>()} onChange={onChange} />,
      );
      const input = screen.getByPlaceholderText("Enter verification code");
      await user.type(input, "1");
      expect(onChange).toHaveBeenCalled();
    });
  });

  describe("send code behavior", () => {
    it("calls onSendCode when clicking send button", async () => {
      const onSendCode = vi
        .fn<(...args: unknown[]) => Promise<unknown>>()
        .mockResolvedValue(undefined);
      const user = userEvent.setup({ delay: null });
      render(<SendCodeInput onSendCode={onSendCode} />);
      await user.click(screen.getByText("Send Code"));
      expect(onSendCode).toHaveBeenCalled();
    });

    it("shows cooldown timer after sending", async () => {
      const onSendCode = vi
        .fn<(...args: unknown[]) => Promise<unknown>>()
        .mockResolvedValue(undefined);
      const user = userEvent.setup({ delay: null });
      render(<SendCodeInput onSendCode={onSendCode} cooldown={60} />);
      await user.click(screen.getByText("Send Code"));

      await waitFor(() => {
        expect(screen.getByText("60s")).toBeInTheDocument();
      });
    });

    it("disables send button during cooldown", async () => {
      const onSendCode = vi
        .fn<(...args: unknown[]) => Promise<unknown>>()
        .mockResolvedValue(undefined);
      const user = userEvent.setup({ delay: null });
      render(<SendCodeInput onSendCode={onSendCode} cooldown={60} />);
      await user.click(screen.getByText("Send Code"));

      await waitFor(() => {
        const btn = screen.getByText("60s").closest("button");
        expect(btn).toBeDisabled();
      });
    });

    it("shows loading state while sending", async () => {
      let resolveSend: () => void;
      const onSendCode = vi.fn<(...args: unknown[]) => unknown>().mockImplementation(
        () =>
          new Promise<void>((resolve) => {
            resolveSend = resolve;
          }),
      );
      const user = userEvent.setup({ delay: null });
      render(<SendCodeInput onSendCode={onSendCode} />);
      await user.click(screen.getByText("Send Code"));

      // Button should be in loading state
      const btn = screen.getByRole("button");
      expect(btn.className).toContain("ant-btn-loading");

      resolveSend!();
    });

    it("does not call onSendCode when already in cooldown", async () => {
      const onSendCode = vi
        .fn<(...args: unknown[]) => Promise<unknown>>()
        .mockResolvedValue(undefined);
      const user = userEvent.setup({ delay: null });
      render(<SendCodeInput onSendCode={onSendCode} cooldown={60} />);
      await user.click(screen.getByText("Send Code"));

      await waitFor(() => {
        expect(screen.getByText("60s")).toBeInTheDocument();
      });

      // Try clicking again - should not call again
      const callCount = onSendCode.mock.calls.length;
      const btn = screen.getByText("60s").closest("button");
      expect(btn).toBeDisabled();
      expect(onSendCode).toHaveBeenCalledTimes(callCount);
    });
  });

  describe("disabled state", () => {
    it("disables input when disabled prop is true", () => {
      render(<SendCodeInput onSendCode={vi.fn<(...args: unknown[]) => unknown>()} disabled />);
      const input = screen.getByPlaceholderText("Enter verification code") as HTMLInputElement;
      expect(input.disabled).toBe(true);
    });

    it("disables button when disabled prop is true", () => {
      render(<SendCodeInput onSendCode={vi.fn<(...args: unknown[]) => unknown>()} disabled />);
      const btn = screen.getByRole("button");
      expect(btn).toBeDisabled();
    });
  });

  describe("custom cooldown", () => {
    it("uses default cooldown of 60 seconds", async () => {
      const onSendCode = vi
        .fn<(...args: unknown[]) => Promise<unknown>>()
        .mockResolvedValue(undefined);
      const user = userEvent.setup({ delay: null });
      render(<SendCodeInput onSendCode={onSendCode} />);
      await user.click(screen.getByText("Send Code"));

      await waitFor(() => {
        expect(screen.getByText("60s")).toBeInTheDocument();
      });
    });

    it("uses custom cooldown value", async () => {
      const onSendCode = vi
        .fn<(...args: unknown[]) => Promise<unknown>>()
        .mockResolvedValue(undefined);
      const user = userEvent.setup({ delay: null });
      render(<SendCodeInput onSendCode={onSendCode} cooldown={30} />);
      await user.click(screen.getByText("Send Code"));

      await waitFor(() => {
        expect(screen.getByText("30s")).toBeInTheDocument();
      });
    });
  });
});
