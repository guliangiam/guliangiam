import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import {
  MY_NOTIFICATIONS,
  NOTIFICATION_BROADCAST,
  MY_NOTIFICATIONS_BATCH_DELETE,
  MY_NOTIFICATIONS_READ_ALL,
  MY_NOTIFICATION_BY_ID,
  MY_NOTIFICATION_CHANNEL_BINDING_BY_ID,
  MY_NOTIFICATION_CHANNEL_BINDINGS,
  MY_NOTIFICATION_PREFERENCES,
  MY_NOTIFICATION_READ,
  MY_NOTIFICATION_REPLIES,
  MY_NOTIFICATION_UNREAD,
  MY_NOTIFICATION_UNREAD_COUNT,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type UserNotificationCategory =
  | "system"
  | "security"
  | "business"
  | "feedback"
  | "announcement";

export type UserNotificationChannel = "inapp" | "email" | "sms" | "wecom_app" | "wechat_official";

export type UserNotificationItem = z.infer<typeof userNotificationItemSchema>;
export type UserNotificationReply = z.infer<typeof userNotificationReplySchema>;
export type UserNotificationPreference = z.infer<typeof userNotificationPreferenceSchema>;
export type UserNotificationChannelBinding = z.infer<typeof userNotificationChannelBindingSchema>;

export type ListMyNotificationsParams = {
  category?: string;
  isRead?: boolean;
  keyword?: string;
  page?: number;
  pageSize?: number;
};

// ─────────────────────────────────────────────
// Zod Schemas
// ─────────────────────────────────────────────

const userNotificationItemSchema = z.object({
  id: z.string(),
  category: z.string(),
  eventType: z.string().optional().default(""),
  title: z.string(),
  content: z.string().optional().default(""),
  link: z.string().optional().default(""),
  sourceType: z.string().optional().default(""),
  sourceId: z.string().nullable().optional(),
  allowReply: z.boolean().optional().default(false),
  isRead: z.boolean(),
  readAt: z.string().nullable().optional(),
  createdAt: z.string(),
});

const userNotificationListSchema = z.object({
  items: z.array(userNotificationItemSchema),
  total: z.number(),
  page: z.number(),
  pageSize: z.number(),
});

const userNotificationReplySchema = z.object({
  id: z.string(),
  content: z.string(),
  userId: z.string(),
  createdAt: z.string(),
});

const userNotificationPreferenceSchema = z.object({
  category: z.string(),
  channel: z.string(),
  enabled: z.boolean(),
});

const userNotificationChannelBindingSchema = z.object({
  id: z.string(),
  channel: z.string(),
  externalId: z.string(),
  verified: z.boolean(),
  createdAt: z.string(),
});

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

/** 动作接口统一返回 {status,message,data:{count}} 信封；兼容裸 {count}。 */
function countOf(response: unknown): number {
  if (!response || typeof response !== "object") return 0;
  const envelope = response as { count?: unknown; data?: { count?: unknown } };
  const raw = envelope.data?.count ?? envelope.count;
  return typeof raw === "number" ? raw : Number(raw ?? 0) || 0;
}

function queryOf(params: ListMyNotificationsParams) {
  const query = new URLSearchParams();
  if (params.category) query.set("category", params.category);
  if (params.isRead !== undefined) query.set("isRead", String(params.isRead));
  if (params.keyword) query.set("keyword", params.keyword);
  query.set("page", String(params.page ?? 1));
  query.set("pageSize", String(params.pageSize ?? 20));
  return `${MY_NOTIFICATIONS}?${query.toString()}`;
}

export async function listMyNotifications(params: ListMyNotificationsParams = {}) {
  const response = await requestDocumentJson(queryOf(params));
  return userNotificationListSchema.parse(
    response ?? { items: [], total: 0, page: 1, pageSize: 20 },
  );
}

export async function getMyUnreadNotificationCount(): Promise<number> {
  const response = await requestDocumentJson<{ count?: number }>(MY_NOTIFICATION_UNREAD_COUNT);
  return response?.count ?? 0;
}

export async function getMyNotification(id: string): Promise<UserNotificationItem> {
  const response = await requestDocumentJson(MY_NOTIFICATION_BY_ID(id));
  return userNotificationItemSchema.parse(response);
}

export async function markMyNotificationRead(id: string) {
  await requestDocumentJson(MY_NOTIFICATION_READ(id), { method: "POST" });
}

export async function markMyNotificationUnread(id: string) {
  await requestDocumentJson(MY_NOTIFICATION_UNREAD(id), { method: "POST" });
}

export async function markAllMyNotificationsRead(category?: string): Promise<number> {
  const path = category
    ? `${MY_NOTIFICATIONS_READ_ALL}?category=${encodeURIComponent(category)}`
    : MY_NOTIFICATIONS_READ_ALL;
  const response = await requestDocumentJson<unknown>(path, { method: "POST" });
  return countOf(response);
}

export async function deleteMyNotification(id: string) {
  await requestDocumentJson(MY_NOTIFICATION_BY_ID(id), { method: "DELETE" });
}

export async function batchDeleteMyNotifications(ids: string[]): Promise<number> {
  const response = await requestDocumentJson<unknown>(MY_NOTIFICATIONS_BATCH_DELETE, {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
  return countOf(response);
}

export async function listMyNotificationReplies(id: string): Promise<UserNotificationReply[]> {
  const response = await requestDocumentJson<{ items?: unknown[] }>(MY_NOTIFICATION_REPLIES(id));
  return z.array(userNotificationReplySchema).parse(response?.items ?? []);
}

export async function replyToMyNotification(
  id: string,
  content: string,
): Promise<UserNotificationReply> {
  const response = await requestDocumentJson(MY_NOTIFICATION_REPLIES(id), {
    method: "POST",
    body: JSON.stringify({ content }),
  });
  return userNotificationReplySchema.parse(response);
}

export async function listMyNotificationPreferences(): Promise<UserNotificationPreference[]> {
  const response = await requestDocumentJson<{ items?: unknown[] }>(MY_NOTIFICATION_PREFERENCES);
  return z.array(userNotificationPreferenceSchema).parse(response?.items ?? []);
}

export async function updateMyNotificationPreferences(
  preferences: UserNotificationPreference[],
): Promise<UserNotificationPreference[]> {
  const response = await requestDocumentJson<{ items?: unknown[] }>(MY_NOTIFICATION_PREFERENCES, {
    method: "PUT",
    body: JSON.stringify({ preferences }),
  });
  return z.array(userNotificationPreferenceSchema).parse(response?.items ?? []);
}

export async function listMyNotificationChannelBindings(): Promise<
  UserNotificationChannelBinding[]
> {
  const response = await requestDocumentJson<{ items?: unknown[] }>(
    MY_NOTIFICATION_CHANNEL_BINDINGS,
  );
  return z.array(userNotificationChannelBindingSchema).parse(response?.items ?? []);
}

export async function upsertMyNotificationChannelBinding(payload: {
  channel: string;
  externalId: string;
  verified?: boolean;
}): Promise<UserNotificationChannelBinding> {
  const response = await requestDocumentJson(MY_NOTIFICATION_CHANNEL_BINDINGS, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
  return userNotificationChannelBindingSchema.parse(response);
}

export async function deleteMyNotificationChannelBinding(id: string) {
  await requestDocumentJson(MY_NOTIFICATION_CHANNEL_BINDING_BY_ID(id), { method: "DELETE" });
}

export type BroadcastNotificationPayload = {
  userIds: string[];
  category: string;
  title: string;
  content: string;
  link?: string;
  allowReply?: boolean;
};

export type BroadcastNotificationResult = {
  delivered: number;
  failed: number;
  failedUserIds?: string[];
};

/** 管理员向指定用户群发通知（写入各自收件箱，并按偏好扇出外部渠道）。 */
export async function broadcastNotification(
  payload: BroadcastNotificationPayload,
): Promise<BroadcastNotificationResult> {
  const response = await requestDocumentJson<BroadcastNotificationResult>(NOTIFICATION_BROADCAST, {
    method: "POST",
    body: JSON.stringify(payload),
  });
  return {
    delivered: response?.delivered ?? 0,
    failed: response?.failed ?? 0,
    failedUserIds: response?.failedUserIds,
  };
}
