import { describe, it, expect } from "vitest";

// MFA method types and configuration
const supportedMfaTypes = new Set(["totp", "sms", "email", "push", "webauthn"]);

function isMfaTypeSupported(type: string): boolean {
  return supportedMfaTypes.has(type);
}

function getMfaMethodLabel(type: string): string {
  const labels: Record<string, string> = {
    totp: "Authenticator App",
    sms: "SMS Verification",
    email: "Email Verification",
    push: "Push Notification",
    webauthn: "Security Key",
  };
  return labels[type] ?? type;
}

// MFA challenge state machine
type MfaChallengeState = "pending" | "push_pending" | "approved" | "rejected";

const mfaValidTransitions: Record<MfaChallengeState, MfaChallengeState[]> = {
  pending: ["push_pending", "approved", "rejected"],
  push_pending: ["approved", "rejected"],
  approved: [],
  rejected: [],
};

function canMfaTransition(from: MfaChallengeState, to: MfaChallengeState): boolean {
  return mfaValidTransitions[from].includes(to);
}

describe("MFA types", () => {
  it("recognizes supported MFA types", () => {
    expect(isMfaTypeSupported("totp")).toBe(true);
    expect(isMfaTypeSupported("sms")).toBe(true);
    expect(isMfaTypeSupported("webauthn")).toBe(true);
  });

  it("rejects unsupported MFA types", () => {
    expect(isMfaTypeSupported("invalid")).toBe(false);
    expect(isMfaTypeSupported("")).toBe(false);
  });
});

describe("getMfaMethodLabel", () => {
  it("returns human-readable labels", () => {
    expect(getMfaMethodLabel("totp")).toBe("Authenticator App");
    expect(getMfaMethodLabel("sms")).toBe("SMS Verification");
    expect(getMfaMethodLabel("webauthn")).toBe("Security Key");
  });

  it("returns type as fallback", () => {
    expect(getMfaMethodLabel("unknown")).toBe("unknown");
  });
});

describe("MFA challenge state machine", () => {
  it("pending → approved", () => {
    expect(canMfaTransition("pending", "approved")).toBe(true);
  });

  it("pending → rejected", () => {
    expect(canMfaTransition("pending", "rejected")).toBe(true);
  });

  it("pending → push_pending", () => {
    expect(canMfaTransition("pending", "push_pending")).toBe(true);
  });

  it("push_pending → approved", () => {
    expect(canMfaTransition("push_pending", "approved")).toBe(true);
  });

  it("approved is terminal", () => {
    expect(canMfaTransition("approved", "pending")).toBe(false);
    expect(canMfaTransition("approved", "rejected")).toBe(false);
  });

  it("rejected is terminal", () => {
    expect(canMfaTransition("rejected", "pending")).toBe(false);
    expect(canMfaTransition("rejected", "approved")).toBe(false);
  });
});
