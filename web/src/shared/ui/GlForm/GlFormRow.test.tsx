import { useEffect } from "react";
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Form, Input } from "antd";
import type { FormInstance } from "antd";
import { GlFormRow } from "./GlFormRow";

/**
 * GlFormRow 是 Form.Item 的直接子元素，受控属性由 antd 通过 cloneElement
 * 注入。这里用真实 Form + Input 覆盖最容易回归的两点：
 * 1. 用户输入必须写回 Form 实例（双向绑定）；
 * 2. label 必须通过 id 关联到内部控件（可访问名 + 自动化定位）。
 */
describe("GlFormRow", () => {
  it("把 Form.Item 注入的 value/onChange 透传给内部控件", async () => {
    const user = userEvent.setup();
    let formInstance: FormInstance<{ slug: string }> | undefined;

    function DemoForm() {
      const [form] = Form.useForm<{ slug: string }>();
      useEffect(() => {
        formInstance = form;
      }, [form]);
      return (
        <Form form={form}>
          <Form.Item name="slug" rules={[{ required: true }]}>
            <GlFormRow label="Slug" required>
              <Input placeholder="slug" />
            </GlFormRow>
          </Form.Item>
        </Form>
      );
    }

    render(<DemoForm />);
    await user.type(screen.getByPlaceholderText("slug"), "e2e-org");

    expect(formInstance?.getFieldValue("slug")).toBe("e2e-org");
  });

  it("把 Form.Item 注入的 id 透传给内部控件并与 label 关联", () => {
    function DemoForm() {
      const [form] = Form.useForm<{ slug: string }>();
      return (
        <Form form={form}>
          <Form.Item name="slug">
            <GlFormRow label="Slug">
              <Input placeholder="slug" />
            </GlFormRow>
          </Form.Item>
        </Form>
      );
    }

    render(<DemoForm />);
    const input = screen.getByPlaceholderText("slug");
    expect(input.id).toBe("slug");
    expect(input.closest("label")).toBeNull();
    expect(screen.getByLabelText("Slug")).toBe(input);
  });
});
