import { App, Button, Popconfirm, Tag } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamLdapsPaginated, deleteIamLdap } from "@/shared/api/iam";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { LinkButton } from "@/shared/ui/LinkButton";

type LdapRow = {
  id: string;
  name: string;
  displayName?: string;
  host?: string;
  port?: number;
  isEnabled: boolean;
};

type LdapParams = {
  page: number;
  pageSize: number;
  search?: string;
  isEnabled?: string;
};

export default function LdapsPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<LdapRow, LdapParams, string>({
    // ── UI ──
    title: t("ldap.title"),
    description: t("ldap.description"),
    searchPlaceholder: t("ldap.searchPlaceholder"),
    rowKey: "name",
    searchableColumns: [
      { dataIndex: "name", label: tc("name") },
      { dataIndex: "displayName", label: tc("displayName") },
    ],
    filters: [
      {
        // API parses "true"/"false" via route.QueryOptionalBool
        key: "isEnabled",
        label: tc("statusLabel"),
        options: [
          { label: tc("enabled"), value: "true" },
          { label: tc("disabled"), value: "false" },
        ],
      },
    ],
    toolbar: (
      <PermissionGuard action="ldaps:write" resource="ldaps" fallback={null}>
        <LinkButton type="primary" to="/ldaps/new">
          {t("ldap.newLdap")}
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
        render: (v: string, record: LdapRow) => <Link to={`/ldaps/${record.id}`}>{v}</Link>,
      },
      { title: tc("displayName"), dataIndex: "displayName", key: "displayName" },
      { title: t("ldap.host"), dataIndex: "host", key: "host" },
      { title: t("ldap.port"), dataIndex: "port", key: "port" },
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
        width: 200,
        fixed: "right" as const,
        render: (_, record) => (
          <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
            <LinkButton type="link" size="small" to={`/ldaps/${record.id}`}>
              {tc("edit")}
            </LinkButton>
            <LinkButton type="link" size="small" to={`/ldaps/${record.id}/sync`}>
              {t("ldap.syncAction")}
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
      ["ldaps", "paginated", p.page, p.pageSize, p.search, p.isEnabled] as const,
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
    paginatedFn: getIamLdapsPaginated,
    deleteFn: deleteIamLdap,
    invalidateKeysOnDelete: [["ldaps"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete ──
    enableBulkDelete: true,
    // rowKey 是 name，删除 API 只认 id —— 批删必须映射，否则把 name 当 id 删。
    rowKeyToDeleteKey: (record) => record.id,
  });
}
