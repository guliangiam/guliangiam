import { useState, useCallback, useMemo } from "react";
import { App } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  deleteUser,
  getUsersPaginated,
  exportUsersToExcel,
  impersonateUser,
  updateUser,
  type UserListItem,
  type UserListParams,
} from "@/shared/api/iam/user";
import { useSessionStore } from "@/features/session/session-store";
import { handleMutationError } from "@/shared/hooks/useMutationError";
import { useOptimisticUserStatus } from "@/shared/hooks/useOptimisticUserStatus";
import { usePrefetch } from "@/shared/hooks/usePrefetch";
import { useOrganizationNames, useApplicationNames } from "@/shared/hooks/useResourceNames";
import { getOrganizationOptions } from "@/shared/api/iam/organization";
import { queryKeys } from "@/shared/api/query-keys";
import { useCrudList } from "@/shared/hooks/useCrudList";

export const DEFAULT_SORT_FIELD = "createdAt";
export const DEFAULT_SORT_ORDER: "ascend" | "descend" = "descend";

const BULK_STATUS_BATCH_SIZE = 10;

// 平台管理员跨组织删用户必须显式携带行所属组织，否则后端回落会话组织 → 404
// （与 GroupsPage 的 GroupDeleteKey 同一模式）。
export type UserDeleteKey = { id: string; organizationId?: string };

