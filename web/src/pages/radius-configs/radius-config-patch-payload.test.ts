import { describe, expect, it } from "vitest";
import {
  buildRadiusConfigPatchPayload,
  type RadiusConfigFormValues,
} from "./radius-config-patch-payload";

const baseValues: RadiusConfigFormValues = {
  host: "radius.example.com",
  port: 1812,
  sharedSecret: "",
  isEnabled: true,
};

const requestBody = (payload: unknown) => JSON.stringify(payload);

describe("buildRadiusConfigPatchPayload", () => {
  it("omits sharedSecret when the input was typed and then cleared", () => {
    const payload = buildRadiusConfigPatchPayload({ ...baseValues, sharedSecret: "" }, 7);

    expect(payload).not.toHaveProperty("sharedSecret");
    expect(requestBody(payload)).not.toContain("sharedSecret");
  });

  it("treats a whitespace-only sharedSecret as blank", () => {
    const payload = buildRadiusConfigPatchPayload({ ...baseValues, sharedSecret: "   " }, 7);

    expect(payload).not.toHaveProperty("sharedSecret");
    expect(requestBody(payload)).not.toContain("sharedSecret");
  });

  it("omits sharedSecret when the field was never touched", () => {
    const payload = buildRadiusConfigPatchPayload({ ...baseValues, sharedSecret: undefined }, 7);

    expect(requestBody(payload)).not.toContain("sharedSecret");
  });

  it("sends a trimmed replacement only when a new secret was typed", () => {
    const payload = buildRadiusConfigPatchPayload(
      { ...baseValues, sharedSecret: "  new-secret  " },
      7,
    );

    expect(payload).toMatchObject({
      host: "radius.example.com",
      port: 1812,
      sharedSecret: "new-secret",
      isEnabled: true,
      revision: 7,
    });
  });
});
