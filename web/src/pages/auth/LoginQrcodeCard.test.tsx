import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import QrSigninCard from "./LoginQrcodeCard";
import type { QrSigninState } from "@/shared/hooks/useQrSigninFlow";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, options?: { time?: string }) =>
      options?.time ? `${key}:${options.time}` : key,
  }),
}));

vi.mock("qrcode.react", () => ({
  QRCodeSVG: ({ value }: { value: string }) => <div data-testid="qr-svg" data-value={value} />,
}));

function renderCard(state: QrSigninState, onRefresh = vi.fn<(...args: unknown[]) => unknown>()) {
  render(<QrSigninCard state={state} onRefresh={onRefresh} />);
  return onRefresh;
}

describe("QrSigninCard", () => {
  it("renders the QR payload with the countdown while pending", () => {
    renderCard({ status: "pending", code: "qr-payload", expiresIn: 300, error: null });

    expect(screen.getByTestId("qr-svg")).toHaveAttribute("data-value", "qr-payload");
    expect(screen.getByText("login.qrcode.expiresIn:5:00")).toBeInTheDocument();
    expect(screen.getByText("login.qrcode.scanPrompt")).toBeInTheDocument();
  });

  it("shows the scanned hint without the QR once the phone scanned it", () => {
    renderCard({ status: "scanned", code: "qr-payload", expiresIn: 120, error: null });

    expect(screen.getByText("login.qrcode.scanned")).toBeInTheDocument();
    expect(screen.queryByTestId("qr-svg")).not.toBeInTheDocument();
  });

  it("offers a refresh when the session expired", () => {
    const onRefresh = renderCard({
      status: "expired",
      code: null,
      expiresIn: null,
      error: "auth:login.qrcode.expired",
    });

    expect(screen.getByText("login.qrcode.expired")).toBeInTheDocument();
    fireEvent.click(screen.getByText("login.qrcode.refreshQrCode"));
    expect(onRefresh).toHaveBeenCalledTimes(1);
  });

  it("shows the failure reason with a retry", () => {
    const onRefresh = renderCard({ status: "failed", code: null, expiresIn: null, error: "boom" });

    expect(screen.getByText("boom")).toBeInTheDocument();
    fireEvent.click(screen.getByText("login.qrcode.refreshQrCode"));
    expect(onRefresh).toHaveBeenCalledTimes(1);
  });

  it("confirms success without rendering a QR", () => {
    renderCard({ status: "confirmed", code: "qr-payload", expiresIn: 300, error: null });

    expect(screen.getByText("login.qrcode.success")).toBeInTheDocument();
    expect(screen.queryByTestId("qr-svg")).not.toBeInTheDocument();
  });

  it("shows the scan prompt while the session is still being created", () => {
    renderCard({ status: "loading", code: null, expiresIn: null, error: null });

    expect(screen.getByText("login.qrcode.scanPrompt")).toBeInTheDocument();
    expect(screen.queryByTestId("qr-svg")).not.toBeInTheDocument();
  });
});
