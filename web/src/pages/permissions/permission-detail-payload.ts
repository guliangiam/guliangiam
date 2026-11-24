/**
 * PermissionDetailPage 的表单值 / 请求体装配
 *
 * 从 PermissionDetailPage.tsx 拆分而来。三处装配共用同一份字段映射：
 *   - 详情响应 → 表单初值（buildPermissionFormValues）
 *   - 表单值 → 更新请求体（buildPermissionUpdatePayload）
 *   - 表单值 → 乐观更新的缓存合并（mergePermissionDetailCache）
 */
import type { PermissionDetail, UpdatePermissionPayload } from "@/shared/api/iam/permission";
import type { PermissionFormValues } from "./tabs/types";

/** Field mapping shared by the update payload and the optimistic cache merge. */
function pickPermissionFields(values: PermissionFormValues) {
  return {
    displayName: values.displayName,
    description: values.description,
    resourceType: values.resourceType,
    resources: values.resources,
    actions: values.actions,
    effect: values.effect,
    domains: values.domains,
    users: values.users,
    groups: values.groups,
    roles: values.roles,
    model: values.model,
    adapter: values.adapter,
    sorter: values.sorter,
    application: values.application,
    state: values.state,
    isEnabled: values.isEnabled,
  };
}

/** Map a permission detail response onto the edit form's field values. */
export function buildPermissionFormValues(detail: PermissionDetail): PermissionFormValues {
  return {
    displayName: detail.displayName,
    description: detail.description,
    model: detail.model ?? "",
    adapter: detail.adapter ?? "",
    resourceType: detail.resourceType,
    resources: detail.resources,
    actions: detail.actions,
    effect: detail.effect,
    domains: detail.domains || [],
    users: detail.users || [],
    groups: detail.groups || [],
    roles: detail.roles || [],
    sorter: detail.sorter ?? "",
    application: detail.application ?? "",
    state: detail.state || "Approved",
    isEnabled: detail.isEnabled,
  };
}

/** Assemble the updatePermission request body from the edit form's field values. */
export function buildPermissionUpdatePayload(
  values: PermissionFormValues,
  revision: number,
): UpdatePermissionPayload {
  return {
    ...pickPermissionFields(values),
    revision,
  };
}

/** Merge the edited fields into the cached detail entry (optimistic update). */
export function mergePermissionDetailCache(
  old: Record<string, unknown> | undefined,
  values: PermissionFormValues,
): Record<string, unknown> | undefined {
  if (!old) return old;
  return {
    ...old,
    ...pickPermissionFields(values),
  };
}
