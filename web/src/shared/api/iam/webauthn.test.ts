// oxlint-disable vitest/require-mock-type-parameters -- 测试替身按最小形状 mock，泛型参数会迫使测试引入实现细节
import { describe, it, expect, beforeEach, vi } from "vitest";
import { webauthnApi } from "./webauthn";
import * as httpModule from "@/shared/api/http";

// 用 vi 手动 mock 模块 — vitest 的 hoisting 会自动提升
const mockRequest = vi.fn();

vi.mock("@/shared/api/http", () => ({
  requestDocumentJson: (...args: unknown[]) => mockRequest(...args),
}));

describe("webauthnApi", () => {
  beforeEach(() => {
    mockRequest.mockReset();
  });

  describe("registrationBegin", () => {
    it("POSTs correct payload shape", async () => {
      mockRequest.mockResolvedValue({
        challenge: "Y2hhbGxlbmdl",
        rpId: "example.com",
        rpName: "Example",
        userId: "dXNlcmlk",
        challengeId: "uuid-123",
        userDisplayName: "Test User",
        timeout: 60000,
      });

      const result = await webauthnApi.registrationBegin({
        organizationSlug: "built-in",
        username: "admin",
      });

      expect(mockRequest).toHaveBeenCalledWith("/api/v1/auth/webauthn/registration/begin", {
        method: "POST",
        body: JSON.stringify({
          organizationSlug: "built-in",
          username: "admin",
        }),
      });
      expect(result.challenge).toBeTruthy();
      expect(result.rpId).toBeTruthy();
    });
  });

  describe("registrationFinish", () => {
    it("POSTs all base64url fields", async () => {
      mockRequest.mockResolvedValue({ status: "ok", credentialId: "abc123" });

      const result = await webauthnApi.registrationFinish({
        organizationSlug: "built-in",
        username: "admin",
        challengeId: "uuid-123",
        challenge: "base64-challenge",
        credentialId: "base64-cred",
        publicKey: "base64-pub",
        clientDataJSON: "base64-cd",
        authenticatorData: "base64-ad",
        attestationObject: "base64-ao",
        attestationType: "none",
        transports: ["internal"],
      });

      const [, options] = mockRequest.mock.calls[0];
      const body = JSON.parse((options as { body: string }).body);
      expect(body).toMatchObject({
        organizationSlug: "built-in",
        username: "admin",
        challengeId: "uuid-123",
        credentialId: "base64-cred",
      });
      expect(result.status).toBe("ok");
    });
  });

  describe("authenticationBegin", () => {
    it("POSTs organization + username", async () => {
      mockRequest.mockResolvedValue({
        challenge: "auth-challenge",
        rpId: "example.com",
        challengeId: "auth-uuid",
        timeout: 30000,
      });

      await webauthnApi.authenticationBegin({
        organizationSlug: "built-in",
        username: "admin",
      });

      const [, options] = mockRequest.mock.calls[0];
      const body = JSON.parse((options as { body: string }).body);
      expect(body.username).toBe("admin");
    });
  });

  describe("authenticationFinish", () => {
    it("POSTs signature + authenticatorData + clientDataJSON", async () => {
      mockRequest.mockResolvedValue({ status: "ok" });

      await webauthnApi.authenticationFinish({
        organizationSlug: "built-in",
        username: "admin",
        challengeId: "uuid-123",
        challenge: "challenge",
        credentialId: "cred",
        signature: "base64-sig",
        authenticatorData: "base64-ad",
        clientDataJSON: "base64-cd",
      });

      const [, options] = mockRequest.mock.calls[0];
      const body = JSON.parse((options as { body: string }).body);
      expect(body.signature).toBe("base64-sig");
      expect(body.authenticatorData).toBe("base64-ad");
    });
  });
});

// 防止 TS 报未使用 import（vi.mock 里用不到）
void httpModule;
