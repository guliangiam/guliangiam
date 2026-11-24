import { useEffect } from "react";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Form } from "antd";
import type { FormInstance } from "antd";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createTestWrapper } from "@/test-utils/render";
import { SecurityTab } from "./SecurityTab";
import type { ApplicationConfigFormValues } from "./types";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

function renderTab(initialValues: Partial<ApplicationConfigFormValues> = {}) {
  const onFinish = vi.fn<(values: ApplicationConfigFormValues) => void>();
  const handle: { form: FormInstance<ApplicationConfigFormValues> | null } = { form: null };

  function Harness() {
    const [form] = Form.useForm<ApplicationConfigFormValues>();
    handle.form = form;

    useEffect(() => {
      if (Object.keys(initialValues).length > 0) {
        form.setFieldsValue(initialValues);
      }
    }, [form]);

    return (
      <Form form={form} onFinish={onFinish}>
        <SecurityTab form={form} />
      </Form>
    );
  }

  render(<Harness />, { wrapper: createTestWrapper() });

  return { form: handle.form as FormInstance<ApplicationConfigFormValues>, onFinish };
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("SecurityTab echo", () => {
  it("prefills text, numeric and boolean controls from form values", () => {
    const { form } = renderTab({
      corsAllowedOrigins: "https://app1.com,https://app2.com",
      failedSigninLimit: 7,
      consentMode: "implicit",
      dpopRequired: true,
      enableBetaOidc: true,
    });

    expect(screen.getByPlaceholderText("https://app1.com, https://app2.com")).toHaveValue(
      "https://app1.com,https://app2.com",
    );
    expect(screen.getByPlaceholderText("5")).toHaveValue("7");
    expect(screen.getByText("application.consentModeOption.implicit")).toBeInTheDocument();
    expect(screen.getByRole("switch", { name: "application.dpopRequired" })).toBeChecked();
    expect(screen.getByRole("switch", { name: "application.enableBetaOidc" })).toBeChecked();

    expect(form.getFieldValue("corsAllowedOrigins")).toBe("https://app1.com,https://app2.com");
    expect(form.getFieldValue("failedSigninLimit")).toBe(7);
  });

  it("keeps CORS origins as the stored string and leaves unset switches unchecked", () => {
    const { form } = renderTab();

    const corsInput = screen.getByPlaceholderText("https://app1.com, https://app2.com");
    expect(corsInput).toHaveValue("");
    expect(screen.getByRole("switch", { name: "application.dpopRequired" })).not.toBeChecked();
    expect(form.getFieldValue("consentMode")).toBeUndefined();
  });
});

describe("SecurityTab submit", () => {
  it("submits the CORS string as-is and numeric limits as numbers", async () => {
    const { form, onFinish } = renderTab();
    const user = userEvent.setup();

    await user.type(
      screen.getByPlaceholderText("https://app1.com, https://app2.com"),
      "https://one.example, https://two.example",
    );
    await user.type(screen.getByPlaceholderText("5"), "9");

    act(() => form.submit());

    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
    const values = onFinish.mock.calls[0][0];

    // CORS origins are stored as one comma-separated string, not an array.
    expect(values.corsAllowedOrigins).toBe("https://one.example, https://two.example");
    expect(values.failedSigninLimit).toBe(9);
    expect(typeof values.failedSigninLimit).toBe("number");
  });

  it("keeps switch values as booleans in the submitted values", async () => {
    const { form, onFinish } = renderTab({
      dpopRequired: true,
      enableBetaOidc: false,
    });
    const user = userEvent.setup();

    await user.click(screen.getByRole("switch", { name: "application.enableBetaOidc" }));

    act(() => form.submit());

    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
    const values = onFinish.mock.calls[0][0];

    expect(values.dpopRequired).toBe(true);
    expect(values.enableBetaOidc).toBe(true);
  });
});
