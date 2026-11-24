import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "antd";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AccountContactBindingSection } from "./AccountContactBindingSection";

const mockSendVerificationCode = vi.fn<(...args: unknown[]) => Promise<unknown>>();
const mockUpdateOwnAccount = vi.fn<(...args: unknown[]) => Promise<unknown>>();

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en", resolvedLanguage: "en" },
  }),
}));

vi.mock("@/shared/lib/auth-verification-shim", () => ({
  sendVerificationCode: (...args: unknown[]) => mockSendVerificationCode(...args),
}));

vi.mock("@/shared/api/iam/auth", () => ({
  updateOwnAccount: (...args: unknown[]) => mockUpdateOwnAccount(...args),
}));

vi.mock("@/shared/lib/auth-captcha-shim", () => ({
  getCaptchaStatus: () => Promise.resolve({ required: false }),
  getCaptcha: () => Promise.resolve({}),
}));

function renderSection(onBound = vi.fn<() => void>()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <App>
        <AccountContactBindingSection organizationId="org-1" onBound={onBound} />
      </App>
    </QueryClientProvider>,
  );
  return onBound;
}

async function selectChina() {
  const user = userEvent.setup();
  const combobox = screen.getByRole("combobox", { name: "auth:countryCode" });
  await user.click(combobox);
  await user.type(combobox, "China");
  await user.click(await screen.findByText("People's Republic of China"));
}

describe("AccountContactBindingSection", () => {
  beforeEach(() => {
    mockSendVerificationCode.mockReset();
    mockUpdateOwnAccount.mockReset();
  });

  it("sends a verification code to the entered contact as E.164", async () => {
    mockSendVerificationCode.mockResolvedValue({});
    renderSection();
    await selectChina();

    fireEvent.change(screen.getByPlaceholderText("account.phone"), {
      target: { value: "13800000000" },
    });
    fireEvent.click(screen.getByText("auth:sendCode"));

    await waitFor(() =>
      expect(mockSendVerificationCode).toHaveBeenCalledWith({
        organizationId: "org-1",
        dest: "+8613800000000",
        type: "phone",
        countryCode: "CN",
        invalidPhoneMessage: "form.invalidPhone",
        captchaToken: undefined,
        captchaId: undefined,
      }),
    );
  });

  it("binds the contact with the code and refreshes the account", async () => {
    mockUpdateOwnAccount.mockResolvedValue({});
    const onBound = renderSection();
    await selectChina();

    fireEvent.change(screen.getByPlaceholderText("account.phone"), {
      target: { value: "13800000000" },
    });
    fireEvent.change(screen.getByPlaceholderText("auth:enterCode"), {
      target: { value: "123456" },
    });
    fireEvent.click(screen.getByText("auth:bind"));

    await waitFor(() =>
      expect(mockUpdateOwnAccount).toHaveBeenCalledWith({
        phone: "+8613800000000",
        verificationCode: "123456",
      }),
    );
    expect(onBound).toHaveBeenCalledTimes(1);
  });

  it("does not send a code without a recipient", async () => {
    renderSection();

    fireEvent.click(screen.getByText("auth:sendCode"));

    await waitFor(() => expect(mockSendVerificationCode).not.toHaveBeenCalled());
  });

  it("requires a country before sending a phone code", async () => {
    renderSection();

    fireEvent.change(screen.getByPlaceholderText("account.phone"), {
      target: { value: "13800000000" },
    });
    fireEvent.click(screen.getByText("auth:sendCode"));

    await waitFor(() => expect(mockSendVerificationCode).not.toHaveBeenCalled());
  });

  it("reports a failed send without binding", async () => {
    mockSendVerificationCode.mockRejectedValue(new Error("rate limited"));
    renderSection();
    await selectChina();

    fireEvent.change(screen.getByPlaceholderText("account.phone"), {
      target: { value: "13800000000" },
    });
    fireEvent.click(screen.getByText("auth:sendCode"));

    await waitFor(() => expect(mockSendVerificationCode).toHaveBeenCalledTimes(1));
    expect(mockUpdateOwnAccount).not.toHaveBeenCalled();
  });
});
