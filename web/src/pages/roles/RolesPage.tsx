import { useState, useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { App, Button, Popconfirm, Space, Tag } from "antd";
import { UploadOutlined } from "@ant-design/icons";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getRolesPaginated, deleteRole, importRolesFromExcel } from "@/shared/api/iam/role";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { BatchImportModal, type ImportResult } from "@/shared/ui/BatchImportModal";
import { LinkButton } from "@/shared/ui/LinkButton";
import { serverErrorMessage } from "@/shared/lib/server-error";

type RoleParams = {
  page: number;
  pageSize: number;
  search?: string;
  organizationId?: string;
  isEnabled?: boolean;
};

export default function RolesPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);
  const organization = useSessionStore((state) => state.organization);
  const queryClient = useQueryClient();

  // ── 额外页面状态（renderCrudListPage 覆盖不到）──
  const [orgFilter, setOrgFilter] = useState(organization?.id ?? "");
  const [importModalOpen, setImportModalOpen] = useState(false);
  const handleOrgChange = useCallback((value: string) => {
    setOrgFilter(value);
  }, []);
  const handleImportFile = async (file: File): Promise<ImportResult> => {
    const result = await importRolesFromExcel(file);
    await queryClient.invalidateQueries({ queryKey: ["roles"] });
    return result;
  };

  return (
    <>
      {renderCrudListPage<
        { id: string; name: string; displayName: string; owner: string; isEnabled: boolean },
        RoleParams,
        string
      >({
        // ── UI ──
        title: t("role.title"),
        description: t("role.description"),
        searchPlaceholder: t("role.searchPlaceholder"),
        rowKey: "id",
        filters: [
          {
            // `roles` has no `status` column; the handler reads a boolean
            // `isEnabled` (`route.QueryOptionalBool` parses "true"/"false").
            key: "isEnabled",
            label: tc("statusLabel"),
            options: [
              { label: tc("enabled"), value: "true" },
              { label: tc("disabled"), value: "false" },
            ],
          },
        ],
        columns: [
          {
            title: tc("name"),
            dataIndex: "name",
            key: "name",
            width: 200,
            fixed: "left",
            render: (v: string, r: { id: string; owner: string; name: string }) =>
              r.id ? <Link to={`/roles/${r.owner}/${r.name}`}>{v}</Link> : v,
          },
          { title: tc("displayName"), dataIndex: "displayName", key: "displayName" },
          { title: t("role.organization"), dataIndex: "owner", key: "owner" },
          {
            title: tc("statusLabel"),
            dataIndex: "isEnabled",
            key: "isEnabled",
            render: (v: boolean) =>
              v ? (
                <Tag color="green">{tc("enabled")}</Tag>
              ) : (
                <Tag color="red">{tc("disabled")}</Tag>
              ),
          },
          {
            title: tc("actions"),
            key: "actions",
            width: 160,
            fixed: "right",
            render: (_, record) => (
              <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
                <LinkButton type="link" size="small" to={`/roles/${record.owner}/${record.name}`}>
                  {tc("edit")}
                </LinkButton>
                <PermissionGuard action="role:write" resource="roles" fallback={null}>
                  <Popconfirm
                    title={tc("confirmDelete")}
                    onConfirm={async () => {
                      try {
                        await deleteRole(record.id);
                        await queryClient.invalidateQueries({ queryKey: ["roles"] });
                        message.success(tc("deleteSuccess"));
                      } catch (error) {
                        message.error(serverErrorMessage(error, tc, "common:deleteFailed"));
                      }
                    }}
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
        toolbar: (
          <Space>
            {account?.isAdmin && (
              <OrganizationSelect
                value={orgFilter}
                onChange={handleOrgChange}
                style={{ width: 180 }}
                allowClear
                autoFillSessionOrg={false}
              />
            )}
            <PermissionGuard action="role:write" resource="roles" fallback={null}>
              <LinkButton type="primary" to="/roles/new">
                {t("role.newRole")}
              </LinkButton>
            </PermissionGuard>
            <PermissionGuard action="role:write" resource="roles" fallback={null}>
              <Button icon={<UploadOutlined />} onClick={() => setImportModalOpen(true)}>
                {tc("action.import")}
              </Button>
            </PermissionGuard>
          </Space>
        ),

        // ── Hooks ──
        enabled: account !== null,
        buildQueryKey: (p) =>
          [
            "roles",
            "paginated",
            p.page,
            p.pageSize,
            p.search,
            p.isEnabled,
            p.organizationId,
          ] as const,
        buildParams: ({ page, pageSize, search, filters }) => ({
          page,
          pageSize,
          search: search || undefined,
          organizationId: orgFilter || undefined,
          isEnabled:
            filters.isEnabled === "true" ? true : filters.isEnabled === "false" ? false : undefined,
        }),
        paginatedFn: getRolesPaginated,
        deleteFn: deleteRole,
        invalidateKeysOnDelete: [["roles"]],
        onDeleteSuccess: () => message.success(tc("deleteSuccess")),

        // ── Bulk Delete ──
        enableBulkDelete: true,
        bulkDeletePermission: { permission: "role:write", resource: "roles" },
      })}
      <BatchImportModal
        open={importModalOpen}
        onClose={() => setImportModalOpen(false)}
        onImport={handleImportFile}
        title={tc("batchImport.roles")}
        accept=".xlsx,.xls"
        templateHint={tc("batchImport.rolesHint")}
      />
    </>
  );
}
