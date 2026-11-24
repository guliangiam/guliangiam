import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { idvPendingStorageKey, useIdvPendingPolling } from "./useIdvPendingPolling";

const getIDVVerification = vi.hoisted(() => vi.fn<(...args: unknown[]) => Promise<unknown>>());

vi.mock("@/shared/api/iam/idv", () => ({ getIDVVerification }));

function setup() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { queryClient, invalidateSpy, wrapper };
}

describe("useIdvPendingPolling", () => {
  beforeEach(() => {
    localStorage.clear();
    getIDVVerification.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("does nothing without a pending verification", async () => {
    vi.useFakeTimers();
    const { wrapper } = setup();
    renderHook(() => useIdvPendingPolling({ userId: "u-1", organizationId: "org-1" }), { wrapper });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(11_000);
    });
    expect(getIDVVerification).not.toHaveBeenCalled();
  });

  it("restores the marker left behind before the hosted page redirect", async () => {
    localStorage.setItem(idvPendingStorageKey("u-1"), "v-1");
    getIDVVerification.mockResolvedValue({ status: "pending" });
    const { wrapper } = setup();

    renderHook(() => useIdvPendingPolling({ userId: "u-1", organizationId: "org-1" }), { wrapper });

    await waitFor(() => expect(getIDVVerification).toHaveBeenCalledWith("v-1", "org-1"));
  });

  it("also polls a verification started without leaving the page", async () => {
    // 管理端弹二维码时不发生跳转，标记要等页面重新挂载才读得到：
    // 必须由 verificationId 直接驱动，否则结果永远不会刷回页面。
    getIDVVerification.mockResolvedValue({ status: "pending" });
    const { wrapper } = setup();

    renderHook(
      () => useIdvPendingPolling({ userId: "u-1", organizationId: "org-1", verificationId: "v-2" }),
      { wrapper },
    );

    await waitFor(() => expect(getIDVVerification).toHaveBeenCalledWith("v-2", "org-1"));
  });

  it("clears the marker, refreshes the user cache and stops on a terminal status", async () => {
    vi.useFakeTimers();
    localStorage.setItem(idvPendingStorageKey("u-1"), "v-3");
    getIDVVerification.mockResolvedValue({ status: "approved" });
    const { invalidateSpy, wrapper } = setup();
    const onSettled = vi.fn<(status: string) => void>();

    renderHook(() => useIdvPendingPolling({ userId: "u-1", organizationId: "org-1", onSettled }), {
      wrapper,
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(localStorage.getItem(idvPendingStorageKey("u-1"))).toBeNull();
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["user"] });
    expect(onSettled).toHaveBeenCalledWith("approved");

    // 终态后必须自停：否则页面每 5 秒一直问到卸载为止。
    await act(async () => {
      await vi.advanceTimersByTimeAsync(20_000);
    });
    expect(getIDVVerification).toHaveBeenCalledTimes(1);
  });

  it("keeps polling while the provider is still processing", async () => {
    vi.useFakeTimers();
    getIDVVerification.mockResolvedValue({ status: "pending" });
    const { wrapper } = setup();

    renderHook(
      () => useIdvPendingPolling({ userId: "u-1", organizationId: "org-1", verificationId: "v-4" }),
      { wrapper },
    );

    await act(async () => {
      await vi.advanceTimersByTimeAsync(11_000);
    });
    // 立即一次 + 5s + 10s
    expect(getIDVVerification).toHaveBeenCalledTimes(3);
  });

  it("survives a transient query failure without clearing the marker", async () => {
    vi.useFakeTimers();
    localStorage.setItem(idvPendingStorageKey("u-1"), "v-5");
    getIDVVerification.mockRejectedValueOnce(new Error("network"));
    getIDVVerification.mockResolvedValue({ status: "rejected" });
    const { invalidateSpy, wrapper } = setup();
    const onSettled = vi.fn<(status: string) => void>();

    renderHook(() => useIdvPendingPolling({ userId: "u-1", organizationId: "org-1", onSettled }), {
      wrapper,
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(5_100);
    });
    expect(onSettled).toHaveBeenCalledWith("rejected");
    expect(invalidateSpy).toHaveBeenCalled();
    expect(localStorage.getItem(idvPendingStorageKey("u-1"))).toBeNull();
  });
});
