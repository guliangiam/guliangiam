import { describe, expect, it, vi } from "vitest";
import { runFaceIdLogin } from "./auth-signin-providers";

vi.mock("i18next", () => {
  const t = (key: string) => key;
  return { default: { t }, t };
});

const completeFaceIdLogin = vi.fn<(...args: unknown[]) => Promise<unknown>>();
vi.mock("@/shared/api/iam/auth-face-id", () => ({
  completeFaceIdLogin: (...args: unknown[]) => completeFaceIdLogin(...args),
}));

const requestDocumentJson = vi.fn<(...args: unknown[]) => Promise<unknown>>();
vi.mock("@/shared/api/http", () => ({
  requestDocumentJson: (...args: unknown[]) => requestDocumentJson(...args),
}));

async function runWithPayloadType(type: string) {
  completeFaceIdLogin.mockResolvedValue({ status: "success" });
  requestDocumentJson.mockResolvedValue({ verified: true, livenessPassed: true });
  const onSuccess = vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined);
  await runFaceIdLogin({
    owner: "org-1",
    username: "alice",
    useImageModal: true,
    loadFaceRecognitionCommonModalModule: vi.fn<() => Promise<unknown>>().mockResolvedValue({}),
    loadFaceRecognitionModalModule: vi.fn<() => Promise<unknown>>(),
    faceIdSigninBeginImpl: vi
      .fn<() => Promise<{ sessionId: string; challenge: string; expiresIn: number }>>()
      .mockResolvedValue({ sessionId: "s1", challenge: "challenge", expiresIn: 300 }),
    requestFaceRecognition: vi
      .fn<() => Promise<{ faceId?: number[]; faceIdImage?: string[] }>>()
      .mockResolvedValue({ faceId: [1], faceIdImage: ["img"] }),
    createPayload: () => ({ type }),
    onSuccess,
  });
  return { onSuccess };
}

describe("runFaceIdLogin protocol context", () => {
  it("does not send the console login sentinel as responseType", async () => {
    await runWithPayloadType("login");
    const args = completeFaceIdLogin.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(args).toMatchObject({ sessionId: expect.any(String) });
    expect(args.responseType).toBeUndefined();
  });

  it("keeps a real OAuth responseType", async () => {
    await runWithPayloadType("code");
    const args = completeFaceIdLogin.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(args.responseType).toBe("code");
  });
});
