import { useCallback, useEffect, type RefObject } from "react";
import { useBlocker, type BlockerFunction } from "react-router-dom";
import { useTranslation } from "react-i18next";

type UseUnsavedChangesGuardOptions = {
  enabled: boolean;
  message?: string;
  /**
   * 导航时同步读取的放行开关。保存请求进行中（含成功回调里的 navigate）时，
   * `enabled` 依赖的 setState 可能尚未提交，只有 ref 能在 navigate 当刻生效。
   */
  bypassRef?: RefObject<boolean>;
};

export function useUnsavedChangesGuard({
  enabled,
  message,
  bypassRef,
}: UseUnsavedChangesGuardOptions) {
  const { t } = useTranslation();
  const resolvedMessage = message ?? t("unsavedChanges.defaultMessage");

  const shouldBlock = useCallback<BlockerFunction>(
    ({ currentLocation, nextLocation }) => {
      if (bypassRef?.current) return false;
      // 同一路径下只改变 query/hash（如详情页 `?tab=` 深链切换）并未离开
      // 当前页面，表单实例与草稿都还在，不应触发「未保存修改」拦截。
      return currentLocation.pathname !== nextLocation.pathname;
    },
    [bypassRef],
  );

  const blocker = useBlocker(enabled ? shouldBlock : false);

  // 未保存提示的唯一来源：路由级 blocker + 一次 confirm。
  // 不再同时注册 `useBeforeUnload`，否则同一份脏表单会产生两套提示
  // （应用内确认框与浏览器原生弹窗），行为不可控也无法断言。
  useEffect(() => {
    if (blocker.state !== "blocked") {
      return;
    }

    if (window.confirm(resolvedMessage)) {
      blocker.proceed();
      return;
    }

    blocker.reset();
  }, [blocker, resolvedMessage]);
}
