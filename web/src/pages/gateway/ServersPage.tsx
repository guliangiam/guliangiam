import { Tag } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getGatewayServersPaginated } from "@/shared/api/iam/gateway";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { LinkButton } from "@/shared/ui/LinkButton";

type ServerRow = {
  id: string;
  name: string;
  endpoint: string;
  protocol: string;
  status: string;
  organizationId: string;
  createdAt?: string;
};

type ServerParams = {
  page: number;
  pageSize: number;
  search?: string;
};

function formatDate(value?: string) {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleDateString();
}

export default function ServersPage() {
  const { t: tr } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<ServerRow, ServerParams, string>({
    // ── UI ──
    title: tr("gateway.servers.title", "MCP Servers"),
    description: tr(
      "gateway.servers.description",
      "Manage MCP Server endpoint configuration, define protocol, address and status.",
    ),
    searchPlaceholder: tr("gateway.servers.searchPlaceholder", "Filter by name"),
    rowKey: "id",
    toolbar: (
      <PermissionGuard action="server:write" resource="servers" fallback={null}>
        <LinkButton type="primary" to="/servers/new">
          {tr("gateway.servers.newServer", "New Server")}
        </LinkButton>
      </PermissionGuard>
    ),
    columns: [
      {
        title: tr("gateway.servers.name", "Name"),
        dataIndex: "name",
        key: "name",
        width: 200,
        fixed: "left",
        render: (v: string, record: ServerRow) => (
          <Link to={`/servers/${record.organizationId}/${record.name}`}>{v}</Link>
        ),
      },
      { title: tr("gateway.servers.endpoint", "Endpoint"), dataIndex: "endpoint", key: "endpoint" },
      { title: tr("gateway.servers.protocol", "Protocol"), dataIndex: "protocol", key: "protocol" },
      {
        title: tc("createdAt"),
        dataIndex: "createdAt",
        key: "createdAt",
        width: 130,
        render: (v: string | undefined) => formatDate(v),
      },
      {
        title: tr("gateway.servers.status", "Status"),
        dataIndex: "status",
        key: "status",
        render: (v: string) =>
          v === "active" ? (
            <Tag color="green">{tr("gateway.servers.active", "Active")}</Tag>
          ) : (
            <Tag color="default">{v}</Tag>
          ),
      },
      {
        title: tr("gateway.servers.actions", "Actions"),
        width: 160,
        key: "actions",
        fixed: "right",
        render: (_, record) => (
          <LinkButton
            type="link"
            size="small"
            to={`/servers/${record.organizationId}/${record.name}`}
          >
            {tr("gateway.servers.edit", "Edit")}
          </LinkButton>
        ),
      },
    ],

    // ── Hooks ──
    enabled: account !== null,
    buildQueryKey: (p) => ["gateway-servers", "paginated", p.page, p.pageSize, p.search] as const,
    buildParams: ({ page, pageSize, search }) => ({
      page,
      pageSize,
      search: search || undefined,
    }),
    paginatedFn: getGatewayServersPaginated,
  });
}
