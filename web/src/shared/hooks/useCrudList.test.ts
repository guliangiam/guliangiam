import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement, type ReactNode } from "react";
import { HttpError } from "@/shared/api/http";
import { useCrudList } from "./useCrudList";

const messageError = vi.hoisted(() => vi.fn<(...args: unknown[]) => unknown>());

vi.mock("antd", () => ({
  App: { useApp: () => ({ message: { error: messageError } }) },
  message: { error: messageError },
}));

vi.mock("i18next", () => ({
  t: (key: string) => key,
}));

afterEach(() => {
  messageError.mockReset();
});

function makeWrapper() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
}

const BASE_CONFIG = {
  buildQueryKey: () => ["test", "list"] as const,
  buildParams: () => ({}),
  listFn: async () => [] as Array<{ id: string }>,
  enabled: false,
};

function renderList<TDeleteKey = string>(config: Record<string, unknown>) {
  return renderHook(() => useCrudList<{ id: string }, unknown, TDeleteKey>(config as never), {
    wrapper: makeWrapper(),
  });
}

describe("useCrudList bulk delete", () => {
  it("reports hasBulkDelete=false when no delete function is configured", () => {
    const { result } = renderList(BASE_CONFIG);
    expect(result.current.hasBulkDelete).toBe(false);
  });

  it("reports hasBulkDelete=true when deleteFn is configured", () => {
    const { result } = renderList({ ...BASE_CONFIG, deleteFn: async () => undefined });
    expect(result.current.hasBulkDelete).toBe(true);
  });

  it("aggregates per-row failures instead of aborting the whole batch", async () => {
    const deleteFn = vi.fn<(key: string) => Promise<unknown>>(async (key: string) => {
      if (key === "b") throw new Error("conflict");
      return undefined;
    });
    const { result } = renderList({ ...BASE_CONFIG, deleteFn });

    act(() => result.current.setSelectedKeys(["a", "b", "c"]));

    let summary: unknown;
    await act(async () => {
      summary = await result.current.bulkDeleteMutation.mutateAsync(["a", "b", "c"]);
    });

    expect(deleteFn).toHaveBeenCalledTimes(3);
    expect(summary).toEqual({ total: 3, succeeded: 2, failed: 1, failedKeys: ["b"] });
    // 失败行保留选中，供用户直接重试
    await waitFor(() => expect(result.current.selectedKeys).toEqual(["b"]));
  });

  it("keeps all rows when every delete succeeds and clears the selection", async () => {
    const deleteFn = vi.fn<(key: string) => Promise<unknown>>(async () => undefined);
    const { result } = renderList({ ...BASE_CONFIG, deleteFn });

    act(() => result.current.setSelectedKeys(["a", "b"]));

    await act(async () => {
      await result.current.bulkDeleteMutation.mutateAsync(["a", "b"]);
    });

    await waitFor(() => expect(result.current.selectedKeys).toEqual([]));
  });

  it("caps concurrent deletes by processing selections in batches", async () => {
    let inFlight = 0;
    let peak = 0;
    const deleteFn = vi.fn<(key: string) => Promise<unknown>>(async () => {
      inFlight += 1;
      peak = Math.max(peak, inFlight);
      await new Promise((resolve) => setTimeout(resolve, 1));
      inFlight -= 1;
      return undefined;
    });
    const keys = Array.from({ length: 12 }, (_, index) => `k${index}`);
    const { result } = renderList({ ...BASE_CONFIG, deleteFn });

    await act(async () => {
      await result.current.bulkDeleteMutation.mutateAsync(keys);
    });

    expect(deleteFn).toHaveBeenCalledTimes(12);
    expect(peak).toBeLessThanOrEqual(10);
  });

  it("clears the selection when the page or search changes", () => {
    const { result } = renderList({ ...BASE_CONFIG, deleteFn: async () => undefined });

    act(() => result.current.setSelectedKeys(["a", "b"]));
    act(() => result.current.serverPagination.onChange(2, 10));
    expect(result.current.selectedKeys).toEqual([]);

    act(() => result.current.setSelectedKeys(["c"]));
    act(() => result.current.onSearch("alice"));
    expect(result.current.selectedKeys).toEqual([]);
  });
});

describe("useCrudList delete errors", () => {
  it("maps a failing delete to a localized status message instead of backend prose", async () => {
    const deleteFn = vi.fn<(key: string) => Promise<unknown>>(async () => {
      throw new HttpError(503, undefined, "backend prose");
    });
    const { result } = renderList({ ...BASE_CONFIG, deleteFn });

    await act(async () => {
      await result.current.deleteMutation.mutateAsync("a").catch(() => undefined);
    });

    expect(messageError).toHaveBeenCalledTimes(1);
    expect(messageError).toHaveBeenCalledWith("common:serverError.unavailable");
  });
});

describe("useCrudList server pagination", () => {
  it("refetches with the next page when page changes", async () => {
    const paginatedFn = vi.fn<
      (params: { page: number; pageSize: number }) => Promise<{
        items: Array<{ id: string }>;
        total: number;
      }>
    >(async () => ({ items: [], total: 41 }));

    const { result } = renderList({
      buildQueryKey: (params: { page: number; pageSize: number }) =>
        ["test", "list", params.page, params.pageSize] as const,
      buildParams: ({ page, pageSize }: { page: number; pageSize: number }) => ({ page, pageSize }),
      paginatedFn,
      initialPageSize: 20,
      enabled: true,
    });

    await waitFor(() => expect(paginatedFn).toHaveBeenCalledWith({ page: 1, pageSize: 20 }));

    act(() => result.current.serverPagination.onChange(2, 20));

    await waitFor(() => expect(paginatedFn).toHaveBeenCalledWith({ page: 2, pageSize: 20 }));
  });
});

describe("useCrudList search debounce", () => {
  it("does not refetch on every keystroke in server mode", async () => {
    const paginatedFn = vi.fn<
      (params: { search?: string }) => Promise<{ items: Array<{ id: string }>; total: number }>
    >(async () => ({ items: [], total: 0 }));
    const { result } = renderList({
      buildQueryKey: (params: { search?: string }) => ["test", "list", params.search ?? ""],
      buildParams: ({ search }: { search?: string }) => ({ search }),
      paginatedFn,
      enabled: true,
    });

    await waitFor(() => expect(paginatedFn).toHaveBeenCalledTimes(1));

    act(() => result.current.onSearch("ali"));
    act(() => result.current.onSearch("alice"));

    await new Promise((resolve) => setTimeout(resolve, 100));
    expect(paginatedFn).toHaveBeenCalledTimes(1);

    await waitFor(() => expect(paginatedFn).toHaveBeenCalledTimes(2), { timeout: 2000 });
  });
});
