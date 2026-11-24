/**
 * Auth-flow internal response envelope.
 *
 * This is NOT an HTTP response envelope — it is an internal data contract used
 * by the login state machine (auth-login-transitions, auth-challenge, etc.)
 * to thread responses through multi-step auth flows.
 *
 * Historical note: the shape mirrors the legacy legacy `{status, msg, data,
 * data2}` API because early versions of GuliangIAM's auth flow were adapted
 * from that protocol. The transport layer (`requestDocumentJson`) never parses
 * or validates against this shape; it only appears inside the frontend auth
 * module.
 */
import { useSessionStore } from "@/features/session/session-store";
import { passwordGateNoticeForCode } from "../lib/password-gate-notice";
import { AUTH_LOGOUT, AUTH_PASSWORD_CHANGE } from "./paths";

export interface AuthFlowResponse {
  status: string;
  msg?: string;
  data?: unknown;
  data2?: unknown;
}

/** @deprecated Renamed to {@link AuthFlowResponse}. Kept as alias for in-flight consumers during auth-flow refactor. */
export type ResponseEnvelope = AuthFlowResponse;

export class HttpError extends Error {
  status: number;
  code: string;
  body: unknown;
  /**
   * Server-supplied field detail from `error.details`, when present.
   *
   * The management surface answers 422 with `{field, value, max}` (pagination)
   * or `{field, value, allowed}` (sort). Dropping it left 422 responses as
   * unactionable prose even though the server had said exactly which field was
   * wrong. `details` is the canonical key; the parser also accepts the
   * deprecated `meta` alias during its compatibility window.
   */
  details: Record<string, unknown> | undefined;

  /**
   * @deprecated Compatibility alias of {@link details}; always the same value
   * on the wire. Read `details`; this property disappears once the server stops
   * emitting `error.meta`.
   */
  meta: Record<string, unknown> | undefined;

