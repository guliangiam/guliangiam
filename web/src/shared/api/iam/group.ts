import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import { GROUPS, GROUPS_BULK_IMPORT, GROUPS_UPLOAD, GROUP_BY_ID } from "@/shared/api/paths";
import { pageMetaSchema } from "./schemas";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type GroupDetail = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  parentId: string;
  isTopGroup: boolean;
  isEnabled: boolean;
  createdAt: string;
  updatedAt: string;
  manager: string;
  contactEmail: string;
  homepage: string;
  users: string[];
  revision: number;
};

export type CreateGroupPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  manager?: string;
  contactEmail?: string;
  homepage?: string;
  type?: string;
  parentId?: string;
  isTopGroup?: boolean;
  isEnabled?: boolean;
  users?: string[];
};

export type UpdateGroupPayload = {
  displayName?: string;
  manager?: string;
  contactEmail?: string;
  homepage?: string;
  type?: string;
  parentId?: string;
  isTopGroup?: boolean;
  isEnabled?: boolean;
  users?: string[];
  revision: number;
};

export type GroupOption = {
  id: string;
  name: string;
  displayName: string;
};

// ─────────────────────────────────────────────
// Zod Schemas
// ─────────────────────────────────────────────

const groupListItemSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  name: z.string(),
  displayName: z.string(),
  type: z.string(),
  parentId: z.string(),
  isTopGroup: z.boolean(),
  isEnabled: z.boolean(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

const groupListResponseSchema = z.object({
  items: z.array(groupListItemSchema),
  page: pageMetaSchema,
});

const groupDetailSchema = groupListItemSchema.extend({
  manager: z.string(),
  contactEmail: z.string(),
  homepage: z.string().default(""),
  users: z.array(z.string()).catch([]),
  revision: z.number(),
});

// ─────────────────────────────────────────────
// Mappers
// ─────────────────────────────────────────────

export type GroupItem = {
  id: string;
  owner: string;
  name: string;
  displayName: string;
  type: string;
  parentId: string;
  isTopGroup: boolean;
  isEnabled: boolean;
};

function toGroupItem(item: z.infer<typeof groupListItemSchema>): GroupItem {
  return {
    ...item,
    owner: item.organizationId,
  };
}

function toGroupDetail(item: z.infer<typeof groupDetailSchema>): GroupDetail {
  return {
    id: item.id,
    organizationId: item.organizationId,
    name: item.name,
    displayName: item.displayName,
    type: item.type,
    parentId: item.parentId,
    isTopGroup: item.isTopGroup,
    isEnabled: item.isEnabled,
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
    manager: item.manager,
    contactEmail: item.contactEmail,
    homepage: item.homepage,
    users: item.users,
    revision: item.revision,
  };
}

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

/**
 * 详情级端点在服务端强制可信租户：平台管理员会话组织可能与目标组所属组织
 * 不同，必须把目标组织显式作为 query 传入，否则请求会落到会话组织导致 404
 * （用户组详情页打不开的根因之一）。租户用户可不传。
 */
function withOrgQuery(base: string, organizationId?: string): string {
  if (!organizationId) return base;
  const separator = base.includes("?") ? "&" : "?";
  return `${base}${separator}organizationId=${encodeURIComponent(organizationId)}`;
}

export async function getGroups(organizationId?: string) {
  const query = new URLSearchParams({ page: "1", pageSize: "50" });
  if (organizationId) {
    query.set("organizationId", organizationId);
  }
  const response = await requestDocumentJson(`${GROUPS}?${query.toString()}`);
  return groupListResponseSchema.parse(response).items.map(toGroupItem);
}

export type GroupListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  organizationId?: string;
  /** 后端列表仅识别 active/inactive（sql.go 的 CASE 映射），其余值等同不过滤。 */
  status?: string;
  /** 闭集 virtual/physical/department（G7-s）；空值不过滤。 */
  type?: string;
  sortBy?: string;
  sortOrder?: string;
};

export async function getGroupsPaginated(params?: GroupListParams) {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 50),
  });
  if (params?.search) query.set("keyword", params.search);
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  if (params?.status) query.set("status", params.status);
  if (params?.type) query.set("type", params.type);
  if (params?.sortBy) query.set("sortField", params.sortBy);
  if (params?.sortOrder) query.set("sortOrder", params.sortOrder);
  const response = await requestDocumentJson(`${GROUPS}?${query.toString()}`);
  const parsed = groupListResponseSchema.parse(response);
  return {
    items: parsed.items.map(toGroupItem),
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  };
}

export async function getGroupOptions(organizationId?: string): Promise<GroupOption[]> {
  const query = new URLSearchParams({ page: "1", pageSize: "50" });
  if (organizationId) {
    query.set("organizationId", organizationId);
  }
  const response = await requestDocumentJson(`${GROUPS}?${query.toString()}`);
  return groupListResponseSchema.parse(response).items.map((item) => ({
    id: item.id,
    name: item.name,
    displayName: item.displayName,
  }));
}

export async function getGroup(id: string, organizationId?: string): Promise<GroupDetail> {
  // 同 organization/application：详情 GET 的 `private, max-age=30` 会让
  // 保存后的失效重取读到旧 revision；编辑器必须绕开 HTTP 缓存。
  const response = await requestDocumentJson(withOrgQuery(GROUP_BY_ID(id), organizationId), {
    cache: "no-store",
  });
  return toGroupDetail(groupDetailSchema.parse(response));
}

export async function createGroup(payload: CreateGroupPayload) {
  await requestDocumentJson(GROUPS, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
}

export async function updateGroup(
  id: string,
  payload: UpdateGroupPayload,
  organizationId?: string,
) {
  await requestDocumentJson(withOrgQuery(GROUP_BY_ID(id), organizationId), {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
}

export async function deleteGroup(id: string, organizationId?: string) {
  await requestDocumentJson(withOrgQuery(GROUP_BY_ID(id), organizationId), {
    method: "DELETE",
  });
}

// ─────────────────────────────────────────────
// Bulk Import
// ─────────────────────────────────────────────

export type ImportGroupItem = {
  name: string;
  displayName: string;
  type?: string;
  parentId?: string;
  organizationId?: string;
};

export type GroupBulkImportResult = {
  total: number;
  succeeded: number;
  failed: number;
  errors?: string[];
};

export async function importGroups(
  organizationId: string,
  groups: ImportGroupItem[],
): Promise<GroupBulkImportResult> {
  const response = await requestDocumentJson(GROUPS_BULK_IMPORT, {
    method: "POST",
    body: JSON.stringify({ organizationId, groups }),
    headers: { "Content-Type": "application/json" },
  });
  return response as GroupBulkImportResult;
}

export async function importGroupsFromExcel(file: File, organizationId?: string) {
  const formData = new FormData();
  formData.append("file", file);
  const query = organizationId ? `?organizationId=${encodeURIComponent(organizationId)}` : "";
  const response = await requestDocumentJson(`${GROUPS_UPLOAD}${query}`, {
    method: "POST",
    body: formData,
  });
  return response as GroupBulkImportResult;
}
