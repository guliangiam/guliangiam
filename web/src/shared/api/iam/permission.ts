import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import { pageMetaSchema } from "./schemas";
import {
  PERMISSIONS,
  PERMISSIONS_BATCH_ENFORCE,
  PERMISSIONS_BULK_IMPORT,
  PERMISSIONS_ENFORCE,
  PERMISSIONS_UPLOAD,
  PERMISSION_ACTIONS,
  PERMISSION_BY_ID,
  PERMISSION_OBJECTS,
  PERMISSION_POLICY,
  PERMISSION_ROLES,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

export type PermissionDetail = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  description: string;
  resourceType: string;
  effect: string;
  isEnabled: boolean;
  submitter: string;
  approver: string;
  approveTime: string;
  state: string;
  model: string;
  adapter: string;
  sorter: string;
  application: string;
  createdAt: string;
  updatedAt: string;
  users: string[];
  groups: string[];
  roles: string[];
  domains: string[];
  resources: string[];
  actions: string[];
  revision: number;
};

export type CreatePermissionPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  description?: string;
  resourceType?: string;
  resources?: string[];
  actions?: string[];
  effect?: string;
  state?: string;
  domains?: string[];
  model?: string;
  adapter?: string;
  sorter?: string;
  application?: string;
  users?: string[];
  groups?: string[];
  roles?: string[];
  isEnabled?: boolean;
};

export type UpdatePermissionPayload = {
  displayName?: string;
  description?: string;
  resourceType?: string;
  resources?: string[];
  actions?: string[];
  effect?: string;
  state?: string;
  domains?: string[];
  model?: string;
  adapter?: string;
  sorter?: string;
  application?: string;
  users?: string[];
  groups?: string[];
  roles?: string[];
  isEnabled?: boolean;
  revision: number;
};

// ─────────────────────────────────────────────
// Zod Schemas
// ─────────────────────────────────────────────

const permissionListItemSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  name: z.string(),
  displayName: z.string(),
  description: z.string(),
  resourceType: z.string(),
  effect: z.string(),
  isEnabled: z.boolean(),
  submitter: z.string(),
  approver: z.string(),
  state: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

const permissionListResponseSchema = z.object({
  items: z.array(permissionListItemSchema),
  page: pageMetaSchema,
});

const permissionDetailSchema = permissionListItemSchema.extend({
  users: z.array(z.string()).catch([]),
  groups: z.array(z.string()).catch([]),
  roles: z.array(z.string()).catch([]),
  domains: z.array(z.string()).catch([]),
  resources: z.array(z.string()).catch([]),
  actions: z.array(z.string()).catch([]),
  model: z.string().default(""),
  adapter: z.string().default(""),
  sorter: z.string().default(""),
  application: z.string().default(""),
  approveTime: z.string().default(""),
  revision: z.number(),
});

// ─────────────────────────────────────────────
// Mappers
// ─────────────────────────────────────────────

export type PermissionItem = {
  id: string;
  owner: string;
  name: string;
  displayName: string;
  description: string;
  resourceType: string;
  effect: string;
  state: string;
  isEnabled: boolean;
  submitter: string;
  approver: string;
  createdAt: string;
  updatedAt: string;
};

function toPermissionItem(item: z.infer<typeof permissionListItemSchema>): PermissionItem {
  return {
    ...item,
    owner: item.organizationId,
  };
}

function toPermissionDetail(item: z.infer<typeof permissionDetailSchema>): PermissionDetail {
  return {
    id: item.id,
    organizationId: item.organizationId,
    name: item.name,
    displayName: item.displayName,
    description: item.description,
    resourceType: item.resourceType,
    effect: item.effect,
    isEnabled: item.isEnabled,
    submitter: item.submitter,
    approver: item.approver,
    approveTime: item.approveTime,
    state: item.state,
    model: item.model,
    adapter: item.adapter,
    sorter: item.sorter,
    application: item.application,
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
    users: item.users,
    groups: item.groups,
    roles: item.roles,
    domains: item.domains,
    resources: item.resources,
    actions: item.actions,
    revision: item.revision,
  };
}

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export async function getPermissions(organizationId?: string) {
  const query = new URLSearchParams({ page: "1", pageSize: "50" });
  if (organizationId) {
    query.set("organizationId", organizationId);
  }
  const response = await requestDocumentJson(`${PERMISSIONS}?${query.toString()}`);
  return permissionListResponseSchema.parse(response).items.map(toPermissionItem);
}

export type PermissionListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  organizationId?: string;
  state?: string;
  resourceType?: string;
  effect?: string;
  isEnabled?: boolean;
};

export async function getPermissionsPaginated(params?: PermissionListParams) {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 50),
  });
  if (params?.search) query.set("keyword", params.search);
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  if (params?.state) query.set("state", params.state);
  if (params?.resourceType) query.set("resourceType", params.resourceType);
  if (params?.isEnabled !== undefined) query.set("isEnabled", String(params.isEnabled));
  const response = await requestDocumentJson(`${PERMISSIONS}?${query.toString()}`);
  const parsed = permissionListResponseSchema.parse(response);
  return {
    items: parsed.items.map(toPermissionItem),
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  };
}

