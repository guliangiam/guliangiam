import { requestDocumentJson } from "@/shared/api/http";
import { AUTH_FACEID, AUTH_FACEID_COMPLETE, AUTH_FACEID_VERIFY } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type FaceIdInitiateResult = {
  sessionId: string;
  challenge: string;
  expiresIn: number;
};

export type FaceIdVerifyResult = {
  verified: boolean;
  livenessPassed: boolean;
};

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export async function initiateFaceId(
  organizationSlug: string,
  username: string,
  applicationId: string,
): Promise<FaceIdInitiateResult> {
  const response = await requestDocumentJson(AUTH_FACEID, {
    method: "POST",
    body: JSON.stringify({ organizationSlug, username, applicationId }),
    headers: { "Content-Type": "application/json" },
  });
  return response as FaceIdInitiateResult;
}

export async function verifyFaceId(
  sessionId: string,
  imageData: string,
  deviceInfo?: string,
): Promise<FaceIdVerifyResult> {
  const response = await requestDocumentJson(AUTH_FACEID_VERIFY, {
    method: "POST",
    body: JSON.stringify({ sessionId, imageData, deviceInfo }),
    headers: { "Content-Type": "application/json" },
  });
  return response as FaceIdVerifyResult;
}

/** complete 的响应文档：与 WebAuthn finish、扫码 exchange 同形的登录结果。 */
export type FaceIdCompleteResult = {
  status: string;
  accessToken?: string;
  idToken?: string;
  code?: string;
  redirectTo?: string;
} & Record<string, unknown>;

/**
 * complete 请求体：会话句柄 + 可选的协议上下文（OAuth2 授权页 / CAS / SAML）。
 * 字段名逐字对齐服务端 CompleteFaceIDLoginRequest，勿凭记忆改写；
 * 带 responseType 却不带 applicationId 会被服务端拦成 422。
 */
export type FaceIdCompleteParams = {
  sessionId: string;
  applicationId?: string;
  responseType?: string;
  redirectUri?: string;
  state?: string;
  nonce?: string;
  scope?: string[];
};

export async function completeFaceIdLogin(
  params: FaceIdCompleteParams,
): Promise<FaceIdCompleteResult> {
  const response = await requestDocumentJson(AUTH_FACEID_COMPLETE, {
    method: "POST",
    body: JSON.stringify(params),
    headers: { "Content-Type": "application/json" },
  });
  return response as FaceIdCompleteResult;
}
