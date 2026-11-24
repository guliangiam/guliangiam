import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Form, Input } from "antd";
import { HelpTip } from "./FieldHelp";
import { GlFormRow } from "./GlForm/GlFormRow";

/**
 * 帮助提示的行为契约：
 * 1. 图标可被辅助技术识别（role=img + aria-label），并可键盘聚焦；
 * 2. 悬停后展示传入的说明文本；
 * 3. GlFormRow 显式 help 时立即渲染图标并标记 data-gl-help-done，
 *    避免全局注入器重复添加。
 */
describe("HelpTip", () => {
  it("悬停后展示说明文本，并暴露可访问名称", async () => {
    const user = userEvent.setup();
    render(<HelpTip text="字段说明" ariaLabel="查看字段说明" />);

    const icon = screen.getByRole("img", { name: "查看字段说明" });
    await user.hover(icon);

    expect(await screen.findByText("字段说明")).toBeInTheDocument();
  });

  it("可通过键盘聚焦触发展示", async () => {
    const user = userEvent.setup();
    render(<HelpTip text="键盘说明" ariaLabel="查看键盘说明" />);

    await user.tab();
    expect(await screen.findByText("键盘说明")).toBeInTheDocument();
  });
});

describe("GlFormRow help", () => {
  it("显式 help 时渲染问号图标并标记已处理", () => {
    render(
      <Form>
        <Form.Item name="email">
          <GlFormRow label="邮箱" help="用于登录与通知">
            <Input />
          </GlFormRow>
        </Form.Item>
      </Form>,
    );

    const label = screen.getByText("邮箱").closest("label");
    expect(label).not.toBeNull();
    expect(label).toHaveAttribute("data-gl-help-done", "1");
    expect(label!.querySelector(".gl-help-tip")).not.toBeNull();
  });
});
