import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import type { PaginatedResult } from "./client";
import { RECORDS, RECORDS_SEARCH } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type RecordItem = z.infer<typeof recordListItemSchema>;

// ─────────────────────────────────────────────
// Zod Schemas
// ─────────────────────────────────────────────

const recordListItemSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  actorId: z.string(),
  action: z.string(),
  resourceType: z.string(),
  resourceId: z.string(),
  ip: z.string(),
  userAgent: z.string().optional().nullable(),
  metadata: z.record(z.string(), z.any()).optional().nullable(),
  createdAt: z.string(),
});

const recordListResponseSchema = z.object({
  items: z.array(recordListItemSchema),
  page: z.object({ total: z.number(), page: z.number(), pageSize: z.number() }),
});

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export async function getRecords(organizationId?: string) {
  const query = new URLSearchParams({ page: "1", pageSize: "50" });
  if (organizationId) query.set("organizationId", organizationId);
  const response = await requestDocumentJson(`${RECORDS}?${query.toString()}`);
  return recordListResponseSchema.parse(response).items;
}

export async function searchRecords(payload: { keyword?: string; organizationId?: string }) {
  const response = await requestDocumentJson(`${RECORDS_SEARCH}`, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
  return recordListResponseSchema.parse(response).items;
}

export type RecordListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  organizationId?: string;
  action?: string;
  resourceType?: string;
};

export async function getRecordsPaginated(params?: RecordListParams) {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 50),
  });
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  // `keyword` is the parameter the audit handlers read; there is no `search`
  // query parameter on the backend, so the previous name was silently ignored.
  if (params?.search) query.set("keyword", params.search);
  if (params?.action) query.set("action", params.action);
  if (params?.resourceType) query.set("resourceType", params.resourceType);

  const response = await requestDocumentJson(`${RECORDS}?${query.toString()}`);
  const parsed = recordListResponseSchema.parse(response);
  return {
    items: parsed.items,
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  } satisfies PaginatedResult<RecordItem>;
}

export async function searchRecordsPaginated(payload: {
  keyword?: string;
  organizationId?: string;
  page?: number;
  pageSize?: number;
}) {
  const response = await requestDocumentJson(`${RECORDS_SEARCH}`, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
  const parsed = recordListResponseSchema.parse(response);
  return {
    items: parsed.items,
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  } satisfies PaginatedResult<RecordItem>;
}
