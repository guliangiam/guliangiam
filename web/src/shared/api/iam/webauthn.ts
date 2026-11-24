import { requestDocumentJson } from "@/shared/api/http";
import {
  AUTH_WEBAUTHN_AUTHENTICATION_BEGIN,
  AUTH_WEBAUTHN_AUTHENTICATION_FINISH,
  AUTH_WEBAUTHN_REGISTRATION_BEGIN,
  AUTH_WEBAUTHN_REGISTRATION_FINISH,
} from "@/shared/api/paths";

// ──────────────────────────────────────────────────────────────────────────
// 注册流程 — 浏览器原生 WebAuthn → 后端 challenge/finish
// ──────────────────────────────────────────────────────────────────────────

export interface WebAuthnRegistrationBeginRequest {
  organizationSlug: string;
  username: string;
}

export interface WebAuthnRegistrationBeginResponse {
  challenge: string; // base64url
  rpId: string;
  rpName: string;
  userId: string; // base64
  challengeId: string;
  userDisplayName: string;
  timeout: number;
  /** 该用户已有活跃凭据的 base64url credentialId；浏览器据此阻止同一认证器重复注册。 */
  excludeCredentials?: string[];
  /** 服务端按组织/部署策略下发；浏览器必须按此要求 PIN/生物识别，不得前端写死。 */
  userVerification: "required" | "preferred";
}

export interface WebAuthnRegistrationFinishRequest {
  organizationSlug: string;
  username: string;
  challengeId: string;
  challenge: string;
  credentialId: string; // base64url
  publicKey: string; // base64url
  clientDataJSON: string; // base64url
  authenticatorData: string; // base64url
  attestationObject: string; // base64url
  attestationType: string;
  transports?: string[];
}

export interface WebAuthnRegistrationFinishResponse {
  // 服务端成功标记是 "registered"（历史面板兼容 "ok"）。
  status: "registered" | "ok" | "error";
  credentialId: string;
}

export const webauthnApi = {
  async registrationBegin(payload: WebAuthnRegistrationBeginRequest) {
    return requestDocumentJson<WebAuthnRegistrationBeginResponse>(
      AUTH_WEBAUTHN_REGISTRATION_BEGIN,
      { method: "POST", body: JSON.stringify(payload) },
    );
  },

  async registrationFinish(payload: WebAuthnRegistrationFinishRequest) {
    return requestDocumentJson<WebAuthnRegistrationFinishResponse>(
      AUTH_WEBAUTHN_REGISTRATION_FINISH,
      { method: "POST", body: JSON.stringify(payload) },
    );
  },

  async authenticationBegin(payload: WebAuthnRegistrationBeginRequest) {
    return requestDocumentJson<WebAuthnAuthenticationBeginResponse>(
      AUTH_WEBAUTHN_AUTHENTICATION_BEGIN,
      { method: "POST", body: JSON.stringify(payload) },
    );
  },

  async authenticationFinish(payload: WebAuthnAuthenticationFinishRequest) {
    return requestDocumentJson<{ status: string }>(AUTH_WEBAUTHN_AUTHENTICATION_FINISH, {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },
};

export interface WebAuthnAuthenticationBeginResponse {
  challenge: string; // base64url
  challengeId: string;
  rpId: string;
  allowedCredentialIds?: string[]; // base64url
  timeout: number;
  /** 服务端按组织/部署策略下发："required" 表示浏览器必须要求 PIN/生物识别。 */
  userVerification?: "required" | "preferred" | "discouraged";
}

export interface WebAuthnAuthenticationFinishRequest {
  organizationSlug: string;
  username: string;
  challengeId: string;
  challenge: string;
  credentialId: string; // base64url
  signature: string; // base64url
  authenticatorData: string; // base64url
  clientDataJSON: string; // base64url
}
