import { describe, expect, it } from "vitest";
import {
  buildFaceIdProviderPatchPayload,
  type FaceIdProviderFormValues,
} from "./face-id-provider-patch-payload";

const baseValues: FaceIdProviderFormValues = {
  accessKeyId: "ak-1",
  accessKeySecret: "",
  endpoint: "https://face.example.com",
  isEnabled: true,
};

const requestBody = (payload: unknown) => JSON.stringify(payload);

describe("buildFaceIdProviderPatchPayload", () => {
  it("omits accessKeySecret when the input was typed and then cleared", () => {
    const payload = buildFaceIdProviderPatchPayload({ ...baseValues, accessKeySecret: "" }, 7);

    expect(payload).not.toHaveProperty("accessKeySecret");
    expect(requestBody(payload)).not.toContain("accessKeySecret");
  });

  it("treats a whitespace-only accessKeySecret as blank", () => {
    const payload = buildFaceIdProviderPatchPayload({ ...baseValues, accessKeySecret: "   " }, 7);

    expect(payload).not.toHaveProperty("accessKeySecret");
    expect(requestBody(payload)).not.toContain("accessKeySecret");
  });

  it("omits accessKeySecret when the field was never touched", () => {
    const payload = buildFaceIdProviderPatchPayload(
      { ...baseValues, accessKeySecret: undefined },
      7,
    );

    expect(requestBody(payload)).not.toContain("accessKeySecret");
  });

  it("sends a trimmed replacement only when a new secret was typed", () => {
    const payload = buildFaceIdProviderPatchPayload(
      { ...baseValues, accessKeySecret: "  new-secret  " },
      7,
    );

    expect(payload).toMatchObject({
      accessKeyId: "ak-1",
      accessKeySecret: "new-secret",
      endpoint: "https://face.example.com",
      isEnabled: true,
      revision: 7,
    });
  });
});
