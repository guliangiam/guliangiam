import { App, Button, Popconfirm, Tag } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamModelsPaginated, deleteIamModel } from "@/shared/api/iam";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { LinkButton } from "@/shared/ui/LinkButton";

type ModelRow = {
  id: string;
  name: string;
  displayName?: string;
  description?: string;
  isEnabled: boolean;
};

type ModelParams = {
  page: number;
  pageSize: number;
  search?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  isEnabled?: string;
};

export default function ModelsPage() {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<ModelRow, ModelParams, string>({
    // ── UI ──
    title: tr("model.title", "Models"),
    description: tr(
      "model.description",
      "Manage Casbin permission models, define RBAC/ABAC policy structures.",
    ),
    searchPlaceholder: tr("model.searchPlaceholder", "Filter by model name"),
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
      <LinkButton type="primary" to="/models/new">
        {tr("model.newModel", "New Model")}
      </LinkButton>
    ),
    columns: (list) => [
      {
        title: tc("name"),
        dataIndex: "name",
        key: "name",
        width: 200,
        fixed: "left" as const,
        sorter: true,
        render: (v: string, record: ModelRow) => <Link to={`/models/${record.id}`}>{v}</Link>,
      },
      { title: tc("displayName"), dataIndex: "displayName", key: "displayName" },
      { title: tc("description"), dataIndex: "description", key: "description" },
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
            <LinkButton type="link" size="small" to={`/models/${record.id}`}>
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
      [
        "models",
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
    paginatedFn: getIamModelsPaginated,
    deleteFn: deleteIamModel,
    invalidateKeysOnDelete: [["models"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
  });
}
