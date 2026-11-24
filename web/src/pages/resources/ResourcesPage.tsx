import { useState } from "react";
import { App, Button, Popconfirm, Space } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamResourcesPaginated, deleteIamResource } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { renderCrudListView } from "@/shared/ui/CrudListPage";
import { useCrudList } from "@/shared/hooks/useCrudList";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { resourceTypeOptions } from "@/shared/constants/enums";
import { useApplicationNames } from "@/shared/hooks/useResourceNames";
import { LinkButton } from "@/shared/ui/LinkButton";
import { formatFileSize } from "@/shared/lib/file-size";
import { localizedError } from "@/shared/lib/server-error";

type ResourceRow = {
  id: string;
  name: string;
  displayName?: string;
  type?: string;
  applicationId?: string;
  tag?: string;
  fileSize?: number;
  mimeType?: string;
};

type ResourceParams = {
  page: number;
  pageSize: number;
  search?: string;
  organizationId?: string;
  filters?: Record<string, string>;
};

export default function ResourcesPage() {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);
  const organization = useSessionStore((state) => state.organization);
  const { nameMap: appNameMap } = useApplicationNames(organization?.id ?? undefined);
  const [orgFilter, setOrgFilter] = useState(organization?.id ?? "");

  const list = useCrudList<ResourceRow, ResourceParams, string>({
    enabled: account !== null,
    buildQueryKey: (p) =>
      [
        "resources",
        "paginated",
        p.page,
        p.pageSize,
        p.search,
        p.organizationId,
        p.filters,
      ] as const,
    buildParams: ({ page, pageSize, search, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      organizationId: orgFilter || undefined,
      filters: {
        ...((filters.type as string | undefined) !== undefined ? { type: filters.type } : {}),
      },
    }),
    paginatedFn: getIamResourcesPaginated,
    deleteFn: deleteIamResource,
    invalidateKeysOnDelete: [queryKeys.resources.all],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),
  });

  return renderCrudListView<ResourceRow, ResourceParams, string>(
    {
      // ── UI ──
      title: tr("title"),
      description: tr("description"),
      searchPlaceholder: tr("searchPlaceholder"),
      rowKey: "id",
      searchableColumns: [
        { dataIndex: "name", label: tc("name") },
        { dataIndex: "displayName", label: tc("displayName") },
      ],
      filters: [
        {
          key: "type",
          label: tc("type"),
          options: resourceTypeOptions(),
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
          <PermissionGuard action="resource:write" resource="resources" fallback={null}>
            <LinkButton type="primary" to="/resources/new">
              {tc("create")}
            </LinkButton>
          </PermissionGuard>
        </Space>
      ),
      columns: [
        {
          title: tc("name"),
          dataIndex: "name",
          key: "name",
          width: 200,
          fixed: "left",
          render: (v: string, r: ResourceRow) =>
            r.id ? <Link to={`/resources/${r.id}`}>{v}</Link> : v,
        },
        { title: tc("displayName"), dataIndex: "displayName", key: "displayName" },
        { title: tc("type"), dataIndex: "type", key: "type" },
        {
          title: tr("iamResource.application"),
          dataIndex: "applicationId",
          key: "applicationId",
          render: (v: string) => (v ? appNameMap.get(v) || v : "-"),
        },
        {
          title: tr("iamResource.tag"),
          dataIndex: "tag",
          key: "tag",
          width: 120,
          render: (v?: string) => v || "-",
        },
        {
          title: tr("iamResource.fileSize"),
          dataIndex: "fileSize",
          key: "fileSize",
          width: 120,
          align: "right",
          render: (v?: number) => formatFileSize(v),
        },
        {
          title: tr("iamResource.mimeType"),
          dataIndex: "mimeType",
          key: "mimeType",
          width: 180,
          render: (v?: string) => v || "-",
        },
        {
          title: tc("actions"),
          key: "actions",
          width: 160,
          fixed: "right",
          render: (_, record) => (
            <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
              <LinkButton type="link" size="small" to={`/resources/${record.id}`}>
                {tc("edit")}
              </LinkButton>
              <Popconfirm
                title={tc("confirmDelete")}
                onConfirm={() =>
                  deleteIamResource(record.id)
                    .then(() => message.success(tc("deleteSuccess")))
                    .catch((e) => message.error(localizedError(e, tc, "common:error")))
                }
              >
                <Button type="link" size="small" danger>
                  {tc("delete")}
                </Button>
              </Popconfirm>
            </div>
          ),
        },
      ],

      // ── Bulk Delete ──
      enableBulkDelete: true,
      bulkDeletePermission: { permission: "resource:delete", resource: "resources" },
    },
    list,
  );
}
