import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  deleteOwnIdentity,
  enableOwnMfaSetup,
  initiateOwnMfaSetup,
  listOwnConsents,
  replaceOwnMfaMethods,
  updateOwnPreferredMfa,
  verifyOwnMfaSetup,
} from "./auth-selfservice";

const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/http", () => ({
  requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
}));

function lastCall() {
  const call = mockRequestDocumentJson.mock.calls.at(-1);
  return { url: String(call?.[0]), options: (call?.[1] ?? {}) as Record<string, unknown> };
}

function bodyOf(options: Record<string, unknown>) {
  return JSON.parse(String(options.body ?? "{}")) as Record<string, unknown>;
}

describe("auth self-service client", () => {
  beforeEach(() => {
    mockRequestDocumentJson.mockReset();
    mockRequestDocumentJson.mockResolvedValue({});
  });

  it("initiates MFA setup on the session-bound /auth path without userID/orgID", async () => {
    mockRequestDocumentJson.mockResolvedValue({ secret: "S", recoveryCodes: ["r"] });
    const result = await initiateOwnMfaSetup({ mfaType: "app" });
    const { url, options } = lastCall();
    expect(url).toBe("/api/v1/auth/mfa-setup/initiate");
    expect(options.method).toBe("POST");
    expect(bodyOf(options)).toEqual({ mfaType: "app" });
    expect(result.secret).toBe("S");
  });

  it("verifies MFA setup with passcode and secret", async () => {
    await verifyOwnMfaSetup({ mfaType: "app", passcode: "123456", secret: "S" });
    const { url, options } = lastCall();
    expect(url).toBe("/api/v1/auth/mfa-setup/verify");
    expect(options.method).toBe("POST");
    expect(bodyOf(options)).toEqual({ mfaType: "app", passcode: "123456", secret: "S" });
  });

  it("enables MFA setup carrying recovery codes and proof passcode", async () => {
    await enableOwnMfaSetup({
      mfaType: "app",
      secret: "S",
      passcode: "123456",
      recoveryCodes: ["r"],
    });
    const { url, options } = lastCall();
    expect(url).toBe("/api/v1/auth/mfa-setup/enable");
    expect(options.method).toBe("POST");
    expect(bodyOf(options).recoveryCodes).toEqual(["r"]);
  });

  it("replaces MFA methods with revision for optimistic concurrency", async () => {
    await replaceOwnMfaMethods(7, [{ methodType: "totp", enabled: true }]);
    const { url, options } = lastCall();
    expect(url).toBe("/api/v1/auth/mfa-methods");
    expect(options.method).toBe("PUT");
    expect(bodyOf(options)).toEqual({
      revision: 7,
      items: [{ methodType: "totp", enabled: true }],
    });
  });

  it("updates preferred MFA type", async () => {
    await updateOwnPreferredMfa({ mfaType: "totp", revision: 3 });
    const { url, options } = lastCall();
    expect(url).toBe("/api/v1/auth/preferred-mfa");
    expect(options.method).toBe("PUT");
    expect(bodyOf(options)).toEqual({ mfaType: "totp", revision: 3 });
  });

  it("unbinds an identity by row id via DELETE", async () => {
    await deleteOwnIdentity("id-1");
    const { url, options } = lastCall();
    expect(url).toBe("/api/v1/auth/identities/id-1");
    expect(options.method).toBe("DELETE");
  });

  it("encodes identity id path segment", async () => {
    await deleteOwnIdentity("a/b");
    expect(lastCall().url).toBe("/api/v1/auth/identities/a%2Fb");
  });

  it("lists own consents and tolerates an empty body", async () => {
    mockRequestDocumentJson.mockResolvedValue([
      { id: "c1", applicationId: "app1", grantedScopes: ["openid"], createdAt: "2026-01-01" },
    ]);
    const items = await listOwnConsents();
    expect(lastCall().url).toBe("/api/v1/auth/consents");
    expect(items).toHaveLength(1);
    expect(items[0].applicationId).toBe("app1");
  });

  it("returns [] when consents response is undefined", async () => {
    mockRequestDocumentJson.mockResolvedValue(undefined);
    expect(await listOwnConsents()).toEqual([]);
  });
});
