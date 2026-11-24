import { requestDocumentJson } from "@/shared/api/http";
import {
  METERING_AGGREGATES,
  METERING_AGGREGATES_REFRESH,
  METERING_EVENTS,
  METERING_EVENTS_INGEST,
} from "@/shared/api/paths";
import type { IamListResponse } from "./client";

// ─────────────────────────────────────────────
// Metering domain — aligned with backend EventDTO / AggregateDTO
// from internal/app/metering/service.go
// ─────────────────────────────────────────────

/** One raw metering event (append-only, idempotent). */
export type MeteringEventItem = {
  id: string;
  organizationId: string;
  eventType: string;
  subject: string;
  quantity: number;
  unit: string;
  occurredAt: string;
  metadata: Record<string, unknown> | null;
  createdAt: string;
};

/** One pre-rolled aggregate cell: (org, period, subject, metric) → quantity. */
export type MeteringAggregateItem = {
  id: string;
  organizationId: string;
  period: string;
  subject: string;
  metric: string;
  quantity: number;
  unit: string;
  calculatedAt: string;
  createdAt: string;
};

/** Filter params passed as query string to GET /metering/events */
export type ListMeteringEventsParams = {
  /** ISO-8601 inclusive lower bound for occurredAt */
  from?: string;
  /** ISO-8601 exclusive upper bound for occurredAt */
  to?: string;
  eventType?: string;
  subject?: string;
  page?: number;
  pageSize?: number;
};

/** Filter params passed as query string to GET /metering/aggregates */
export type ListMeteringAggregatesParams = {
  period?: string;
  subject?: string;
  metric?: string;
  page?: number;
  pageSize?: number;
};

export type IngestMeteringEventsRequest = {
  events: Array<{
    eventType: string;
    subject?: string;
    quantity: number;
    unit?: string;
    idempotencyKey: string;
    metadata?: Record<string, unknown>;
  }>;
};

/** Build URL with optional query params. Empty values are dropped. */
function withQuery(base: string, params?: Record<string, string | number | undefined>): string {
  if (!params) return base;
  const pairs = Object.entries(params)
    .filter(([, v]) => v !== undefined && v !== "")
    .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`);
  return pairs.length === 0 ? base : `${base}?${pairs.join("&")}`;
}

export function listMeteringEvents(params?: ListMeteringEventsParams) {
  return requestDocumentJson<IamListResponse<MeteringEventItem>>(
    withQuery(METERING_EVENTS, {
      from: params?.from,
      to: params?.to,
      eventType: params?.eventType,
      subject: params?.subject,
      page: params?.page,
      pageSize: params?.pageSize,
    }),
  );
}

export function listMeteringAggregates(params?: ListMeteringAggregatesParams) {
  return requestDocumentJson<IamListResponse<MeteringAggregateItem>>(
    withQuery(METERING_AGGREGATES, {
      period: params?.period,
      subject: params?.subject,
      metric: params?.metric,
      page: params?.page,
      pageSize: params?.pageSize,
    }),
  );
}

export function ingestMeteringEvents(input: IngestMeteringEventsRequest) {
  return requestDocumentJson<{ inserted: number; duplicates: number }>(METERING_EVENTS_INGEST, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

/** POST /metering/aggregates/refresh — admin trigger single-cell re-roll. */
export function refreshMeteringAggregate(params: {
  period: string;
  metric: string;
  subject?: string;
}) {
  return requestDocumentJson<MeteringAggregateItem>(
    withQuery(METERING_AGGREGATES_REFRESH, {
      period: params.period,
      metric: params.metric,
      subject: params.subject,
    }),
    { method: "POST" },
  );
}
