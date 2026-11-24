import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import { SEARCH } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Zod Schemas
// ─────────────────────────────────────────────

export const searchItemSchema = z.object({
  id: z.string(),
  title: z.string(),
  subtitle: z.string().default(""),
  name: z.string().optional(),
  username: z.string().optional(),
  organizationId: z.string().optional(),
  organizationSlug: z.string().optional(),
});

export const searchGroupSchema = z.object({
  entity: z.string(),
  items: z.array(searchItemSchema),
});

export const searchResultSchema = z.object({
  query: z.string(),
  total: z.number(),
  groups: z.array(searchGroupSchema),
});

export type SearchItem = z.infer<typeof searchItemSchema>;
export type SearchGroup = z.infer<typeof searchGroupSchema>;
export type SearchResult = z.infer<typeof searchResultSchema>;

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

/**
 * 全局聚合搜索：按关键词检索可信租户范围内的核心实体，返回按实体分组的结果。
 * 后端 GET /api/v1/search（权限 search:read）。
 */
export async function globalSearch(keyword: string, limit?: number): Promise<SearchResult> {
  const query = new URLSearchParams({ q: keyword });
  if (limit !== undefined) query.set("limit", String(limit));
  const response = await requestDocumentJson(`${SEARCH}?${query.toString()}`);
  return searchResultSchema.parse(response);
}
