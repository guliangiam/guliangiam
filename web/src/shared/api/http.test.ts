import { afterEach, describe, expect, it, vi } from "vitest";
import { useSessionStore } from "@/features/session/session-store";
import { HttpError, isErrorCode, requestDocumentJson, shouldRedirectOnPasswordGate } from "./http";

function mockJsonResponse(status: number, body: unknown) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: new Headers({ "content-type": "application/json" }),
    json: async () => body,
  } as unknown as Response;
}

const fetchMock = vi.fn<(...args: unknown[]) => unknown>();

vi.stubGlobal("fetch", (...args: unknown[]) => fetchMock(...args));

afterEach(() => {
  fetchMock.mockReset();
});

describe("requestDocumentJson error mapping", () => {
  it("prefers error.details over the deprecated meta alias", async () => {
    fetchMock.mockResolvedValue(
      mockJsonResponse(422, {
        error: {
          code: "unprocessable_entity",
          message: "pageSize must be an integer between 1 and 100",
          details: { field: "pageSize", value: "200", max: 100 },
          meta: { field: "legacyMeta" },
        },
      }),
    );
    const error = await requestDocumentJson("/api/v1/users").catch((e: unknown) => e);
    expect(error).toBeInstanceOf(HttpError);
    const httpError = error as HttpError;
    expect(httpError.status).toBe(422);
    expect(httpError.code).toBe("unprocessable_entity");
    expect(httpError.details).toEqual({ field: "pageSize", value: "200", max: 100 });
    // The deprecated alias mirrors the canonical value, never the stale body.
    expect(httpError.meta).toEqual(httpError.details);
  });

  it("falls back to error.meta when details is absent", async () => {
    fetchMock.mockResolvedValue(
      mockJsonResponse(422, {
        error: {
          code: "unprocessable_entity",
          message: "pageSize must be an integer between 1 and 100",
          meta: { field: "pageSize", value: "200", max: 100 },
        },
      }),
    );
    const error = await requestDocumentJson("/api/v1/users").catch((e: unknown) => e);
    const httpError = error as HttpError;
    // Older servers only emit the compatibility alias; the client must still
    // make the 422 actionable.
    expect(httpError.details).toEqual({ field: "pageSize", value: "200", max: 100 });
    expect(httpError.meta).toEqual(httpError.details);
  });

  it("leaves details undefined when neither details nor meta is present", async () => {
    fetchMock.mockResolvedValue(mockJsonResponse(422, { msg: "nope" }));
    const error = (await requestDocumentJson("/x").catch((e: unknown) => e)) as HttpError;
    expect(error.code).toBe("unprocessable_entity");
    expect(error.details).toBeUndefined();
    expect(error.meta).toBeUndefined();
  });

  it("falls back to unprocessable_entity for a 422 with no code in the body", async () => {
    fetchMock.mockResolvedValue(mockJsonResponse(422, { msg: "nope" }));
    const error = (await requestDocumentJson("/x").catch((e: unknown) => e)) as HttpError;
    expect(error.code).toBe("unprocessable_entity");
  });

  it("reads the protocol error shape where `error` is a string", async () => {
    fetchMock.mockResolvedValue(
      mockJsonResponse(400, {
        error: "invalid_grant",
        error_description: "authorization code already used",
      }),
    );
    const error = (await requestDocumentJson("/oauth2/token").catch(
      (e: unknown) => e,
    )) as HttpError;
    expect(error.code).toBe("invalid_grant");
    expect(error.message).toBe("authorization code already used");
  });

  it("reads the SCIM error shape", async () => {
    fetchMock.mockResolvedValue(
      mockJsonResponse(409, {
        schemas: ["urn:ietf:params:scim:api:messages:2.0:Error"],
        detail: "already exists",
      }),
    );
    const error = (await requestDocumentJson("/scim/v2/Users").catch(
      (e: unknown) => e,
    )) as HttpError;
    expect(error.message).toBe("already exists");
    // No application code in the body, so the status fallback applies.
    expect(error.code).toBe("conflict");
  });

  it("maps a 400 without a code to invalid_argument", async () => {
    fetchMock.mockResolvedValue(mockJsonResponse(400, { msg: "bad" }));
    const error = (await requestDocumentJson("/x").catch((e: unknown) => e)) as HttpError;
    expect(error.code).toBe("invalid_argument");
    expect(isErrorCode(error, "invalid_argument")).toBe(true);
  });
});

describe("CSRF double-submit cookie", () => {
  // jsdom 可能按 __Host- 前缀规则拒绝写入，这里直接替换 document.cookie 读取值，
  // 只验证客户端读取逻辑本身。
  function stubCookie(value: string) {
    Object.defineProperty(document, "cookie", { configurable: true, get: () => value });
  }

  async function postedCsrfHeader(): Promise<string | undefined> {
    fetchMock.mockResolvedValue(mockJsonResponse(200, { ok: true }));
    await requestDocumentJson("/api/v1/x", { method: "POST", body: "{}" });
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    return (init.headers as Record<string, string>)["X-CSRF-Token"];
  }

  afterEach(() => {
    Reflect.deleteProperty(document, "cookie");
  });

  it("reads the production __Host-csrf cookie", async () => {
    stubCookie("__Host-csrf=prod-token");
    expect(await postedCsrfHeader()).toBe("prod-token");
  });

  it("falls back to the development csrf_token cookie", async () => {
    stubCookie("csrf_token=dev-token");
    expect(await postedCsrfHeader()).toBe("dev-token");
  });

  it("prefers __Host-csrf when both names are present", async () => {
    stubCookie("csrf_token=dev-token; __Host-csrf=prod-token");
    expect(await postedCsrfHeader()).toBe("prod-token");
  });
});

