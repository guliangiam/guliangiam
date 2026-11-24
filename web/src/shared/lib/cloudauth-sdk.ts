// 阿里云 CloudAuth 网页 SDK 封装（实名认证 metaInfo 采集）。
//
// metaInfo 是设备指纹参数，官方要求发起认证时实时采集，不能缓存复用；
// 后端对空 metaInfo fail-closed（恒 400），因此采集失败必须整段中止——
// 发一个注定 400 的请求只会把真实原因（SDK 加载失败）埋进通用错误文案。
// 加载模式镜像 FaceRecognitionModal 的脚本注入先例：模块级 Promise 单例 +
// data-* 标记防重复注入，失败后重置单例允许下一次显式重试。

declare global {
  interface Window {
    getMetaInfo?: () => string;
  }
}

export const CLOUDAUTH_SDK_DEFAULT_URL =
  "https://o.alicdn.com/yd-cloudauth/cloudauth-cdn/jsvm_all.js";
export const CLOUDAUTH_SDK_MARKER = "data-cloudauth-sdk";

export type CloudAuthMetaFailure = "load" | "timeout" | "missing" | "invalid";

export class CloudAuthMetaError extends Error {
  reason: CloudAuthMetaFailure;

  constructor(reason: CloudAuthMetaFailure, message: string) {
    super(message);
    this.name = "CloudAuthMetaError";
    this.reason = reason;
  }
}

function sdkUrl(): string {
  const configured = import.meta.env.VITE_CLOUDAUTH_SDK_URL as string | undefined;
  return configured && configured.trim() ? configured : CLOUDAUTH_SDK_DEFAULT_URL;
}

let sdkPromise: Promise<void> | null = null;

export function loadCloudAuthSdk(timeoutMs = 8000): Promise<void> {
  if (typeof window.getMetaInfo === "function") {
    return Promise.resolve();
  }
  if (sdkPromise) {
    return sdkPromise;
  }
  sdkPromise = new Promise<void>((resolve, reject) => {
    const timer = window.setTimeout(() => {
      reject(new CloudAuthMetaError("timeout", `CloudAuth SDK did not load within ${timeoutMs}ms`));
    }, timeoutMs);
    const settle = () => {
      window.clearTimeout(timer);
      if (typeof window.getMetaInfo === "function") {
        resolve();
        return;
      }
      reject(
        new CloudAuthMetaError("missing", "CloudAuth SDK loaded but getMetaInfo is unavailable"),
      );
    };
    const existing = document.querySelector<HTMLScriptElement>(`script[${CLOUDAUTH_SDK_MARKER}]`);
    if (existing) {
      existing.addEventListener("load", settle);
      existing.addEventListener("error", () => {
        window.clearTimeout(timer);
        reject(new CloudAuthMetaError("load", "CloudAuth SDK script failed to load"));
      });
      return;
    }
    const script = document.createElement("script");
    script.src = sdkUrl();
    script.async = true;
    script.setAttribute(CLOUDAUTH_SDK_MARKER, "true");
    script.onload = settle;
    script.onerror = () => {
      window.clearTimeout(timer);
      reject(new CloudAuthMetaError("load", "CloudAuth SDK script failed to load"));
    };
    document.head.appendChild(script);
  }).catch((error: unknown) => {
    // 失败不缓存：下一次调用重新注入（超时/网络抖动不应永久封死认证入口）。
    sdkPromise = null;
    throw error;
  });
  return sdkPromise;
}

export async function collectMetaInfo(): Promise<string> {
  await loadCloudAuthSdk();
  const raw = window.getMetaInfo?.();
  if (typeof raw !== "string" || raw.trim() === "") {
    throw new CloudAuthMetaError("invalid", "getMetaInfo returned no usable metaInfo");
  }
  try {
    JSON.parse(raw);
  } catch {
    throw new CloudAuthMetaError("invalid", "getMetaInfo returned a non-JSON payload");
  }
  return raw;
}

// returnUrl 必须是纯 origin+path：阿里云在 URL 后追加认证结果参数，
// 带 query/hash 的返回值会被官方解析破坏。
export function buildIdvReturnUrl(path: string): string {
  return `${window.location.origin}${path}`;
}
