import { useState } from "react";
import { Button, Popconfirm } from "antd";
import { CopyOutlined, DownloadOutlined } from "@ant-design/icons";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { getApplicationDeleteSummary } from "@/shared/api/iam/application";
import { queryKeys } from "@/shared/api/query-keys";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { LinkButton } from "@/shared/ui/LinkButton";
import { renderDeleteDescription, type ApplicationRow } from "./application-list-helpers";

type ApplicationsRowActionsProps = {
  record: ApplicationRow;
  duplicatingId: string | null;
  exportingId: string | null;
  onDuplicate: (record: ApplicationRow) => void;
  onExport: (record: ApplicationRow) => void;
  onDelete: (record: ApplicationRow) => Promise<unknown>;
};

/**
 * 行内操作按设计口径提供 Duplicate / Export / Delete（applications.md 生命周期
 * 操作节 + interaction.md 列表页矩阵）；Import 创建新应用，留在工具栏。
 * 删除确认异步加载 delete summary，展示对象名称与将清理的依赖计数。
 */
export function ApplicationsRowActions({
  record,
  duplicatingId,
  exportingId,
  onDuplicate,
  onExport,
  onDelete,
}: ApplicationsRowActionsProps) {
  const { t: tr } = useTranslation("resource");

  if (!record.id) return "-";

  const applicationId = record.id;

  return (
    <>
      <LinkButton type="link" size="small" to={`/applications/${applicationId}`}>
        {tr("application.detail", "Detail")}
      </LinkButton>
      <PermissionGuard action="app:write" resource="applications" fallback={null}>
        <Button
          type="link"
          size="small"
          icon={<CopyOutlined />}
          loading={duplicatingId === applicationId}
          onClick={() => onDuplicate(record)}
          aria-label={tr("application.duplicate", "Duplicate")}
        >
          {tr("application.duplicate", "Duplicate")}
        </Button>
      </PermissionGuard>
      <PermissionGuard action="app:read" resource="applications" fallback={null}>
        <Button
          type="link"
          size="small"
          icon={<DownloadOutlined />}
          loading={exportingId === applicationId}
          onClick={() => onExport(record)}
          aria-label={tr("application.export", "Export")}
        >
          {tr("application.export", "Export")}
        </Button>
      </PermissionGuard>
      <PermissionGuard action="app:delete" resource="applications" fallback={null}>
        <RowDeleteAction record={record} onDelete={onDelete} />
      </PermissionGuard>
    </>
  );
}

function RowDeleteAction({
  record,
  onDelete,
}: {
  record: ApplicationRow;
  onDelete: (record: ApplicationRow) => Promise<unknown>;
}) {
  const { t: tr } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const [open, setOpen] = useState(false);
  const [pending, setPending] = useState(false);

  const summaryQuery = useQuery({
    queryKey: queryKeys.applications.deleteSummary(record.id ?? ""),
    queryFn: () => getApplicationDeleteSummary(record.id ?? "", record.organizationId),
    enabled: open && Boolean(record.id),
    staleTime: 30_000,
  });

  const description = renderDeleteDescription(
    record.displayName || record.name,
    {
      summary: summaryQuery.data,
      isLoading: summaryQuery.isLoading,
      hasError: summaryQuery.isError,
      onRetry: () => void summaryQuery.refetch(),
    },
    tr,
  );
  // 摘要未就绪（加载中）不允许确认；加载失败允许在明确报错后继续删除。
  const canConfirm = !summaryQuery.isLoading && Boolean(summaryQuery.data || summaryQuery.isError);

  return (
    <Popconfirm
      title={tr("application.deleteAppTitle")}
      description={description}
      okText={tc("delete")}
      cancelText={tc("cancel")}
      okButtonProps={{ danger: true, loading: pending, disabled: !canConfirm }}
      onOpenChange={setOpen}
      onConfirm={async () => {
        setPending(true);
        try {
          await onDelete(record);
        } catch {
          // deleteMutation 的 onError 已给出具体错误提示，这里不重复 toast。
        } finally {
          setPending(false);
        }
      }}
    >
      <Button type="link" size="small" danger data-testid={`application-delete-${record.id}`}>
        {tc("delete")}
      </Button>
    </Popconfirm>
  );
}
