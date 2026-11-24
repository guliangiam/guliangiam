import { useQuery } from "@tanstack/react-query";
import { getMyUnreadNotificationCount } from "@/shared/api/iam/user-notification";

/**
 * 顶栏铃铛与收件箱共用同一查询键：任意入口标记已读后，invalidating
 * ["my-notifications"] 会让未读数同步刷新，避免两处显示不一致。
 */
export function useUnreadNotificationCount(enabled = true) {
  return useQuery({
    queryKey: ["my-notifications", "unread-count"],
    queryFn: getMyUnreadNotificationCount,
    enabled,
    refetchInterval: 60_000,
    staleTime: 30_000,
  });
}
