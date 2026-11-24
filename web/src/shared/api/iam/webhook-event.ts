import {
  iamList,
  iamGet,
  iamDelete,
  iamPostRaw,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { WEBHOOK_EVENT_BY_ID, WEBHOOK_EVENTS, WEBHOOK_EVENT_REPLAY } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Webhook Events
// ─────────────────────────────────────────────
export type IamWebhookEventItem = {
  id: string;
  organizationId: string;
  webhookId: string;
  eventType: string;
  status: string;
  createdAt: string;
  updatedAt: string;
  payload?: unknown;
  responseBody?: string;
};
export type IamWebhookEventDetail = IamWebhookEventItem & {
  payload: Record<string, unknown>;
  responseCode: number;
  responseBody: string;
  deliveredAt: string;
  failedAt: string;
  failureReason: string;
  nextRetryAt: string;
  retryCount: number;
};

export const getIamWebhookEvents = (webhookId: string, params?: IamListParams) =>
  iamList<IamWebhookEventItem>(WEBHOOK_EVENTS(webhookId), params);
export const getIamWebhookEvent = (eventId: string) =>
  iamGet<IamWebhookEventDetail>(`${WEBHOOK_EVENT_BY_ID(eventId)}`);
export const deleteIamWebhookEvent = (eventId: string) =>
  iamDelete(`${WEBHOOK_EVENT_BY_ID(eventId)}`);
export const replayIamWebhookEvent = (eventId: string) => iamPostRaw(WEBHOOK_EVENT_REPLAY(eventId));

export const getIamWebhookEventsPaginated = (webhookId: string, params?: IamListParams) =>
  iamListWithPagination<IamWebhookEventItem>(WEBHOOK_EVENTS(webhookId), params);
