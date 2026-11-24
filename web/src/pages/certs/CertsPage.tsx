import { useState } from "react";
import { App, Button, Popconfirm, Space } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamCertsPaginated, deleteIamCert } from "@/shared/api/iam";
import { renderCrudListView } from "@/shared/ui/CrudListPage";
import { useCrudList } from "@/shared/hooks/useCrudList";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { LinkButton } from "@/shared/ui/LinkButton";

type CertRow = {
  id: string;
  name: string;
  displayName?: string;
  type?: string;
  cryptoAlgorithm?: string;
};

type CertParams = {
  page: number;
  pageSize: number;
  search?: string;
  organizationId?: string;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
  filters?: Record<string, string>;
};

export default function CertsPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);
  const organization = useSessionStore((state) => state.organization);
  const [orgFilter, setOrgFilter] = useState(organization?.id ?? "");

  const list = useCrudList<CertRow, CertParams, string>({
    enabled: account !== null,
    buildQueryKey: (p) =>
      [
        "certs",
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
        ...((filters.scope as string | undefined) !== undefined ? { scope: filters.scope } : {}),
        ...((filters.type as string | undefined) !== undefined ? { type: filters.type } : {}),
      },
    }),
    paginatedFn: getIamCertsPaginated,
    deleteFn: deleteIamCert,
    invalidateKeysOnDelete: [["certs"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),
  });

  return renderCrudListView<CertRow, CertParams, string>(
    {
      // ── UI ──
      title: t("cert.title"),
      description: t("cert.description"),
      searchPlaceholder: t("cert.searchPlaceholder"),
      rowKey: "name",
      defaultSortField: "name",
      defaultSortOrder: "ascend",
      searchableColumns: [
        { dataIndex: "name", label: tc("name") },
        { dataIndex: "displayName", label: tc("displayName") },
      ],
      filters: [
        {
          // `certs` has no `is_enabled` column, so the earlier `status` →
          // `isEnabled` rewrite still matched no parameter (the handler reads
          // only `organizationId`/`keyword`/`scope`/`type`): the dropdown
          // rendered, forwarded, and did nothing.
          //
          // Its real closed-set attribute is `scope` (`JWT` | `SSL`, enforced by
          // `app/certs/validation.go`), which `queryListCerts` already filters.
          key: "scope",
          label: tc("statusLabel"),
          options: [
            { label: "JWT", value: "JWT" },
            { label: "SSL", value: "SSL" },
          ],
        },
        {
          key: "type",
          label: tc("type"),
          options: [{ label: "X.509", value: "x509" }],
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
          <PermissionGuard action="cert:write" resource="certs" fallback={null}>
            <LinkButton type="primary" to="/certs/new">
              {t("cert.newCert")}
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
          fixed: "left" as const,
          sorter: (a, b) => String(a.name ?? "").localeCompare(String(b.name ?? "")),
          render: (v: string, record: CertRow) => <Link to={`/certs/${record.id}`}>{v}</Link>,
        },
        {
          title: tc("displayName"),
          dataIndex: "displayName",
          key: "displayName",
        },
        {
          title: tc("type"),
          dataIndex: "type",
          key: "type",
        },
        {
          title: t("cert.algorithm"),
          // `cryptoAlgorithm` is the field the server sends; `algorithm` was
          // never one, so this column always rendered blank.
          dataIndex: "cryptoAlgorithm",
          key: "cryptoAlgorithm",
        },
        {
          title: tc("actions"),
          key: "actions",
          width: 160,
          fixed: "right" as const,
          render: (_, record) => (
            <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
              <PermissionGuard action="cert:write" resource="certs" fallback={null}>
                <LinkButton type="link" size="small" to={`/certs/${record.id}`}>
                  {tc("edit")}
                </LinkButton>
              </PermissionGuard>
              <PermissionGuard action="cert:delete" resource="certs" fallback={null}>
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
      bulkDeletePermission: { permission: "cert:delete", resource: "certs" },
    },
    list,
  );
}
