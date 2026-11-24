import { Alert, Tag, Typography } from "antd";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";
import { AuditSectionNav } from "@/features/navigation/AuditSectionNav";
import { getLoginLogs } from "@/shared/api/iam/audit-log";
import type { AuditLogResult, LoginLogItem } from "@/shared/api/iam/audit-log";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { AuditFilterBar } from "./AuditFilterBar";
import type { AuditFilterValue } from "./AuditFilterBar";
import { auditFilterParams } from "./audit-filter-params";

function formatDateTime(value?: string | null) {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

type LoginLogParams = {
  page: number;
  pageSize: number;
  search?: string;
  result?: AuditLogResult;
  applicationId?: string;
  startTime?: string;
  endTime?: string;
};

export default function LoginLogsPage() {
  const { t } = useTranslation("resource");
  const [searchParams] = useSearchParams();
  const initialApplicationId = searchParams.get("applicationId") ?? undefined;
  const [filters, setFilters] = useState<AuditFilterValue>({
    applicationId: initialApplicationId,
  });

  return (
    <div className="gl-resource-page">
      <AuditSectionNav />
      {initialApplicationId ? (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          title={t("audit.loginLogs.filteredByApplication")}
          description={<Typography.Text code>{initialApplicationId}</Typography.Text>}
        />
      ) : null}
      {renderCrudListPage<LoginLogItem, LoginLogParams, string>({
        title: t("audit.loginLogs.title"),
        description: t("audit.loginLogs.description"),
        searchPlaceholder: t("audit.loginLogs.searchPlaceholder"),
        toolbar: <AuditFilterBar value={filters} onChange={setFilters} />,
        rowKey: "id",
        filters: [
          {
            key: "result",
            label: t("audit.loginLogs.result"),
            options: [
              { label: t("audit.result.success"), value: "success" },
              { label: t("audit.result.failure"), value: "failure" },
            ],
          },
        ],
        columns: [
          {
            title: t("audit.loginLogs.username"),
            dataIndex: "username",
            key: "username",
            width: 180,
            render: (value?: string | null) => value || "-",
          },
          {
            title: t("audit.loginLogs.result"),
            dataIndex: "result",
            key: "result",
            width: 110,
            render: (value: string) => (
              <Tag color={value === "success" ? "green" : "red"}>
                {value === "success" ? t("audit.result.success") : t("audit.result.failure")}
              </Tag>
            ),
          },
          {
            title: t("audit.loginLogs.applicationId"),
            dataIndex: "applicationId",
            key: "applicationId",
            width: 220,
            ellipsis: true,
            render: (value?: string | null) => value || "-",
          },
          { title: t("audit.loginLogs.ip"), dataIndex: "ip", key: "ip", width: 140 },
          {
            title: t("audit.loginLogs.reason"),
            dataIndex: "reason",
            key: "reason",
            ellipsis: true,
            render: (value?: string | null) => value || "-",
          },
          {
            title: t("audit.loginLogs.createdAt"),
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
            "login-logs",
            p.page,
            p.pageSize,
            p.search,
            p.result,
            filters.applicationId,
            filters.range?.[0]?.valueOf(),
            filters.range?.[1]?.valueOf(),
          ] as const,
        buildParams: ({ page, pageSize, search, filters: columnFilters }) => ({
          page,
          pageSize,
          keyword: search?.trim() || undefined,
          result: (columnFilters.result as AuditLogResult | undefined) || undefined,
          ...auditFilterParams(filters),
        }),
        paginatedFn: getLoginLogs,
      })}
    </div>
  );
}
