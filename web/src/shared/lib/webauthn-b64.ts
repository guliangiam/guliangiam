/**
 * WebAuthn 浏览器原生 API ↔ 后端 JSON 协议的 base64url 桥接工具。
 *
 * 背景：WebAuthn 的 PublicKeyCredential 返回的是 ArrayBuffer，
 * 后端 JSON 协议全部用 base64url 字符串（URL-safe base64 无 padding）。
 * 这里是唯一转换入口 — 避免到处散落 atob/btoa。
 */

// ──────────────────────────────────────────────────────────────────────────
// ArrayBuffer → base64url（编码）
// ──────────────────────────────────────────────────────────────────────────

export function arrayBufferToBase64Url(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]!);
  }
  const b64 = btoa(binary);
  return b64.replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

// ──────────────────────────────────────────────────────────────────────────
// base64url → ArrayBuffer（解码）
// ──────────────────────────────────────────────────────────────────────────

export function base64UrlToArrayBuffer(b64url: string): ArrayBuffer {
  // 补 padding
  let b64 = b64url.replace(/-/g, "+").replace(/_/g, "/");
  const pad = b64.length % 4;
  if (pad > 0) b64 += "=".repeat(4 - pad);

  const binary = atob(b64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer;
}

// ──────────────────────────────────────────────────────────────────────────
// PublicKeyCredential <→ 后端 JSON 转换
// ──────────────────────────────────────────────────────────────────────────

export function credentialToRegistrationFinishRequest(
  credential: PublicKeyCredential,
  challengeId: string,
  challenge: string,
  organizationSlug: string,
  username: string,
) {
  const response = credential.response as AuthenticatorAttestationResponse;

  // WebAuthn Level 3 API 兼容性保护 — Safari < 16 / 旧 Chrome 可能缺这些方法
  // 缺少时 fallback 到 undefined / []，后端对 optional 字段已有容错。
  const getAuthenticatorData = (): ArrayBuffer => {
    const fn = (response as unknown as { getAuthenticatorData?: () => ArrayBuffer })
      .getAuthenticatorData;
    return typeof fn === "function" ? fn.call(response) : new ArrayBuffer(0);
  };
  const getAttestationType = (): string | undefined => {
    const fn = (response as unknown as { getAttestationType?: () => string }).getAttestationType;
    return typeof fn === "function" ? fn.call(response) : undefined;
  };
  const getTransports = (): string[] | undefined => {
    const fn = (response as unknown as { getTransports?: () => string[] }).getTransports;
    return typeof fn === "function" ? fn.call(response) : undefined;
  };
  const getPublicKey = (): ArrayBuffer | undefined => {
    // 标准 API 是 getPublicKey()；response.publicKey 仅是部分实现的 Level 3 属性，
    // Chrome 实际不暴露该属性（只给方法），只读属性会让注册 finish 报 publicKey is required。
    const fn = (response as unknown as { getPublicKey?: () => ArrayBuffer | null }).getPublicKey;
    if (typeof fn === "function") {
      return fn.call(response) ?? undefined;
    }
    return (response as unknown as { publicKey?: ArrayBuffer }).publicKey;
  };

  const publicKey = getPublicKey();

  return {
    organizationSlug,
    username,
    challengeId,
    challenge,
    credentialId: credential.id, // 已经是 base64url
    publicKey: publicKey ? arrayBufferToBase64Url(publicKey) : "",
    clientDataJSON: arrayBufferToBase64Url(response.clientDataJSON),
    authenticatorData: arrayBufferToBase64Url(getAuthenticatorData()),
    attestationObject: arrayBufferToBase64Url(response.attestationObject),
    attestationType: getAttestationType() ?? "none",
    transports: getTransports(),
  };
}

export function credentialToAuthenticationFinishRequest(
  credential: PublicKeyCredential,
  challengeId: string,
  challenge: string,
  organizationSlug: string,
  username: string,
) {
  const response = credential.response as AuthenticatorAssertionResponse;

  return {
    organizationSlug,
    username,
    challengeId,
    challenge,
    credentialId: credential.id, // 已经是 base64url
    signature: arrayBufferToBase64Url(response.signature),
    authenticatorData: arrayBufferToBase64Url(response.authenticatorData),
    clientDataJSON: arrayBufferToBase64Url(response.clientDataJSON),
  };
}

export function isWebAuthnSupported(): boolean {
  return (
    typeof window !== "undefined" && !!window.PublicKeyCredential && !!navigator.credentials?.create
  );
}

export async function hasPlatformAuthenticator(): Promise<boolean> {
  if (!isWebAuthnSupported()) return false;
  try {
    const fn = window.PublicKeyCredential?.isUserVerifyingPlatformAuthenticatorAvailable;
    if (typeof fn === "function") {
      return await fn.call(window.PublicKeyCredential);
    }
  } catch {
    // 忽略
  }
  return false;
}
