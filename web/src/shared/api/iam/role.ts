import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import { pageMetaSchema } from "./schemas";
import { ROLES, ROLE_BY_ID, ROLES_UPLOAD } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type RoleOption = {
  id: string;
  name: string;
  displayName: string;
};

export type RoleDetail = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  description: string;
  isEnabled: boolean;
  createdAt: string;
  updatedAt: string;
  users: string[];
  groups: string[];
  groupIds: string[];
  roleIds: string[];
  domains: string[];
  revision: number;
};

export type CreateRolePayload = {
  organizationId: string;
  name: string;
  displayName: string;
  description?: string;
  users?: string[];
  groupIds?: string[];
  roleIds?: string[];
  domains?: string[];
  isEnabled?: boolean;
};

export type UpdateRolePayload = {
  displayName?: string;
  description?: string;
  users?: string[];
  groupIds?: string[];
  roleIds?: string[];
  domains?: string[];
  isEnabled?: boolean;
  revision: number;
};

// ─────────────────────────────────────────────
// Zod Schemas
// ─────────────────────────────────────────────

const roleListItemSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  name: z.string(),
  displayName: z.string(),
  description: z.string(),
  groupIds: z
    .array(z.string())
    .nullish()
    .transform((v) => v ?? []),
  isEnabled: z.boolean(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

const roleListResponseSchema = z.object({
  items: z.array(roleListItemSchema),
  page: pageMetaSchema,
});

const roleDetailSchema = roleListItemSchema.extend({
  users: z.array(z.string()).catch([]),
  groups: z.array(z.string()).catch([]),
  groupIds: z.array(z.string()).catch([]),
  roleIds: z.array(z.string()).catch([]),
  domains: z.array(z.string()).catch([]),
  revision: z.number(),
});

// ─────────────────────────────────────────────
// Mappers
// ─────────────────────────────────────────────

function toRoleOption(item: z.infer<typeof roleListItemSchema>): RoleOption {
  return {
    id: item.id,
    name: item.name,
    displayName: item.displayName,
  };
}

function toRoleItem(item: z.infer<typeof roleListItemSchema>) {
  return {
    ...item,
    owner: item.organizationId,
  };
}

function toRoleDetail(item: z.infer<typeof roleDetailSchema>): RoleDetail {
  return {
    id: item.id,
    organizationId: item.organizationId,
    name: item.name,
    displayName: item.displayName,
    description: item.description,
    isEnabled: item.isEnabled,
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
    users: item.users,
    groups: item.groups,
    groupIds: item.groupIds,
    roleIds: item.roleIds,
    domains: item.domains,
    revision: item.revision,
  };
}

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export async function getRoles(organizationId?: string) {
  const query = new URLSearchParams({ page: "1", pageSize: "50" });
  if (organizationId) {
    query.set("organizationId", organizationId);
  }
  const response = await requestDocumentJson(`${ROLES}?${query.toString()}`);
  return roleListResponseSchema.parse(response).items.map(toRoleItem);
}

export type RoleListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  organizationId?: string;
  isEnabled?: boolean;
};

export async function getRolesPaginated(params?: RoleListParams) {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 50),
  });
  if (params?.search) query.set("keyword", params.search);
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  if (params?.isEnabled !== undefined) query.set("isEnabled", String(params.isEnabled));
  const response = await requestDocumentJson(`${ROLES}?${query.toString()}`);
  const parsed = roleListResponseSchema.parse(response);
  return {
    items: parsed.items.map(toRoleItem),
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  };
}

export async function getRoleOptions(organizationId?: string) {
  const query = new URLSearchParams({ page: "1", pageSize: "50" });
  if (organizationId) {
    query.set("organizationId", organizationId);
  }
  const response = await requestDocumentJson(`${ROLES}?${query.toString()}`);
  return roleListResponseSchema.parse(response).items.map(toRoleOption);
}

export async function getRole(id: string): Promise<RoleDetail> {
  const response = await requestDocumentJson(`${ROLE_BY_ID(id)}`);
  return toRoleDetail(roleDetailSchema.parse(response));
}

export async function createRole(payload: CreateRolePayload) {
  await requestDocumentJson(`${ROLES}`, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
}

export async function updateRole(id: string, payload: UpdateRolePayload) {
  await requestDocumentJson(`${ROLE_BY_ID(id)}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
}

export async function deleteRole(id: string) {
  await requestDocumentJson(`${ROLE_BY_ID(id)}`, {
    method: "DELETE",
  });
}

export async function importRolesFromExcel(file: File, organizationId?: string) {
  const formData = new FormData();
  formData.append("file", file);
  const query = organizationId ? `?organizationId=${encodeURIComponent(organizationId)}` : "";
  const response = await requestDocumentJson(`${ROLES_UPLOAD}${query}`, {
    method: "POST",
    body: formData,
  });
  return response as { total: number; succeeded: number; failed: number; errors?: string[] };
}
