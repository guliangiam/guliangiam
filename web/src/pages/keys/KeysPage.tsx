import { useState } from "react";
import { Button, Popconfirm, Space, Tag, App } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamKeysPaginated, deleteIamKey, type IamKeyItem } from "@/shared/api/iam";
import { renderCrudListView } from "@/shared/ui/CrudListPage";
import { useCrudList } from "@/shared/hooks/useCrudList";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { LinkButton } from "@/shared/ui/LinkButton";

const KEY_TYPE_OPTIONS = [
  { label: "Organization", value: "Organization" },
  { label: "Application", value: "Application" },
  { label: "User", value: "User" },
  { label: "General", value: "General" },
  { label: "Prometheus", value: "Prometheus" },
];

type KeyParams = {
  page: number;
  pageSize: number;
  search?: string;
  organizationId?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  filters?: Record<string, string>;
};

export default function KeysPage() {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);
  const organization = useSessionStore((state) => state.organization);
  const [orgFilter, setOrgFilter] = useState(organization?.id ?? "");

  const list = useCrudList<IamKeyItem, KeyParams, string>({
    enabled: account !== null,
    buildQueryKey: (p) =>
      [
        "keys",
        "paginated",
        p.page,
        p.pageSize,
        p.search,
        p.sortBy,
        p.sortOrder,
        p.organizationId,
        p.filters,
      ] as const,
    buildParams: ({ page, pageSize, search, sortField, sortOrder, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      organizationId: orgFilter || undefined,
      sortBy: sortField || undefined,
      sortOrder: sortOrder === "ascend" ? "asc" : sortOrder === "descend" ? "desc" : undefined,
      filters: {
        ...((filters.keyType as string | undefined) !== undefined
          ? { keyType: filters.keyType }
          : {}),
        ...((filters.isEnabled as string | undefined) !== undefined
          ? { isEnabled: filters.isEnabled }
          : {}),
      },
    }),
    paginatedFn: getIamKeysPaginated,
    deleteFn: deleteIamKey,
    invalidateKeysOnDelete: [["keys"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),
  });

  return renderCrudListView<IamKeyItem, KeyParams, string>(
    {
      // ── UI ──
      title: tr("key.title", "Key Management"),
      description: tr("key.description", "Manage OAuth, API, encryption and signing keys."),
      searchPlaceholder: tr("key.searchPlaceholder", "Filter by key name or display name"),
      rowKey: "id",
      defaultSortField: "name",
      defaultSortOrder: "ascend",
      searchableColumns: [
        { dataIndex: "name", label: tc("name") },
        { dataIndex: "displayName", label: tc("displayName") },
      ],
      filters: [
        {
          key: "keyType",
          label: tr("key.type", "Type"),
          options: KEY_TYPE_OPTIONS,
        },
        {
          // `isEnabled` is the parameter the handler reads
          // (`route.QueryOptionalBool` parses "true"/"false"). The previous
          // `status=enabled` matched no parameter, and the `state` filter beside
          // it referenced a field that exists nowhere: `keys` has no `state`
          // column, the list response has no `state` field, and its column
          // render fabricated "Active" for every row.
          key: "isEnabled",
          label: tc("statusLabel"),
          options: [
            { label: tc("enabled"), value: "true" },
            { label: tc("disabled"), value: "false" },
          ],
        },
      ],
      toolbar: (
        <Space>
          {account?.isAdmin && (
            <OrganizationSelect
              value={orgFilter}
              onChange={(v) => {
                setOrgFilter(v);
                list.serverPagination.onChange(1, list.pagination.pageSize);
              }}
              style={{ width: 180 }}
              allowClear
            />
          )}
          <PermissionGuard action="key:write" resource="keys" fallback={null}>
            <LinkButton type="primary" to="/keys/new">
              {tr("key.newKey", "New Key")}
            </LinkButton>
          </PermissionGuard>
        </Space>
      ),
      columns: [
        {
          title: tr("key.name", "Name"),
          dataIndex: "name",
          key: "name",
          width: 200,
          fixed: "left" as const,
          sorter: (a, b) => String(a.name ?? "").localeCompare(String(b.name ?? "")),
          render: (v: string, record: IamKeyItem) => <Link to={`/keys/${record.id}`}>{v}</Link>,
        },
        {
          title: tr("key.displayName", "Display Name"),
          dataIndex: "displayName",
          key: "displayName",
        },
        {
          title: tr("key.type", "Type"),
          dataIndex: "keyType",
          key: "keyType",
          render: (v: string) => (v ? <Tag>{v}</Tag> : "-"),
        },
        {
          title: tc("statusLabel"),
          dataIndex: "isEnabled",
          key: "isEnabled",
          render: (v: boolean) => (
            <Tag color={v ? "green" : "default"}>
              {v ? tr("key.enabled", "Enabled") : tr("key.disabled", "Disabled")}
            </Tag>
          ),
        },
        {
          title: tr("key.actions", "Actions"),
          key: "actions",
          width: 160,
          fixed: "right" as const,
          render: (_: unknown, record: IamKeyItem) => (
            <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
              <PermissionGuard action="key:write" resource="keys" fallback={null}>
                <LinkButton type="link" size="small" to={`/keys/${record.id}`}>
                  {tr("key.edit", "Edit")}
                </LinkButton>
              </PermissionGuard>
              <PermissionGuard action="key:delete" resource="keys" fallback={null}>
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

      // ── Bulk Delete ──
      enableBulkDelete: true,
      bulkDeletePermission: { permission: "key:delete", resource: "keys" },
    },
    list,
  );
}
