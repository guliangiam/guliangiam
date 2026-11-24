/**
 * Data-fetching hook for UserDetailPage.
 *
 * Consolidates all useQuery + prefetch useEffect calls into a single composable
 * so the page component can focus on form state, mutation handlers and tab
 * layout — per the "fat component → thin controller" refactor pattern.
 */
import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  getUserByOrganizationSlugAndUsername,
  getUserEffectivePermissions,
  listCustomAttributeDefinitions,
  type UserDetail,
} from "@/shared/api/iam/user";
import { useProviderOptions } from "@/shared/hooks/useProviderOptions";
import { queryKeys } from "@/shared/api/query-keys";
import { getRoleOptions } from "@/shared/api/iam/role";
import { getGroups } from "@/shared/api/iam/group";
import { getOrganization, type AccountItem } from "@/shared/api/iam/organization";
import { getApplication } from "@/shared/api/iam/application";
import { usePrefetch } from "@/shared/hooks/usePrefetch";

/**
 * 路由 `users/:organizationName/:userName` 携带的是组织 slug + 用户名，
 * 不是用户 id：这里先用 slug 解析组织、再按用户名命中用户，最后取详情。
 * 传 undefined 时查询保持 disabled（非法 URL 不会打空请求）。
 */
export function useUserDetailPageQueries(
  organizationSlug: string | undefined,
  username: string | undefined,
) {
  const { prefetchUserRelated } = usePrefetch();

  const userQuery = useQuery({
    queryKey: queryKeys.user.byPath(organizationSlug, username),
    queryFn: () => getUserByOrganizationSlugAndUsername(organizationSlug!, username!),
    enabled: !!organizationSlug && !!username,
  });

  // Prefetch related data when the user's organization is known
  useEffect(() => {
    if (userQuery.data?.organizationId) {
      prefetchUserRelated(userQuery.data.organizationId);
    }
  }, [userQuery.data?.organizationId, prefetchUserRelated]);

  const providersQuery = useProviderOptions(userQuery.data?.organizationId, {
    enabled: Boolean(userQuery.data?.organizationId),
  });

  const roleOptionsQuery = useQuery({
    queryKey: ["role-options", userQuery.data?.organizationId],
    queryFn: () => getRoleOptions(userQuery.data?.organizationId),
    enabled: !!userQuery.data?.organizationId,
  });

  const groupsQuery = useQuery({
    queryKey: ["groups", userQuery.data?.organizationId],
    queryFn: () => getGroups(userQuery.data?.organizationId),
    enabled: !!userQuery.data?.organizationId,
  });

  const organizationQuery = useQuery({
    queryKey: ["organization", userQuery.data?.organizationId],
    queryFn: () => getOrganization(userQuery.data!.organizationId),
    enabled: !!userQuery.data?.organizationId,
  });

  const accountItems = (organizationQuery.data?.settings?.accountItems ?? []) as AccountItem[];

  // Tenant-defined custom attribute keys. They only feed the profile editor
  // suggestions; a failed request must not break the page.
  const definitionsQuery = useQuery({
    queryKey: ["user", "custom-attribute-definitions", userQuery.data?.organizationId],
    queryFn: () => listCustomAttributeDefinitions(userQuery.data!.organizationId),
    enabled: !!userQuery.data?.organizationId,
    retry: false,
  });

  // Effective grants expanded from the user's bound roles. A failure leaves the
  // permissions card empty instead of breaking the page.
  const permissionsQuery = useQuery({
    queryKey: ["user", "effective-permissions", userQuery.data?.id, userQuery.data?.organizationId],
    queryFn: () => getUserEffectivePermissions(userQuery.data!.id, userQuery.data!.organizationId),
    enabled: !!userQuery.data?.id,
    retry: false,
  });

  // 注册应用：按 signupApplicationId 反查显示名，权限分区展示可读名称而非裸 UUID。
  const signupApplicationQuery = useQuery({
    queryKey: ["application", userQuery.data?.signupApplicationId],
    queryFn: () => getApplication(userQuery.data!.signupApplicationId!),
    enabled: !!userQuery.data?.signupApplicationId,
    retry: false,
  });

  return {
    user: userQuery.data ?? undefined,
    isLoading: userQuery.isLoading,
    isError: userQuery.isError,
    // 解析成功但命中不到用户（组织或用户名不存在）
    isNotFound: userQuery.isSuccess && !userQuery.data,
    error: userQuery.error,
    refetchUser: userQuery.refetch,
    providers: providersQuery.data,
    roleOptions: roleOptionsQuery.data,
    groupList: groupsQuery.data,
    organization: organizationQuery.data,
    signupApplicationName: signupApplicationQuery.data?.displayName,
    accountItems,
    customAttributeDefinitions: definitionsQuery.data ?? [],
    effectivePermissions: permissionsQuery.data ?? [],
  } as const;
}

export type UserFromQuery = UserDetail | undefined;
