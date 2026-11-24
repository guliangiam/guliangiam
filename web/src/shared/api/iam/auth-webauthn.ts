import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import {
  AUTH_WEBAUTHN_AUTH_BEGIN,
  AUTH_WEBAUTHN_AUTH_FINISH,
  AUTH_WEBAUTHN_CREDENTIAL_BY_ID,
  AUTH_WEBAUTHN_CREDENTIALS,
  AUTH_WEBAUTHN_REG_BEGIN,
  AUTH_WEBAUTHN_REG_FINISH,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type WebAuthnRegistrationBeginResult = {
  challenge: string;
  challengeId: string;
  rpId: string;
  rpName: string;
  userId: string;
  userDisplayName: string;
  timeout: number;
  /** 由 begin 下发：组织/部署策略要求 UV 时为 "required"。 */
  userVerification?: "required" | "preferred" | "discouraged";
  /** 该用户已有活跃凭据的 base64url credentialId，浏览器据此阻止重复注册。 */
  excludeCredentials?: string[];
};

export type WebAuthnRegistrationFinishResult = {
  status: string;
  credentialId: string;
};

export type WebAuthnAuthenticationBeginResult = {
  challenge: string;
  challengeId: string;
  rpId: string;
  allowedCredentialIds?: string[];
  timeout: number;
  /** 服务端按组织/部署策略下发："required" 表示浏览器必须要求 PIN/生物识别。 */
  userVerification?: "required" | "preferred" | "discouraged";
};

/**
 * finish 的响应文档：与 `/faceid/complete`、`/qrcode/exchange` 同形的登录结果
 * （status 为 success；协议上下文下还会带授权码 / 回调地址字段）。会话由服务端在
 * 该响应上 Set-Cookie 签发；凭据无效时返回非 2xx，requestDocumentJson 直接抛出。
 */
export type WebAuthnAuthenticationFinishResult = {
  status: string;
  accessToken?: string;
  idToken?: string;
  code?: string;
  redirectTo?: string;
} & Record<string, unknown>;

/**
 * finish 请求体：挑战材料 + 可选的协议上下文（OAuth2 授权页 / CAS / SAML）。
 * 字段名逐字对齐服务端 WebAuthnAuthenticationFinishRequest，勿凭记忆改写。
 */
export type WebAuthnAuthenticationFinishParams = {
  organizationSlug: string;
  username: string;
  applicationId?: string;
  challengeId: string;
  challenge: string;
  credentialId: string;
  signature: string;
  authenticatorData: string;
  clientDataJSON: string;
  responseType?: string;
  redirectUri?: string;
  state?: string;
  nonce?: string;
  scope?: string[];
};

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export async function webAuthnRegistrationBegin(
  organizationSlug: string,
  username: string,
): Promise<WebAuthnRegistrationBeginResult> {
  const response = await requestDocumentJson(AUTH_WEBAUTHN_REG_BEGIN, {
    method: "POST",
    body: JSON.stringify({ organizationSlug, username }),
    headers: { "Content-Type": "application/json" },
  });
  return response as WebAuthnRegistrationBeginResult;
}

export async function webAuthnRegistrationFinish(
  organizationSlug: string,
  username: string,
  challengeId: string,
  challenge: string,
  credentialId: string,
  publicKey: string,
  clientDataJSON: string,
  authenticatorData: string,
  attestationObject: string,
  attestationType: string,
  transports?: string[],
): Promise<WebAuthnRegistrationFinishResult> {
  const response = await requestDocumentJson(AUTH_WEBAUTHN_REG_FINISH, {
    method: "POST",
    body: JSON.stringify({
      organizationSlug,
      username,
      challengeId,
      challenge,
      credentialId,
      publicKey,
      clientDataJSON,
      authenticatorData,
      attestationObject,
      attestationType,
      transports,
    }),
    headers: { "Content-Type": "application/json" },
  });
  return response as WebAuthnRegistrationFinishResult;
}

export async function webAuthnAuthenticationBegin(
  organizationSlug: string,
  username: string,
): Promise<WebAuthnAuthenticationBeginResult> {
  const response = await requestDocumentJson(AUTH_WEBAUTHN_AUTH_BEGIN, {
    method: "POST",
    body: JSON.stringify({ organizationSlug, username }),
    headers: { "Content-Type": "application/json" },
  });
  return response as WebAuthnAuthenticationBeginResult;
}

export async function webAuthnAuthenticationFinish(
  values: WebAuthnAuthenticationFinishParams,
): Promise<WebAuthnAuthenticationFinishResult> {
  const response = await requestDocumentJson(AUTH_WEBAUTHN_AUTH_FINISH, {
    method: "POST",
    body: JSON.stringify({
      organizationSlug: values.organizationSlug,
      username: values.username,
      ...(values.applicationId ? { applicationId: values.applicationId } : {}),
      challengeId: values.challengeId,
      challenge: values.challenge,
      credentialId: values.credentialId,
      signature: values.signature,
      authenticatorData: values.authenticatorData,
      clientDataJSON: values.clientDataJSON,
      ...(values.responseType ? { responseType: values.responseType } : {}),
      ...(values.redirectUri ? { redirectUri: values.redirectUri } : {}),
      ...(values.state ? { state: values.state } : {}),
      ...(values.nonce ? { nonce: values.nonce } : {}),
      ...(values.scope?.length ? { scope: values.scope } : {}),
    }),
    headers: { "Content-Type": "application/json" },
  });
  return response as WebAuthnAuthenticationFinishResult;
}

// ─────────────────────────────────────────────
// Self-service credential management (session-bound)
// ─────────────────────────────────────────────

/** 自助 Passkey 列表投影：与后端 WebAuthnCredentialSummary 同形（不暴露公钥）。 */
export type WebAuthnCredentialSummary = {
  id: string;
  credentialId: string;
  name: string;
  status: string;
  aaguid?: string;
  signCount: number;
  createdAt: string;
  updatedAt: string;
};

const webAuthnCredentialSummarySchema = z.object({
  id: z.string(),
  credentialId: z.string(),
  name: z.string(),
  status: z.string(),
  aaguid: z.string().optional(),
  signCount: z.number(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

/** 列出会话本人名下的 Passkey 凭据（GET /auth/webauthn/credentials）。 */
export async function listOwnWebAuthnCredentials(): Promise<WebAuthnCredentialSummary[]> {
  const response = await requestDocumentJson(AUTH_WEBAUTHN_CREDENTIALS);
  return z.array(webAuthnCredentialSummarySchema).parse(response ?? []);
}

/** 撤销会话本人名下的一条 Passkey（DELETE /auth/webauthn/credentials/{id}，204）。 */
export async function revokeOwnWebAuthnCredential(id: string): Promise<void> {
  await requestDocumentJson(AUTH_WEBAUTHN_CREDENTIAL_BY_ID(id), { method: "DELETE" });
}

/** 自助注册 finish 请求体：由 credentialToRegistrationFinishRequest 构造。 */
export type WebAuthnRegistrationFinishBody = {
  organizationSlug: string;
  username: string;
  challengeId: string;
  challenge: string;
  credentialId: string;
  publicKey: string;
  clientDataJSON: string;
  authenticatorData: string;
  attestationObject: string;
  attestationType: string;
  transports?: string[];
};

/** 完成自助 Passkey 注册（POST /auth/webauthn/registration/finish）；finish 会轮换当前会话。 */
export async function finishOwnWebAuthnRegistration(
  body: WebAuthnRegistrationFinishBody,
): Promise<WebAuthnRegistrationFinishResult> {
  const response = await requestDocumentJson(AUTH_WEBAUTHN_REG_FINISH, {
    method: "POST",
    body: JSON.stringify(body),
    headers: { "Content-Type": "application/json" },
  });
  return response as WebAuthnRegistrationFinishResult;
}
