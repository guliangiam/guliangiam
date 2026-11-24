import { Tag } from "antd";
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

function reasonOf(record: RecordItem): string {
  const metadata = record.metadata ?? {};
  const reason = metadata.reason ?? metadata.message ?? metadata.error ?? "";
  return typeof reason === "string" ? reason : JSON.stringify(reason);
}

type ErrorLogParams = {
  page: number;
  pageSize: number;
  search?: string;
  applicationId?: string;
  startTime?: string;
  endTime?: string;
};

export default function ErrorLogsPage() {
  const { t } = useTranslation("resource");
  const { nameMap: userNameMap } = useUserNames();
  const [filters, setFilters] = useState<AuditFilterValue>({});

  return (
    <div className="gl-resource-page">
      <AuditSectionNav />
      {renderCrudListPage<RecordItem, ErrorLogParams, string>({
        title: t("audit.errorLogs.title"),
        description: t("audit.errorLogs.description"),
        searchPlaceholder: t("audit.errorLogs.searchPlaceholder"),
        toolbar: <AuditFilterBar value={filters} onChange={setFilters} />,
        rowKey: "id",
        filters: [],
        columns: [
          {
            title: t("audit.errorLogs.actor"),
            dataIndex: "actorId",
            key: "actorId",
            width: 180,
            render: (value: string) => (value ? userNameMap.get(value) || value : "-"),
          },
          {
            title: t("audit.errorLogs.action"),
            dataIndex: "action",
            key: "action",
            width: 220,
            render: (value: string) => <Tag color="red">{value}</Tag>,
          },
          {
            title: t("audit.errorLogs.resourceId"),
            dataIndex: "resourceId",
            key: "resourceId",
            width: 200,
            ellipsis: true,
          },
          {
            title: t("audit.errorLogs.reason"),
            key: "reason",
            ellipsis: true,
            render: (_: unknown, record: RecordItem) => reasonOf(record) || "-",
          },
          { title: t("audit.errorLogs.ip"), dataIndex: "ip", key: "ip", width: 140 },
          {
            title: t("audit.errorLogs.createdAt"),
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
            "error-logs",
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
          category: "error" as const,
          keyword: search?.trim() || undefined,
          ...auditFilterParams(filters),
        }),
        paginatedFn: getAuditLogs,
      })}
    </div>
  );
}
