import { describe, expect, it } from "vitest";
import dayjs from "dayjs";
import { buildKeyPatchPayload, type KeyFormValues } from "./key-patch-payload";

const baseValues: KeyFormValues = {
  keyType: "Organization",
  algorithm: "RSA",
  publicKey: "pub",
  privateKey: "",
  displayName: "key",
  expiresAt: null,
  isEnabled: true,
};

// BE-B-6: the detail endpoint never returns the stored private key, so the
// form initializes it blank. PATCH semantics are "omit = keep, '' = clear":
// the blank init value must never be submitted, or every save wipes the key.
describe("buildKeyPatchPayload", () => {
  it("omits privateKey when the field is left blank", () => {
    const payload = buildKeyPatchPayload(baseValues, 7);
    expect(payload).not.toHaveProperty("privateKey");
  });

  it("omits privateKey when the field is whitespace only", () => {
    const payload = buildKeyPatchPayload({ ...baseValues, privateKey: "   " }, 7);
    expect(payload).not.toHaveProperty("privateKey");
  });

  it("sends privateKey only when the operator typed a replacement", () => {
    const payload = buildKeyPatchPayload({ ...baseValues, privateKey: "new-private-key" }, 7);
    expect(payload.privateKey).toBe("new-private-key");
  });

  it("carries the detail revision for optimistic locking (never hardcoded 0)", () => {
    expect(buildKeyPatchPayload(baseValues, 7).revision).toBe(7);
    expect(buildKeyPatchPayload(baseValues, 42).revision).toBe(42);
  });

  it("serializes expiresAt only when set", () => {
    const expiresAt = dayjs("2027-01-01T00:00:00Z");
    const payload = buildKeyPatchPayload({ ...baseValues, expiresAt }, 1);
    expect(payload.expiresAt).toBe(expiresAt.toISOString());
    expect(buildKeyPatchPayload(baseValues, 1)).not.toHaveProperty("expiresAt");
  });
});
