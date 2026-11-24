import { describe, it, expect, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AsyncSearchSelect } from "./AsyncSearchSelect";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

type Option = { label: string; value: string };

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

describe("AsyncSearchSelect", () => {
  it("renders with placeholder", () => {
    const loader = vi.fn<(...args: unknown[]) => Promise<unknown>>().mockResolvedValue([]);
    render(<AsyncSearchSelect placeholder="Select item" loader={loader} />);
    expect(screen.getByText("Select item")).toBeInTheDocument();
  });

  it("does not leak the raw value when the loader has not resolved a label", () => {
    // The component deliberately renders an empty label for a value it cannot
    // name yet (see normalizeValueOptions) so that IDs/UUIDs never reach the UI.
    const loader = vi.fn<(...args: unknown[]) => Promise<unknown>>().mockResolvedValue([]);
    render(<AsyncSearchSelect value="current-val" loader={loader} />);
    expect(screen.queryByText("current-val")).not.toBeInTheDocument();
  });

  it("shows the resolved label for the current value", async () => {
    const loader = vi
      .fn<(...args: unknown[]) => Promise<unknown>>()
      .mockResolvedValue([{ label: "Current item", value: "current-val" }]);
    render(<AsyncSearchSelect value="current-val" loader={loader} />);
    await waitFor(() => expect(screen.getByText("Current item")).toBeInTheDocument());
  });

  it("calls loader after search input", async () => {
    const loader = vi
      .fn<(...args: unknown[]) => Promise<unknown>>()
      .mockResolvedValue([{ label: "Result", value: "r" }]);
    const user = userEvent.setup();
    render(<AsyncSearchSelect loader={loader} placeholder="Search..." />);

    const input = screen.getByRole("combobox");
    await user.type(input, "test");

    await waitFor(() => expect(loader).toHaveBeenCalled(), { timeout: 500 });
  });

  it("shows spinner while loading", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const loader = vi
        .fn<(...args: unknown[]) => unknown>()
        .mockImplementation(() => new Promise((resolve) => setTimeout(() => resolve([]), 1000)));
      const user = userEvent.setup({ delay: null });
      render(<AsyncSearchSelect loader={loader} placeholder="Search..." />);

      const input = screen.getByRole("combobox");
      await user.type(input, "x");
      await act(async () => {
        vi.advanceTimersByTime(300);
      });

      await waitFor(() => {
        expect(loader).toHaveBeenCalled();
        expect(document.querySelector(".ant-skeleton")).toBeInTheDocument();
      });
      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
    } finally {
      vi.useRealTimers();
    }
  });

  it("ignores a stale response after a newer search resolves", async () => {
    const initial = deferred<Option[]>();
    const fresh = deferred<Option[]>();
    const loader = vi.fn<(search: string) => Promise<Option[]>>((search: string) =>
      search === "fresh" ? fresh.promise : initial.promise,
    );
    const user = userEvent.setup();
    render(<AsyncSearchSelect loader={loader} placeholder="Search..." />);

    const input = screen.getByRole("combobox");
    await user.click(input);
    await waitFor(() => expect(loader).toHaveBeenCalledWith(""));
    await user.type(input, "fresh");
    await waitFor(() => expect(loader).toHaveBeenCalledWith("fresh"));

    await act(async () => {
      fresh.resolve([{ label: "Fresh result", value: "fresh" }]);
      await fresh.promise;
    });
    expect(await screen.findByText("Fresh result")).toBeInTheDocument();

    await act(async () => {
      initial.resolve([{ label: "Stale result", value: "stale" }]);
      await initial.promise;
    });
    expect(screen.queryByText("Stale result")).not.toBeInTheDocument();
    expect(screen.getByText("Fresh result")).toBeInTheDocument();
  });

  it("recovers from a failed load with an explicit retry", async () => {
    const loader = vi
      .fn<(search: string) => Promise<Option[]>>()
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValue([{ label: "Recovered result", value: "recovered" }]);
    const user = userEvent.setup();
    render(<AsyncSearchSelect loader={loader} placeholder="Search..." value="current-id" />);

    await user.click(screen.getByRole("combobox"));
    await user.click(await screen.findByRole("button", { name: "retry" }));

    expect(await screen.findByText("Recovered result")).toBeInTheDocument();
    expect(loader).toHaveBeenCalledTimes(2);
  });

  it("renders in multiple mode", () => {
    const loader = vi.fn<(...args: unknown[]) => Promise<unknown>>().mockResolvedValue([]);
    render(<AsyncSearchSelect loader={loader} mode="multiple" placeholder="Pick items" />);
    expect(screen.getByText("Pick items")).toBeInTheDocument();
  });
});
