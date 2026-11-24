import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { Form, Input } from "antd";
import { useUiConfigHistory } from "./useUiConfigHistory";

const FIELDS = ["formCss", "footerHtml"] as const;

function Harness({ entityKey = "app-1", onApply }: { entityKey?: string; onApply?: () => void }) {
  const [form] = Form.useForm();
  const history = useUiConfigHistory(form, FIELDS, {
    entityKey,
    debounceMs: 10,
    onApply,
  });
  return (
    <Form form={form} initialValues={{ formCss: "base", footerHtml: "" }}>
      <Form.Item name="formCss">
        <Input data-testid="css" />
      </Form.Item>
      <button data-testid="undo" disabled={!history.canUndo} onClick={history.undo}>
        undo
      </button>
      <button data-testid="redo" disabled={!history.canRedo} onClick={history.redo}>
        redo
      </button>
    </Form>
  );
}

describe("useUiConfigHistory", () => {
  it("captures edits and undoes/redoes them", async () => {
    render(<Harness />);
    const input = screen.getByTestId("css");
    expect(screen.getByTestId("undo")).toBeDisabled();

    fireEvent.change(input, { target: { value: "edited" } });
    await waitFor(() => expect(screen.getByTestId("undo")).not.toBeDisabled());

    fireEvent.click(screen.getByTestId("undo"));
    await waitFor(() => expect(input).toHaveValue("base"));
    expect(screen.getByTestId("redo")).not.toBeDisabled();

    fireEvent.click(screen.getByTestId("redo"));
    await waitFor(() => expect(input).toHaveValue("edited"));
  });

  it("calls onApply after programmatic undo so dirty checks re-run", async () => {
    const onApply = vi.fn<() => void>();
    render(<Harness onApply={onApply} />);
    fireEvent.change(screen.getByTestId("css"), { target: { value: "x" } });
    await waitFor(() => expect(screen.getByTestId("undo")).not.toBeDisabled());
    fireEvent.click(screen.getByTestId("undo"));
    expect(onApply).toHaveBeenCalled();
  });

  it("clears history when the entity key changes", async () => {
    const { rerender } = render(<Harness entityKey="app-1" />);
    fireEvent.change(screen.getByTestId("css"), { target: { value: "x" } });
    await waitFor(() => expect(screen.getByTestId("undo")).not.toBeDisabled());

    rerender(<Harness entityKey="app-2" />);
    await waitFor(() => expect(screen.getByTestId("undo")).toBeDisabled());
  });

  it("does not record undo/redo writes as new history entries", async () => {
    render(<Harness />);
    const input = screen.getByTestId("css");
    fireEvent.change(input, { target: { value: "one" } });
    await waitFor(() => expect(screen.getByTestId("undo")).not.toBeDisabled());
    fireEvent.click(screen.getByTestId("undo"));
    await waitFor(() => expect(input).toHaveValue("base"));
    // undo 写回不得入栈：redo 仍可用且 undo 已到底
    expect(screen.getByTestId("undo")).toBeDisabled();
    expect(screen.getByTestId("redo")).not.toBeDisabled();
  });
});
