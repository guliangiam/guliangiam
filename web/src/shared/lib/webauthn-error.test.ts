import { describe, expect, it } from "vitest";

import { HttpError } from "@/shared/api/http";
import { registrationErrorMessageKey } from "./webauthn-error";

describe("registrationErrorMessageKey", () => {
  it("maps 410 to the restart copy", () => {
    expect(registrationErrorMessageKey(new HttpError(410, "gone"))).toBe(
      "user.webauthn.challengeExpired",
    );
  });

  it("maps 422 to the verification copy", () => {
    expect(registrationErrorMessageKey(new HttpError(422, "invalid"))).toBe(
      "user.webauthn.verificationFailed",
    );
  });

  it("maps 409 to the duplicate copy", () => {
    expect(registrationErrorMessageKey(new HttpError(409, "exists"))).toBe(
      "user.webauthn.duplicate",
    );
  });

  it("maps authenticator cancellation to the cancelled copy", () => {
    expect(registrationErrorMessageKey(new DOMException("cancel", "NotAllowedError"))).toBe(
      "user.webauthn.registerCancelled",
    );
  });

  it("falls back to the generic failure copy", () => {
    expect(registrationErrorMessageKey(new Error("boom"))).toBe("user.webauthn.registerFailed");
  });
});
