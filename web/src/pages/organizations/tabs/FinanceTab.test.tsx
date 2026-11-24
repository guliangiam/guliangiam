import { beforeEach, describe, expect, it, vi } from "vitest";
import { useEffect } from "react";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Form } from "antd";
import type { FormInstance } from "antd";
import { createTestWrapper } from "@/test-utils/render";
import { FinanceTab } from "./FinanceTab";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
}));

vi.mock("@/shared/api/iam/transaction", () => ({
  getIamTransactions: vi.fn<(...args: unknown[]) => Promise<unknown>>(async () => []),
}));

const initialValues = {
  balance: 100.5,
  currency: "USD",
  enableDeposit: true,
  balanceCredit: 20,
  mu: 1.5,
  defaultQuota: 25,
  defaultCurrency: "USD",
};

let formInstance: FormInstance | undefined;

function Harness({ onFinish }: { onFinish: (values: Record<string, unknown>) => void }) {
  const [form] = Form.useForm();
  useEffect(() => {
    formInstance = form;
  }, [form]);
  return (
    <Form form={form} initialValues={initialValues} onFinish={onFinish}>
      <FinanceTab organizationId="org-1" />
    </Form>
  );
}

function submitForm() {
  act(() => {
    formInstance!.submit();
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  formInstance = undefined;
});

describe("FinanceTab", () => {
  it("echoes the finance and quota fields from the form values", () => {
    render(<Harness onFinish={vi.fn<(values: Record<string, unknown>) => void>()} />, {
      wrapper: createTestWrapper(),
    });

    expect(Number((screen.getByLabelText("organization.balance") as HTMLInputElement).value)).toBe(
      100.5,
    );
    expect(
      Number((screen.getByLabelText("organization.balanceCredit") as HTMLInputElement).value),
    ).toBe(20);
    expect(Number((screen.getByLabelText("organization.mu") as HTMLInputElement).value)).toBe(1.5);
    expect(
      Number((screen.getByLabelText("organization.defaultQuota") as HTMLInputElement).value),
    ).toBe(25);
    expect(screen.getByRole("switch")).toHaveAttribute("aria-checked", "true");
  });

  it("submits the edited balance as a number under the expected field names", async () => {
    const onFinish = vi.fn<(values: Record<string, unknown>) => void>();
    const user = userEvent.setup();
    render(<Harness onFinish={onFinish} />, { wrapper: createTestWrapper() });

    const balance = screen.getByLabelText("organization.balance") as HTMLInputElement;
    await user.clear(balance);
    await user.type(balance, "250.75");

    submitForm();

    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
    const values = onFinish.mock.calls[0]![0];
    expect(values).toMatchObject({
      balance: 250.75,
      currency: "USD",
      enableDeposit: true,
      balanceCredit: 20,
      mu: 1.5,
      defaultQuota: 25,
      defaultCurrency: "USD",
    });
    expect(typeof values.balance).toBe("number");
    expect(typeof values.defaultQuota).toBe("number");
  });
});
