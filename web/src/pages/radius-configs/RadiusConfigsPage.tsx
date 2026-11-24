import { App, Button, Popconfirm, Tag } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamRadiusConfigsPaginated, deleteIamRadiusConfig } from "@/shared/api/iam";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { LinkButton } from "@/shared/ui/LinkButton";

const AUTH_PROTOCOL_OPTIONS = [
  { label: "PAP", value: "PAP" },
  { label: "CHAP", value: "CHAP" },
  { label: "MS-CHAPv2", value: "MS-CHAPv2" },
  { label: "EAP", value: "EAP" },
];

type RadiusConfigRow = {
  id: string;
  name: string;
  host?: string;
  port?: number;
  authProtocol?: string;
  isEnabled: boolean;
};

type RadiusConfigParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  authProtocol?: string;
};

export default function RadiusConfigsPage() {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<RadiusConfigRow, RadiusConfigParams, string>({
    // ── UI ──
    title: tr("radius.title", "RADIUS Configuration"),
    description: tr(
      "radius.description",
      "Manage RADIUS server configurations for network authentication.",
    ),
    searchPlaceholder: tr("radius.searchPlaceholder", "Filter by configuration name"),
    rowKey: "id",
    defaultSortField: "name",
    defaultSortOrder: "ascend",
    searchableColumns: [{ dataIndex: "name", label: tc("name") }],
    filters: [
      {
        key: "authProtocol",
        label: tr("radius.authProtocol", "Auth Protocol"),
        options: AUTH_PROTOCOL_OPTIONS,
      },
    ],
    toolbar: (
      <LinkButton type="primary" to="/radius-configs/new">
        {tr("radius.newConfig", "New RADIUS Configuration")}
      </LinkButton>
    ),
    columns: (list) => [
      {
        title: tr("radius.name", "Name"),
        dataIndex: "name",
        key: "name",
        width: 200,
        fixed: "left",
        sorter: true,
        render: (v: string, record: RadiusConfigRow) => (
          <Link to={`/radius-configs/${record.id}`}>{v}</Link>
        ),
      },
      { title: tr("radius.host", "Host"), dataIndex: "host", key: "host" },
      { title: tr("radius.port", "Port"), dataIndex: "port", key: "port", width: 80 },
      {
        title: tr("radius.authProtocol", "Auth Protocol"),
        dataIndex: "authProtocol",
        key: "authProtocol",
        render: (v: string) => (v ? <Tag>{v}</Tag> : "-"),
      },
      {
        title: tr("radius.status", "Status"),
        dataIndex: "isEnabled",
        key: "isEnabled",
        render: (v: boolean) =>
          v ? (
            <Tag color="green">{tr("radius.enabled", "Enabled")}</Tag>
          ) : (
            <Tag color="red">{tr("radius.disabled", "Disabled")}</Tag>
          ),
      },
      {
        title: tr("radius.actions", "Actions"),
        key: "actions",
        fixed: "right",
        width: 160,
        render: (_, record) => (
          <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
            <LinkButton type="link" size="small" to={`/radius-configs/${record.id}`}>
              {tr("radius.edit", "Edit")}
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
      [
        "radius-configs",
        "paginated",
        p.page,
        p.pageSize,
        p.search,
        p.sortBy,
        p.sortOrder,
        p.authProtocol,
      ] as const,
    buildParams: ({ page, pageSize, search, sortField, sortOrder, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      sortBy: sortField || undefined,
      sortOrder: sortOrder === "ascend" ? "asc" : sortOrder === "descend" ? "desc" : undefined,
      filters: {
        ...((filters.authProtocol as string | undefined) !== undefined
          ? { authProtocol: filters.authProtocol }
          : {}),
      },
    }),
    paginatedFn: getIamRadiusConfigsPaginated,
    deleteFn: deleteIamRadiusConfig,
    invalidateKeysOnDelete: [["radius-configs"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
  });
}