describe("password gate notices", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it("allows redirect from an ordinary API path", () => {
    expect(shouldRedirectOnPasswordGate("/api/v1/users", "/dashboard")).toBe(true);
  });

  it("does not redirect when already on the account page", () => {
    expect(shouldRedirectOnPasswordGate("/api/v1/users", "/account")).toBe(false);
  });

  it("does not redirect the change-password or logout calls themselves", () => {
    expect(shouldRedirectOnPasswordGate("/api/v1/auth/password/change", "/dashboard")).toBe(false);
    expect(shouldRedirectOnPasswordGate("/api/v1/auth/logout", "/dashboard")).toBe(false);
  });

  it("ignores query strings when matching the gate-exempt calls", () => {
    expect(
      shouldRedirectOnPasswordGate(
        "/api/v1/auth/password/change?organizationId=built-in",
        "/dashboard",
      ),
    ).toBe(false);
    expect(shouldRedirectOnPasswordGate("/api/v1/auth/logout?redirect=1", "/dashboard")).toBe(
      false,
    );
  });

  const expectGateNotice = async (
    code: "password_change_required" | "password_expired",
    eventName: string,
    messageKey: string,
  ) => {
    vi.useFakeTimers();
    fetchMock.mockResolvedValue(
      mockJsonResponse(403, {
        error: {
          code,
          message: "password change required",
          details: { reason: "any", changePasswordUrl: "/account" },
        },
      }),
    );
    const dispatch = vi.spyOn(window, "dispatchEvent");

    const error = (await requestDocumentJson("/api/v1/users").catch(
      (e: unknown) => e,
    )) as HttpError;

    expect(error.status).toBe(403);
    expect(error.code).toBe(code);
    expect(error.details).toEqual({ reason: "any", changePasswordUrl: "/account" });
    expect(dispatch).toHaveBeenCalledWith(expect.objectContaining({ type: eventName }));
    const event = dispatch.mock.calls.find(([value]) => (value as Event).type === eventName)?.[0];
    const detail = (event as CustomEvent).detail;
    expect(detail.messageKey).toBe(messageKey);
    vi.advanceTimersByTime(2000);
    return detail;
  };

  it("maps password_change_required to its own notice and event", async () => {
    const detail = await expectGateNotice(
      "password_change_required",
      "auth:password-change-required",
      "passwordChangeRequiredNotice",
    );
    expect(detail.code).toBe("password_change_required");
    expect(detail.messageKey).toBe("passwordChangeRequiredNotice");
  });

  it("maps password_expired to its own notice and event", async () => {
    const detail = await expectGateNotice(
      "password_expired",
      "auth:password-expired",
      "passwordExpiredNotice",
    );
    expect(detail.code).toBe("password_expired");
    expect(detail.messageKey).toBe("passwordExpiredNotice");
  });
});

describe("impersonation Authorization injection", () => {
  afterEach(() => {
    useSessionStore.getState().setImpersonatedUser(null);
  });

  function lastRequestHeaders(): Record<string, string> {
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    return init.headers as Record<string, string>;
  }

  it("injects Bearer imp token while impersonating", async () => {
    useSessionStore.getState().setImpersonationToken("imp_test_token");
    fetchMock.mockResolvedValue(mockJsonResponse(200, { ok: true }));
    await requestDocumentJson("/api/v1/users");
    expect(lastRequestHeaders().Authorization).toBe("Bearer imp_test_token");
  });

  it("sends no Authorization without an impersonation token", async () => {
    fetchMock.mockResolvedValue(mockJsonResponse(200, { ok: true }));
    await requestDocumentJson("/api/v1/users");
    expect(lastRequestHeaders().Authorization).toBeUndefined();
  });

  it("lets explicit request headers override the injected token", async () => {
    useSessionStore.getState().setImpersonationToken("imp_test_token");
    fetchMock.mockResolvedValue(mockJsonResponse(200, { ok: true }));
    await requestDocumentJson("/api/v1/users", {
      headers: { Authorization: "Bearer explicit_token" },
    });
    expect(lastRequestHeaders().Authorization).toBe("Bearer explicit_token");
  });

  it("stops injecting after impersonation state is cleared", async () => {
    useSessionStore.getState().setImpersonationToken("imp_test_token");
    useSessionStore.getState().setImpersonatedUser(null);
    fetchMock.mockResolvedValue(mockJsonResponse(200, { ok: true }));
    await requestDocumentJson("/api/v1/users");
    expect(lastRequestHeaders().Authorization).toBeUndefined();
  });
});
