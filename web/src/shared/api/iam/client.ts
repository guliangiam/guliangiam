import { z } from "zod";
import { requestDocumentJson, requestDocumentJsonIfChanged } from "@/shared/api/http";
import type { Paginated } from "@/shared/api/pagination";

// ─────────────────────────────────────────────
// Common list response shape from guliangiam API
// ─────────────────────────────────────────────

/** 列表 envelope；类型定义唯一源在 shared/api/pagination.ts。 */
export type IamListResponse<T> = Paginated<T>;

// Flat paginated result for UI components
export type PaginatedResult<T> = {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
};

export type IamListParams = {
  page?: number;
  pageSize?: number;
  /**
   * Free-text filter. Sent as `keyword` — the name every list handler reads
   * (`r.URL.Query().Get("keyword")`). The backend has no `search` parameter, so
   * sending `search` made the search box a silent no-op.
   */
  search?: string;
  /**
   * Sort column. Sent as `sortField` — the name the list handlers read. It must
   * be a camelCase field name from the endpoint's SortFields whitelist; an
   * unknown field is ignored (see the backend's buildOrderBy).
   */
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  organizationId?: string;
  owner?: string;
  status?: string;
  /** Arbitrary filter key-value pairs passed as query parameters */
  filters?: Record<string, string>;
};

// Default pagination for IAM API queries.
const defaultPageSize = 50;

export function buildQueryString(params?: IamListParams): string {
  if (!params) return `page=1&pageSize=${defaultPageSize}`;
  const parts: string[] = [];
  parts.push(`page=${params.page ?? 1}`);
  parts.push(`pageSize=${params.pageSize ?? defaultPageSize}`);
  if (params.search) parts.push(`keyword=${encodeURIComponent(params.search)}`);
  if (params.sortBy) parts.push(`sortField=${encodeURIComponent(params.sortBy)}`);
  if (params.sortOrder) parts.push(`sortOrder=${params.sortOrder}`);
  if (params.organizationId)
    parts.push(`organizationId=${encodeURIComponent(params.organizationId)}`);
  if (params.owner) parts.push(`owner=${encodeURIComponent(params.owner)}`);
  if (params.status) parts.push(`status=${encodeURIComponent(params.status)}`);
  if (params.filters) {
    for (const [key, value] of Object.entries(params.filters)) {
      if (value) parts.push(`${encodeURIComponent(key)}=${encodeURIComponent(value)}`);
    }
  }
  return parts.join("&");
}

// iamListResponseSchema validates the common list response shape at runtime.
const iamListResponseSchema = z.object({
  items: z.array(z.unknown()),
  page: z
    .object({
      page: z.number(),
      pageSize: z.number(),
      total: z.number(),
    })
    .optional(),
});

export async function iamList<T>(
  path: string,
  params?: IamListParams,
  itemSchema?: z.ZodType<T>,
): Promise<T[]> {
  const separator = path.includes("?") ? "&" : "?";
  const qs = buildQueryString(params);
  const res = await requestDocumentJson(`${path}${separator}${qs}`);

  // Validate the response shape at runtime.
  const parsed = iamListResponseSchema.parse(res);
  const rawItems = parsed.items ?? [];

  if (itemSchema) {
    return rawItems.map((item) => itemSchema.parse(item));
  }
  return rawItems as T[];
}

export async function iamListWithPagination<T>(
  path: string,
  params?: IamListParams,
  itemSchema?: z.ZodType<T>,
): Promise<{ items: T[]; total: number; page: number; pageSize: number }> {
  const separator = path.includes("?") ? "&" : "?";
  const qs = buildQueryString(params);
  const res = await requestDocumentJson(`${path}${separator}${qs}`);

  const parsed = iamListResponseSchema.parse(res);
  const p = parsed.page ?? { page: 1, pageSize: defaultPageSize, total: 0 };
  const rawItems = parsed.items ?? [];

  let items: T[];
  if (itemSchema) {
    items = rawItems.map((item) => itemSchema.parse(item));
  } else {
    items = rawItems as T[];
  }

  return { items, total: p.total, page: p.page, pageSize: p.pageSize };
}

export async function iamGet<T>(path: string, schema?: z.ZodType<T>): Promise<T> {
  // 多数详情路由未声明 no-store，ETag 中间件给 private,max-age=30；
  // 保存后 invalidate 重取若命中浏览器缓存旧体，revision 停留旧值导致二次保存 409。
  const res = await requestDocumentJson(path, { cache: "no-store" });
  if (schema) {
    return schema.parse(res);
  }
  return res as T;
}

/**
 * Conditional read of a single resource (§42.3.11).
 *
 * Returns `notModified: true` when the server confirms the caller's copy is still
 * current, which costs an empty 304 instead of re-serialising the resource. Only
 * resources whose responses carry an `ETag` benefit; callers pass the previous
 * response's `etag` back in, and must treat a `200` as "replace your copy".
 */
export async function iamGetIfChanged<T>(path: string, etag?: string) {
  return requestDocumentJsonIfChanged<T>(path, undefined, etag);
}

function jsonHeaders(): Record<string, string> {
  return { "Content-Type": "application/json" };
}

export async function iamCreate(path: string, payload: unknown) {
  return requestDocumentJson(path, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: jsonHeaders(),
  });
}

/** Type-safe POST that returns a typed response. */
export async function iamPost<T>(
  path: string,
  payload: unknown,
  schema?: z.ZodType<T>,
): Promise<T> {
  const res = await requestDocumentJson(path, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: jsonHeaders(),
  });
  if (schema) {
    return schema.parse(res);
  }
  return res as T;
}

export async function iamPatch(path: string, payload: unknown) {
  return requestDocumentJson(path, {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: jsonHeaders(),
  });
}

export async function iamUpdate(path: string, payload: unknown) {
  return requestDocumentJson(path, {
    method: "PUT",
    body: JSON.stringify(payload),
    headers: jsonHeaders(),
  });
}

export async function iamDelete(path: string, payload?: unknown) {
  if (payload === undefined) {
    return requestDocumentJson(path, { method: "DELETE" });
  }
  // The enforcer policy endpoints identify the rule in the body
  // (`PolicyRequest`), so DELETE must be able to carry one.
  return requestDocumentJson(path, {
    method: "DELETE",
    body: JSON.stringify(payload),
    headers: jsonHeaders(),
  });
}

export async function iamPostRaw(path: string, body?: unknown) {
  const opts: RequestInit = { method: "POST" };
  if (body !== undefined) {
    opts.headers = { "Content-Type": "application/json" };
    opts.body = JSON.stringify(body);
  }
  return requestDocumentJson(path, opts);
}
