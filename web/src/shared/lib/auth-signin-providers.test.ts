import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  getFaceIdProvider,
  createQrLoginSession,
  scanQrLogin,
  confirmQrLogin,
  exchangeQrLogin,
  pollQrLoginStatus,
  waitForQrLoginOutcome,
  runQrLogin,
  type SigninProvider,
} from "./auth-signin-providers";
import type { ApplicationItem } from "@/shared/types/domain";
import { expectKind, expectStatus } from "@/test-utils/assertions";

// Mock i18next
// i18n is consumed through the named `t` export; keep default.t for legacy call sites.
vi.mock("i18next", () => {
  const t = (key: string) => key;
  return { default: { t }, t };
});

// Mock the HTTP layer: the v1 QR login contract is issued through
// requestDocumentJson, and the closure keeps the mock reachable without a
// vi.hoisted() dance.
const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();
vi.mock("@/shared/api/http", () => ({
  requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
}));

// Mock provider-auth
vi.mock("@/shared/lib/provider-auth", () => ({
  getDirectOAuthProviderAuthUrl: vi.fn<(...args: unknown[]) => unknown>(),
  canStartDirectOAuthProvider: vi.fn<(...args: unknown[]) => unknown>().mockReturnValue(true),
}));

type ApplicationProviderItem = { provider?: unknown };

function makeApplication(providers: ApplicationProviderItem[] = []): ApplicationItem {
  return { providers } as ApplicationItem;
}

describe("getFaceIdProvider", () => {
  it("returns Face ID provider from application", () => {
    // `FaceId` is the canonical category value (useProviderCategories
    // PROVIDER_CATEGORIES). The old fixture used the legacy display label
    // "Face ID", which no provider row ever carries — so the old assertion
    // pinned the lookup to a value that could never match.
    const provider: SigninProvider = { name: "faceid", category: "FaceId" };
    const app = makeApplication([{ provider }]);
    const result = getFaceIdProvider(app);
    expect(result).not.toBeNull();
    expect(result?.category).toBe("FaceId");
  });

  it("returns null when no Face ID provider exists", () => {
    const provider: SigninProvider = { name: "github", category: "OAuth" };
    const app = makeApplication([{ provider }]);
    const result = getFaceIdProvider(app);
    expect(result).toBeNull();
  });

  it("returns null for null application", () => {
    const result = getFaceIdProvider(null);
    expect(result).toBeNull();
  });
});

