import { App, Button, Popconfirm, Tag } from "antd";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import {
  getIamEnforcersPaginated,
  deleteIamEnforcer,
  getIamModels,
  getIamAdapters,
} from "@/shared/api/iam";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { LinkButton } from "@/shared/ui/LinkButton";

type EnforcerRow = {
  id: string;
  name: string;
  displayName?: string;
  modelId?: string;
  adapterId?: string;
  isEnabled: boolean;
};

type EnforcerParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  isEnabled?: string;
};

export default function EnforcersPage() {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  // EnforcerListItem carries modelId/adapterId only; resolve their display
  // names through the same option queries the detail page uses.
  const { data: models } = useQuery({
    queryKey: ["models", "iam", "enforcer-options"],
    queryFn: () => getIamModels({ pageSize: 100 }),
    enabled: account !== null,
  });
  const { data: adapters } = useQuery({
    queryKey: ["adapters", "iam", "enforcer-options"],
    queryFn: () => getIamAdapters({ pageSize: 100 }),
    enabled: account !== null,
  });
  const modelNames = new Map((models ?? []).map((m) => [m.id, m.displayName || m.name]));
  const adapterNames = new Map((adapters ?? []).map((a) => [a.id, a.displayName || a.name]));

  return renderCrudListPage<EnforcerRow, EnforcerParams, string>({
    // ── UI ──
    title: tr("enforcer.title"),
    description: tr("enforcer.description"),
    searchPlaceholder: tr("enforcer.searchPlaceholder"),
    rowKey: "id",
    defaultSortField: "name",
    defaultSortOrder: "ascend",
    searchableColumns: [
      { dataIndex: "name", label: tc("name") },
      { dataIndex: "displayName", label: tc("displayName") },
    ],
    filters: [
      {
        // API parses "true"/"false" via route.QueryOptionalBool, not "enabled"/"disabled"
        key: "isEnabled",
        label: tc("statusLabel"),
        options: [
          { label: tc("enabled"), value: "true" },
          { label: tc("disabled"), value: "false" },
        ],
      },
    ],
    toolbar: (
      <PermissionGuard action="enforcer:write" resource="enforcers" fallback={null}>
        <LinkButton type="primary" to="/enforcers/new">
          {tc("create")}
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
        sorter: true,
        render: (v: string, record: EnforcerRow) => <Link to={`/enforcers/${record.id}`}>{v}</Link>,
      },
      { title: tc("displayName"), dataIndex: "displayName", key: "displayName" },
      {
        // `type` is not on EnforcerListItem, so the old column always rendered
        // "-". Model/adapter IDs are real list fields and match Casdoor's list.
        title: tr("enforcer.model", "Model"),
        dataIndex: "modelId",
        key: "modelId",
        render: (v?: string) =>
          v ? <Link to={`/models/${v}`}>{modelNames.get(v) ?? v}</Link> : "-",
      },
      {
        title: tr("enforcer.adapter", "Adapter"),
        dataIndex: "adapterId",
        key: "adapterId",
        render: (v?: string) =>
          v ? <Link to={`/adapters/${v}`}>{adapterNames.get(v) ?? v}</Link> : "-",
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
            <PermissionGuard action="enforcer:write" resource="enforcers" fallback={null}>
              <LinkButton type="link" size="small" to={`/enforcers/${record.id}`}>
                {tc("edit")}
              </LinkButton>
            </PermissionGuard>
            <PermissionGuard action="enforcer:delete" resource="enforcers" fallback={null}>
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
        "enforcers",
        "paginated",
        p.page,
        p.pageSize,
        p.search,
        p.sortBy,
        p.sortOrder,
        p.isEnabled,
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
      },
    }),
    paginatedFn: getIamEnforcersPaginated,
    deleteFn: deleteIamEnforcer,
    invalidateKeysOnDelete: [["enforcers"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
    bulkDeletePermission: { permission: "enforcer:delete", resource: "enforcers" },
  });
}