export function useUsersPageController() {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { t: ta } = useTranslation("auth");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const account = useSessionStore((state) => state.account);
  const organization = useSessionStore((state) => state.organization);
  const organizationName = organization?.name ?? "built-in";
  const navigate = useNavigate();
  const { prefetchUserDetail, hoverPrefetch } = usePrefetch();

  const { nameMap: orgNameMap } = useOrganizationNames();
  // Detail routes are keyed by organization slug, not display name. Reuse the
  // same cached options query used by useOrganizationNames (no extra request).
  const { data: orgOptions } = useQuery({
    queryKey: [...queryKeys.organizations.auth(), "options"],
    queryFn: getOrganizationOptions,
    enabled: account !== null,
    staleTime: 5 * 60_000,
  });
  const orgSlugById = useMemo(() => {
    const map = new Map<string, string>();
    for (const org of orgOptions ?? []) {
      if (org.id && org.slug) map.set(org.id, org.slug);
    }
    return map;
  }, [orgOptions]);
  const { nameMap: appNameMap } = useApplicationNames(organization?.id ?? undefined);

  // 组织列表行的「用户」快捷入口会带 ?organizationId=<id>，进入即建立组织上下文。
  // URL 是唯一可信来源：显式选择记录在 orgSelection 中，且只在它绑定的 URL 上下文
  // 仍然成立时生效；前进/后退或同页跳转改变 URL 后它会自动失效，不会覆盖 URL。
  // 保留显式清除（override 为 ""）语义，避免回落到会话组织而无法查看全部组织。
  const [searchParams, setSearchParams] = useSearchParams();
  const orgContext = searchParams.get("organizationId") ?? "";
  const [orgSelection, setOrgSelection] = useState<{ context: string; value: string } | null>(null);
  const orgFilter =
    orgSelection && orgSelection.context === orgContext
      ? orgSelection.value
      : orgContext || (organization?.id ?? "");

  const optimisticUserStatus = useOptimisticUserStatus();

  const crud = useCrudList<UserListItem, UserListParams, UserDeleteKey>({
    buildQueryKey: (p) => queryKeys.users.paginatedWithParams(p),
    buildParams: ({ page, pageSize, search, filters, sortField, sortOrder }) => ({
      page,
      pageSize,
      search: search || undefined,
      organizationId: orgFilter || undefined,
      userType: filters.userTypeFilter || undefined,
      // GET /api/v1/users 只识别 status/sortField；isEnabled/sortBy 之类的
      // 自造键会被 API 层静默丢弃，筛选与排序等于没生效。
      status: filters.statusFilter || undefined,
      sortField: sortField || undefined,
      sortOrder: sortOrder ? (sortOrder === "ascend" ? "asc" : "desc") : "desc",
    }),
    paginatedFn: getUsersPaginated,
    deleteFn: (key) => deleteUser(key.id, key.organizationId),
    invalidateKeysOnDelete: [queryKeys.users.paginated(), queryKeys.users.all],
    onDeleteSuccess: () => message.success(t("deleteSuccess")),
    enabled: account !== null,
    defaultSortField: DEFAULT_SORT_FIELD,
    defaultSortOrder: DEFAULT_SORT_ORDER,
  });

  const impersonateMutation = useMutation({
    mutationFn: async (userId: string) => impersonateUser(userId),
    onSuccess: (data, userId) => {
      const targetUser = crud.query.data?.find((u) => u.id === userId);
      const sessionStore = useSessionStore.getState();
      sessionStore.setImpersonatedUser({
        id: userId,
        username: targetUser?.username ?? "",
        displayName: targetUser?.displayName ?? targetUser?.username ?? "",
      });
      // imp token 只存内存并由 http 层注入 Authorization，后续请求以被冒充用户身份执行。
      sessionStore.setImpersonationToken(data.token);
      // 目标身份通常没有 users:read，管理侧用户详情页会 403；落到恒可达的首页，
      // 并失效身份/权限缓存让顶栏与菜单立即切换为目标身份。
      void queryClient.invalidateQueries();
      message.success(t("action.impersonateSuccess"));
      navigate("/");
    },
    onError: (error) => handleMutationError(error, t("action.impersonateFailed")),
  });

  const exportMutation = useMutation({
    mutationFn: async (orgId: string | undefined) => {
      await exportUsersToExcel(orgId);
    },
    onSuccess: () => message.success(t("action.exportSuccess")),
    onError: (error) => handleMutationError(error, t("action.exportFailed")),
  });

  const toggleAdminMutation = useMutation({
    mutationFn: async ({
      userId,
      isAdmin,
      revision,
      organizationId,
    }: {
      userId: string;
      isAdmin: boolean;
      revision: number;
      organizationId?: string;
    }) => {
      await updateUser(userId, { isAdmin, revision }, organizationId);
    },
    onMutate: async ({ userId, isAdmin }) => {
      await queryClient.cancelQueries({ queryKey: queryKeys.users.paginated() });
      const previousData = queryClient.getQueriesData<{ items: UserListItem[] }>({
        queryKey: queryKeys.users.paginated(),
      });
      queryClient.setQueriesData<{ items: UserListItem[] }>(
        { queryKey: queryKeys.users.paginated() },
        (old) =>
          old
            ? { ...old, items: old.items.map((u) => (u.id === userId ? { ...u, isAdmin } : u)) }
            : old,
      );
      return { previousData };
    },
    onError: (error, _variables, context) => {
      if (context?.previousData) {
        for (const [key, data] of context.previousData) queryClient.setQueryData(key, data);
      }
      handleMutationError(error, t("action.toggleAdminFailed"));
    },
    onSettled: () => void queryClient.invalidateQueries({ queryKey: queryKeys.users.paginated() }),
  });

  // 与壳层 bulkDelete / 邀请码批量一致的分批并发上限，避免百条选中瞬时百并发触发限流。
  const bulkStatusMutation = useMutation({
    mutationFn: async ({ status, ids }: { status: "active" | "disabled"; ids: string[] }) => {
      const targets = (crud.query.data ?? []).filter(
        (user) => ids.includes(user.id) && user.status !== status,
      );
      let succeeded = 0;
      const failedIds: string[] = [];
      for (let i = 0; i < targets.length; i += BULK_STATUS_BATCH_SIZE) {
        const batch = targets.slice(i, i + BULK_STATUS_BATCH_SIZE);
        const results = await Promise.allSettled(
          batch.map((user) =>
            optimisticUserStatus.mutateAsync({
              userId: user.id,
              status,
              revision: user.revision ?? 0,
              organizationId: user.organizationId,
            }),
          ),
        );
        results.forEach((result, index) => {
          if (result.status === "fulfilled") succeeded += 1;
          else failedIds.push(batch[index].id);
        });
      }
      return { total: targets.length, succeeded, failedIds };
    },
    onSuccess: (summary) => {
      if (summary.total === 0) return;
      if (summary.failedIds.length > 0) {
        message.warning(
          t("bulkStatusPartial", {
            succeeded: summary.succeeded,
            failed: summary.failedIds.length,
            total: summary.total,
          }),
        );
        // 失败行保留选中，便于直接重试；成功行状态已随乐观更新与失效刷新。
        crud.setSelectedKeys(summary.failedIds);
      } else {
        message.success(t("updateSuccess"));
        crud.setSelectedKeys([]);
      }
    },
  });

  // 导入只有一个入口：独立页 /users/import。列表页只负责把当前组织上下文带过去，
  // 管理员在列表上选中的组织（或 URL 里的 organizationId）与导入目标保持一致。
  const handleImport = useCallback(() => {
    const query = orgFilter ? `?organizationId=${encodeURIComponent(orgFilter)}` : "";
    navigate(`/users/import${query}`);
  }, [navigate, orgFilter]);
  // 选择/清除组织时同步 URL：清除即移除 organizationId，避免上下文被静默丢弃。
  const handleOrgChange = useCallback(
    (value: string) => {
      const next = value || "";
      // 选择结果与被写回的 URL 上下文绑定；URL 后续被历史导航改变时自动失效。
      setOrgSelection({ context: next, value: next });
      setSearchParams(
        (prev) => {
          const params = new URLSearchParams(prev);
          if (next) params.set("organizationId", next);
          else params.delete("organizationId");
          return params;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );
  const handleReset = useCallback(() => {
    crud.setSearch("");
    crud.setFilters({});
    crud.setSortField(DEFAULT_SORT_FIELD);
    crud.setSortOrder(DEFAULT_SORT_ORDER);
  }, [crud]);

  // Note: renderUserLink is string-based here because columns are useMemo'd in View.
  // We export what View needs to build its columns array.
  return {
    // i18n
    t,
    tr,
    ta,
    // session / env
    account,
    organizationName,
    navigate,
    // state
    orgFilter,
    orgContext,
    // helpers
    orgNameMap,
    orgSlugById,
    appNameMap,
    optimisticUserStatus,
    hoverPrefetch,
    prefetchUserDetail,
    // mutations
    impersonateMutation,
    exportMutation,
    toggleAdminMutation,
    bulkStatusMutation,
    // crud
    crud,
    // handlers
    handleImport,
    handleOrgChange,
    handleReset,
    // utils
    DEFAULT_SORT_FIELD,
    DEFAULT_SORT_ORDER,
  };
}