// ─────────────────────────────────────────────
// QR Code 登录契约（/api/v1/auth/qrcode 四步 + 轮询）
//
// 字段名与端点逐条对齐服务端：request.go 的四个请求体、qrcode.go 的三个响应体。
// 这里断言的是「控制台实际发出的字节」——字段拼错不会让 tsc 报错，只会让请求在
// 运行期被服务端忽略（绑定失败时字段为零值），所以必须在调用点锁死。
// ─────────────────────────────────────────────
describe("QR login contract", () => {
  beforeEach(() => {
    mockRequestDocumentJson.mockReset();
  });

  it("step 1 create posts organizationId/applicationId and returns the QR payload", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      codeId: "qr-001",
      code: "qr-payload",
      expiresIn: 300,
      interval: 3,
    });

    const challenge = await createQrLoginSession({
      organizationId: "org-1",
      applicationId: "app-1",
    });

    expect(mockRequestDocumentJson).toHaveBeenCalledTimes(1);
    const [path, init] = mockRequestDocumentJson.mock.calls[0] as [string, RequestInit];
    expect(path).toBe("/api/v1/auth/qrcode");
    expect(init.method).toBe("POST");
    expect(JSON.parse(String(init.body))).toEqual({
      organizationId: "org-1",
      applicationId: "app-1",
    });
    expect(challenge).toEqual({
      codeId: "qr-001",
      code: "qr-payload",
      expiresIn: 300,
      interval: 3,
    });
  });

  it("step 1 sends an empty applicationId when the caller has none", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      codeId: "qr-001",
      code: "qr-payload",
      expiresIn: 300,
      interval: 3,
    });

    await createQrLoginSession({ organizationId: "org-1" });

    const [, init] = mockRequestDocumentJson.mock.calls[0] as [string, RequestInit];
    expect(JSON.parse(String(init.body))).toEqual({ organizationId: "org-1", applicationId: "" });
  });

  it("step 1 rejects a response without the QR payload", async () => {
    mockRequestDocumentJson.mockResolvedValue({ codeId: "qr-001" });

    await expect(createQrLoginSession({ organizationId: "org-1" })).rejects.toThrow(
      "auth:signin.loginFailed",
    );
  });

  it("step 2 scan posts the QR payload code (never a client-asserted userId)", async () => {
    mockRequestDocumentJson.mockResolvedValue({ status: "ok", isNewUser: true });

    const result = await scanQrLogin({ code: "qr-payload", username: "newcomer" });

    const [path, init] = mockRequestDocumentJson.mock.calls[0] as [string, RequestInit];
    expect(path).toBe("/api/v1/auth/qrcode/scan");
    expect(init.method).toBe("POST");
    expect(JSON.parse(String(init.body))).toEqual({ code: "qr-payload", username: "newcomer" });
    expect(result.isNewUser).toBe(true);
  });

  it("step 2 scan defaults isNewUser to false when the server omits it", async () => {
    mockRequestDocumentJson.mockResolvedValue({ status: "ok" });

    const result = await scanQrLogin({ code: "qr-payload" });

    const [, init] = mockRequestDocumentJson.mock.calls[0] as [string, RequestInit];
    expect(JSON.parse(String(init.body))).toEqual({ code: "qr-payload" });
    expect(result).toEqual({ status: "ok", isNewUser: false });
  });

  it("step 2 scan forwards displayName/ipAddress/userAgent when supplied", async () => {
    mockRequestDocumentJson.mockResolvedValue({ status: "ok", isNewUser: false });

    await scanQrLogin({
      code: "qr-payload",
      username: "newcomer",
      displayName: "New Comer",
      ipAddress: "203.0.113.7",
      userAgent: "IAM-Mobile/1.0",
    });

    const [, init] = mockRequestDocumentJson.mock.calls[0] as [string, RequestInit];
    expect(JSON.parse(String(init.body))).toEqual({
      code: "qr-payload",
      username: "newcomer",
      displayName: "New Comer",
      ipAddress: "203.0.113.7",
      userAgent: "IAM-Mobile/1.0",
    });
  });

  it("step 3 confirm posts code/approved", async () => {
    mockRequestDocumentJson.mockResolvedValue({ status: "ok" });

    await confirmQrLogin({ code: "qr-payload", approved: false });

    const [path, init] = mockRequestDocumentJson.mock.calls[0] as [string, RequestInit];
    expect(path).toBe("/api/v1/auth/qrcode/confirm");
    expect(init.method).toBe("POST");
    expect(JSON.parse(String(init.body))).toEqual({ code: "qr-payload", approved: false });
  });

  it("step 4 exchange posts codeId and returns the login result document", async () => {
    mockRequestDocumentJson.mockResolvedValue({ status: "success" });

    const result = await exchangeQrLogin({ codeId: "qr-001" });

    const [path, init] = mockRequestDocumentJson.mock.calls[0] as [string, RequestInit];
    expect(path).toBe("/api/v1/auth/qrcode/exchange");
    expect(init.method).toBe("POST");
    expect(JSON.parse(String(init.body))).toEqual({ codeId: "qr-001" });
    expect(result.status).toBe("success");
  });

  it("step 4 forwards the protocol context like /faceid/complete", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      status: "success",
      redirectTo: "https://rp.example.com/cb?code=abc",
    });

    const result = await exchangeQrLogin({
      codeId: "qr-001",
      responseType: "code",
      redirectUri: "https://rp.example.com/cb",
      state: "state-1",
      nonce: "nonce-1",
      scope: ["openid", "profile"],
    });

    const [, init] = mockRequestDocumentJson.mock.calls[0] as [string, RequestInit];
    expect(JSON.parse(String(init.body))).toEqual({
      codeId: "qr-001",
      responseType: "code",
      redirectUri: "https://rp.example.com/cb",
      state: "state-1",
      nonce: "nonce-1",
      scope: ["openid", "profile"],
    });
    expect(result.redirectTo).toBe("https://rp.example.com/cb?code=abc");
  });

  it("step 4 rejects a response without a status", async () => {
    mockRequestDocumentJson.mockResolvedValue({});

    await expect(exchangeQrLogin({ codeId: "qr-001" })).rejects.toThrow("auth:signin.loginFailed");
  });

  it("polls GET /qrcode/{codeId}/status with the codeId percent-encoded", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      status: "scanned",
      codeId: "qr/001",
      expiresIn: 42,
    });

    const result = await pollQrLoginStatus({ codeId: "qr/001" });

    const [path] = mockRequestDocumentJson.mock.calls[0] as [string, RequestInit];
    expect(path).toBe("/api/v1/auth/qrcode/qr%2F001/status");
    expect(result).toEqual({ status: "scanned", codeId: "qr/001", expiresIn: 42 });
  });

  it("treats an unknown poll status as pending instead of stalling", async () => {
    mockRequestDocumentJson.mockResolvedValue({ status: "something-new", codeId: "qr-001" });

    const result = await pollQrLoginStatus({ codeId: "qr-001" });

    expect(result.status).toBe("pending");
    expect(result.expiresIn).toBeUndefined();
  });
});

