import { describe, it, expect, vi } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useQrSigninFlow } from "./useQrSigninFlow";

const challenge = { codeId: "qr-001", code: "qr-payload", expiresIn: 300, interval: 3 };
const tokens = {
  status: "issued",
  accessToken: "token-value",
  tokenType: "Bearer",
  expiresIn: 3600,
};

function setup(overrides: Partial<Parameters<typeof useQrSigninFlow>[0]> = {}) {
  const createImpl = vi.fn<(...args: unknown[]) => unknown>().mockResolvedValue(challenge);
  const waitImpl = vi
    .fn<(...args: unknown[]) => unknown>()
    .mockResolvedValue({ kind: "confirmed", codeId: challenge.codeId });
  const exchangeImpl = vi.fn<(...args: unknown[]) => unknown>().mockResolvedValue(tokens);
  const onConfirmed = vi.fn<(...args: unknown[]) => unknown>().mockResolvedValue(undefined);
  const view = renderHook(() =>
    useQrSigninFlow({
      active: true,
      organizationId: "org-1",
      applicationId: "app-1",
      onConfirmed,
      createImpl,
      waitImpl,
      exchangeImpl,
      ...overrides,
    }),
  );
  return { view, createImpl, waitImpl, exchangeImpl, onConfirmed };
}

describe("useQrSigninFlow", () => {
  it("creates a session, waits for confirmation and exchanges it", async () => {
    const { view, createImpl, waitImpl, exchangeImpl, onConfirmed } = setup();

    await waitFor(() => expect(onConfirmed).toHaveBeenCalledTimes(1));

    expect(createImpl).toHaveBeenCalledWith({
      organizationId: "org-1",
      applicationId: "app-1",
      signal: expect.any(AbortSignal),
    });
    expect(waitImpl).toHaveBeenCalledWith(
      expect.objectContaining({
        codeId: "qr-001",
        intervalSeconds: 3,
        timeoutSeconds: 300,
      }),
    );
    expect(exchangeImpl).toHaveBeenCalledWith({
      codeId: "qr-001",
      signal: expect.any(AbortSignal),
    });
    expect(onConfirmed).toHaveBeenCalledWith(tokens);
    expect(view.result.current.qrSigninState.status).toBe("confirmed");
  });

  it("does nothing while the QR method is not selected", () => {
    const { view, createImpl } = setup({ active: false });

    expect(createImpl).not.toHaveBeenCalled();
    expect(view.result.current.qrSigninState.status).toBe("idle");
  });

  it("stays idle when the application identifiers are missing", () => {
    const { createImpl, view } = setup({ applicationId: null });

    expect(createImpl).not.toHaveBeenCalled();
    expect(view.result.current.qrSigninState.status).toBe("idle");
  });

  it("reports the server outcome when the session expires", async () => {
    const waitImpl = vi.fn<(...args: unknown[]) => unknown>().mockResolvedValue({
      kind: "expired",
      codeId: challenge.codeId,
      message: "auth:login.qrcode.expired",
    });
    const { view, exchangeImpl, onConfirmed } = setup({ waitImpl });

    await waitFor(() => expect(view.result.current.qrSigninState.status).toBe("expired"));

    expect(view.result.current.qrSigninState.error).toBe("auth:login.qrcode.expired");
    expect(exchangeImpl).not.toHaveBeenCalled();
    expect(onConfirmed).not.toHaveBeenCalled();
  });

  it("reports a failure when the session cannot be created", async () => {
    const createImpl = vi
      .fn<(...args: unknown[]) => unknown>()
      .mockRejectedValue(new Error("network down"));
    const { view } = setup({ createImpl });

    await waitFor(() => expect(view.result.current.qrSigninState.status).toBe("failed"));

    expect(view.result.current.qrSigninState.error).toBe("network down");
  });

  it("aborts the in-flight run when the caller reloads", async () => {
    const waitImpl = vi.fn<(...args: unknown[]) => unknown>().mockImplementation(
      (options: { signal?: AbortSignal }) =>
        new Promise((_resolve, reject) => {
          options.signal?.addEventListener("abort", () => {
            reject(new DOMException("aborted", "AbortError"));
          });
        }),
    );
    const { view, createImpl } = setup({ waitImpl });

    await waitFor(() => expect(createImpl).toHaveBeenCalledTimes(1));
    const firstSignal = (createImpl.mock.calls[0][0] as { signal: AbortSignal }).signal;

    act(() => {
      view.result.current.reloadQrSignin();
    });

    await waitFor(() => expect(createImpl).toHaveBeenCalledTimes(2));
    expect(firstSignal.aborted).toBe(true);
  });
});
