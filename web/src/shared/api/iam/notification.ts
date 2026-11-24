import { z } from "zod";
import { iamList, iamCreate, iamPostRaw, type IamListParams } from "./client";
import {
  NOTIFICATIONS,
  NOTIFICATIONS_MARK_ALL_READ,
  NOTIFICATION_MARK_READ,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type NotificationItem = z.infer<typeof notificationItemSchema>;

// ─────────────────────────────────────────────
// Zod Schemas
// ─────────────────────────────────────────────

const notificationItemSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  providerId: z.string(),
  recipient: z.string(),
  subject: z.string().optional().default(""),
  content: z.string(),
  templateId: z.string().optional().default(""),
  variables: z.record(z.string(), z.any()).optional().nullable(),
  status: z.string(),
  error: z.string().optional().default(""),
  sentAt: z.string().optional().nullable(),
  retryCount: z.number().optional().default(0),
  createdAt: z.string(),
  updatedAt: z.string(),
});

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export const getNotifications = (params?: IamListParams) =>
  iamList<NotificationItem>(NOTIFICATIONS, params, notificationItemSchema);

export type CreateNotificationPayload = {
  organizationId: string;
  providerId: string;
  recipient: string;
  subject?: string;
  content: string;
  templateId?: string;
  variables?: Record<string, unknown>;
};

export const createNotification = (payload: CreateNotificationPayload) =>
  iamCreate(NOTIFICATIONS, payload);

export const markNotificationRead = (id: string) => iamPostRaw(NOTIFICATION_MARK_READ(id));

export const markAllNotificationsRead = (organizationId: string) =>
  iamPostRaw(NOTIFICATIONS_MARK_ALL_READ, {
    organizationId,
  });
