import { App, Button, Popconfirm, Tag } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamSyncersPaginated, deleteIamSyncer } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { LinkButton } from "@/shared/ui/LinkButton";

type SyncerRow = {
  id: string;
  name: string;
  displayName?: string;
  type?: string;
  host?: string;
  port?: number;
  isEnabled: boolean;
  syncInterval?: number;
  lastSyncAt?: string | null;
};

type SyncerParams = {
  page: number;
  pageSize: number;
  search?: string;
  isEnabled?: string;
};

function formatDate(value?: string | null) {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleDateString();
}

export default function SyncersPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<SyncerRow, SyncerParams, string>({
    // ── UI ──
    title: t("syncer.title"),
    description: t("syncer.description"),
    searchPlaceholder: t("syncer.searchPlaceholder"),
    rowKey: "name",
    // rowKey is the visible name, but DELETE takes the record id.
    rowKeyToDeleteKey: (record) => record.id,
    searchableColumns: [
      { dataIndex: "name", label: tc("name") },
      { dataIndex: "displayName", label: tc("displayName") },
    ],
    filters: [
      {
        key: "isEnabled",
        label: tc("statusLabel"),
        options: [
          { label: tc("enabled"), value: "true" },
          { label: tc("disabled"), value: "false" },
        ],
      },
    ],
    toolbar: (
      <PermissionGuard action="syncer:write" resource="syncers" fallback={null}>
        <LinkButton type="primary" to="/syncers/new">
          {t("syncer.newSyncer")}
        </LinkButton>
      </PermissionGuard>
    ),
    columns: (list) => [
      {
        title: tc("name"),
        dataIndex: "name",
        key: "name",
        width: 200,
        fixed: "left" as const,
        render: (v: string, record: SyncerRow) => <Link to={`/syncers/${record.id}`}>{v}</Link>,
      },
      { title: tc("displayName"), dataIndex: "displayName", key: "displayName" },
      { title: tc("type"), dataIndex: "type", key: "type" },
      {
        title: t("syncer.host"),
        dataIndex: "host",
        key: "host",
        render: (v: string | undefined, record: SyncerRow) =>
          v ? (record.port ? `${v}:${record.port}` : v) : "-",
      },
      {
        title: t("syncer.syncIntervalMinutes"),
        dataIndex: "syncInterval",
        key: "syncInterval",
        width: 130,
        render: (v: number | undefined) => (v ? String(v) : "-"),
      },
      {
        title: t("ldap.lastSyncAt"),
        dataIndex: "lastSyncAt",
        key: "lastSyncAt",
        width: 130,
        render: (v: string | null | undefined) => formatDate(v),
      },
      {
        title: tc("statusLabel"),
        dataIndex: "isEnabled",
        key: "isEnabled",
        render: (v: boolean) =>
          v ? <Tag color="green">{tc("enabled")}</Tag> : <Tag color="red">{tc("disabled")}</Tag>,
      },
      {
        title: tc("actions"),
        key: "actions",
        width: 160,
        fixed: "right" as const,
        render: (_, record) => (
          <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
            <LinkButton type="link" size="small" to={`/syncers/${record.id}`}>
              {tc("edit")}
            </LinkButton>
            <Popconfirm
              title={tc("confirmDelete")}
              okButtonProps={{ danger: true, loading: list.deleteMutation.isPending }}
              onConfirm={() => list.deleteMutation.mutate(record.id)}
            >
              <Button type="link" size="small" danger>
                {tc("delete")}
              </Button>
            </Popconfirm>
          </div>
        ),
      },
    ],

    // ── Hooks ──
    enabled: account !== null,
    buildQueryKey: (p) =>
      ["syncers", "paginated", p.page, p.pageSize, p.search, p.isEnabled] as const,
    buildParams: ({ page, pageSize, search, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      filters: {
        ...((filters.isEnabled as string | undefined) !== undefined
          ? { isEnabled: filters.isEnabled }
          : {}),
      },
    }),
    paginatedFn: getIamSyncersPaginated,
    deleteFn: deleteIamSyncer,
    invalidateKeysOnDelete: [queryKeys.syncers.all],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
  });
}
