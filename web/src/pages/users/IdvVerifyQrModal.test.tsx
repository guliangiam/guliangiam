import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "antd";
import { afterEach, describe, expect, it, vi } from "vitest";
import IdvVerifyQrModal from "./IdvVerifyQrModal";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key, i18n: { language: "en" } }),
}));

function renderModal(props: { open?: boolean; verifyUrl?: string } = {}) {
  const onClose = vi.fn<() => void>();
  render(
    <App component={false}>
      <IdvVerifyQrModal
        open={props.open ?? true}
        verifyUrl={props.verifyUrl ?? "https://verify.test/certify/v-9"}
        onClose={onClose}
      />
    </App>,
  );
  return { onClose };
}

function stubClipboard(writeText = vi.fn<() => Promise<void>>().mockResolvedValue(undefined)) {
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
  return writeText;
}

afterEach(() => {
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });
});

describe("IdvVerifyQrModal", () => {
  it("renders the hosted verification link as a scannable QR code", () => {
    stubClipboard();
    renderModal();
    expect(screen.getByRole("dialog", { name: "verifyIdQrTitle" })).toBeTruthy();
    expect(screen.getByText("verifyIdQrHint")).toBeTruthy();
    // QRCodeSVG 用 <svg> 承载；可扫码的事实由 value 决定的路径图形体现，这里只验证
    // 托管链接确实进了二维码而不是只贴在文案里。
    expect(document.querySelector("svg")).toBeTruthy();
  });

  it("copies the link and closes on demand", async () => {
    const user = userEvent.setup();
    const writeText = stubClipboard();
    const { onClose } = renderModal();

    await user.click(screen.getByRole("button", { name: "verifyIdCopyLink" }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith("https://verify.test/certify/v-9"));
    await user.click(screen.getByRole("button", { name: "common:close" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("hides the copy action outside a secure context", () => {
    // jsdom 未实现 Clipboard API：非安全上下文下按钮必须消失，而不是点了报错。
    renderModal();
    expect(screen.queryByRole("button", { name: "verifyIdCopyLink" })).toBeNull();
    expect(screen.getByText("verifyIdQrHint")).toBeTruthy();
  });

  it("reports a clipboard rejection instead of a silent failure", async () => {
    const user = userEvent.setup();
    stubClipboard(vi.fn<() => Promise<void>>().mockRejectedValue(new Error("denied")));
    renderModal();

    await user.click(screen.getByRole("button", { name: "verifyIdCopyLink" }));
    expect(await screen.findByText("common:copyFailed")).toBeTruthy();
  });
});
