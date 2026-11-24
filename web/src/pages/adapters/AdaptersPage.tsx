import { App, Button, Popconfirm, Tag } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamAdaptersPaginated, deleteIamAdapter } from "@/shared/api/iam";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { LinkButton } from "@/shared/ui/LinkButton";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";

const DB_TYPE_OPTIONS = [
  { label: "PostgreSQL", value: "postgresql" },
  { label: "MySQL", value: "mysql" },
  { label: "SQL Server", value: "mssql" },
  { label: "Oracle", value: "oracle" },
  { label: "SQLite3", value: "sqlite3" },
];

type AdapterRow = {
  id: string;
  name: string;
  displayName?: string;
  type?: string;
  server?: string;
  database?: string;
  useSameDb?: boolean;
  isEnabled: boolean;
};

type AdapterParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  isEnabled?: string;
  databaseType?: string;
};

export default function AdaptersPage() {
  return (
    <PermissionGuard action="adapter:read">
      <AdaptersWorkspace />
    </PermissionGuard>
  );
}

function AdaptersWorkspace() {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<AdapterRow, AdapterParams, string>({
    // ── UI ──
    title: tr("adapter.title", "Adapters"),
    description: tr(
      "adapter.description",
      "Manage Casbin policy adapters, configure storage backends and connection settings.",
    ),
    searchPlaceholder: tr("adapter.searchPlaceholder", "Filter by adapter name"),
    rowKey: "id",
    defaultSortField: "name",
    defaultSortOrder: "ascend",
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
      {
        key: "databaseType",
        label: tr("adapter.databaseType", "Database Type"),
        options: DB_TYPE_OPTIONS,
      },
    ],
    toolbar: (
      <PermissionGuard action="adapter:write" fallback={null}>
        <LinkButton type="primary" to="/adapters/new">
          {tr("adapter.newAdapter", "New Adapter")}
        </LinkButton>
      </PermissionGuard>
    ),
    columns: (list) => [
      {
        title: tc("name"),
        dataIndex: "name",
        key: "name",
        width: 200,
        fixed: "left",
        sorter: (a, b) => String(a.name ?? "").localeCompare(String(b.name ?? "")),
        render: (v: string, record: AdapterRow) => <Link to={`/adapters/${record.id}`}>{v}</Link>,
      },
      { title: tc("displayName"), dataIndex: "displayName", key: "displayName" },
      {
        // The paginated list returns AdapterListItem, which has no
        // `databaseType` (that field only exists on AdapterDetail). Showing it
        // here rendered an empty column; list the fields the list DTO carries.
        title: tc("type"),
        dataIndex: "type",
        key: "type",
        render: (v?: string) => (v ? <Tag>{v}</Tag> : "-"),
      },
      {
        title: tc("adapterUseSameDb"),
        dataIndex: "useSameDb",
        key: "useSameDb",
        render: (v?: boolean) => (v ? <Tag color="green">{tc("yes")}</Tag> : <Tag>{tc("no")}</Tag>),
      },
      {
        title: tc("adapterHost"),
        dataIndex: "server",
        key: "server",
        render: (v?: string) => v || "-",
      },
      {
        title: tc("adapterDatabase"),
        dataIndex: "database",
        key: "database",
        render: (v?: string) => v || "-",
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
        fixed: "right",
        width: 160,
        render: (_, record) => (
          <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
            <PermissionGuard action="adapter:write" fallback={null}>
              <LinkButton type="link" size="small" to={`/adapters/${record.id}`}>
                {tc("edit")}
              </LinkButton>
            </PermissionGuard>
            <PermissionGuard action="adapter:delete" fallback={null}>
              <Popconfirm
                title={tc("confirmDelete")}
                okButtonProps={{ danger: true, loading: list.deleteMutation.isPending }}
                onConfirm={() => list.deleteMutation.mutate(record.id)}
              >
                <Button type="link" size="small" danger>
                  {tc("delete")}
                </Button>
              </Popconfirm>
            </PermissionGuard>
          </div>
        ),
      },
    ],

    // ── Hooks ──
    enabled: account !== null,
    buildQueryKey: (p) =>
      [
        "adapters",
        "paginated",
        p.page,
        p.pageSize,
        p.search,
        p.sortBy,
        p.sortOrder,
        p.isEnabled,
        p.databaseType,
      ] as const,
    buildParams: ({ page, pageSize, search, sortField, sortOrder, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      sortBy: sortField || undefined,
      sortOrder: sortOrder === "ascend" ? "asc" : sortOrder === "descend" ? "desc" : undefined,
      filters: {
        ...((filters.isEnabled as string | undefined) !== undefined
          ? { isEnabled: filters.isEnabled }
          : {}),
        ...((filters.databaseType as string | undefined) !== undefined
          ? { databaseType: filters.databaseType }
          : {}),
      },
    }),
    paginatedFn: getIamAdaptersPaginated,
    deleteFn: deleteIamAdapter,
    invalidateKeysOnDelete: [["adapters"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
    bulkDeletePermission: { permission: "adapter:delete", resource: "adapters" },
  });
}
