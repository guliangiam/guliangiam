import { useEffect, useState } from "react";

/**
 * 浏览器能力检测 Hook — 评估当前浏览器是否支持各类登录方式的前置 API。
 *
 * 检测时机: useEffect(mount)，避免 SSR 环境访问 navigator；
 * 异步能力 (WebAuthn isUserVerifyingPlatformAuthenticatorAvailable) 通过 Promise 回填。
 *
 * 返回字段:
 * - isSecureContext:   WebAuthn / Passkey / Face ID 都要求 HTTPS 或 localhost；
 * - supportsWebAuthn:   navigator.credentials?.get / create 存在；
 * - supportsPlatformAuth:    platform authenticator 可用性（TouchID / Windows Hello / Android 指纹）；
 * - hasMediaDevices:    camera/microphone 设备访问权限（Face ID 采集前置）；
 * - hasFaceDetector:    FaceDetector API 原生支持（Android Chrome）；
 */
export type BrowserCapabilities = {
  isSecureContext: boolean;
  supportsWebAuthn: boolean;
  supportsPlatformAuth: boolean;
  hasMediaDevices: boolean;
  hasFaceDetector: boolean;
};

const SERVER_SENTINEL: BrowserCapabilities = {
  isSecureContext: false,
  supportsWebAuthn: false,
  supportsPlatformAuth: false,
  hasMediaDevices: false,
  hasFaceDetector: false,
};

function detectSynchronousCapabilities(): BrowserCapabilities {
  if (typeof window === "undefined" || typeof navigator === "undefined") {
    return SERVER_SENTINEL;
  }

  const hasCredentialsAPI = "credentials" in navigator;
  const supportsWebAuthn =
    hasCredentialsAPI &&
    typeof (navigator as Navigator & { credentials?: CredentialsContainer }).credentials?.create ===
      "function" &&
    typeof (navigator as Navigator & { credentials?: CredentialsContainer }).credentials?.get ===
      "function";

  const hasMediaDevices =
    "mediaDevices" in navigator &&
    typeof (navigator as Navigator & { mediaDevices?: MediaDevices }).mediaDevices?.getUserMedia ===
      "function";

  const hasFaceDetector = "FaceDetector" in window;

  return {
    isSecureContext: window.isSecureContext,
    supportsWebAuthn,
    supportsPlatformAuth: false, // 异步回填
    hasMediaDevices,
    hasFaceDetector,
  };
}

export function useBrowserCapabilities(): BrowserCapabilities {
  // Synchronous capabilities are computed once during the first render (lazy
  // initializer), so they do not require a setState-in-effect cascade.
  const [caps, setCaps] = useState<BrowserCapabilities>(detectSynchronousCapabilities);

  useEffect(() => {
    if (typeof navigator === "undefined") {
      return;
    }

    // 异步检测 platform authenticator（TouchID / Windows Hello / Android 指纹）
    const creds = (
      navigator as Navigator & {
        credentials?: CredentialsContainer & {
          isUserVerifyingPlatformAuthenticatorAvailable?: () => Promise<boolean>;
        };
      }
    ).credentials;

    if (creds && typeof creds.isUserVerifyingPlatformAuthenticatorAvailable === "function") {
      creds
        .isUserVerifyingPlatformAuthenticatorAvailable()
        .then((available) => {
          setCaps((prev) => ({ ...prev, supportsPlatformAuth: Boolean(available) }));
          return;
        })
        .catch(() => {
          // 浏览器不支持该方法或拒绝访问 — 保持 false
        });
    }
  }, []);

  return caps;
}
