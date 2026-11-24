import { beforeEach, describe, expect, it, vi } from "vitest";
import { sendVerificationCode } from "./auth-verification-shim";

const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/http", () => ({
  requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
}));

function lastUrl(): string {
  return String(mockRequestDocumentJson.mock.calls.at(-1)?.[0] ?? "");
}

function lastBody(): Record<string, unknown> {
  const init = mockRequestDocumentJson.mock.calls.at(-1)?.[1] as { body?: string } | undefined;
  return JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
}

describe("sendVerificationCode", () => {
  beforeEach(() => {
    mockRequestDocumentJson.mockReset();
    mockRequestDocumentJson.mockResolvedValue({ id: "v1" });
  });

  // The legacy console posted to legacy `/api/send-verification-code`, which
  // this server never registered, so every "send code" button 404'd.
  it("posts to the v1 verification-code endpoint", async () => {
    await sendVerificationCode({
      organizationId: "built-in",
      dest: "user@example.com",
      type: "email",
    });

    expect(lastUrl()).toBe("/api/v1/auth/verification-code/send");
  });

  // The old call sent FormData, which the v1 handler cannot decode: it reads a
  // JSON document of {organizationId, type, recipient}.
  it("sends a JSON document with organizationId/type/recipient", async () => {
    await sendVerificationCode({
      organizationId: "built-in",
      dest: "user@example.com",
      type: "email",
    });

    expect(lastBody()).toEqual({
      organizationId: "built-in",
      type: "email",
      recipient: "user@example.com",
    });
  });

  // `type` must be "phone". The old code path spelled the phone case "sms",
  // which the handler rejects with 400 invalid_argument.
  it('spells the phone case "phone", not "sms"', async () => {
    await sendVerificationCode({
      organizationId: "built-in",
      dest: "13800000000",
      type: "phone",
      countryCode: "CN",
    });

    expect(lastBody().type).toBe("phone");
  });

  it("converts an ISO country code into the calling code", async () => {
    await sendVerificationCode({
      organizationId: "built-in",
      dest: "13800000000",
      type: "phone",
      countryCode: "CN",
    });

    expect(lastBody().recipient).toBe("+8613800000000");
  });

  it("still accepts a legacy dial prefix for pending flows", async () => {
    await sendVerificationCode({
      organizationId: "built-in",
      dest: "13800000000",
      type: "phone",
      countryCode: "+86",
    });

    expect(lastBody().recipient).toBe("+8613800000000");
  });

  it("rejects a local phone without a usable country/dial prefix", async () => {
    await expect(
      sendVerificationCode({
        organizationId: "built-in",
        dest: "13800000000",
        type: "phone",
      }),
    ).rejects.toThrow("Invalid phone number");
    expect(mockRequestDocumentJson).not.toHaveBeenCalled();
  });

  it("keeps an already-E.164 recipient", async () => {
    await sendVerificationCode({
      organizationId: "built-in",
      dest: "+8613800000000",
      type: "phone",
      countryCode: "US",
    });

    expect(lastBody().recipient).toBe("+8613800000000");
  });

  it("leaves an email recipient untouched", async () => {
    await sendVerificationCode({
      organizationId: "built-in",
      dest: "user@example.com",
      type: "email",
      countryCode: "+86",
    });

    expect(lastBody().recipient).toBe("user@example.com");
  });
});
