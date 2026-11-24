import { HttpError } from "@/shared/api/http";

/**
 * Maps admin on-behalf passkey registration failures to i18n keys.
 * 410: challenge expired/already consumed → the operator must restart.
 * 422: attestation/clientData verification failed.
 * 409: credential already registered.
 */
export function registrationErrorMessageKey(err: unknown): string {
  if (err instanceof HttpError) {
    if (err.status === 410) return "user.webauthn.challengeExpired";
    if (err.status === 422) return "user.webauthn.verificationFailed";
    if (err.status === 409) return "user.webauthn.duplicate";
  }
  if (
    typeof DOMException !== "undefined" &&
    err instanceof DOMException &&
    err.name === "NotAllowedError"
  ) {
    return "user.webauthn.registerCancelled";
  }
  return "user.webauthn.registerFailed";
}
