import { useState } from "react";
import { useTranslation } from "react-i18next";
import { AuditSectionNav } from "@/features/navigation/AuditSectionNav";
import { getAuditLogs } from "@/shared/api/iam/audit-log";
import type { RecordItem } from "@/shared/api/iam/record";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { useUserNames } from "@/shared/hooks/useResourceNames";
import { AuditFilterBar } from "./AuditFilterBar";
import type { AuditFilterValue } from "./AuditFilterBar";
import { auditFilterParams } from "./audit-filter-params";

function formatDateTime(value?: string | null) {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

type OperationLogParams = {
  page: number;
  pageSize: number;
  search?: string;
  applicationId?: string;
  startTime?: string;
  endTime?: string;
};

export default function OperationLogsPage() {
  const { t } = useTranslation("resource");
  const { nameMap: userNameMap } = useUserNames();
  const [filters, setFilters] = useState<AuditFilterValue>({});

  return (
    <div className="gl-resource-page">
      <AuditSectionNav />
      {renderCrudListPage<RecordItem, OperationLogParams, string>({
        title: t("audit.operationLogs.title"),
        description: t("audit.operationLogs.description"),
        searchPlaceholder: t("audit.operationLogs.searchPlaceholder"),
        toolbar: <AuditFilterBar value={filters} onChange={setFilters} />,
        rowKey: "id",
        filters: [],
        columns: [
          {
            title: t("audit.operationLogs.actor"),
            dataIndex: "actorId",
            key: "actorId",
            width: 180,
            render: (value: string) => (value ? userNameMap.get(value) || value : "-"),
          },
          {
            title: t("audit.operationLogs.action"),
            dataIndex: "action",
            key: "action",
            width: 200,
          },
          {
            title: t("audit.operationLogs.resourceType"),
            dataIndex: "resourceType",
            key: "resourceType",
            width: 140,
          },
          {
            title: t("audit.operationLogs.resourceId"),
            dataIndex: "resourceId",
            key: "resourceId",
            width: 220,
            ellipsis: true,
          },
          { title: t("audit.operationLogs.ip"), dataIndex: "ip", key: "ip", width: 140 },
          {
            title: t("audit.operationLogs.createdAt"),
            dataIndex: "createdAt",
            key: "createdAt",
            width: 180,
            render: (value: string) => formatDateTime(value),
          },
        ],
        enabled: true,
        buildQueryKey: (p) =>
          [
            "audit",
            "operation-logs",
            p.page,
            p.pageSize,
            p.search,
            filters.applicationId,
            filters.range?.[0]?.valueOf(),
            filters.range?.[1]?.valueOf(),
          ] as const,
        buildParams: ({ page, pageSize, search }) => ({
          page,
          pageSize,
          category: "operation" as const,
          keyword: search?.trim() || undefined,
          ...auditFilterParams(filters),
        }),
        paginatedFn: getAuditLogs,
      })}
    </div>
  );
}
