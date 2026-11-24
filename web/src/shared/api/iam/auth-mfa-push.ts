import { requestDocumentJson } from "@/shared/api/http";
import {
  AUTH_MFA_PUSH_DISABLE,
  AUTH_MFA_PUSH_ENABLE,
  AUTH_MFA_PUSH_INITIATE,
  AUTH_MFA_PUSH_VERIFY,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type PushMfaInitiateResult = {
  challengeId: string;
  challengeState: string;
  expiresAt: string;
};

export type PushMfaVerifyResult = {
  approved: boolean;
};

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export async function initiatePushMfa(
  organizationId: string,
  userId: string,
  applicationId: string,
): Promise<PushMfaInitiateResult> {
  const response = await requestDocumentJson(AUTH_MFA_PUSH_INITIATE, {
    method: "POST",
    body: JSON.stringify({ organizationId, userId, applicationId }),
    headers: { "Content-Type": "application/json" },
  });
  return response as PushMfaInitiateResult;
}

export async function verifyPushMfa(challengeId: string): Promise<PushMfaVerifyResult> {
  const response = await requestDocumentJson(AUTH_MFA_PUSH_VERIFY, {
    method: "POST",
    body: JSON.stringify({ challengeId }),
    headers: { "Content-Type": "application/json" },
  });
  return response as PushMfaVerifyResult;
}

export async function enablePushMfa(
  userId: string,
  mfaPushReceiver: string,
  mfaPushProvider: string,
): Promise<void> {
  await requestDocumentJson(AUTH_MFA_PUSH_ENABLE, {
    method: "POST",
    body: JSON.stringify({ userId, mfaPushReceiver, mfaPushProvider }),
    headers: { "Content-Type": "application/json" },
  });
}

export async function disablePushMfa(userId: string): Promise<void> {
  await requestDocumentJson(AUTH_MFA_PUSH_DISABLE, {
    method: "POST",
    body: JSON.stringify({ userId }),
    headers: { "Content-Type": "application/json" },
  });
}
