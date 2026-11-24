import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import { RECORDS_AUDIT_LOGS, RECORDS_LOGIN_LOGS } from "@/shared/api/paths";
import type { PaginatedResult } from "./client";
import type { RecordItem } from "./record";

/**
 * 审计中心分类日志 API。
 *
 * - 操作/错误日志：`GET /api/v1/records/audit-logs`（records 的业务分类视图）
 * - 登录日志：`GET /api/v1/records/login-logs`（成功会话 + 失败审计记录）
 *
 * 分类与结果由后端按 action 约定推导，前端只传筛选条件。
 */

export type AuditLogCategory = "login" | "operation" | "error";
export type AuditLogResult = "success" | "failure";

export type AuditLogParams = {
  page?: number;
  pageSize?: number;
  category?: AuditLogCategory;
  result?: AuditLogResult;
  applicationId?: string;
  keyword?: string;
  startTime?: string;
  endTime?: string;
  organizationId?: string;
};

const loginLogItemSchema = z.object({
  id: z.string(),
  result: z.enum(["success", "failure"]),
  username: z.string().optional().nullable(),
  organizationId: z.string().optional().nullable(),
  applicationId: z.string().optional().nullable(),
  ip: z.string().optional().nullable(),
  userAgent: z.string().optional().nullable(),
  reason: z.string().optional().nullable(),
  createdAt: z.string(),
});

export type LoginLogItem = z.infer<typeof loginLogItemSchema>;

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

function buildQuery(params: AuditLogParams): string {
  const query = new URLSearchParams({
    page: String(params.page ?? 1),
    pageSize: String(params.pageSize ?? 50),
  });
  if (params.category) query.set("category", params.category);
  if (params.result) query.set("result", params.result);
  if (params.applicationId) query.set("applicationId", params.applicationId);
  if (params.keyword) query.set("keyword", params.keyword);
  if (params.startTime) query.set("startTime", params.startTime);
  if (params.endTime) query.set("endTime", params.endTime);
  if (params.organizationId) query.set("organizationId", params.organizationId);
  return query.toString();
}

function toPaginated<T>(parsed: {
  items: T[];
  page: { total: number; page: number; pageSize: number };
}): PaginatedResult<T> {
  return {
    items: parsed.items,
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  };
}

export async function getAuditLogs(
  params: AuditLogParams = {},
): Promise<PaginatedResult<RecordItem>> {
  const response = await requestDocumentJson(`${RECORDS_AUDIT_LOGS}?${buildQuery(params)}`);
  const parsed = z
    .object({
      items: z.array(recordListItemSchema),
      page: z.object({ total: z.number(), page: z.number(), pageSize: z.number() }),
    })
    .parse(response);
  return toPaginated(parsed);
}

export async function getLoginLogs(
  params: AuditLogParams = {},
): Promise<PaginatedResult<LoginLogItem>> {
  const response = await requestDocumentJson(`${RECORDS_LOGIN_LOGS}?${buildQuery(params)}`);
  const parsed = z
    .object({
      items: z.array(loginLogItemSchema),
      page: z.object({ total: z.number(), page: z.number(), pageSize: z.number() }),
    })
    .parse(response);
  return toPaginated(parsed);
}
