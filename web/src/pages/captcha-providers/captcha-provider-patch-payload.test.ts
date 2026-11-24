import { describe, expect, it } from "vitest";
import {
  buildCaptchaConfigPatch,
  buildCaptchaProviderPatchPayload,
  type CaptchaProviderFormValues,
} from "./captcha-provider-patch-payload";

const baseValues: CaptchaProviderFormValues = {
  name: "captcha-1",
  siteKey: "site-key",
  secretKey: "",
  isEnabled: true,
};

const requestBody = (payload: unknown) => JSON.stringify(payload);

describe("buildCaptchaProviderPatchPayload", () => {
  it("omits secretKey when the input was typed and then cleared", () => {
    const payload = buildCaptchaProviderPatchPayload({ ...baseValues, secretKey: "" }, 7);

    expect(payload).not.toHaveProperty("secretKey");
    expect(requestBody(payload)).not.toContain("secretKey");
  });

  it("treats a whitespace-only secretKey as blank", () => {
    const payload = buildCaptchaProviderPatchPayload({ ...baseValues, secretKey: "   " }, 7);

    expect(payload).not.toHaveProperty("secretKey");
    expect(requestBody(payload)).not.toContain("secretKey");
  });

  it("omits secretKey when the field was never touched", () => {
    const payload = buildCaptchaProviderPatchPayload({ ...baseValues, secretKey: undefined }, 7);

    expect(requestBody(payload)).not.toContain("secretKey");
  });

  it("sends a trimmed replacement only when a new secret was typed", () => {
    const payload = buildCaptchaProviderPatchPayload(
      { ...baseValues, secretKey: "  new-secret  " },
      7,
    );

    expect(payload).toMatchObject({
      name: "captcha-1",
      siteKey: "site-key",
      secretKey: "new-secret",
      isEnabled: true,
      revision: 7,
    });
  });

  it("sends every config key of the kind so mergeConfig does not drop stored values", () => {
    const payload = buildCaptchaProviderPatchPayload(
      { ...baseValues, captchaCamSecretId: "AKID-new", captchaCamSecretKey: "" },
      3,
      "tencent_captcha",
    );

    // password 类：留空 = 保持原值（后端 mergeConfig 对空密钥保留既有值）；
    // 未出现的键会被整体替换语义删除，所以两个 CAM 键都必须发送。
    expect(payload.config).toEqual({ camSecretId: "AKID-new", camSecretKey: "" });
  });

  it("omits untouched number fields and clears untouched text fields", () => {
    expect(buildCaptchaConfigPatch("recaptcha_enterprise", { captchaProjectId: "p-1" })).toEqual({
      projectId: "p-1",
      action: "",
    });
  });

  it("leaves config out entirely for kinds without extension fields", () => {
    const payload = buildCaptchaProviderPatchPayload(baseValues, 2, "geetest");
    expect(payload).not.toHaveProperty("config");
  });
});
