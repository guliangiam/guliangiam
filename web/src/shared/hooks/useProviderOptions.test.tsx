import { renderHook, waitFor } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { createTestQueryClient } from "@/test-utils/render";
import { useProviderOptions } from "./useProviderOptions";

const { mockGetProviders } = vi.hoisted(() => ({
  mockGetProviders: vi.fn<(organizationId?: string) => Promise<unknown>>(),
}));

vi.mock("@/shared/api/iam/provider-extended", () => ({
  getProviders: (organizationId?: string) => mockGetProviders(organizationId),
}));

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={createTestQueryClient()}>{children}</QueryClientProvider>;
}

describe("useProviderOptions", () => {
  it("scopes the provider query to the requested organization", async () => {
    mockGetProviders.mockResolvedValue([]);

    const { result } = renderHook(() => useProviderOptions("org-1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockGetProviders).toHaveBeenCalledWith("org-1");
  });

  it("does not fetch when disabled", () => {
    mockGetProviders.mockClear();
    renderHook(() => useProviderOptions("org-2", { enabled: false }), { wrapper });

    expect(mockGetProviders).not.toHaveBeenCalled();
  });
});
