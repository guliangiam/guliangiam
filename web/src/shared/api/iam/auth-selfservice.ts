import { z } from "zod";

import { requestDocumentJson } from "../http";
import {
  AUTH_CONSENTS,
  AUTH_IDENTITY_BY_ID,
  AUTH_MFA_METHODS,
  AUTH_MFA_SETUP_ENABLE,
  AUTH_MFA_SETUP_INITIATE,
  AUTH_MFA_SETUP_VERIFY,
  AUTH_PREFERRED_MFA,
} from "../paths";
import {
  userMfaSetupInitiateResultSchema,
  type EnableUserMfaSetupPayload,
  type InitiateUserMfaSetupPayload,
  type ReplaceUserMfaMethodItem,
  type UpdateUserPreferredMfaPayload,
  type UserConsentItem,
  type UserMfaSetupInitiateResult,
  type VerifyUserMfaSetupPayload,
} from "./user-types";

const jsonHeaders = { "Content-Type": "application/json" };

// ─────────────────────────────────────────────
// Self-service MFA setup (session-bound /auth/*)
// 身份由后端会话裁决，前端不传 userID/orgID。
// ─────────────────────────────────────────────

export async function initiateOwnMfaSetup(
  payload: InitiateUserMfaSetupPayload,
): Promise<UserMfaSetupInitiateResult> {
  const response = await requestDocumentJson(AUTH_MFA_SETUP_INITIATE, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: jsonHeaders,
  });
  return userMfaSetupInitiateResultSchema.parse(response);
}

export async function verifyOwnMfaSetup(payload: VerifyUserMfaSetupPayload): Promise<void> {
  await requestDocumentJson(AUTH_MFA_SETUP_VERIFY, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: jsonHeaders,
  });
}

export async function enableOwnMfaSetup(payload: EnableUserMfaSetupPayload): Promise<void> {
  await requestDocumentJson(AUTH_MFA_SETUP_ENABLE, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: jsonHeaders,
  });
}

export async function replaceOwnMfaMethods(
  revision: number,
  items: ReplaceUserMfaMethodItem[],
): Promise<void> {
  await requestDocumentJson(AUTH_MFA_METHODS, {
    method: "PUT",
    body: JSON.stringify({ revision, items }),
    headers: jsonHeaders,
  });
}

export async function updateOwnPreferredMfa(payload: UpdateUserPreferredMfaPayload): Promise<void> {
  await requestDocumentJson(AUTH_PREFERRED_MFA, {
    method: "PUT",
    body: JSON.stringify(payload),
    headers: jsonHeaders,
  });
}

// ─────────────────────────────────────────────
// Self-service third-party identity unbind
// ─────────────────────────────────────────────

export async function deleteOwnIdentity(identityId: string): Promise<void> {
  await requestDocumentJson(AUTH_IDENTITY_BY_ID(identityId), { method: "DELETE" });
}

// ─────────────────────────────────────────────
// Self-service consent listing
// ─────────────────────────────────────────────

const ownConsentSchema = z.object({
  id: z.string(),
  applicationId: z.string(),
  applicationSlug: z.string().optional(),
  grantedScopes: z.array(z.string()),
  createdAt: z.string(),
  revokedAt: z.string().optional(),
});

export async function listOwnConsents(): Promise<UserConsentItem[]> {
  const response = await requestDocumentJson(AUTH_CONSENTS);
  return z.array(ownConsentSchema).parse(response ?? []);
}
