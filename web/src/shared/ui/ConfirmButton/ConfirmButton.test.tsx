import { afterEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConfirmButton } from "./ConfirmButton";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

afterEach(() => {
  vi.useRealTimers();
});

describe("ConfirmButton", () => {
  it("requires a second click before invoking the handler", async () => {
    const onConfirm = vi.fn<() => void>();
    const user = userEvent.setup();
    render(<ConfirmButton onConfirm={onConfirm}>Delete user</ConfirmButton>);

    await user.click(screen.getByRole("button", { name: "Delete user" }));
    expect(screen.getByRole("button", { name: "common:confirm" })).toBeInTheDocument();
    expect(onConfirm).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "common:confirm" }));
    expect(onConfirm).toHaveBeenCalledOnce();
    await waitFor(() => expect(screen.getByRole("button")).toBeEnabled());
    expect(screen.getByRole("button")).toHaveTextContent("Delete user");
  });

  it("confirms through native keyboard activation when the confirm button is focused", async () => {
    const onConfirm = vi.fn<() => void>();
    const user = userEvent.setup();
    render(<ConfirmButton onConfirm={onConfirm}>Delete user</ConfirmButton>);

    await user.click(screen.getByRole("button", { name: "Delete user" }));
    screen.getByRole("button", { name: "common:confirm" }).focus();
    await user.keyboard("{Enter}");

    expect(onConfirm).toHaveBeenCalledOnce();
  });

  it("cancels confirmation with Escape", async () => {
    const user = userEvent.setup();
    render(<ConfirmButton onConfirm={vi.fn<() => void>()}>Delete user</ConfirmButton>);

    await user.click(screen.getByRole("button", { name: "Delete user" }));
    await user.keyboard("{Escape}");

    expect(screen.getByRole("button", { name: "Delete user" })).toBeInTheDocument();
  });

  it("cancels confirmation after an outside click", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn<() => void>();
    render(
      <>
        <ConfirmButton onConfirm={onConfirm}>Delete user</ConfirmButton>
        <button type="button">Other action</button>
      </>,
    );

    await user.click(screen.getByRole("button", { name: "Delete user" }));
    screen.getByRole("button", { name: "Other action" }).focus();
    await user.keyboard("{Enter}");
    expect(onConfirm).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "common:confirm" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Other action" }));

    expect(screen.getByRole("button", { name: "Delete user" })).toBeInTheDocument();
  });

  it("automatically returns to the primary state after the timeout", async () => {
    vi.useFakeTimers();
    render(
      <ConfirmButton onConfirm={vi.fn<() => void>()} timeout={1000}>
        Delete user
      </ConfirmButton>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Delete user" }));
    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(screen.getByRole("button")).toHaveTextContent("Delete user");
  });

  it("prevents a second handler call while an async confirmation is pending", async () => {
    let resolveConfirmation!: () => void;
    const onConfirm = vi.fn<() => Promise<void>>(
      () =>
        new Promise<void>((resolve) => {
          resolveConfirmation = resolve;
        }),
    );
    const user = userEvent.setup();
    render(<ConfirmButton onConfirm={onConfirm}>Delete user</ConfirmButton>);

    await user.click(screen.getByRole("button", { name: "Delete user" }));
    await user.click(screen.getByRole("button", { name: "common:confirm" }));
    expect(onConfirm).toHaveBeenCalledOnce();
    expect(screen.getByRole("button")).toBeDisabled();

    resolveConfirmation();
    await waitFor(() => expect(screen.getByRole("button")).toBeEnabled());
    expect(screen.getByRole("button")).toHaveTextContent("Delete user");
  });
});
