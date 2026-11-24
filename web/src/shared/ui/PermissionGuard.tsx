import type { ReactNode } from "react";
import { memo } from "react";
import { Result, Skeleton } from "antd";
import { useTranslation } from "react-i18next";
import { usePermissions } from "@/shared/hooks/usePermissions";

type PermissionGuardProps = {
  action?: string;
  resource?: string;
  fallback?: ReactNode;
  children: ReactNode;
};

export function PermissionDeniedResult() {
  const { t } = useTranslation();

  return <Result role="alert" status="403" title="403" subTitle={t("common:forbidden")} />;
}

export const PermissionGuard = memo(function PermissionGuard({
  action,
  resource,
  fallback,
  children,
}: PermissionGuardProps) {
  const { hasAction, hasResource, isLoading } = usePermissions();

  if (isLoading) {
    return <Skeleton active paragraph={{ rows: 2 }} />;
  }

  if (action && !hasAction(action)) {
    if (fallback !== undefined) return <>{fallback}</>;
    return <PermissionDeniedResult />;
  }
  if (resource && !hasResource(resource)) {
    if (fallback !== undefined) return <>{fallback}</>;
    return <PermissionDeniedResult />;
  }

  return <>{children}</>;
});
