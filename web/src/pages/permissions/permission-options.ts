/** Casbin 模型规范名（后端 permission.Model 为不透明字符串，创建页与详情页共用同一词表）。 */
export const PERMISSION_MODEL_OPTIONS = [
  { label: "ACL", value: "ACL" },
  { label: "ACL with domains", value: "ACL-with-domains" },
  { label: "RBAC", value: "RBAC" },
  { label: "RBAC with domains", value: "RBAC-with-domains" },
  { label: "RBAC with resource roles", value: "RBAC-with-resource-roles" },
  { label: "ABAC", value: "ABAC" },
];