describe("waitForQrLoginOutcome", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  function sequencePoll(statuses: string[], codeId = "qr-001") {
    const impl = vi.fn<(...args: unknown[]) => Promise<unknown>>();
    for (const status of statuses) {
      impl.mockResolvedValueOnce({ status, codeId });
    }
    return impl;
  }

  it("resolves confirmed after pending -> scanned, reporting each new status once", async () => {
    const pollImpl = sequencePoll(["pending", "scanned", "confirmed"]);
    const onStatus = vi.fn<(...args: unknown[]) => unknown>();

    const outcome = waitForQrLoginOutcome({
      codeId: "qr-001",
      intervalSeconds: 1,
      timeoutSeconds: 10,
      pollImpl: pollImpl as unknown as typeof pollQrLoginStatus,
      onStatus,
    });

    await vi.advanceTimersByTimeAsync(2000);
    await expect(outcome).resolves.toEqual({ kind: "confirmed", codeId: "qr-001" });
    expect(onStatus.mock.calls.map((call) => call[0])).toEqual(["pending", "scanned"]);
    expect(pollImpl).toHaveBeenCalledTimes(3);
  });

  it("resolves expired when the server reports expiry", async () => {
    const pollImpl = sequencePoll(["scanned", "expired"]);

    const outcome = waitForQrLoginOutcome({
      codeId: "qr-001",
      intervalSeconds: 1,
      timeoutSeconds: 10,
      pollImpl: pollImpl as unknown as typeof pollQrLoginStatus,
    });

    await vi.advanceTimersByTimeAsync(1000);
    const result = await outcome;
    expectKind(result, "expired");
    expect(result.message).toBe("auth:login.qrcode.expired");
  });

  it("stops polling and rejects with AbortError when the caller aborts", async () => {
    const pollImpl = sequencePoll(["pending", "pending", "pending"]);
    const controller = new AbortController();

    const outcome = waitForQrLoginOutcome({
      codeId: "qr-001",
      intervalSeconds: 1,
      timeoutSeconds: 10,
      signal: controller.signal,
      pollImpl: pollImpl as unknown as typeof pollQrLoginStatus,
    });

    await vi.advanceTimersByTimeAsync(0);
    controller.abort();
    await expect(outcome).rejects.toMatchObject({ name: "AbortError" });

    await vi.advanceTimersByTimeAsync(5000);
    expect(pollImpl).toHaveBeenCalledTimes(1);
  });

  it("ends as expired once the local deadline passes without an answer", async () => {
    const pollImpl = sequencePoll(["pending"]);
    pollImpl.mockResolvedValue({ status: "pending", codeId: "qr-001" });

    const outcome = waitForQrLoginOutcome({
      codeId: "qr-001",
      intervalSeconds: 1,
      timeoutSeconds: 2,
      pollImpl: pollImpl as unknown as typeof pollQrLoginStatus,
    });

    await vi.advanceTimersByTimeAsync(2000);
    await expect(outcome).resolves.toMatchObject({ kind: "expired" });
  });
});

describe("runQrLogin", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockRequestDocumentJson.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("creates the challenge, waits for confirmation and exchanges it", async () => {
    mockRequestDocumentJson
      .mockResolvedValueOnce({ codeId: "qr-001", code: "payload", expiresIn: 300, interval: 1 })
      .mockResolvedValueOnce({ status: "success" });
    const pollImpl = vi.fn<(...args: unknown[]) => Promise<unknown>>().mockResolvedValue({
      status: "confirmed",
      codeId: "qr-001",
    });
    const onChallenge = vi.fn<(...args: unknown[]) => unknown>();

    const result = runQrLogin({
      organizationId: "org-1",
      applicationId: "app-1",
      onChallenge,
      pollImpl: pollImpl as unknown as typeof pollQrLoginStatus,
    });

    await vi.advanceTimersByTimeAsync(0);
    const settled = await result;

    expectStatus(settled, "confirmed");
    expect(onChallenge).toHaveBeenCalledWith({
      codeId: "qr-001",
      code: "payload",
      expiresIn: 300,
      interval: 1,
    });
    expect(settled.loginResult.status).toBe("success");

    const exchangeCall = mockRequestDocumentJson.mock.calls[1] as [string, RequestInit];
    expect(exchangeCall[0]).toBe("/api/v1/auth/qrcode/exchange");
  });

  it("returns expired without exchanging when the session lapses", async () => {
    mockRequestDocumentJson.mockResolvedValueOnce({
      codeId: "qr-001",
      code: "payload",
      expiresIn: 300,
      interval: 1,
    });
    const pollImpl = vi.fn<(...args: unknown[]) => Promise<unknown>>().mockResolvedValue({
      status: "expired",
      codeId: "qr-001",
    });

    const result = runQrLogin({
      organizationId: "org-1",
      pollImpl: pollImpl as unknown as typeof pollQrLoginStatus,
    });

    await vi.advanceTimersByTimeAsync(0);
    const settled = await result;

    expect(settled.status).toBe("expired");
    expect(mockRequestDocumentJson).toHaveBeenCalledTimes(1);
  });
});
