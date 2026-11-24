import { lazy } from "react";
import type { RouteObject } from "react-router-dom";
import { lazyRouteLoader, withSuspense } from "./shared";
import { getQueryClient } from "@/shared/api/query-client";
import { queryKeys } from "@/shared/api/query-keys";
const applicationsListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/access-control-loaders"),
  "applicationsListLoader",
);
const rolesListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/access-control-loaders"),
  "rolesListLoader",
);
const permissionsListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/access-control-loaders"),
  "permissionsListLoader",
);
const sessionsListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/access-control-loaders"),
  "sessionsListLoader",
);
const tokensListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/access-control-loaders"),
  "tokensListLoader",
);
const providersListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/access-control-loaders"),
  "providersListLoader",
);

const ApplicationsPage = lazy(() => import("@/pages/applications/ApplicationsPage"));
const ApplicationDetailPage = lazy(() => import("@/pages/applications/ApplicationDetailPage"));
const ProvidersPage = lazy(() => import("@/pages/providers/ProvidersPage"));
const ProviderDetailPage = lazy(() => import("@/pages/providers/ProviderDetailPage"));
const RolesPage = lazy(() => import("@/pages/roles/RolesPage"));
const RoleCreatePage = lazy(() => import("@/pages/roles/RoleCreatePage"));
const RoleDetailPage = lazy(() => import("@/pages/roles/RoleDetailPage"));
const PermissionsPage = lazy(() => import("@/pages/permissions/PermissionsPage"));
const PermissionCreatePage = lazy(() => import("@/pages/permissions/PermissionCreatePage"));
const PermissionDetailPage = lazy(() => import("@/pages/permissions/PermissionDetailPage"));
const SessionsPage = lazy(() => import("@/pages/sessions/SessionsPage"));
const TokensPage = lazy(() => import("@/pages/tokens/TokensPage"));
const TokenDetailPage = lazy(() => import("@/pages/tokens/TokenDetailPage"));
const RebacPage = lazy(() => import("@/pages/rebac/RebacPage"));
const ActionsPage = lazy(() => import("@/pages/actions/ActionsPage"));

/** Route loader: prefetch application detail data */
async function applicationDetailLoader({ params }: { params: { applicationId?: string } }) {
  if (!params.applicationId || params.applicationId === "new") return null;
  const client = getQueryClient();
  if (!client) return null;
  return client.ensureQueryData({
    queryKey: queryKeys.applications.detail(params.applicationId),
    queryFn: async () => {
      const { getApplication } = await import("@/shared/api/iam/application");
      return getApplication(params.applicationId!);
    },
  });
}

/** Route loader: prefetch provider detail data */
async function providerDetailLoader({ params }: { params: { providerId?: string } }) {
  if (!params.providerId || params.providerId === "new") return null;
  const client = getQueryClient();
  if (!client) return null;
  return client.prefetchQuery({
    queryKey: queryKeys.providers.detail(params.providerId),
    queryFn: async () => {
      const { getProvider } = await import("@/shared/api/iam/provider-extended");
      return getProvider(params.providerId!);
    },
  });
}

/** Access control routes — applications, providers, roles, permissions, sessions, tokens */
export const accessControlRoutes: RouteObject[] = [
  // ── Applications ──
  // 注意：applications/new 不再声明为独立静态路由，而是让 applications/:applicationId 动态路由
  // 匹配 "new" 值。否则 useParams() 在静态路由下返回 {}（无参数），导致 ApplicationDetailPage
  // 里的 isNew 判断（applicationId === "new"）永远为 false，新建页面显示"未找到"。
  // loader 已处理 applicationId === "new" 的情况：applicationDetailLoader 会直接 return null。
  {
    path: "applications",
    element: withSuspense(<ApplicationsPage />),
    loader: applicationsListLoader,
  },
  {
    path: "applications/:applicationId",
    loader: applicationDetailLoader,
    element: withSuspense(<ApplicationDetailPage />),
  },

  // ── Providers ── （同 Applications，providers/new 由 :providerId 动态路由匹配）
  { path: "providers", element: withSuspense(<ProvidersPage />), loader: providersListLoader },
  {
    path: "providers/:providerId",
    loader: providerDetailLoader,
    element: withSuspense(<ProviderDetailPage />),
  },

  // ── Roles ──
  { path: "roles", element: withSuspense(<RolesPage />), loader: rolesListLoader },
  { path: "roles/new", element: withSuspense(<RoleCreatePage />) },
  { path: "roles/:organizationName/:roleName", element: withSuspense(<RoleDetailPage />) },

  // ── Permissions ──
  {
    path: "permissions",
    element: withSuspense(<PermissionsPage />),
    loader: permissionsListLoader,
  },
  { path: "permissions/new", element: withSuspense(<PermissionCreatePage />) },
  {
    path: "permissions/:organizationName/:permissionName",
    element: withSuspense(<PermissionDetailPage />),
  },

  // ── Sessions & Tokens ──
  { path: "sessions", element: withSuspense(<SessionsPage />), loader: sessionsListLoader },
  { path: "tokens", element: withSuspense(<TokensPage />), loader: tokensListLoader },
  { path: "tokens/:tokenName", element: withSuspense(<TokenDetailPage />) },

  // ── ReBAC (Relationship-Based Access Control) ──
  { path: "rebac", element: withSuspense(<RebacPage />) },

  // ── Identity Actions (Sandboxed Executions, ADR-029) ──
  { path: "actions", element: withSuspense(<ActionsPage />) },
];
