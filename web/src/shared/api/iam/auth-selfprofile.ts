import { requestDocumentJson } from "../http";
import { AUTH_FACEIDS, AUTH_PROFILE, AUTH_VERIFY_IDENTIFICATION } from "../paths";
import { toUserDetail } from "./user";
import {
  userDetailSchema,
  type FaceIdItem,
  type IdentityVerificationTicket,
  type VerifyIdentificationPayload,
  type UpdateUserProfilePayload,
  type UserDetail,
} from "./user-types";

const jsonHeaders = { "Content-Type": "application/json" };

// ─────────────────────────────────────────────
// Self-service profile read / update (session-bound /auth/*)
// 读侧返回与 admin getUser 同形的 UserDetail（含 profile/mfaMethods/identities/
// revision/faceIds），让普通成员无需 users:read 即可加载本人资料。
// 写侧为部分合并语义；email/phone 不在自助面开放（须走 /auth/account 验证码绑定）。
// ─────────────────────────────────────────────

export async function getOwnProfile(): Promise<UserDetail> {
  const response = await requestDocumentJson(AUTH_PROFILE, { cache: "no-store" });
  return toUserDetail(userDetailSchema.parse(response));
}

export async function updateOwnProfile(payload: UpdateUserProfilePayload): Promise<void> {
  await requestDocumentJson(AUTH_PROFILE, {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: jsonHeaders,
  });
}

// ─────────────────────────────────────────────
// Self-service real-name verification (fail-close, no token issuance)
// ─────────────────────────────────────────────

export async function verifyOwnIdentification(
  payload?: VerifyIdentificationPayload,
): Promise<IdentityVerificationTicket | undefined> {
  return requestDocumentJson<IdentityVerificationTicket>(AUTH_VERIFY_IDENTIFICATION, {
    method: "POST",
    headers: jsonHeaders,
    ...(payload ? { body: JSON.stringify(payload) } : {}),
  });
}

// ─────────────────────────────────────────────
// Self-service FaceID enroll / revoke (multipart upload, encrypted base image)
// ─────────────────────────────────────────────

export async function registerOwnFaceId(params: {
  file: File;
  providerId?: string;
  deviceInfo?: string;
}): Promise<FaceIdItem> {
  const formData = new FormData();
  if (params.providerId) formData.append("providerId", params.providerId);
  if (params.deviceInfo) formData.append("deviceInfo", params.deviceInfo);
  formData.append("file", params.file, params.file.name);

  const response = await requestDocumentJson<FaceIdItem>(AUTH_FACEIDS, {
    method: "POST",
    body: formData,
  });
  return response as FaceIdItem;
}

export async function revokeOwnFaceId(): Promise<void> {
  await requestDocumentJson<unknown>(AUTH_FACEIDS, { method: "DELETE" });
}
