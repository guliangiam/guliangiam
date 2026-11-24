import { Button, Tag } from "antd";
import { EyeOutlined, PlusOutlined } from "@ant-design/icons";
import { Link, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamEntriesPaginated } from "@/shared/api/iam";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";

type EntryRow = {
  id: string;
  organizationId: string;
  name: string;
  displayName?: string;
  type: string;
  application?: string;
  createdAt?: string;
};

type EntryParams = {
  page: number;
  pageSize: number;
  search?: string;
};

function formatDate(v?: string) {
  return v ? new Date(v).toLocaleString() : "-";
}

export default function EntriesPage() {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const account = useSessionStore((state) => state.account);
  const { nameMap: orgNameMap } = useOrganizationNames();

  return renderCrudListPage<EntryRow, EntryParams, string>({
    // ── UI ──
    title: tr("entry.title"),
    description: tr("entry.description"),
    searchPlaceholder: tr("entry.searchPlaceholder"),
    rowKey: "id",
    searchableColumns: [
      { dataIndex: "name", label: t("name") },
      { dataIndex: "displayName", label: t("displayName") },
    ],
    toolbar: (
      <PermissionGuard action="entry:write" resource="entries" fallback={null}>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate("/entries/new")}>
          {tr("entry.newEntry")}
        </Button>
      </PermissionGuard>
    ),
    // EntryListItem carries no clientIp/userAgent/provider: those are detail-only
    // and stay on EntryDetail/transcript, so the list never fabricates a column.
    columns: [
      {
        title: t("name"),
        dataIndex: "name",
        key: "name",
        width: 200,
        fixed: "left" as const,
        render: (v: string, record: EntryRow) => <Link to={`/entries/${record.id}`}>{v}</Link>,
      },
      { title: t("displayName"), dataIndex: "displayName", key: "displayName" },
      {
        title: t("type"),
        dataIndex: "type",
        key: "type",
        render: (v: string) => <Tag>{v}</Tag>,
      },
      { title: tr("entry.application"), dataIndex: "application", key: "application" },
      {
        title: t("organization"),
        dataIndex: "organizationId",
        key: "organizationId",
        width: 160,
        render: (v: string) => (v ? orgNameMap.get(v) || v : "-"),
      },
      {
        title: tr("entry.createdAt"),
        dataIndex: "createdAt",
        key: "createdAt",
        render: (v: string) => formatDate(v),
      },
      {
        title: t("actions"),
        key: "actions",
        width: 160,
        fixed: "right" as const,
        render: (_, record) => (
          <Button
            type="link"
            size="small"
            icon={<EyeOutlined />}
            onClick={() => navigate(`/entries/${record.id}`)}
          >
            {tr("entry.viewDetail")}
          </Button>
        ),
      },
    ],

    // ── Hooks ──
    enabled: account !== null,
    buildQueryKey: (p) => ["entries", "paginated", p.page, p.pageSize, p.search] as const,
    buildParams: ({ page, pageSize, search }) => ({
      page,
      pageSize,
      search: search || undefined,
    }),
    paginatedFn: getIamEntriesPaginated,
  });
}
