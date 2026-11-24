import { requestDocumentJson } from "@/shared/api/http";
import {
  AUTH_DEVICE_AUTHORIZE,
  AUTH_DEVICE_CANCEL,
  AUTH_DEVICE_TOKEN,
  AUTH_DEVICE_VERIFY,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type DeviceAuthorizeResult = {
  deviceCode: string;
  userCode: string;
  verificationUri: string;
  expiresIn: number;
  interval: number;
};

export type DeviceTokenResult = {
  status: string;
  accessToken: string;
  tokenType: string;
  expiresIn: number;
  scope?: string;
};

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export async function deviceAuthorize(
  clientId: string,
  scope?: string,
): Promise<DeviceAuthorizeResult> {
  const response = await requestDocumentJson(AUTH_DEVICE_AUTHORIZE, {
    method: "POST",
    body: JSON.stringify({ clientId, scope }),
    headers: { "Content-Type": "application/json" },
  });
  return response as DeviceAuthorizeResult;
}

export async function deviceToken(
  clientId: string,
  deviceCode: string,
): Promise<DeviceTokenResult> {
  const response = await requestDocumentJson(AUTH_DEVICE_TOKEN, {
    method: "POST",
    body: JSON.stringify({ clientId, deviceCode }),
    headers: { "Content-Type": "application/json" },
  });
  return response as DeviceTokenResult;
}

export async function deviceVerify(userCode: string): Promise<void> {
  await requestDocumentJson(AUTH_DEVICE_VERIFY, {
    method: "POST",
    body: JSON.stringify({ userCode }),
    headers: { "Content-Type": "application/json" },
  });
}

/** Cancel a device authorization flow. */
export async function deviceCancel(deviceCode: string): Promise<void> {
  await requestDocumentJson(AUTH_DEVICE_CANCEL, {
    method: "POST",
    body: JSON.stringify({ deviceCode }),
    headers: { "Content-Type": "application/json" },
  });
}
