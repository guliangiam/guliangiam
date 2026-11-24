/**
 * PermissionDetailPage 的静态选项常量
 *
 * 从 PermissionDetailPage.tsx 拆分而来：这些选项是纯数据、不参与 i18n，
 * 供结构化编辑器 tab 与审批段共用。
 */

export const RESOURCE_TYPE_OPTIONS = [
  { label: "Application", value: "Application" },
  { label: "TreeNode", value: "TreeNode" },
  { label: "Custom", value: "Custom" },
  { label: "API", value: "API" },
  { label: "User", value: "User" },
  { label: "Organization", value: "Organization" },
  { label: "Provider", value: "Provider" },
  { label: "Role", value: "Role" },
  { label: "Permission", value: "Permission" },
  { label: "Group", value: "Group" },
  { label: "Resource", value: "Resource" },
  { label: "Session", value: "Session" },
];

export const APPLICATION_ACTIONS = [
  { label: "Read", value: "Read" },
  { label: "Write", value: "Write" },
  { label: "Admin", value: "Admin" },
];

export const API_ACTIONS = [
  { label: "GET", value: "GET" },
  { label: "POST", value: "POST" },
  { label: "PUT", value: "PUT" },
  { label: "DELETE", value: "DELETE" },
  { label: "PATCH", value: "PATCH" },
];

export const EFFECT_OPTIONS = [
  { label: "Allow", value: "allow" },
  { label: "Deny", value: "deny" },
];

export const STATE_OPTIONS = [
  { label: "Approved", value: "Approved" },
  { label: "Pending", value: "Pending" },
];
