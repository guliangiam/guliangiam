import { requestDocumentJson } from "@/shared/api/http";
import { toE164 } from "@/shared/ui/country-data";

import { AUTH_VERIFICATION_CODE_SEND } from "@/shared/api/paths";

/**
 * Thin adapter over the GuliangIAM v1 verification-code API.
 *
 * The legacy console called legacy `POST /api/send-verification-code` with a
 * `FormData` body carrying `{captchaType, captchaToken, clientSecret, method,
 * countryCode, dest, type, applicationId}`. GuliangIAM never implemented that
 * route *and* answers with a bare JSON document rather than the legacy
 * `{status, msg, data}` envelope, so every "send code" button failed twice over.
 *
 * The v1 contract is a JSON document:
 *   POST /api/v1/auth/verification-code/send
 *   {organizationId, type: "email" | "phone", recipient} -> {id, type, recipient, expiresAt}
 *
 * `type` must be `"phone"`, not `"sms"` — see `appverifications.VerificationTypePhone`.
 *
 * Phone recipients are always sent as E.164. `countryCode` may be an ISO
 * alpha-2 code (converted through `libphonenumber-js`) or a legacy dial prefix
 * (`86` / `+86`); a local number without either is rejected instead of being
 * sent as an unprefixed recipient.
 */
export async function sendVerificationCode(values: {
  organizationId: string;
  /** 当前登录/注册的应用；后端据此优先使用该应用绑定的短信/邮件提供商。 */
  applicationId?: string;
  dest: string;
  type: "email" | "phone";
  countryCode?: string;
  invalidPhoneMessage?: string;
  captchaToken?: string;
  captchaId?: string;
}) {
  let recipient = values.dest;

  if (values.type === "phone") {
    recipient = toE164(values.countryCode ?? "", values.dest);
    if (!recipient) {
      throw new Error(values.invalidPhoneMessage ?? "Invalid phone number");
    }
  }

  return requestDocumentJson(AUTH_VERIFICATION_CODE_SEND, {
    method: "POST",
    body: JSON.stringify({
      organizationId: values.organizationId,
      applicationId: values.applicationId || undefined,
      type: values.type,
      recipient,
      captchaToken: values.captchaToken,
      captchaId: values.captchaId,
    }),
  });
}
