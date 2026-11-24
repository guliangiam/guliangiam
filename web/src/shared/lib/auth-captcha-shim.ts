import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";

import { AUTH_CAPTCHA, AUTH_CAPTCHA_STATUS } from "@/shared/api/paths";

/**
 * Adapter over the GuliangIAM v1 CAPTCHA API.
 *
 * The legacy console called legacy `GET /api/get-captcha?applicationId=`
 * through `request()`, which parses legacy `{status, msg, data}` envelope.
 * GuliangIAM implements neither that route nor that envelope: the v1 endpoints
 * answer with bare JSON documents, so `response.data` was always `undefined`.
 *
 *   GET /api/v1/auth/captcha?organizationId=        -> {captchaId, captchaImage, type}
 *   GET /api/v1/auth/captcha-status?organizationId= -> {required, providerType, siteKey}
 */
const captchaPayloadSchema = z
  .object({
    captchaId: z.string().optional().nullable(),
    captchaImage: z.string().optional().nullable(),
    type: z.string().optional().nullable(),
    // go_captcha challenges carry the shape ("click" | "slide") and the sub
    // image (click hint thumb / slide tile) alongside the master image.
    shape: z.string().optional().nullable(),
    captchaImageSub: z.string().optional().nullable(),
    // Third-party widgets need the public site key to render; it comes from the
    // captcha-status document, not from the challenge document.
    clientId: z.string().optional().nullable(),
  })
  .passthrough();

export type CaptchaState = z.infer<typeof captchaPayloadSchema>;

const captchaStatusSchema = z
  .object({
    required: z.boolean().optional().nullable(),
    providerType: z.string().optional().nullable(),
    siteKey: z.string().optional().nullable(),
  })
  .passthrough();

export type CaptchaStatus = z.infer<typeof captchaStatusSchema>;

export async function getCaptcha(values: { organizationId: string }) {
  const params = new URLSearchParams({ organizationId: values.organizationId });
  const response = await requestDocumentJson(`${AUTH_CAPTCHA}?${params.toString()}`);
  return captchaPayloadSchema.parse(response);
}

export async function getCaptchaStatus(values: { organizationId: string }): Promise<CaptchaStatus> {
  const params = new URLSearchParams();
  if (values.organizationId) {
    params.set("organizationId", values.organizationId);
  }

  const data = await requestDocumentJson(`${AUTH_CAPTCHA_STATUS}?${params.toString()}`);
  return captchaStatusSchema.parse(data ?? {});
}

/**
 * The token a caller submits alongside a CAPTCHA challenge.
 *
 * GuliangIAM answers with `{captchaId, captchaImage, type}` — the challenge id
 * *is* the token. legacy `clientId`/`clientSecret`/`clientId2`/`clientSecret2`
 * fields do not exist on the v1 response, so reading them always produced an
 * empty string.
 */
export function getCaptchaClientSecret(captchaState: CaptchaState | null) {
  return captchaState?.captchaId ?? "";
}
