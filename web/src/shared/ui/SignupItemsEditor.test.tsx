import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SignupItemsEditor, type SignupItemValue } from "./SignupItemsEditor";

describe("SignupItemsEditor", () => {
  it("keeps backend-only signup fields when editing a visible column", () => {
    const onChange = vi.fn<(value: SignupItemValue[]) => void>();
    const legacy: SignupItemValue = {
      name: "Email",
      visible: true,
      required: false,
      prompted: false,
      type: "Input",
      regex: "^.+@.+$",
      options: ["corporate"],
      label: "Email",
    };
    render(<SignupItemsEditor value={[legacy]} onChange={onChange} />);

    fireEvent.click(screen.getAllByRole("switch")[1]);

    expect(onChange).toHaveBeenCalledTimes(1);
    const next = onChange.mock.calls[0][0][0];
    expect(next.required).toBe(true);
    expect(next.type).toBe("Input");
    expect(next.regex).toBe("^.+@.+$");
    expect(next.options).toEqual(["corporate"]);
  });
});
