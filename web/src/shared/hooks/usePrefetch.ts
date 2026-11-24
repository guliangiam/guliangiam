/**
 * Smart prefetch hook for TanStack Query.
 *
 * Provides:
 * - Detail prefetch by ID (user, application, organization, permission, role)
 * - Hover-triggered prefetch for menu items and list rows
 * - Detail-page related data prefetch (e.g., org settings when loading org detail)
 *
 * @see https://tanstack.com/query/latest/docs/react/guides/prefetching
 */
import { useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import { queryKeys, CACHE_SEMI_STATIC } from "@/shared/api/query-keys";
import { getUserByOrganizationSlugAndUsername } from "@/shared/api/iam/user";
import { getApplication } from "@/shared/api/iam/application";
import { getOrganization } from "@/shared/api/iam/organization";
import { getPermission } from "@/shared/api/iam/permission";
import { getRoleOptions } from "@/shared/api/iam/role";
import { getGroups } from "@/shared/api/iam/group";

export function usePrefetch() {
  const queryClient = useQueryClient();

  // ── Detail prefetchers ──────────────────────────────────────

  /**
   * 用户详情页的主查询按路由参数（组织 slug + 用户名）取键，不是用户 id：
   * 预热必须写同一个键，否则详情页读不到预热结果（旧实现预热
   * users.detail(id)，无任何生产读取方，纯粹浪费请求）。
   */
  const prefetchUserDetail = useCallback(
    (organizationSlug: string, username: string) => {
      if (!organizationSlug || !username) return;
      void queryClient.prefetchQuery({
        queryKey: queryKeys.user.byPath(organizationSlug, username),
        queryFn: () => getUserByOrganizationSlugAndUsername(organizationSlug, username),
      });
    },
    [queryClient],
  );

  const prefetchApplicationDetail = useCallback(
    (applicationId: string) => {
      void queryClient.prefetchQuery({
        queryKey: queryKeys.applications.detail(applicationId),
        queryFn: () => getApplication(applicationId),
      });
    },
    [queryClient],
  );

  const prefetchOrganizationDetail = useCallback(
    (orgId: string) => {
      void queryClient.prefetchQuery({
        queryKey: queryKeys.organizations.detail(orgId),
        queryFn: () => getOrganization(orgId),
      });
    },
    [queryClient],
  );

  const prefetchPermissionDetail = useCallback(
    (permissionId: string) => {
      void queryClient.prefetchQuery({
        queryKey: queryKeys.permissions.detail(permissionId),
        queryFn: () => getPermission(permissionId),
      });
    },
    [queryClient],
  );

  // ── Related data prefetchers (for detail pages) ─────────────

  /**
   * Prefetch related data when loading a user detail page:
   * - Role options for the user's organization
   * - Groups for the user's organization
   */
  const prefetchUserRelated = useCallback(
    (organizationId: string) => {
      void queryClient.prefetchQuery({
        queryKey: queryKeys.roles.options(organizationId),
        queryFn: () => getRoleOptions(organizationId),
        staleTime: CACHE_SEMI_STATIC.staleTime,
      });
      void queryClient.prefetchQuery({
        queryKey: queryKeys.groups.list({ organizationId }),
        queryFn: () => getGroups(organizationId),
        staleTime: CACHE_SEMI_STATIC.staleTime,
      });
    },
    [queryClient],
  );

  // ── Hover prefetch helpers ──────────────────────────────────

  /**
   * Create mouse event handlers that prefetch data on hover.
   * Usage: <div {...hoverPrefetch(() => prefetchUserDetail(orgSlug, username))}>
   */
  const hoverPrefetch = useCallback(
    (prefetchFn: () => void) => ({
      onMouseEnter: () => prefetchFn(),
      onFocus: () => prefetchFn(),
    }),
    [],
  );

  return {
    // Detail prefetchers
    prefetchUserDetail,
    prefetchApplicationDetail,
    prefetchOrganizationDetail,
    prefetchPermissionDetail,
    // Related data prefetchers
    prefetchUserRelated,
    // Hover helpers
    hoverPrefetch,
  };
}
