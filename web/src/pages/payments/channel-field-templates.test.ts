import { describe, expect, it } from "vitest";
import { buildChannelConfigPayload, validateChannelConfigText } from "./channel-config-payload";
import {
  CHANNEL_FIELD_TEMPLATES,
  assembleChannelTemplateConfig,
  getChannelTemplate,
} from "./channel-field-templates";

describe("getChannelTemplate", () => {
  it("provides structured templates for square, checkout and wechat_pay", () => {
    expect(getChannelTemplate("square")?.map((f) => f.key)).toEqual([
      "accessToken",
      "locationId",
      "webhookKey",
      "apiVersion",
    ]);
    expect(getChannelTemplate("checkout")?.map((f) => f.key)).toEqual(["apiKey", "webhookSecret"]);
    expect(getChannelTemplate("wechat_pay")?.map((f) => f.key)).toEqual([
      "appId",
      "mchId",
      "serialNo",
      "privateKey",
      "platformCertPEM",
    ]);
  });

  it("keeps other channels on the raw JSON editor", () => {
    for (const channel of ["stripe", "alipay", "paypal", "dummy", "", undefined]) {
      expect(getChannelTemplate(channel)).toBeUndefined();
    }
  });

  it("marks exactly the factory-required keys as required", () => {
    // internal/infra/payment/factory.go: square accessToken+locationId,
    // checkout apiKey, wechat_pay all five keys.
    const required = (channel: string) =>
      CHANNEL_FIELD_TEMPLATES[channel].filter((f) => f.required).map((f) => f.key);
    expect(required("square")).toEqual(["accessToken", "locationId"]);
    expect(required("checkout")).toEqual(["apiKey"]);
    expect(required("wechat_pay")).toEqual([
      "appId",
      "mchId",
      "serialNo",
      "privateKey",
      "platformCertPEM",
    ]);
  });
});

describe("assembleChannelTemplateConfig", () => {
  const square = getChannelTemplate("square")!;

  it("assembles required fields and omits empty optional ones", () => {
    const json = assembleChannelTemplateConfig(square, "production", {
      accessToken: "EAAA-token",
      locationId: "L123",
      webhookKey: "  ",
      apiVersion: undefined,
    });
    expect(JSON.parse(json)).toEqual({ accessToken: "EAAA-token", locationId: "L123" });
  });

  it("injects isSandbox=true only for sandbox environments", () => {
    const values = { accessToken: "t", locationId: "L" };
    expect(JSON.parse(assembleChannelTemplateConfig(square, "sandbox", values))).toEqual({
      accessToken: "t",
      locationId: "L",
      isSandbox: "true",
    });
    expect(
      JSON.parse(assembleChannelTemplateConfig(square, "production", values)),
    ).not.toHaveProperty("isSandbox");
  });

  it("trims values but preserves PEM interior newlines", () => {
    const pem = "-----BEGIN PRIVATE KEY-----\nMIIE\n-----END PRIVATE KEY-----";
    const json = assembleChannelTemplateConfig(getChannelTemplate("wechat_pay")!, "production", {
      appId: " wx1 ",
      mchId: "1900000109",
      serialNo: "ABC",
      privateKey: `\n${pem}\n`,
      platformCertPEM: pem,
    });
    const config = JSON.parse(json) as Record<string, string>;
    expect(config.appId).toBe("wx1");
    expect(config.privateKey).toBe(pem);
  });

  it("refuses to assemble when a required field is missing", () => {
    expect(() => assembleChannelTemplateConfig(square, "sandbox", { accessToken: "t" })).toThrow(
      /locationId/,
    );
  });

  it("feeds the existing payload path with valid plaintext JSON", () => {
    const config = assembleChannelTemplateConfig(square, "sandbox", {
      accessToken: "t",
      locationId: "L",
    });
    expect(validateChannelConfigText(config)).toBeNull();
    const payload = buildChannelConfigPayload({
      channel: "square",
      environment: "sandbox",
      config,
    });
    expect(payload.config).toEqual({ accessToken: "t", locationId: "L", isSandbox: "true" });
  });
});
