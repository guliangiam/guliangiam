import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Form } from "antd";
import type { FormInstance } from "antd";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createTestWrapper } from "@/test-utils/render";
import CustomScopesTab from "./CustomScopesTab";
import type { ApplicationConfigFormValues } from "./types";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

type CustomScope = NonNullable<ApplicationConfigFormValues["customScopes"]>[number];

function renderTab(initialValues: Partial<ApplicationConfigFormValues> = {}) {
  const onFinish = vi.fn<(values: ApplicationConfigFormValues) => void>();
  const handle: { form: FormInstance<ApplicationConfigFormValues> | null } = { form: null };

  function Harness() {
    const [form] = Form.useForm<ApplicationConfigFormValues>();
    handle.form = form;

    return (
      <Form form={form} initialValues={initialValues} onFinish={onFinish}>
        <CustomScopesTab form={form} />
      </Form>
    );
  }

  render(<Harness />, { wrapper: createTestWrapper() });

  return { form: handle.form as FormInstance<ApplicationConfigFormValues>, onFinish };
}

function scopesOf(form: FormInstance<ApplicationConfigFormValues>): CustomScope[] {
  return (form.getFieldValue("customScopes") as CustomScope[] | undefined) ?? [];
}

/**
 * Scope 列是 AutoComplete（基于 Select），占位符渲染为 span 而非 input 属性，
 * 因此用 combobox 角色定位输入框。
 */
function scopeInputAt(row: number): HTMLElement {
  return screen.getAllByRole("combobox")[row];
}

async function fillRow(
  row: number,
  values: { scope: string; displayName: string; description: string },
) {
  const user = userEvent.setup();
  await user.type(scopeInputAt(row), values.scope);
  await user.type(screen.getAllByPlaceholderText("e.g. Read Reports")[row], values.displayName);
  await user.type(screen.getAllByPlaceholderText("scopeDescription")[row], values.description);
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("CustomScopesTab", () => {
  it("appends a row with empty editable fields when Add is clicked", async () => {
    const { form } = renderTab();
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "application.customScopesSection.add" }));
    await user.click(screen.getByRole("button", { name: "application.customScopesSection.add" }));

    expect(scopesOf(form)).toEqual([
      { scope: "", displayName: "", description: "" },
      { scope: "", displayName: "", description: "" },
    ]);
  });

  it("writes edited cells back into the customScopes array", async () => {
    const { form } = renderTab();
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "application.customScopesSection.add" }));
    await fillRow(0, {
      scope: "read:reports",
      displayName: "Read Reports",
      description: "View reports",
    });

    expect(scopesOf(form)).toEqual([
      { scope: "read:reports", displayName: "Read Reports", description: "View reports" },
    ]);
  });

  it("fills display name and description when a preset scope is selected", async () => {
    const { form } = renderTab();
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "application.customScopesSection.add" }));
    await user.type(scopeInputAt(0), "openid");
    // 失焦时按精确匹配补全预置项的显示名称与描述。
    await user.tab();

    expect(scopesOf(form)).toEqual([
      {
        scope: "openid",
        displayName: "application.customScopePresets.openid.displayName",
        description: "application.customScopePresets.openid.description",
      },
    ]);
  });

  it("removes only the deleted row and keeps the remaining values", async () => {
    const { form } = renderTab({
      customScopes: [
        { scope: "read:reports", displayName: "Read Reports", description: "" },
        { scope: "write:reports", displayName: "Write Reports", description: "" },
      ],
    });
    const user = userEvent.setup();

    expect(scopesOf(form)).toHaveLength(2);

    await user.click(screen.getAllByRole("button", { name: "delete" })[0]);
    await user.click(await screen.findByRole("button", { name: "OK" }));

    await waitFor(() => expect(scopesOf(form)).toHaveLength(1));
    expect(scopesOf(form)).toEqual([
      { scope: "write:reports", displayName: "Write Reports", description: "" },
    ]);
  });
});
