/**
 * Optimistic update hook for user status toggle (enable/disable).
 *
 * Immediately updates the list cache before the server confirms and rolls
 * back on error. The detail page is not updated optimistically: its query is
 * keyed by route params (["user","path",slug,username]) and may not be
 * mounted; invalidating the ["user"] prefix on settle refreshes it if open.
 *
 * @see https://tanstack.com/query/latest/docs/react/guides/optimistic-updates
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useCallback, useRef } from "react";
import { queryKeys } from "@/shared/api/query-keys";
import { updateUser } from "@/shared/api/iam/user";
import type { UserListItem } from "@/shared/api/iam/user";
import { handleMutationError } from "@/shared/hooks/useMutationError";

type UserStatusVariables = {
  userId: string;
  status: string;
  revision: number;
  /** 平台管理员跨组织改状态必须显式携带行所属组织，否则后端回落会话组织 → 404。 */
  organizationId?: string;
};

type UseOptimisticUserStatusOptions = {
  onError?: (error: Error, variables: UserStatusVariables) => void;
};

export function useOptimisticUserStatus(options?: UseOptimisticUserStatusOptions) {
  const queryClient = useQueryClient();
  const pendingIdRef = useRef<string | null>(null);

  const mutation = useMutation({
    mutationFn: async (variables: UserStatusVariables) => {
      await updateUser(
        variables.userId,
        {
          status: variables.status,
          revision: variables.revision,
        },
        variables.organizationId,
      );
    },
    onMutate: async (variables) => {
      pendingIdRef.current = variables.userId;

      // Cancel any in-flight list queries to avoid overwriting the optimistic value
      await queryClient.cancelQueries({ queryKey: queryKeys.users.paginated() });

      // Snapshot previous list data for rollback
      const previousListData = queryClient.getQueriesData<{ items: UserListItem[] }>({
        queryKey: queryKeys.users.paginated(),
      });

      // Optimistically update list caches
      queryClient.setQueriesData<{ items: UserListItem[] }>(
        { queryKey: queryKeys.users.paginated() },
        (old) => {
          if (!old) return old;
          return {
            ...old,
            items: old.items.map((user) =>
              user.id === variables.userId ? { ...user, status: variables.status } : user,
            ),
          };
        },
      );

      return { previousListData };
    },
    onError: (error, variables, context) => {
      if (context?.previousListData) {
        for (const [queryKey, data] of context.previousListData) {
          queryClient.setQueryData(queryKey, data);
        }
      }
      options?.onError?.(error as Error, variables);
      handleMutationError(error);
    },
    onSettled: () => {
      pendingIdRef.current = null;
      // Refetch to ensure consistency with server
      void queryClient.invalidateQueries({ queryKey: queryKeys.users.paginated() });
      // 详情页主查询键为 ["user","path",slug,username]，按前缀失效才能刷新。
      void queryClient.invalidateQueries({ queryKey: queryKeys.user.all });
    },
  });

  const isLoading = useCallback(
    (userId: string) => mutation.isPending && pendingIdRef.current === userId,
    [mutation.isPending],
  );

  return {
    mutate: mutation.mutate,
    mutateAsync: mutation.mutateAsync,
    isLoading,
    isPending: mutation.isPending,
  };
}
