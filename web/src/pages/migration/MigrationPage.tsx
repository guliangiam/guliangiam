import { App, Button, Popconfirm, Tag, Typography } from "antd";
import { PlayCircleOutlined, ApiOutlined, DeleteOutlined } from "@ant-design/icons";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import {
  getIamMigrationSyncersPaginated,
  runIamMigrationSyncer,
  testIamMigrationSyncerConnection,
  deleteIamMigrationSyncer,
} from "@/shared/api/iam/migration";
import { useCrudList } from "@/shared/hooks/useCrudList";
import { renderCrudListView } from "@/shared/ui/CrudListPage";
import { LinkButton } from "@/shared/ui/LinkButton";
import { serverErrorMessage } from "@/shared/lib/server-error";

const { Text } = Typography;

const syncerTypeLabels: Record<string, string> = {
  ldap: "LDAP",
  database: "Database",
  dingtalk: "DingTalk",
  lark: "Lark",
  wecom: "WeCom",
  google: "Google",
  okta: "Okta",
  azuread: "Azure AD",
  keycloak: "Keycloak",
  awsiam: "AWS IAM",
};

export default function MigrationPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  // ── useCrudList 处理 query/search/sort/delete ──
  const list = useCrudList<
    {
      id: string;
      name: string;
      displayName: string;
      type: string;
      isEnabled: boolean;
      lastSyncAt: string | null;
    },
    {
      page: number;
      pageSize: number;
      search?: string;
      sortBy?: string;
      sortOrder?: "asc" | "desc";
    },
    string
  >({
    buildQueryKey: (p) =>
      ["migration", "paginated", p.page, p.pageSize, p.search, p.sortBy, p.sortOrder] as const,
    buildParams: ({ page, pageSize, search, sortField, sortOrder }) => ({
      page,
      pageSize,
      search: search || undefined,
      sortBy: sortField || undefined,
      sortOrder: sortOrder === "ascend" ? "asc" : sortOrder === "descend" ? "desc" : undefined,
    }),
    paginatedFn: getIamMigrationSyncersPaginated,
    deleteFn: deleteIamMigrationSyncer,
    invalidateKeysOnDelete: [["migration"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),
    enabled: account !== null,
    defaultSortField: "name",
    defaultSortOrder: "ascend",
  });

  // ── 页面级自定义 mutations（run/test 不适合 deleteFn 模式） ──
  const runMutation = useMutation({
    mutationFn: runIamMigrationSyncer,
    onSuccess: () => {
      message.success(t("migration.runSuccess"));
      list.refetch();
    },
    onError: (error) => message.error(serverErrorMessage(error, t, "migration.runFailed")),
  });

  const testMutation = useMutation({
    mutationFn: testIamMigrationSyncerConnection,
    onSuccess: () => message.success(t("migration.testSuccess")),
    onError: (error) => message.error(serverErrorMessage(error, t, "migration.testFailed")),
  });

  return renderCrudListView(
    {
      title: t("migration.title"),
      description: t("migration.description"),
      searchPlaceholder: t("migration.searchPlaceholder"),
      rowKey: "id",
      enableBulkDelete: true,
      bulkDeleteDescription: tc("deleteConfirm"),
      toolbar: (
        <LinkButton type="primary" to="/syncers/new">
          {t("migration.newMigration")}
        </LinkButton>
      ),
      columns: [
        { title: tc("name"), dataIndex: "name", key: "name", sorter: true },
        { title: tc("displayName"), dataIndex: "displayName", key: "displayName" },
        {
          title: t("migration.type"),
          dataIndex: "type",
          key: "type",
          render: (type: string) => <Tag>{syncerTypeLabels[type] ?? type}</Tag>,
        },
        {
          title: tc("statusLabel"),
          dataIndex: "isEnabled",
          key: "isEnabled",
          render: (v: boolean) =>
            v ? <Tag color="green">{tc("enabled")}</Tag> : <Tag color="red">{tc("disabled")}</Tag>,
        },
        {
          title: t("migration.lastSync"),
          dataIndex: "lastSyncAt",
          key: "lastSyncAt",
          render: (v: string | null) =>
            v ? (
              <Text type="secondary">{new Date(v).toLocaleString()}</Text>
            ) : (
              <Text type="secondary">—</Text>
            ),
        },
        {
          title: tc("actions"),
          key: "actions",
          width: 220,
          fixed: "right",
          render: (_, record) => (
            <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
              <LinkButton size="small" to={`/syncers/${record.id}`}>
                {tc("edit")}
              </LinkButton>
              <Popconfirm
                title={t("migration.runConfirm")}
                onConfirm={() => runMutation.mutate(record.id)}
              >
                <Button
                  size="small"
                  icon={<PlayCircleOutlined />}
                  loading={runMutation.isPending && runMutation.variables === record.id}
                  disabled={
                    !record.isEnabled ||
                    (runMutation.isPending && runMutation.variables !== record.id)
                  }
                >
                  {t("migration.run")}
                </Button>
              </Popconfirm>
              <Button
                size="small"
                icon={<ApiOutlined />}
                onClick={() => testMutation.mutate(record.id)}
                loading={testMutation.isPending && testMutation.variables === record.id}
                disabled={testMutation.isPending && testMutation.variables !== record.id}
              >
                {t("migration.test")}
              </Button>
              <Popconfirm
                title={tc("confirmDelete")}
                onConfirm={() => list.deleteMutation.mutate(record.id)}
              >
                <Button size="small" danger icon={<DeleteOutlined />} />
              </Popconfirm>
            </div>
          ),
        },
      ],
    },
    list,
  );
}
