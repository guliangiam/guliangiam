import { describe, it, expect, vi } from "vitest";
import {
  createSamlPendingMfaVerifyPayload,
  verifySamlPendingMfa,
  verifyPendingMfa,
  type PendingMfaVerifyValues,
} from "./auth-mfa-verify";

// Mock i18next
// i18n is consumed through the named `t` export; keep default.t for legacy call sites.
vi.mock("i18next", () => {
  const t = (key: string) => key;
  return { default: { t }, t };
});

// Mock loginWithPassword
vi.mock("@/shared/api/iam/auth", () => ({
  loginWithPassword: vi.fn<(...args: unknown[]) => unknown>(),
}));

// Mock auth-mfa-setup
vi.mock("@/shared/lib/auth-mfa-setup", () => ({
  toVerificationMfaType: (mfaType: string) => mfaType,
}));

// Mock auth-challenge
vi.mock("@/shared/lib/auth-challenge", () => ({
  submitCasChallenge: vi.fn<(...args: unknown[]) => unknown>(),
}));

describe("createSamlPendingMfaVerifyPayload", () => {
  type TestPayload = { organization: string; username: string };

  it("merges loginValues with verifyValues", () => {
    const loginValues: TestPayload = { organization: "org-1", username: "admin" };
    const verifyValues: PendingMfaVerifyValues = {
      mfaType: "totp",
      passcode: "123456",
      recoveryCode: "ABCD",
      enableMfaRemember: true,
    };

    const payload = createSamlPendingMfaVerifyPayload({ loginValues, verifyValues });

    expect(payload.organization).toBe("org-1");
    expect(payload.username).toBe("admin");
    expect(payload.mfaType).toBe("totp");
    expect(payload.passcode).toBe("123456");
    expect(payload.recoveryCode).toBe("ABCD");
    expect(payload.enableMfaRemember).toBe(true);
  });

  it("defaults passcode to empty string", () => {
    const loginValues: TestPayload = { organization: "org-1", username: "admin" };
    const verifyValues: PendingMfaVerifyValues = { mfaType: "totp" };

    const payload = createSamlPendingMfaVerifyPayload({ loginValues, verifyValues });

    expect(payload.passcode).toBe("");
  });

  it("defaults recoveryCode to empty string", () => {
    const loginValues: TestPayload = { organization: "org-1", username: "admin" };
    const verifyValues: PendingMfaVerifyValues = { mfaType: "totp" };

    const payload = createSamlPendingMfaVerifyPayload({ loginValues, verifyValues });

    expect(payload.recoveryCode).toBe("");
  });

  it("defaults enableMfaRemember to false", () => {
    const loginValues: TestPayload = { organization: "org-1", username: "admin" };
    const verifyValues: PendingMfaVerifyValues = { mfaType: "totp" };

    const payload = createSamlPendingMfaVerifyPayload({ loginValues, verifyValues });

    expect(payload.enableMfaRemember).toBe(false);
  });

  it("preserves all loginValues properties", () => {
    const loginValues = { organization: "org-1", username: "admin", extra: "value" };
    const verifyValues: PendingMfaVerifyValues = { mfaType: "sms" };

    const payload = createSamlPendingMfaVerifyPayload({ loginValues, verifyValues });

    expect(payload).toHaveProperty("extra", "value");
  });
});

describe("verifySamlPendingMfa", () => {
  it("creates payload and calls submitCasChallenge", async () => {
    const { submitCasChallenge } = await import("@/shared/lib/auth-challenge");
    const mockSubmitCasChallenge = vi.mocked(submitCasChallenge);
    mockSubmitCasChallenge.mockResolvedValue(undefined);

    type TestPayload = { organization: string; username: string };
    const loginValues: TestPayload = { organization: "org-1", username: "admin" };
    const verifyValues: PendingMfaVerifyValues = {
      mfaType: "totp",
      passcode: "123456",
    };

    const submit = vi
      .fn<(...args: unknown[]) => Promise<unknown>>()
      .mockResolvedValue({ status: "ok" });
    const onPendingMfa = vi.fn<(...args: unknown[]) => unknown>();
    const showInfo = vi.fn<(...args: unknown[]) => unknown>();
    const onComplete = vi
      .fn<(...args: unknown[]) => Promise<unknown>>()
      .mockResolvedValue(undefined);

    await verifySamlPendingMfa({
      loginValues,
      verifyValues,
      submit,
      onPendingMfa,
      showInfo,
      onComplete,
    });

    expect(mockSubmitCasChallenge).toHaveBeenCalled();
    const callArgs = mockSubmitCasChallenge.mock.calls[0][0] as {
      payload: { mfaType: string; passcode: string };
    };
    expect(callArgs.payload.mfaType).toBe("totp");
    expect(callArgs.payload.passcode).toBe("123456");
  });
});

