import type { TFunction } from "i18next";
import { getErrorDetails, isHttpError } from "@/shared/api/http";

export type PasskeyMode = "login" | "register";

// 后端 details.reason → 语言包 key。未知 reason 一律回落到 Passkey 专属文案，
// 禁止再落到 common:serverError.validation（那是表单校验的文案）。
const PASSKEY_REASON_KEYS = {
  challenge_expired: "auth:passkey.reason.challenge_expired",
  challenge_mismatch: "auth:passkey.reason.challenge_mismatch",
  credential_not_found: "auth:passkey.reason.credential_not_found",
  credential_inactive: "auth:passkey.reason.credential_inactive",
  origin_not_allowed: "auth:passkey.reason.origin_not_allowed",
  rp_id_mismatch: "auth:passkey.reason.rp_id_mismatch",
  user_presence_required: "auth:passkey.reason.user_presence_required",
  user_verification_required: "auth:passkey.reason.user_verification_required",
  signature_invalid: "auth:passkey.reason.signature_invalid",
  malformed_assertion: "auth:passkey.reason.malformed_assertion",
  sign_count_regression: "auth:passkey.reason.sign_count_regression",
  assertion_invalid: "auth:passkey.reason.assertion_invalid",
} as const;

function passkeyReason(err: unknown): string | undefined {
  const fromDetails = getErrorDetails(err)?.reason;
  if (typeof fromDetails === "string") {
    return fromDetails;
  }
  // WebAuthn 端点走 OAuth 协议面：后端把 reason 以稳定标记追加在 description 里。
  if (!isHttpError(err) || typeof err.body !== "object" || err.body === null) {
    return undefined;
  }
  const description = (err.body as { error_description?: unknown }).error_description;
  if (typeof description !== "string") {
    return undefined;
  }
  return /\[reason:\s*([a-z_]+)\]/.exec(description)?.[1];
}

export function passkeyErrorMessage(err: unknown, mode: PasskeyMode, t: TFunction): string {
  const fallbackKey =
    mode === "register" ? "auth:passkey.registrationFailed" : "auth:passkey.authenticationFailed";
  if (!isHttpError(err)) {
    return (err as Error)?.message ?? t("auth:passkey.unknownError");
  }

  const reason = passkeyReason(err);
  if (typeof reason === "string") {
    const key = PASSKEY_REASON_KEYS[reason as keyof typeof PASSKEY_REASON_KEYS];
    if (key) {
      return t(key);
    }
  }
  if (err.status === 410) {
    return t(PASSKEY_REASON_KEYS.challenge_expired);
  }
  // WebAuthn 规范把「credential 已存在/被 excludeCredentials 命中」映射为
  // InvalidStateError；服务端对应 409，这里给出可操作的提示。
  if (err.status === 409) {
    return t("auth:passkey.reason.credential_already_registered");
  }
  return t(fallbackKey);
}
