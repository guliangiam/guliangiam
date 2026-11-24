import { describe, expect, it } from "vitest";
import {
  MAX_CHANNEL_CONFIG_BYTES,
  buildChannelConfigPayload,
  validateChannelConfigText,
  type ChannelConfigFormValues,
} from "./channel-config-payload";

const baseValues: ChannelConfigFormValues = {
  channel: "stripe",
  environment: "sandbox",
  config: '{"appId":"acct_1","privateKey":"sk_test"}',
};

const requestBody = (payload: unknown) => JSON.stringify(payload);

describe("validateChannelConfigText", () => {
  it("requires a value", () => {
    expect(validateChannelConfigText("")).toBe("required");
    expect(validateChannelConfigText("   ")).toBe("required");
    expect(validateChannelConfigText(undefined)).toBe("required");
  });

  it("accepts a non-empty JSON object", () => {
    expect(validateChannelConfigText(baseValues.config)).toBeNull();
    expect(validateChannelConfigText('{"nested":{"a":1},"list":[1,2]}')).toBeNull();
  });

  it("rejects non-objects, empty objects and malformed JSON", () => {
    for (const value of ["null", "[]", '"text"', "42", "true", "{}", "{bad"]) {
      expect(validateChannelConfigText(value)).toBe("invalid");
    }
  });

  it("rejects plaintext over the server limit", () => {
    const oversized = `{"a":"${"x".repeat(MAX_CHANNEL_CONFIG_BYTES)}"}`;
    expect(validateChannelConfigText(oversized)).toBe("tooLarge");
  });

  it("measures UTF-8 bytes rather than UTF-16 length", () => {
    const multibyte = `{"a":"${"中".repeat(MAX_CHANNEL_CONFIG_BYTES / 3 + 1)}"}`;
    expect(validateChannelConfigText(multibyte)).toBe("tooLarge");
  });
});

describe("buildChannelConfigPayload", () => {
  it("sends the plaintext object and never a sealed blob or client configSet", () => {
    const payload = buildChannelConfigPayload(baseValues);

    expect(payload).toEqual({
      channel: "stripe",
      environment: "sandbox",
      config: { appId: "acct_1", privateKey: "sk_test" },
    });
    expect(payload).not.toHaveProperty("configSealed");
    expect(payload).not.toHaveProperty("configSet");
    expect(requestBody(payload)).not.toContain("configSealed");
    expect(requestBody(payload)).toContain('"config":{"appId":"acct_1","privateKey":"sk_test"}');
  });

  it("trims surrounding whitespace", () => {
    const payload = buildChannelConfigPayload({
      ...baseValues,
      config: '  {"appId":"acct_1"}  ',
    });
    expect(payload.config).toEqual({ appId: "acct_1" });
  });

  it("refuses invalid text instead of sending it", () => {
    expect(() => buildChannelConfigPayload({ ...baseValues, config: "{}" })).toThrow(
      /invalid|required/,
    );
    expect(() => buildChannelConfigPayload({ ...baseValues, config: "" })).toThrow(
      /invalid channel config: required/,
    );
  });
});
