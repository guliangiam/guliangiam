/**
 * Sessions 模块的设备 / User-Agent / 会话状态辅助函数
 *
 * 从 SessionsPage.tsx 模块级 helper 区（行 19-198）拆分而来，
 * 给 Tokens / Login / Audit 等同样需要设备识别的页面复用。
 *
 * 类型全部直接引用 API 返回的真实类型（SessionDetail / SessionItem），
 * 避免手写近似类型导致的 optional/nullable 字段漂移。
 */
import { Tag, Typography } from "antd";
import type { SessionDetail, SessionItem } from "@/shared/api/iam/session";

/** Null-safe locale string fallback. Unknown / empty → "-"; malformed Date → raw value. */
export function formatDateTime(value?: string | null) {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

/** Render `<Tag>` showing session state with color convention. */
export function renderSessionState(state: string, tr: (key: string) => string) {
  if (state === "active") return <Tag color="green">{tr("session.stateActive")}</Tag>;
  if (state === "revoked") return <Tag color="red">{tr("session.stateRevoked")}</Tag>;
  return <Tag>{state || tr("session.stateUnknown")}</Tag>;
}

/** User-Agent → platform name (iOS / Android / ChromeOS / macOS / Windows / Linux). */
export function detectPlatformFromUserAgent(userAgent?: string | null) {
  const s = (userAgent || "").toLowerCase();
  if (!s) return "";
  if (s.includes("iphone") || s.includes("ipad") || s.includes("ios")) return "iOS";
  if (s.includes("android")) return "Android";
  if (s.includes("cros")) return "ChromeOS";
  if (s.includes("mac os x") || s.includes("macintosh")) return "macOS";
  if (s.includes("windows")) return "Windows";
  if (s.includes("linux")) return "Linux";
  return "";
}

/** User-Agent → browser name (Edge / Opera / Chrome / Firefox / Safari). */
export function detectBrowserFromUserAgent(userAgent?: string | null) {
  const s = (userAgent || "").toLowerCase();
  if (!s) return "";
  if (s.includes("edg/")) return "Edge";
  if (s.includes("opr/") || s.includes("opera")) return "Opera";
  if (s.includes("chrome/") && !s.includes("edg/")) return "Chrome";
  if (s.includes("firefox/")) return "Firefox";
  if (s.includes("safari/") && !s.includes("chrome/") && !s.includes("chromium/")) return "Safari";
  return "";
}

/** User-Agent → device family (Tablet / Mobile / Desktop). */
export function detectDeviceTypeFromUserAgent(userAgent?: string | null) {
  const s = (userAgent || "").toLowerCase();
  if (!s) return "";
  if (s.includes("ipad") || s.includes("tablet")) return "Tablet";
  if (s.includes("mobile") || s.includes("iphone") || s.includes("android")) return "Mobile";
  return "Desktop";
}

/** Join platform + device family + browser, de-duplicated, with "-" fallback. */
export function formatUserAgentSummary(userAgent?: string | null) {
  const values = [
    detectPlatformFromUserAgent(userAgent),
    detectDeviceTypeFromUserAgent(userAgent),
    detectBrowserFromUserAgent(userAgent),
  ].filter((v, i, arr) => v && arr.indexOf(v) === i);
  if (values.length === 0) return userAgent || "-";
  return values.join(" / ");
}

/** Look up the first non-empty string value from deviceInfo using a priority-key list. */
export function getDeviceInfoValue(
  deviceInfo: Record<string, unknown> | undefined,
  keys: string[],
) {
  if (!deviceInfo) return "";
  for (const key of keys) {
    const value = deviceInfo[key];
    if (typeof value === "string" && value.trim()) return value.trim();
  }
  return "";
}

/** Prefer structured deviceInfo fields; fall back to User-Agent summary. */
export function formatDeviceSummary(session?: SessionDetail | null) {
  if (!session) return "-";
  const deviceInfo = session.deviceInfo;
  const platform = getDeviceInfoValue(deviceInfo, ["platform", "os", "osName", "system"]);
  const deviceType = getDeviceInfoValue(deviceInfo, ["deviceType", "type", "category"]);
  const deviceName = getDeviceInfoValue(deviceInfo, [
    "device",
    "deviceName",
    "model",
    "deviceModel",
  ]);
  const vendor = getDeviceInfoValue(deviceInfo, ["vendor", "brand", "manufacturer"]);
  const browser = getDeviceInfoValue(deviceInfo, [
    "browser",
    "browserName",
    "client",
    "clientName",
  ]);

  const values = [platform, deviceType, vendor, deviceName, browser].filter(
    (v, i, arr) => v && arr.indexOf(v) === i,
  );
  if (values.length > 0) return values.join(" / ");
  return formatUserAgentSummary(session.userAgent);
}

/** Serialize deviceInfo entries to a compact "key: value, …" string for inspection. */
export function formatDeviceMetadata(session?: SessionDetail | null) {
  if (!session) return "-";
  const entries = Object.entries(session.deviceInfo ?? {})
    .filter(([, value]) => value !== null && value !== undefined && `${value}`.trim() !== "")
    .map(([key, value]) => `${key}: ${typeof value === "string" ? value : JSON.stringify(value)}`);
  if (entries.length === 0) return "-";
  return entries.join(", ");
}

/** Render a compact UA summary + full UA below it as muted subtitle. */
export function renderSessionDeviceSummary(session: SessionItem) {
  const summary = formatUserAgentSummary(session.userAgent);
  return (
    <div>
      <div>{summary}</div>
      <Typography.Text type="secondary">{session.userAgent || "-"}</Typography.Text>
    </div>
  );
}
