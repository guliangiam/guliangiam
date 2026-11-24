import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StickyFormFooter } from "./StickyFormFooter";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: { defaultValue?: string }) => opts?.defaultValue ?? key,
  }),
}));

describe("StickyFormFooter", () => {
  it("renders save button with default text", () => {
    const onSave = vi.fn<(...args: unknown[]) => unknown>();
    render(<StickyFormFooter onSave={onSave} />);
    expect(screen.getByRole("button", { name: /save/i })).toBeInTheDocument();
  });

  it("renders reset button with default text", () => {
    const onReset = vi.fn<(...args: unknown[]) => unknown>();
    render(<StickyFormFooter onReset={onReset} />);
    expect(screen.getByRole("button", { name: /reset/i })).toBeInTheDocument();
  });

  it("renders custom save/reset text", () => {
    render(
      <StickyFormFooter
        onSave={vi.fn<(...args: unknown[]) => unknown>()}
        onReset={vi.fn<(...args: unknown[]) => unknown>()}
        saveText="Submit"
        resetText="Cancel"
      />,
    );
    expect(screen.getByText("Submit")).toBeInTheDocument();
    expect(screen.getByText("Cancel")).toBeInTheDocument();
  });

  it("does not render reset when onReset is not provided", () => {
    render(<StickyFormFooter onSave={vi.fn<(...args: unknown[]) => unknown>()} />);
    expect(screen.queryByRole("button", { name: /reset/i })).not.toBeInTheDocument();
  });

  it("calls onSave on click", async () => {
    const onSave = vi.fn<(...args: unknown[]) => unknown>();
    const user = userEvent.setup();
    render(<StickyFormFooter onSave={onSave} />);
    await user.click(screen.getByRole("button", { name: /save/i }));
    expect(onSave).toHaveBeenCalledOnce();
  });

  it("calls onReset on click", async () => {
    const onReset = vi.fn<(...args: unknown[]) => unknown>();
    const user = userEvent.setup();
    render(<StickyFormFooter onReset={onReset} />);
    await user.click(screen.getByRole("button", { name: /reset/i }));
    expect(onReset).toHaveBeenCalledOnce();
  });

  it("disables save button when loading", () => {
    render(<StickyFormFooter onSave={vi.fn<(...args: unknown[]) => unknown>()} loading />);
    const saveBtn = screen.getByRole("button", { name: /save/i });
    expect(saveBtn).toBeDisabled();
  });

  it("renders extra content", () => {
    render(
      <StickyFormFooter
        onSave={vi.fn<(...args: unknown[]) => unknown>()}
        extra={<span data-testid="extra">Extra</span>}
      />,
    );
    expect(screen.getByTestId("extra")).toBeInTheDocument();
  });
});
