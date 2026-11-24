import { Tag, Typography } from "antd";
import { useTranslation } from "react-i18next";
import { AuditSectionNav } from "@/features/navigation/AuditSectionNav";
import { useSessionStore } from "@/features/session/session-store";
import { getRecordsPaginated } from "@/shared/api/iam/record";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { useUserNames } from "@/shared/hooks/useResourceNames";

const ACTION_VALUES = [
  "login",
  "login-failed",
  "logout",
  "password-change",
  "mfa-enable",
  "mfa-disable",
  "token-issue",
  "token-revoke",
] as const;

const RESOURCE_TYPE_VALUES = ["user", "organization", "application", "session", "token"] as const;

function formatDateTime(value?: string | null) {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

function renderActionTag(action: string) {
  const colorMap: Record<string, string> = {
    login: "green",
    logout: "default",
    "login-failed": "red",
    "password-change": "orange",
    "mfa-enable": "blue",
    "mfa-disable": "volcano",
    "token-issue": "cyan",
    "token-revoke": "magenta",
  };
  return <Tag color={colorMap[action] ?? "default"}>{action}</Tag>;
}

type RecordRow = {
  id: string;
  organizationId?: string;
  actorId: string;
  action: string;
  resourceType: string;
  resourceId: string;
  ip?: string;
  userAgent?: string | null;
  metadata?: Record<string, unknown> | null;
  createdAt?: string;
};

type RecordParams = {
  page: number;
  pageSize: number;
  search?: string;
  action?: string;
  resourceType?: string;
  organizationId?: string;
};

export interface RecordExpandDetailProps {
  record: RecordRow;
  tRecord: (key: string) => string;
}

function RecordExpandDetail({ record, tRecord }: RecordExpandDetailProps) {
  return (
    <div style={{ display: "grid", gap: 4, padding: "4px 8px" }}>
      <div>
        {tRecord("record.id")}: {record.id}
      </div>
      <div>
        {tRecord("record.organizationId")}: {record.organizationId || "-"}
      </div>
      <div>
        {tRecord("record.userAgent")}: {record.userAgent || "-"}
      </div>
      <div>
        {tRecord("record.metadata")}:{" "}
        <Typography.Paragraph
          copyable
          style={{
            marginBottom: 0,
            display: "inline",
            whiteSpace: "pre-wrap",
            fontFamily: "monospace",
          }}
        >
          {JSON.stringify(record.metadata ?? {}, null, 2)}
        </Typography.Paragraph>
      </div>
    </div>
  );
}

function createExpandedRowRender(tRecord: (key: string) => string) {
  return function renderRow(record: RecordRow) {
    return <RecordExpandDetail record={record} tRecord={tRecord} />;
  };
}

export default function RecordsPage() {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const account = useSessionStore((state) => state.account);
  const organization = useSessionStore((state) => state.organization);
  const { nameMap: userNameMap } = useUserNames();

  return (
    <div className="gl-resource-page">
      <AuditSectionNav />
      {renderCrudListPage<RecordRow, RecordParams, string>({
        // ── UI ──
        title: t("nav.records"),
        description: "",
        searchPlaceholder: t("placeholder.searchByName"),
        rowKey: "id",
        filters: [
          {
            key: "action",
            label: t("actions"),
            options: ACTION_VALUES.map((v) => ({ label: tr(`records.action.${v}`), value: v })),
          },
          {
            key: "resourceType",
            label: t("type"),
            options: RESOURCE_TYPE_VALUES.map((v) => ({
              label: tr(`records.resourceType.${v}`),
              value: v,
            })),
          },
        ],
        columns: [
          {
            title: t("name"),
            dataIndex: "actorId",
            key: "actorId",
            width: 150,
            fixed: "left",
            render: (v: string) => (v ? userNameMap.get(v) || v : "-"),
          },
          {
            title: t("actions"),
            dataIndex: "action",
            key: "action",
            width: 140,
            render: (value: string) => renderActionTag(value),
          },
          { title: t("type"), dataIndex: "resourceType", key: "resourceType", width: 120 },
          {
            title: t("id"),
            dataIndex: "resourceId",
            key: "resourceId",
            width: 200,
            ellipsis: true,
          },
          { title: tr("record.ip"), dataIndex: "ip", key: "ip", width: 140 },
          {
            title: t("createdAt"),
            dataIndex: "createdAt",
            key: "createdAt",
            width: 180,
            render: (value: string) => formatDateTime(value),
          },
        ],

        // ── Hooks ──
        enabled: account !== null,
        buildQueryKey: (p) =>
          [
            "records",
            "paginated",
            p.organizationId ?? "",
            p.page,
            p.pageSize,
            p.search,
            p.action,
            p.resourceType,
          ] as const,
        buildParams: ({ page, pageSize, search, filters }) => ({
          page,
          pageSize,
          search: search?.trim() || undefined,
          action: (filters.action as string | undefined) || undefined,
          resourceType: (filters.resourceType as string | undefined) || undefined,
          organizationId: organization?.id ?? undefined,
        }),
        expandable: {
          expandedRowRender: createExpandedRowRender(tr),
        },
        paginatedFn: getRecordsPaginated,
      })}
    </div>
  );
}
