import { describe, expect, it } from "vitest";
import { buildWebhookPatchPayload, type WebhookFormValues } from "./webhook-patch-payload";

const baseValues: WebhookFormValues = {
  displayName: "hook",
  organization: "org",
  url: "https://example.com/hook",
  method: "POST",
  contentType: "application/json",
  headers: '{"X-Token":"abc"}',
  events: ["user.created"],
  customBody: "",
  customHeaders: "",
  certId: "",
  secret: "",
  isUserExtended: false,
  tokenFields: [],
  singleOrgOnly: false,
  objectFields: [],
  maxRetries: 3,
  retryInterval: 60,
  useExponentialBackoff: false,
  timeoutMs: 5000,
  isEnabled: true,
};

// BE-B-7: reads only return the masked secret projection, so the form starts
// blank. PATCH semantics are "omit = keep, non-nil '' = clear": the blank init
// value must never be submitted, or every save wipes the signing secret.
describe("buildWebhookPatchPayload", () => {
  it("omits secret when the field is left blank", () => {
    const payload = buildWebhookPatchPayload(baseValues, 5);
    expect(payload).not.toHaveProperty("secret");
  });

  it("omits secret when the field is whitespace only", () => {
    const payload = buildWebhookPatchPayload({ ...baseValues, secret: "  " }, 5);
    expect(payload).not.toHaveProperty("secret");
  });

  it("sends secret only when the operator typed a replacement", () => {
    const payload = buildWebhookPatchPayload({ ...baseValues, secret: "whsec_new" }, 5);
    expect(payload.secret).toBe("whsec_new");
  });

  it("round-trips the detail headers value instead of clearing it", () => {
    const payload = buildWebhookPatchPayload(baseValues, 5);
    expect(payload.headers).toBe('{"X-Token":"abc"}');
  });

  it("carries the detail revision for optimistic locking (never hardcoded 0)", () => {
    expect(buildWebhookPatchPayload(baseValues, 5).revision).toBe(5);
    expect(buildWebhookPatchPayload(baseValues, 11).revision).toBe(11);
  });
});
