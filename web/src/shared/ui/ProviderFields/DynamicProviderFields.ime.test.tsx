import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { Form } from "antd";
import type { FormInstance } from "antd";
import { DynamicProviderFields } from "@/shared/ui/ProviderFields";

function Harness({ category, onForm }: { category: string; onForm: (form: FormInstance) => void }) {
  const [form] = Form.useForm();
  onForm(form);
  return (
    <Form form={form} layout="vertical">
      <DynamicProviderFields category={category} form={form} />
    </Form>
  );
}

// 回归：useWatch 的通知走 macroTask 异步批处理，受控 value 若只靠它回读会晚于
// DOM 一个任务，中文 IME 组合期间旧值被回写——拼音重复、闪跳、无法输入。
// 桥接层必须让 value 与 DOM 同帧一致（读活 store + onChange 同步重渲染）。
describe("DynamicProviderFields IME composition", () => {
  it("keeps every composition update in the SMS signName input", async () => {
    let form: FormInstance | undefined;
    render(<Harness category="SMS" onForm={(f) => (form = f)} />);
    const input = (await screen.findByPlaceholderText("Sign Name / Sender ID")) as HTMLInputElement;
    input.focus();

    fireEvent.compositionStart(input);
    fireEvent.change(input, { target: { value: "zhong" } });
    expect(input.value).toBe("zhong");

    // 关键断言：第二次组合更新不得被异步 watch 的旧值回写。
    fireEvent.change(input, { target: { value: "zhong'wen" } });
    expect(input.value).toBe("zhong'wen");

    fireEvent.change(input, { target: { value: "中文" } });
    fireEvent.compositionEnd(input);
    expect(input.value).toBe("中文");

    // 异步 watch 通知到达后也不得回退 DOM 值。
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    expect(input.value).toBe("中文");
    expect(form?.getFieldValue("sender")).toBe("中文");
  });

  it("keeps plain edits and store writes working for bridged OAuth fields", async () => {
    let form: FormInstance | undefined;
    render(<Harness category="OAuth" onForm={(f) => (form = f)} />);
    const input = (await screen.findByPlaceholderText("Client ID / App ID")) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "gl-client-1" } });
    await waitFor(() => expect(form?.getFieldValue("clientId")).toBe("gl-client-1"));
    expect(input.value).toBe("gl-client-1");
  });
});
