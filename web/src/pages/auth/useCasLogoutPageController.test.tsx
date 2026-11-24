import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, render, renderHook, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { QueryClientProvider } from "@tanstack/react-query";
import { useEffect, type ReactNode } from "react";
import { createTestQueryClient } from "@/test-utils/render";
import {
  useCasLogoutPageController,
  type CasLogoutPageControllerState,
  type CasLogoutSettledState,
} from "./useCasLogoutPageController";

const casLogoutMock = vi.fn<(service: string) => Promise<unknown>>();

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key, i18n: { language: "en" } }),
}));

vi.mock("@/shared/api/iam/auth", () => ({
  casLogout: (service: string) => casLogoutMock(service),
}));

vi.mock("@/shared/api/query-keys", () => ({
  queryKeys: { session: { all: ["session"] } },
}));

const ENTRY =
  "/cas/acme/portal/logout?service=" + encodeURIComponent("https://rp.example.com/after-logout");

/**
 * Reports the controller state from an effect. Reading the state off the hook's
 * return value directly would mean writing to an outer value during render, which
 * is both a lint error and a render-time side effect.
 */
function LogoutHarness({ onState }: { onState: (state: CasLogoutPageControllerState) => void }) {
  const state = useCasLogoutPageController();
  useEffect(() => onState(state), [state, onState]);
  return null;
}

function renderController(entry = ENTRY) {
  const queryClient = createTestQueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[entry]}>{children}</MemoryRouter>
    </QueryClientProvider>
  );

  return renderHook(() => useCasLogoutPageController(), { wrapper });
}

describe("useCasLogoutPageController", () => {
  const replace = vi.fn<(url: string) => void>();

  beforeEach(() => {
    casLogoutMock.mockReset();
    replace.mockReset();
    // jsdom's location.replace is not writable in place, so the assertion is
    // installed as a property rather than assigned through `location`.
    Object.defineProperty(window, "location", {
      configurable: true,
      value: { ...window.location, replace, href: "http://localhost/" },
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("forwards the entry URL's service to the server", async () => {
    casLogoutMock.mockResolvedValue({
      revokedSessions: 0,
      redirectRejected: false,
      redirectUri: "",
    });

    renderController();

    await waitFor(() => {
      expect(casLogoutMock).toHaveBeenCalledWith("https://rp.example.com/after-logout");
    });
  });

  it("navigates to the server-adjudicated redirectUri", async () => {
    casLogoutMock.mockResolvedValue({
      revokedSessions: 3,
      redirectRejected: false,
      redirectUri: "https://rp.example.com/after-logout",
    });

    renderController();

    await waitFor(() => {
      expect(replace).toHaveBeenCalledWith("https://rp.example.com/after-logout");
    });
  });

  it("never navigates to the raw service when the server refuses the redirect", async () => {
    casLogoutMock.mockResolvedValue({
      revokedSessions: 1,
      redirectRejected: true,
      redirectUri: "",
    });

    const { result } = renderController();

    await waitFor(() => {
      expect(result.current.status).toBe("settled");
    });

    expect(replace).not.toHaveBeenCalled();
    if (result.current.status !== "settled") throw new Error("expected settled state");
    expect(result.current.redirectRejected).toBe(true);
    expect(result.current.revokedSessions).toBe(1);
  });

  it("reports a settled logout when no return target was requested", async () => {
    casLogoutMock.mockResolvedValue({
      revokedSessions: 2,
      redirectRejected: false,
      redirectUri: "",
    });

    const { result } = renderController("/cas/acme/portal/logout");

    await waitFor(() => {
      expect(result.current.status).toBe("settled");
    });

    if (result.current.status !== "settled") throw new Error("expected settled state");
    expect(result.current.redirectRejected).toBe(false);
    expect(result.current.revokedSessions).toBe(2);
    expect(replace).not.toHaveBeenCalled();
  });

  it("surfaces a failure as an error state with a retry", async () => {
    casLogoutMock.mockRejectedValue(new Error("network down"));

    const { result } = renderController();

    await waitFor(() => {
      expect(result.current.status).toBe("error");
    });

    if (result.current.status !== "error") throw new Error("expected error state");
    expect(result.current.errorMessage).toBe("network down");

    casLogoutMock.mockResolvedValue({
      revokedSessions: 1,
      redirectRejected: false,
      redirectUri: "",
    });
    result.current.onRetry();

    await waitFor(() => {
      expect(result.current.status).toBe("settled");
    });
  });

  it("starts in the pending state", () => {
    casLogoutMock.mockReturnValue(new Promise(() => {}));

    const { result } = renderController();

    expect(result.current.status).toBe("pending");
  });

  it("lands on this application's CAS sign-in page when the redirect is refused", async () => {
    casLogoutMock.mockResolvedValue({
      revokedSessions: 1,
      redirectRejected: true,
      redirectUri: "",
    });

    const onState = vi.fn<(state: CasLogoutPageControllerState) => void>();

    render(
      <QueryClientProvider client={createTestQueryClient()}>
        <MemoryRouter initialEntries={[ENTRY]}>
          <Routes>
            <Route
              path="/cas/:owner/:casApplicationName/logout"
              element={<LogoutHarness onState={onState} />}
            />
            <Route path="/cas/:owner/:casApplicationName/login" element={<p>cas sign-in</p>} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(onState.mock.lastCall?.[0].status).toBe("settled");
    });

    const settled: CasLogoutSettledState | undefined = onState.mock.lastCall?.[0];
    if (settled?.status !== "settled") throw new Error("expected settled state");
    act(() => settled.onReturnToSignIn());

    expect(await screen.findByText("cas sign-in")).toBeTruthy();
    expect(replace).not.toHaveBeenCalled();
  });
});
