import { describe, it, expect, vi, beforeEach, afterAll } from "vitest";
import { renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { useLoginTransitionFlow } from "./useLoginTransitionFlow";

const originalLocation = window.location;

function mockLocation(pathname: string, search: string) {
  const assign = vi.fn<() => void>();
  Object.defineProperty(window, "location", {
    value: { ...originalLocation, pathname, search, assign },
    writable: true,
    configurable: true,
  });
  return assign;
}

afterAll(() => {
  Object.defineProperty(window, "location", {
    value: originalLocation,
    writable: true,
    configurable: true,
  });
});

function setup() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  const navigate = vi.fn<(to: string, options?: { replace?: boolean }) => void>();
  const showSuccess = vi.fn<(content: string) => void>();
  const showError = vi.fn<(content: string) => void>();
  const showInfo = vi.fn<(content: string) => void>();
  const setPendingMfa = vi.fn<(value: unknown) => void>();
  const view = renderHook(
    () =>
      useLoginTransitionFlow({
        mfaSetupCompleted: false,
        pendingMfa: null,
        setPendingMfa,
        returnTo: "/dashboard",
        application: null,
        navigate,
        showSuccess,
        showError,
        showInfo,
      }),
    { wrapper },
  );
  return { view, navigate, showSuccess };
}

const successTransition = {
  kind: "success" as const,
  finishContext: { organization: "org-1", application: "app-1" },
};

describe("useLoginTransitionFlow OAuth authorize early-exit", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("redirects to the authorize endpoint and persists last login when params exist", async () => {
    const search =
      "?client_id=app-1&redirect_uri=https%3A%2F%2Frp.example.com%2Fcb&response_type=code&state=xyz";
    const assign = mockLocation("/login/oauth/authorize", search);
    const { view, navigate } = setup();

    await view.result.current.applyLoginTransition(successTransition);

    expect(assign).toHaveBeenCalledWith(`/api/v1/auth/authorize${search}`);
    expect(navigate).not.toHaveBeenCalled();
    expect(localStorage.getItem("lastLoginOrg")).toBe("org-1");
    expect(localStorage.getItem("lastLoginApplication")).toBe("app-1");
  });

  it("falls back to the console flow on a parameter-less manual visit (INC-FE-05)", async () => {
    const assign = mockLocation("/login/oauth/authorize", "");
    const { view, navigate } = setup();

    await view.result.current.applyLoginTransition(successTransition);

    expect(assign).not.toHaveBeenCalled();
    expect(navigate).toHaveBeenCalledWith("/dashboard", { replace: true });
    expect(localStorage.getItem("lastLoginOrg")).toBe("org-1");
  });

  it("keeps the console flow on non-authorize pages", async () => {
    const assign = mockLocation("/login", "?client_id=app-1&redirect_uri=https%3A%2F%2Fx%2Fcb");
    const { view, navigate } = setup();

    await view.result.current.applyLoginTransition(successTransition);

    expect(assign).not.toHaveBeenCalled();
    expect(navigate).toHaveBeenCalledWith("/dashboard", { replace: true });
  });
});
