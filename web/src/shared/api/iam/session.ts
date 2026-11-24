import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import {
  SESSIONS,
  SESSION_BY_ID,
  SESSION_REVOKE,
  SESSIONS_CURRENT,
  SESSIONS_CURRENT_REVOKE_OTHERS,
  SESSIONS_IS_DUPLICATED,
  USER_SESSIONS,
  USER_SESSION_BY_ID,
} from "@/shared/api/paths";
import { pageMetaSchema } from "./schemas";
import type { PaginatedResult } from "./client";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type SessionItem = {
  id: string;
  userId: string;
  username: string;
  organizationId: string;
  organizationSlug: string;
  applicationId: string;
  applicationSlug: string;
  sessionState: string;
  ipAddress: string;
  userAgent: string;
  expiresAt: string;
  revokedAt: string | null;
  createdAt: string;
  lastSeenAt: string;
  updatedAt: string;
  /** Optional device fingerprint info — populated on detail reads, may be absent on list reads. */
  deviceInfo?: Record<string, unknown>;
};

export type SessionDetail = SessionItem & {
  deviceInfo: Record<string, unknown>;
  revision: number;
};

// ─────────────────────────────────────────────
// Zod Schemas
// ─────────────────────────────────────────────

const sessionListItemSchema = z.object({
  id: z.string(),
  userId: z.string(),
  username: z.string(),
  organizationId: z.string(),
  organizationSlug: z.string(),
  applicationId: z.string(),
  applicationSlug: z.string(),
  sessionState: z.string(),
  ipAddress: z.string(),
  userAgent: z.string(),
  expiresAt: z.string(),
  revokedAt: z.string().nullable().optional(),
  createdAt: z.string(),
  lastSeenAt: z.string(),
  updatedAt: z.string(),
});

const sessionDetailSchema = sessionListItemSchema.extend({
  deviceInfo: z.record(z.string(), z.unknown()),
  revision: z.number(),
});

const sessionListResponseSchema = z.object({
  items: z.array(sessionListItemSchema),
  page: pageMetaSchema,
});

const revokeOtherSessionsResponseSchema = z.object({
  status: z.literal("revoked"),
  revokedCount: z.number(),
});

// ─────────────────────────────────────────────
// Mappers
// ─────────────────────────────────────────────

function toSessionItem(item: z.infer<typeof sessionListItemSchema>): SessionItem {
  return {
    id: item.id,
    userId: item.userId,
    username: item.username,
    organizationId: item.organizationId,
    organizationSlug: item.organizationSlug,
    applicationId: item.applicationId,
    applicationSlug: item.applicationSlug,
    sessionState: item.sessionState,
    ipAddress: item.ipAddress,
    userAgent: item.userAgent,
    expiresAt: item.expiresAt,
    revokedAt: item.revokedAt ?? null,
    createdAt: item.createdAt,
    lastSeenAt: item.lastSeenAt,
    updatedAt: item.updatedAt,
  };
}

function toSessionDetail(item: z.infer<typeof sessionDetailSchema>): SessionDetail {
  return {
    ...toSessionItem(item),
    deviceInfo: item.deviceInfo,
    revision: item.revision,
  };
}

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export async function getSessions(organizationId?: string) {
  const query = new URLSearchParams({
    page: "1",
    pageSize: "50",
    activeOnly: "true",
  });
  if (organizationId) {
    query.set("organizationId", organizationId);
  }

  const response = await requestDocumentJson(`${SESSIONS}?${query.toString()}`);
  return sessionListResponseSchema.parse(response).items.map(toSessionItem);
}

export type SessionListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  organizationId?: string;
  applicationId?: string;
  activeOnly?: boolean;
};

export async function getSessionsPaginated(params?: SessionListParams) {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 50),
    activeOnly: String(params?.activeOnly ?? true),
  });
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  if (params?.applicationId) query.set("applicationId", params.applicationId);
  if (params?.search) query.set("keyword", params.search);

  const response = await requestDocumentJson(`${SESSIONS}?${query.toString()}`);
  const parsed = sessionListResponseSchema.parse(response);
  return {
    items: parsed.items.map(toSessionItem),
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  } satisfies PaginatedResult<SessionItem>;
}

