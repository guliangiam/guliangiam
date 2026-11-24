import { App, Button, Popconfirm, Tag } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamAgentsPaginated, deleteIamAgent } from "@/shared/api/iam";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { LinkButton } from "@/shared/ui/LinkButton";
import { agentPath } from "@/shared/lib/routePaths";

type AgentRow = {
  id: string;
  name: string;
  displayName?: string;
  host?: string;
  port?: number;
  isEnabled: boolean;
};

type AgentParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
};

export default function AgentsPage() {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<AgentRow, AgentParams, string>({
    // ── UI ──
    title: tr("agent.title", "Agents"),
    description: tr(
      "agent.description",
      "Manage Agent proxy services, monitor and execute remote tasks.",
    ),
    searchPlaceholder: tr("agent.searchPlaceholder", "Filter by Agent name or display name"),
    rowKey: "id",
    defaultSortField: "name",
    defaultSortOrder: "ascend",
    searchableColumns: [
      { dataIndex: "name", label: tc("name") },
      { dataIndex: "displayName", label: tc("displayName") },
    ],
    toolbar: (
      <PermissionGuard action="agent:write" resource="agents" fallback={null}>
        <LinkButton type="primary" to="/ai/agents/new">
          {tr("agent.newAgent", "New Agent")}
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
        sorter: true,
        render: (v: string, record: AgentRow) => <Link to={agentPath(record.id)}>{v}</Link>,
      },
      { title: tc("displayName"), dataIndex: "displayName", key: "displayName" },
      { title: tr("agent.host", "Host"), dataIndex: "host", key: "host" },
      { title: tr("agent.port", "Port"), dataIndex: "port", key: "port" },
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
            <LinkButton type="link" size="small" to={agentPath(record.id)}>
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
      ["agents", "paginated", p.page, p.pageSize, p.search, p.sortBy, p.sortOrder] as const,
    buildParams: ({ page, pageSize, search, sortField, sortOrder }) => ({
      page,
      pageSize,
      search: search || undefined,
      sortBy: sortField || undefined,
      sortOrder: sortOrder === "ascend" ? "asc" : sortOrder === "descend" ? "desc" : undefined,
    }),
    paginatedFn: getIamAgentsPaginated,
    deleteFn: deleteIamAgent,
    invalidateKeysOnDelete: [["agents"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
  });
}
