/**
 * 前端路由路径 helper。
 *
 * 与 web/src/app/router/*-routes.tsx 注册的 path 逐段一致：
 *   - /ai/agents/:id
 *   - /ai/mcp-servers/:id
 *   - /groups/:organizationName/:groupName
 *   - /permissions/:organizationName/:permissionName
 *
 * API endpoint 仍以 web/src/shared/api/paths.ts 为唯一源；
 * 本文件只负责 React Router 导航目标，动态段统一 encodeURIComponent。
 */

export function agentPath(id: string): string {
  return `/ai/agents/${encodeURIComponent(id)}`;
}

export function mcpServerPath(id: string): string {
  return `/ai/mcp-servers/${encodeURIComponent(id)}`;
}

export function groupPath(organizationName: string, groupName: string): string {
  return `/groups/${encodeURIComponent(organizationName)}/${encodeURIComponent(groupName)}`;
}

export function permissionPath(organizationName: string, permissionName: string): string {
  return `/permissions/${encodeURIComponent(organizationName)}/${encodeURIComponent(permissionName)}`;
}