export async function getCurrentSession(): Promise<SessionDetail> {
  const response = await requestDocumentJson(SESSIONS_CURRENT);
  return toSessionDetail(sessionDetailSchema.parse(response));
}

export async function revokeSession(sessionId: string) {
  await requestDocumentJson(SESSION_REVOKE(sessionId), {
    method: "POST",
  });
}

export async function revokeOtherSessions() {
  const response = await requestDocumentJson(SESSIONS_CURRENT_REVOKE_OTHERS, {
    method: "POST",
  });

  return revokeOtherSessionsResponseSchema.parse(response);
}

// ─────────────────────────────────────────────
// Session Is-Duplicated Check
// ─────────────────────────────────────────────

export async function isSessionDuplicated(
  userId: string,
  currentSessionId: string,
): Promise<boolean> {
  const response = await requestDocumentJson(SESSIONS_IS_DUPLICATED, {
    method: "POST",
    body: JSON.stringify({ userId, currentSessionId }),
    headers: { "Content-Type": "application/json" },
  });
  return (response as { duplicated: boolean }).duplicated;
}

/** Get a session by ID. */
export async function getSession(id: string): Promise<SessionDetail> {
  const response = await requestDocumentJson(SESSION_BY_ID(id));
  return toSessionDetail(sessionDetailSchema.parse(response));
}

/** Delete a session by ID. */
export async function deleteSession(id: string): Promise<void> {
  await requestDocumentJson(SESSION_BY_ID(id), {
    method: "DELETE",
  });
}

/** Patch (partial update) a session. */
export async function patchSession(
  id: string,
  payload: Partial<Pick<SessionDetail, "sessionState">>,
): Promise<void> {
  await requestDocumentJson(SESSION_BY_ID(id), {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
}

// ─────────────────────────────────────────────
// AC-AUTH-009: Admin user-scoped session endpoints
// ─────────────────────────────────────────────

export type ForceLogoutAdminResponse = {
  status: "revoked";
  revokedCount: number;
  refreshCount: number;
  accessCount: number;
};

/**
 * 详情级端点在服务端强制可信租户：平台管理员会话组织可能与目标用户所属
 * 组织不同，必须把目标组织显式作为 query 传入，否则请求会落到会话组织
 * 导致 404。租户用户可不传。
 */
function withOrgQuery(base: string, organizationId?: string): string {
  if (!organizationId) return base;
  const separator = base.includes("?") ? "&" : "?";
  return `${base}${separator}organizationId=${encodeURIComponent(organizationId)}`;
}

/** List sessions for a specific user (admin /api/v1/users/{userId}/sessions). */
export async function listUserSessions(
  userId: string,
  params?: { page?: number; pageSize?: number; organizationId?: string },
): Promise<{
  items: SessionItem[];
  total: number;
  page: { page: number; pageSize: number; total: number };
}> {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 50),
  });
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  return requestDocumentJson(`${USER_SESSIONS(userId)}?${query.toString()}`) as Promise<{
    items: SessionItem[];
    total: number;
    page: { page: number; pageSize: number; total: number };
  }>;
}

/** Revoke a single session for a specific user (admin AC-AUTH-009). */
export async function revokeUserSession(
  userId: string,
  sessionId: string,
  organizationId?: string,
): Promise<void> {
  await requestDocumentJson(withOrgQuery(USER_SESSION_BY_ID(userId, sessionId), organizationId), {
    method: "DELETE",
  });
}

/** Force-logout ALL sessions for a specific user — full AC-AUTH-009 cascade. */
export async function forceLogoutUserSessions(
  userId: string,
  organizationId?: string,
): Promise<ForceLogoutAdminResponse> {
  const res = await requestDocumentJson(withOrgQuery(USER_SESSIONS(userId), organizationId), {
    method: "DELETE",
  });
  return res as ForceLogoutAdminResponse;
}
