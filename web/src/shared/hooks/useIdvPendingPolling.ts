import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { getIDVVerification } from "@/shared/api/iam/idv";
import { queryKeys } from "@/shared/api/query-keys";

/**
 * 托管认证（IDV）发起成功后写入的待收尾标记：`location.assign` 跳走再回跳时
 * 页面是全新挂载，只能靠 localStorage 恢复轮询。
 */
export const idvPendingStorageKey = (userId: string) => `idv:pending:${userId}`;

const TERMINAL_STATUSES = new Set(["approved", "rejected", "expired"]);
const POLL_INTERVAL_MS = 5000;

/**
 * 轮询未完成的实名认证结果，终态后刷新用户，让 `isVerified` 反映厂商裁决。
 *
 * 两种发起方式共用：自服务端直接跳转厂商托管页，回跳后靠 localStorage 标记恢复；
 * 管理端停留在当前页面展示二维码，必须由调用方显式传入 `verificationId` 才会开始
 * 轮询（同页既不卸载也不触发依赖变化，仅靠标记永远起不来）。
 *
 * `onSettled` 用 ref 透传：调用方传内联回调不会重建 interval。
 */
export function useIdvPendingPolling(options: {
  userId?: string;
  organizationId?: string;
  verificationId?: string;
  onSettled?: (status: string) => void;
}) {
  const { userId, organizationId, verificationId } = options;
  const queryClient = useQueryClient();
  const settledRef = useRef(options.onSettled);
  useEffect(() => {
    settledRef.current = options.onSettled;
  });

  useEffect(() => {
    if (!organizationId) return;
    const storageKey = userId ? idvPendingStorageKey(userId) : undefined;
    const pendingId =
      verificationId ?? (storageKey ? window.localStorage.getItem(storageKey) : null);
    if (!pendingId) return;

    let cancelled = false;
    let timer: number | undefined;
    const stop = () => {
      if (timer !== undefined) {
        window.clearInterval(timer);
        timer = undefined;
      }
    };
    const poll = async () => {
      try {
        const verification = await getIDVVerification(pendingId, organizationId);
        if (cancelled || !verification || !TERMINAL_STATUSES.has(verification.status)) return;
        // 终态即自停：不清 interval 的话页面会每 5 秒一直问到卸载为止。
        stop();
        if (storageKey) window.localStorage.removeItem(storageKey);
        // ["user"] 前缀同时覆盖自服务的 ["user", userId] 与详情页的 ["user","path",…]。
        void queryClient.invalidateQueries({ queryKey: queryKeys.user.all });
        settledRef.current?.(verification.status);
      } catch {
        // Keep polling: a transient provider outage must not clear the pending
        // marker or surface a hard error on the page.
      }
    };
    void poll();
    timer = window.setInterval(poll, POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      stop();
    };
  }, [userId, organizationId, verificationId, queryClient]);
}
