import { Button, Result } from "antd";
import { useTranslation } from "react-i18next";
import { isHttpError } from "@/shared/api/http";
import { PermissionDeniedResult } from "@/shared/ui/PermissionGuard";

type NotFoundResultProps = {
  title?: string;
  description?: string;
};

export function NotFoundResult({ title, description }: NotFoundResultProps) {
  const { t } = useTranslation("common");

  return (
    <Result
      role="alert"
      status="404"
      title={title ?? t("notFound")}
      subTitle={description ?? t("resourceNotFound")}
    />
  );
}

type QueryErrorResultProps = {
  error: unknown;
  onRetry: () => void;
  notFoundTitle?: string;
  notFoundDescription?: string;
};

export function QueryErrorResult({
  error,
  onRetry,
  notFoundTitle,
  notFoundDescription,
}: QueryErrorResultProps) {
  const { t } = useTranslation("common");

  if (isHttpError(error) && error.status === 403) {
    return <PermissionDeniedResult />;
  }

  if (isHttpError(error) && error.status === 404) {
    return <NotFoundResult title={notFoundTitle} description={notFoundDescription} />;
  }

  return (
    <Result
      role="alert"
      status="error"
      title={t("loadFailed")}
      subTitle={String(error)}
      extra={
        <Button type="primary" onClick={onRetry}>
          {t("retry")}
        </Button>
      }
    />
  );
}