describe("verifyPendingMfa", () => {
  it("throws error when challengeId is missing", async () => {
    type TestPayload = {
      organization: string;
      username: string;
      application: string;
      challengeId?: string;
    };

    const pendingMfa = {
      source: "auth" as const,
      loginValues: {
        organization: "org-1",
        username: "admin",
        application: "app-1",
        password: "test-password",
        // challengeId is intentionally omitted to test the error path
      } as TestPayload,
      mfaProps: [] as Array<{ methodType: string; enabled: boolean; verified: boolean }>,
      challengeState: "mfa_required" as const,
    };

    await expect(
      verifyPendingMfa({
        pendingMfa: pendingMfa as Parameters<typeof verifyPendingMfa>[0]["pendingMfa"],
        verifyValues: { mfaType: "totp", passcode: "123456" },
        createTransition: vi.fn<(...args: unknown[]) => unknown>(),
        applyTransition: vi.fn<(...args: unknown[]) => unknown>(),
        setPendingMfa: vi.fn<(...args: unknown[]) => unknown>(),
        showInfo: vi.fn<(...args: unknown[]) => unknown>(),
      }),
    ).rejects.toThrow(/mfa/i);
  });

  it("calls verifyImpl with correct parameters", async () => {
    const mockVerifyImpl = vi
      .fn<(...args: unknown[]) => Promise<unknown>>()
      .mockResolvedValue({ status: "success" });
    const mockCreateTransition = vi
      .fn<(...args: unknown[]) => unknown>()
      .mockReturnValue({ kind: "success" });
    const mockApplyTransition = vi
      .fn<(...args: unknown[]) => Promise<unknown>>()
      .mockResolvedValue(undefined);

    type TestPayload = {
      organization: string;
      username: string;
      application: string;
      challengeId?: string;
    };

    const pendingMfa = {
      source: "auth" as const,
      loginValues: {
        organization: "org-1",
        username: "admin",
        application: "app-1",
        challengeId: "challenge-1",
        password: "test-password",
      } as TestPayload,
      mfaProps: [] as Array<{ methodType: string; enabled: boolean; verified: boolean }>,
      challengeState: "mfa_required" as const,
    };

    await verifyPendingMfa({
      pendingMfa: pendingMfa as Parameters<typeof verifyPendingMfa>[0]["pendingMfa"],
      verifyValues: {
        mfaType: "totp",
        passcode: "654321",
        recoveryCode: "XYZ",
        enableMfaRemember: true,
      },
      createTransition: mockCreateTransition,
      applyTransition: mockApplyTransition,
      setPendingMfa: vi.fn<(...args: unknown[]) => unknown>(),
      showInfo: vi.fn<(...args: unknown[]) => unknown>(),
      verifyImpl: mockVerifyImpl,
    });

    expect(mockVerifyImpl).toHaveBeenCalledWith({
      challengeId: "challenge-1",
      mfaType: "totp",
      passcode: "654321",
      recoveryCode: "XYZ",
      enableMfaRemember: true,
    });
  });
});

describe("PendingMfaVerifyValues defaults", () => {
  it("allows optional passcode and recoveryCode", () => {
    const values: PendingMfaVerifyValues = { mfaType: "totp" };
    expect(values.passcode).toBeUndefined();
    expect(values.recoveryCode).toBeUndefined();
    expect(values.enableMfaRemember).toBeUndefined();
  });

  it("supports all MFA types", () => {
    const mfaTypes = ["totp", "sms", "email", "recovery"];
    for (const mfaType of mfaTypes) {
      const values: PendingMfaVerifyValues = { mfaType };
      expect(values.mfaType).toBe(mfaType);
    }
  });
});