  /**
   * Seconds the server asked the client to wait before retrying, taken from the
   * `Retry-After` header.
   *
   * Only set when the server actually sent one. It is the only legitimate source
   * for backoff: a client-invented delay fights the server's own rate-limit
   * budget (design report §42.3.11), so callers must fall back to their own
   * schedule rather than guess a value here.
   */
  retryAfterSeconds: number | undefined;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Request failed with status ${status}`);
    this.status = status;
    this.code = errorCodeFromStatus(status);
    this.body = body;
    this.details = undefined;
    this.meta = undefined;
    this.retryAfterSeconds = undefined;
    this.name = "HttpError";
  }
}

/**
 * Read `Retry-After` as a delay in seconds.
 *
 * RFC 9110 allows either delta-seconds or an HTTP-date, so both shapes are
 * accepted; an unparseable value yields `undefined` rather than `NaN` so a
 * caller can branch on "the server told me" vs "the server stayed silent".
 */
export function parseRetryAfterSeconds(response: Response): number | undefined {
  const raw = response.headers.get("Retry-After");
  if (!raw) return undefined;

  const trimmed = raw.trim();
  if (/^\d+$/.test(trimmed)) {
    const seconds = Number.parseInt(trimmed, 10);
    return Number.isFinite(seconds) ? seconds : undefined;
  }

  const target = Date.parse(trimmed);
  if (Number.isNaN(target)) return undefined;
  return Math.max(0, Math.ceil((target - Date.now()) / 1000));
}

/**
 * Map HTTP status to an application-level error code.
 *
 * The codes must come from the backend's single namespace (错误码注册表 §1, 17
 * codes in `internal/platform/errors`). This fallback is only used when the
 * response body carries no code of its own — a proxy or gateway error, say —
 * so returning a name the backend never emits (`bad_request`, `internal_error`) <!-- naming-allow：本行仅声明该字面量已废弃，不得在代码或契约中使用 -->
 * would break every `isErrorCode(err, "invalid_argument")` style check.
 */
function errorCodeFromStatus(status: number): string {
  switch (status) {
    case 400:
      return "invalid_argument";
    case 401:
      return "unauthorized";
    case 403:
      return "forbidden";
    case 404:
      return "not_found";
    case 409:
      return "conflict";
    case 422:
      return "unprocessable_entity";
    case 429:
      return "too_many_requests";
    default:
      return status >= 500 ? "internal" : "unknown";
  }
}

/** Check if an HttpError matches a specific application error code. */
export function isErrorCode(error: unknown, code: string): error is HttpError {
  return error instanceof HttpError && error.code === code;
}

export function isHttpError(error: unknown): error is HttpError {
  return error instanceof HttpError;
}

/**
 * Read the structured error detail through the canonical `details` property.
 *
 * Every consumer uses this accessor instead of poking at `HttpError.details`
 * or the deprecated `meta` alias directly, so the details-then-meta fallback
 * lives in exactly one place.
 */
export function getErrorDetails(error: unknown): Record<string, unknown> | undefined {
  return isHttpError(error) ? error.details : undefined;
}

// ── 401 Redirect Guard ──
// Prevents multiple concurrent 401 responses from triggering repeated redirects.
let isRedirecting = false;

function handleUnauthorized() {
  if (isRedirecting) return;

  // Already on an auth page — don't redirect, let the calling code
  // (useSessionBootstrap / AppShell) handle it via React Router.
  if (typeof window !== "undefined") {
    const pathname = window.location.pathname;
    if (
      pathname === "/login" ||
      pathname === "/signup" ||
      pathname === "/forgot-password" ||
      pathname.startsWith("/login/") ||
      pathname.startsWith("/signup/")
    ) {
      return;
    }
  }

  isRedirecting = true;

  // Clear session state
  try {
    localStorage.removeItem("gl-session");
  } catch {
    // Ignore storage errors
  }

  // Dispatch event to let AppShell handle redirect via React Router (preserves SPA state).
  // AppShell listens for "auth:unauthorized" and uses navigate() instead of window.location.href.
  if (typeof window !== "undefined") {
    window.dispatchEvent(
      new CustomEvent("auth:unauthorized", {
        detail: { from: window.location.pathname },
      }),
    );
  }

  // Reset guard after 2 seconds to allow re-login
  setTimeout(() => {
    isRedirecting = false;
  }, 2000);
}

// ── Password Gate Redirect Guard ──
// The backend gate answers 403 with one of two distinct codes. Concurrent
// in-flight requests must not each dispatch a notice/navigate.
let isPasswordGateRedirecting = false;

/** Decide whether a password-gate response should navigate to /account. */
export function shouldRedirectOnPasswordGate(requestPath: string, currentPath: string): boolean {
  if (currentPath === "/account") return false;
  const normalized = requestPath.split("?")[0];
  // The change-password and logout calls themselves must never bounce.
  if (normalized.endsWith(AUTH_PASSWORD_CHANGE)) return false;
  if (normalized.endsWith(AUTH_LOGOUT)) return false;
  return true;
}

function handlePasswordGateNotice(
  notice: ReturnType<typeof passwordGateNoticeForCode>,
  requestPath: string,
) {
  if (typeof window === "undefined" || !notice) return;

  const currentPath = window.location.pathname;
  if (!shouldRedirectOnPasswordGate(requestPath, currentPath)) return;
  if (isPasswordGateRedirecting) return;

  isPasswordGateRedirecting = true;
  // AppShell listens for each code's own event and shows its own notice copy.
  window.dispatchEvent(
    new CustomEvent(notice.eventName, {
      detail: { from: currentPath, requestPath, code: notice.code, messageKey: notice.messageKey },
    }),
  );

  setTimeout(() => {
    isPasswordGateRedirecting = false;
  }, 2000);
}

function getAcceptLanguage() {
  if (typeof window !== "undefined") {
    const storedLanguage = window.localStorage.getItem("preferredLanguage");
    if (storedLanguage) {
      return storedLanguage;
    }
  }

  return typeof navigator !== "undefined" ? navigator.language : "en";
}

// Read the CSRF token from the Double Submit Cookie. The backend picks the
// cookie name from its deployment mode:
//   COOKIE_SECURE=true  -> __Host-csrf (production)
//   COOKIE_SECURE=false -> csrf_token   (local development)
// Both names are accepted here so the same bundle works in either mode.
function readCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp(`(?:^|;\\s*)${name}=([^;]*)`));
  return match ? decodeURIComponent(match[1]) : null;
}

function getCsrfToken(): string | null {
  if (typeof document === "undefined") {
    return null;
  }

  return readCookie("__Host-csrf") ?? readCookie("csrf_token");
}

function isMutatingMethod(method?: string): boolean {
  return method === "POST" || method === "PUT" || method === "PATCH" || method === "DELETE";
}

function buildHeaders(init?: RequestInit, json?: boolean) {
  const csrfToken = getCsrfToken();
  // 冒充会话：内存中的 imp token 随所有请求注入 Authorization，后端在 cookie
  // 会话存在时按 impersonated_by 绑定校验后让 imp 身份优先（裁定 Q2-A）。
  // 显式传入的 headers 仍然最后展开，调用方可覆盖。
  const impersonationToken = useSessionStore.getState().impersonationToken;
  // FormData bodies must keep the browser-generated multipart boundary; an
  // explicit application/json header would make the request unparseable.
  const isFormData = typeof FormData !== "undefined" && init?.body instanceof FormData;
  return {
    "Accept-Language": getAcceptLanguage(),
    ...(json && !isFormData ? { "Content-Type": "application/json" } : {}),
    ...(csrfToken && isMutatingMethod(init?.method) ? { "X-CSRF-Token": csrfToken } : {}),
    ...(impersonationToken ? { Authorization: `Bearer ${impersonationToken}` } : {}),
    ...(init?.headers ?? {}),
  };
}

export async function requestDocumentJson<T = unknown>(
  path: string,
  init?: RequestInit,
): Promise<T | undefined> {
  const response = await fetch(path, {
    credentials: "include",
    ...init,
    headers: buildHeaders(
      init,
      (init?.method === "POST" || init?.method === "PUT" || init?.method === "PATCH") &&
        init?.body !== null &&
        init?.body !== undefined,
    ),
    signal: init?.signal,
  });

  const contentType = response.headers.get("content-type") ?? "";
  const payload = contentType.includes("application/json")
    ? ((await response.json()) as unknown)
    : undefined;

  if (!response.ok) {
    throwMappedError(response, payload, path);
  }

  return payload as T | undefined;
}

/**
 * Read the structured detail object, preferring `error.details` and falling
 * back to the deprecated `error.meta` alias. The backend emits both with the
 * same value during the compatibility window; reading either shape here keeps
 * older responses working without letting call sites each invent their own
 * fallback order.
 */
function readErrorDetails(
  errorObj: Record<string, unknown> | undefined,
): Record<string, unknown> | undefined {
  for (const key of ["details", "meta"] as const) {
    const value = errorObj?.[key];
    if (typeof value === "object" && value !== null && !Array.isArray(value)) {
      return value as Record<string, unknown>;
    }
  }
  return undefined;
}

/**
 * Rebuild the server's error explanation and throw it as an `HttpError`.
 *
 * Two error namespaces reach the browser (手册 附录 C):
 *   management / authentication -> {"error":{"code":…,"message":…}}
 *   protocol (OAuth2/SCIM)      -> {"error":"invalid_grant","error_description":…}
 *   SCIM                        -> {"schemas":[…],"detail":…,"status":"409"}
 * In the protocol shape `error` is a *string*, so reading `.message`/`.code`
 * off it silently produced a generic "Request failed with status N" and
 * threw away the only explanation the server sent.
 */
function throwMappedError(response: Response, payload: unknown, requestPath = ""): never {
  if (response.status === 401) {
    handleUnauthorized();
  }

  const errorBody = payload as Record<string, unknown> | undefined;
  const rawError = errorBody?.error;
  const errorObj =
    typeof rawError === "object" && rawError !== null
      ? (rawError as Record<string, unknown>)
      : undefined;
  const protocolCode = typeof rawError === "string" ? rawError : undefined;
  const protocolMessage =
    typeof errorBody?.error_description === "string" && errorBody.error_description
      ? errorBody.error_description
      : undefined;
  const scimMessage = typeof errorBody?.detail === "string" ? errorBody.detail : undefined;

  const errorMessage =
    (errorObj && typeof errorObj.message === "string" && errorObj.message) ||
    protocolMessage ||
    scimMessage ||
    (typeof errorBody?.msg === "string" && errorBody.msg) ||
    `Request failed with status ${response.status}`;
  const errorCode =
    (errorObj && typeof errorObj.code === "string" && errorObj.code) ||
    protocolCode ||
    errorCodeFromStatus(response.status);
  const errorDetails = readErrorDetails(errorObj);

  if (response.status === 403) {
    handlePasswordGateNotice(passwordGateNoticeForCode(errorCode), requestPath);
  }

  const httpErr = new HttpError(response.status, payload, errorMessage);
  httpErr.code = errorCode;
  httpErr.details = errorDetails;
  // Keep the deprecated alias in lockstep for any caller still reading `meta`.
  httpErr.meta = errorDetails;
  httpErr.retryAfterSeconds = parseRetryAfterSeconds(response);
  throw httpErr;
}

/** HTTP 304 — the caller's cached representation is still current. */
export const HTTP_NOT_MODIFIED = 304;

export type ConditionalJsonResult<T> = {
  /** True when the server answered 304 and `body` is therefore absent. */
  notModified: boolean;
  status: number;
  /** Response validator to send back as `If-None-Match` on the next read. */
  etag: string | undefined;
  body: T | undefined;
};

/**
 * Read a resource carrying a conditional request (§42.3.11).
 *
 * A polling page re-reads the same resource every few seconds, and almost every
 * read returns the same bytes. `requestDocumentJson` cannot serve that: a 304 has
 * `ok === false` and no body, so it was mapped into an `HttpError` and the poller
 * saw a failure where the server had actually confirmed "still current".
 *
 * This variant completes the conditional GET round trip instead. It never throws
 * on 304, returns the `ETag` from either outcome so the caller can keep it, and
 * leaves every other failure to the shared error mapper.
 *
 * Note: `ETag` is only readable cross-origin because the API exposes it via
 * `Access-Control-Expose-Headers`.
 */
export async function requestDocumentJsonIfChanged<T = unknown>(
  path: string,
  init?: RequestInit,
  etag?: string,
): Promise<ConditionalJsonResult<T>> {
  const requestInit: RequestInit = {
    ...init,
    headers: {
      ...(init?.headers as Record<string, string> | undefined),
      ...(etag ? { "If-None-Match": etag } : {}),
    },
  };

  const response = await fetch(path, {
    credentials: "include",
    ...requestInit,
    headers: buildHeaders(requestInit),
    signal: init?.signal,
  });

  const responseETag = response.headers.get("ETag") ?? undefined;

  if (response.status === HTTP_NOT_MODIFIED) {
    return {
      notModified: true,
      status: response.status,
      etag: responseETag ?? etag,
      body: undefined,
    };
  }

  const contentType = response.headers.get("content-type") ?? "";
  const payload = contentType.includes("application/json")
    ? ((await response.json()) as unknown)
    : undefined;

  if (!response.ok) {
    throwMappedError(response, payload, path);
  }

  return {
    notModified: false,
    status: response.status,
    etag: responseETag,
    body: payload as T | undefined,
  };
}

/**
 * Request a plain-text response (e.g. SAML XML metadata, OpenAPI spec text).
 *
 * Unlike request/requestDocumentJson which expect JSON envelope, this returns
 * the raw string body. Still centralises credentials + CSRF + error mapping so
 * callers don't hand-roll fetch + !ok checks.
 *
 * The path/URL can be absolute (external IdP metadata) — fetch handles both
 * relative and absolute transparently. buildHeaders adds Accept-Language + CSRF
 * for same-origin calls; cross-origin servers will simply ignore those headers.
 */
export async function requestText(path: string, init?: RequestInit): Promise<string> {
  const response = await fetch(path, {
    credentials: "include",
    ...init,
    headers: buildHeaders(init),
    signal: init?.signal,
  });

  const body = await response.text();

  if (!response.ok) {
    throw new HttpError(
      response.status,
      body,
      `Request failed with status ${response.status}: ${body.slice(0, 200)}`,
    );
  }

  return body;
}

/**
 * Request a binary/blob response (e.g. file exports).
 *
 * Returns the raw `Response` so callers can read `response.blob()` and headers
 * such as `Content-Disposition`. Centralises credentials + CSRF so export
 * endpoints don't need to hand-roll `fetch` calls.
 */
export async function requestBlob(path: string, init?: RequestInit): Promise<Response> {
  const response = await fetch(path, {
    credentials: "include",
    ...init,
    headers: buildHeaders(init),
    signal: init?.signal,
  });

  if (!response.ok) {
    const contentType = response.headers.get("content-type") ?? "";
    const payload = contentType.includes("application/json")
      ? ((await response.json()) as unknown)
      : undefined;
    throwMappedError(response, payload, path);
  }

  return response;
}
