import { describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PaginateSelect } from "./PaginateSelect";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

type Option = { label: string; value: string };
type Page = { options: Option[]; hasMore: boolean };

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function scrollPopupToBottom() {
  const holder = document.querySelector(".ant-select-dropdown-list-holder");
  if (!holder) throw new Error("Select popup scroll holder not found");
  Object.defineProperties(holder, {
    scrollTop: { configurable: true, value: 0, writable: true },
    scrollHeight: { configurable: true, value: 100 },
    clientHeight: { configurable: true, value: 100 },
  });
  fireEvent.scroll(holder);
}

describe("PaginateSelect", () => {
  it("ignores a stale first-page response after a newer search resolves", async () => {
    const initial = deferred<Page>();
    const fresh = deferred<Page>();
    const loader = vi.fn<(search: string, page: number) => Promise<Page>>(
      (search: string, page: number) => {
        if (page > 1) return Promise.resolve({ options: [], hasMore: false });
        return search === "fresh" ? fresh.promise : initial.promise;
      },
    );
    const user = userEvent.setup();
    render(<PaginateSelect loader={loader} placeholder="Search..." />);

    const input = screen.getByRole("combobox");
    await user.click(input);
    await waitFor(() => expect(loader).toHaveBeenCalledWith("", 1));
    await user.type(input, "fresh");
    await waitFor(() => expect(loader).toHaveBeenCalledWith("fresh", 1));

    await act(async () => {
      fresh.resolve({ options: [{ label: "Fresh result", value: "fresh" }], hasMore: false });
      await fresh.promise;
    });
    expect(await screen.findByText("Fresh result")).toBeInTheDocument();

    await act(async () => {
      initial.resolve({ options: [{ label: "Stale result", value: "stale" }], hasMore: true });
      await initial.promise;
    });
    expect(screen.queryByText("Stale result")).not.toBeInTheDocument();
    expect(screen.getByText("Fresh result")).toBeInTheDocument();
  });

  it("loads more once, appends options, and stops at the last page", async () => {
    const nextPage = deferred<Page>();
    const loader = vi.fn<(search: string, page: number) => Promise<Page>>(
      (_search: string, page: number) =>
        page === 1
          ? Promise.resolve({ options: [{ label: "First", value: "first" }], hasMore: true })
          : nextPage.promise,
    );
    const user = userEvent.setup();
    render(<PaginateSelect loader={loader} />);

    await user.click(screen.getByRole("combobox"));
    expect(await screen.findByText("First")).toBeInTheDocument();
    scrollPopupToBottom();
    scrollPopupToBottom();
    await waitFor(() => expect(loader).toHaveBeenCalledWith("", 2));
    expect(loader).toHaveBeenCalledTimes(2);

    await act(async () => {
      nextPage.resolve({ options: [{ label: "Second", value: "second" }], hasMore: false });
      await nextPage.promise;
    });
    expect(await screen.findByText("Second")).toBeInTheDocument();
    expect(screen.getByText("First")).toBeInTheDocument();

    scrollPopupToBottom();
    expect(loader).toHaveBeenCalledTimes(2);
  });

  it("does not append a stale load-more response after the search changes", async () => {
    const staleNextPage = deferred<Page>();
    const loader = vi.fn<(search: string, page: number) => Promise<Page>>(
      (search: string, page: number) => {
        if (search === "fresh") {
          return Promise.resolve({
            options: [{ label: "Fresh result", value: "fresh" }],
            hasMore: false,
          });
        }
        if (page === 2) return staleNextPage.promise;
        return Promise.resolve({ options: [{ label: "First", value: "first" }], hasMore: true });
      },
    );
    const user = userEvent.setup();
    render(<PaginateSelect loader={loader} />);

    const input = screen.getByRole("combobox");
    await user.click(input);
    expect(await screen.findByText("First")).toBeInTheDocument();
    scrollPopupToBottom();
    await waitFor(() => expect(loader).toHaveBeenCalledWith("", 2));

    await user.type(input, "fresh");
    expect(await screen.findByText("Fresh result")).toBeInTheDocument();
    await act(async () => {
      staleNextPage.resolve({
        options: [{ label: "Stale next page", value: "stale-next" }],
        hasMore: false,
      });
      await staleNextPage.promise;
    });

    expect(screen.queryByText("Stale next page")).not.toBeInTheDocument();
    expect(screen.getByText("Fresh result")).toBeInTheDocument();
  });

  it("recovers from a failed initial load with an explicit retry", async () => {
    const loader = vi
      .fn<(search: string, page: number) => Promise<Page>>()
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValue({
        options: [{ label: "Recovered result", value: "recovered" }],
        hasMore: false,
      });
    const user = userEvent.setup();
    render(<PaginateSelect loader={loader} value={["current-id"]} />);

    await user.click(screen.getByRole("combobox"));
    await user.click(await screen.findByRole("button", { name: "retry" }));

    expect(await screen.findByText("Recovered result")).toBeInTheDocument();
    expect(loader).toHaveBeenCalledTimes(2);
  });

  it("retries a failed next page without losing loaded options", async () => {
    const failedNextPage = deferred<Page>();
    const loader = vi
      .fn<(search: string, page: number) => Promise<Page>>()
      .mockResolvedValueOnce({ options: [{ label: "First", value: "first" }], hasMore: true })
      .mockImplementationOnce(() => failedNextPage.promise)
      .mockResolvedValueOnce({ options: [{ label: "Second", value: "second" }], hasMore: false });
    const user = userEvent.setup();
    render(<PaginateSelect loader={loader} />);

    await user.click(screen.getByRole("combobox"));
    expect(await screen.findByText("First")).toBeInTheDocument();
    scrollPopupToBottom();
    await waitFor(() => expect(loader).toHaveBeenCalledWith("", 2));

    await act(async () => {
      failedNextPage.reject(new Error("offline"));
      await failedNextPage.promise.catch(() => undefined);
    });
    expect(screen.getByText("First")).toBeInTheDocument();

    scrollPopupToBottom();
    expect(await screen.findByText("Second")).toBeInTheDocument();
    expect(screen.getByText("First")).toBeInTheDocument();
    expect(loader).toHaveBeenCalledTimes(3);
  });

  it("does not expose a raw selected value before its label is available", () => {
    const loader = vi.fn<(search: string, page: number) => Promise<Page>>(
      () => new Promise<Page>(() => undefined),
    );
    render(<PaginateSelect loader={loader} value={["internal-id"]} />);

    expect(screen.queryByText("internal-id")).not.toBeInTheDocument();
  });
});
