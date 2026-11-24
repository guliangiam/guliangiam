import { beforeEach, describe, expect, it, vi } from "vitest";
import { getCaptcha, getCaptchaClientSecret, getCaptchaStatus } from "./auth-captcha-shim";

const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/http", () => ({
  requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
}));

function lastUrl(): string {
  return String(mockRequestDocumentJson.mock.calls.at(-1)?.[0] ?? "");
}

describe("getCaptcha", () => {
  beforeEach(() => {
    mockRequestDocumentJson.mockReset();
  });

  // The legacy console called legacy `/api/get-captcha?applicationId=`, which
  // this server never registered, so the login captcha never loaded.
  it("gets the v1 captcha for an organization", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      captchaId: "c-1",
      captchaImage: "data:image/png;base64,AAA",
      type: "default",
    });

    await getCaptcha({ organizationId: "built-in" });

    expect(lastUrl()).toBe("/api/v1/auth/captcha?organizationId=built-in");
  });

  // `request()` parses legacy `{status, msg, data}` envelope, so it read
  // `undefined` on a v1 bare document; `requestDocumentJson` reads it directly.
  it("returns the document fields without unwrapping an envelope", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      captchaId: "c-2",
      captchaImage: "img",
      type: "recaptcha",
    });

    const state = await getCaptcha({ organizationId: "built-in" });

    expect(state.captchaId).toBe("c-2");
    expect(state.type).toBe("recaptcha");
  });
});

describe("getCaptchaClientSecret", () => {
  // The token to submit is the challenge id. legacy clientId/clientSecret/
  // clientId2/clientSecret2 do not exist on the v1 response, so the old
  // implementation always produced an empty string.
  it("returns the challenge id", () => {
    expect(getCaptchaClientSecret({ captchaId: "c-3" })).toBe("c-3");
  });

  it("returns an empty string when there is no captcha state", () => {
    expect(getCaptchaClientSecret(null)).toBe("");
  });
});

describe("getCaptchaStatus", () => {
  beforeEach(() => {
    mockRequestDocumentJson.mockReset();
  });

  it("reads the bare document's requirement and rendering fields", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      required: true,
      providerType: "recaptcha",
      siteKey: "site-key-1",
    });

    const status = await getCaptchaStatus({ organizationId: "built-in" });

    expect(status.required).toBe(true);
    expect(status.providerType).toBe("recaptcha");
    expect(status.siteKey).toBe("site-key-1");
  });

  it("treats a missing document as not required", async () => {
    mockRequestDocumentJson.mockResolvedValue(undefined);

    const status = await getCaptchaStatus({ organizationId: "built-in" });

    expect(status.required).toBeUndefined();
  });

  it("omits organizationId when it is empty", async () => {
    mockRequestDocumentJson.mockResolvedValue({ required: false });

    await getCaptchaStatus({ organizationId: "" });

    expect(lastUrl()).toBe("/api/v1/auth/captcha-status?");
  });
});