export async function getPermission(id: string): Promise<PermissionDetail> {
  const response = await requestDocumentJson(PERMISSION_BY_ID(id));
  return toPermissionDetail(permissionDetailSchema.parse(response));
}

export async function createPermission(payload: CreatePermissionPayload) {
  await requestDocumentJson(PERMISSIONS, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
}

export async function updatePermission(id: string, payload: UpdatePermissionPayload) {
  await requestDocumentJson(PERMISSION_BY_ID(id), {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
}

export async function deletePermission(id: string) {
  await requestDocumentJson(PERMISSION_BY_ID(id), {
    method: "DELETE",
  });
}

// ─────────────────────────────────────────────
// Permission enforcement
// ─────────────────────────────────────────────

export type PermissionEnforceRequest = {
  v0: string;
  v1: string;
  v2?: string;
  v3?: string;
  v4?: string;
  v5?: string;
};

export type PermissionBatchEnforceRequest = {
  requests: PermissionEnforceRequest[];
};

export async function enforcePermission(req: PermissionEnforceRequest): Promise<boolean> {
  const response = await requestDocumentJson(PERMISSIONS_ENFORCE, {
    method: "POST",
    body: JSON.stringify(req),
    headers: { "Content-Type": "application/json" },
  });
  const data = response as Record<string, unknown>;
  return data.data === true || data.allow === true;
}

export async function batchEnforcePermission(
  req: PermissionBatchEnforceRequest,
): Promise<boolean[]> {
  const response = await requestDocumentJson(PERMISSIONS_BATCH_ENFORCE, {
    method: "POST",
    body: JSON.stringify(req),
    headers: { "Content-Type": "application/json" },
  });
  const data = response as Record<string, unknown>;
  return (data.data ?? []) as boolean[];
}

// ─────────────────────────────────────────────
// Permission policy & metadata
// ─────────────────────────────────────────────

export type PermissionPolicyItem = {
  pType: string;
  rules: string[];
};

export async function getPermissionPolicy(id: string): Promise<string[][]> {
  const response = await requestDocumentJson(PERMISSION_POLICY(id));
  const data = response as Record<string, unknown>;
  // GET /permissions/{id}/policy returns {"policies": [][]string}
  // (internal/transport/http/api/v1/permissions/chi.go).
  const raw = (data.policies ?? []) as unknown[];
  return raw.map((row) => {
    if (Array.isArray(row)) return row.map(String);
    return String(row).split(",");
  });
}

export async function updatePermissionPolicy(
  id: string,
  payload: { policies: PermissionPolicyItem[] },
): Promise<void> {
  // PUT replaces the whole policy set: {"policies":[{"pType","rules"}]}
  // (UpdatePolicyRequest in permissions/request.go).
  await requestDocumentJson(PERMISSION_POLICY(id), {
    method: "PUT",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
}

export async function getPermissionObjects(id: string): Promise<string[]> {
  const response = await requestDocumentJson(PERMISSION_OBJECTS(id));
  const data = response as Record<string, unknown>;
  return (data.data ?? []) as string[];
}

export async function getPermissionActions(id: string): Promise<string[]> {
  const response = await requestDocumentJson(PERMISSION_ACTIONS(id));
  const data = response as Record<string, unknown>;
  return (data.data ?? []) as string[];
}

export async function getPermissionRoles(id: string): Promise<string[]> {
  const response = await requestDocumentJson(PERMISSION_ROLES(id));
  const data = response as Record<string, unknown>;
  return (data.data ?? []) as string[];
}

// ─────────────────────────────────────────────
// Bulk Import
// ─────────────────────────────────────────────

export type ImportPermissionItem = {
  name: string;
  displayName: string;
  resourceType?: string;
  effect?: string;
  resources?: string[];
  actions?: string[];
  users?: string[];
  groups?: string[];
  roles?: string[];
  domains?: string[];
  organizationId?: string;
};

export type PermissionBulkImportResult = {
  total: number;
  succeeded: number;
  failed: number;
  errors?: string[];
};

export async function importPermissions(
  organizationId: string,
  permissions: ImportPermissionItem[],
): Promise<PermissionBulkImportResult> {
  const response = await requestDocumentJson(PERMISSIONS_BULK_IMPORT, {
    method: "POST",
    body: JSON.stringify({ organizationId, permissions }),
    headers: { "Content-Type": "application/json" },
  });
  return response as PermissionBulkImportResult;
}

export async function importPermissionsFromExcel(file: File, organizationId?: string) {
  const formData = new FormData();
  formData.append("file", file);
  const query = organizationId ? `?organizationId=${encodeURIComponent(organizationId)}` : "";
  const response = await requestDocumentJson(`${PERMISSIONS_UPLOAD}${query}`, {
    method: "POST",
    body: formData,
  });
  return response as PermissionBulkImportResult;
}
