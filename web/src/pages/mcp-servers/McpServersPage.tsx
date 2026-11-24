import { App, Button, Popconfirm, Tag } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamMcpServersPaginated, deleteIamMcpServer } from "@/shared/api/iam";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { LinkButton } from "@/shared/ui/LinkButton";
import { mcpServerPath } from "@/shared/lib/routePaths";

type McpServerRow = {
  id: string;
  name: string;
  displayName?: string;
  url?: string;
  application?: string;
  isEnabled: boolean;
};

type McpServerParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
};

export default function McpServersPage() {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<McpServerRow, McpServerParams, string>({
    // ── UI ──
    title: tr("mcpServer.title", "MCP Servers"),
    description: tr(
      "mcpServer.description",
      "Manage MCP server configurations, define server addresses and authentication.",
    ),
    searchPlaceholder: tr("mcpServer.searchPlaceholder", "Filter by server name"),
    rowKey: "id",
    defaultSortField: "name",
    defaultSortOrder: "ascend",
    searchableColumns: [
      { dataIndex: "name", label: tc("name") },
      { dataIndex: "displayName", label: tc("displayName") },
    ],
    toolbar: (
      <>
        <LinkButton style={{ marginRight: 8 }} to="/ai/mcp-servers/store">
          {tr("mcpServer.store", "Store")}
        </LinkButton>
        <PermissionGuard action="mcp-server:write" resource="mcp-servers" fallback={null}>
          <LinkButton type="primary" to="/ai/mcp-servers/new">
            {tr("mcpServer.newServer", "New MCP Server")}
          </LinkButton>
        </PermissionGuard>
      </>
    ),
    columns: (list) => [
      {
        title: tr("mcpServer.name", "Name"),
        dataIndex: "name",
        key: "name",
        width: 200,
        fixed: "left",
        sorter: true,
        render: (v: string, record: McpServerRow) => <Link to={mcpServerPath(record.id)}>{v}</Link>,
      },
      {
        title: tr("mcpServer.displayName", "Display Name"),
        dataIndex: "displayName",
        key: "displayName",
      },
      { title: "URL", dataIndex: "url", key: "url" },
      {
        title: tr("mcpServer.application", "Application"),
        dataIndex: "application",
        key: "application",
      },
      {
        title: tr("mcpServer.enabled", "Enabled"),
        dataIndex: "isEnabled",
        key: "isEnabled",
        render: (v: boolean) =>
          v ? <Tag color="green">{tc("enabled")}</Tag> : <Tag color="red">{tc("disabled")}</Tag>,
      },
      {
        title: tr("mcpServer.actions", "Actions"),
        key: "actions",
        fixed: "right",
        width: 160,
        render: (_, record) => (
          <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
            <LinkButton type="link" size="small" to={mcpServerPath(record.id)}>
              {tr("mcpServer.edit", "Edit")}
            </LinkButton>
            <PermissionGuard action="mcp-server:delete" resource="mcp-servers" fallback={null}>
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
      ["mcp-servers", "paginated", p.page, p.pageSize, p.search, p.sortBy, p.sortOrder] as const,
    buildParams: ({ page, pageSize, search, sortField, sortOrder }) => ({
      page,
      pageSize,
      search: search || undefined,
      sortBy: sortField || undefined,
      sortOrder: sortOrder === "ascend" ? "asc" : sortOrder === "descend" ? "desc" : undefined,
    }),
    paginatedFn: getIamMcpServersPaginated,
    deleteFn: deleteIamMcpServer,
    invalidateKeysOnDelete: [["mcp-servers"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
  });
}
