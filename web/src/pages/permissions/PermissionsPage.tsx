import { useState } from "react";
import { Button, Popconfirm, Space, Tag, Upload, App } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { UploadOutlined } from "@ant-design/icons";
import { useSessionStore } from "@/features/session/session-store";
import {
  getPermissionsPaginated,
  deletePermission,
  importPermissions,
  importPermissionsFromExcel,
  type ImportPermissionItem,
} from "@/shared/api/iam/permission";
import { renderCrudListView } from "@/shared/ui/CrudListPage";
import { useCrudList } from "@/shared/hooks/useCrudList";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { BatchUploadModal } from "@/shared/components/BatchUploadModal";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { LinkButton } from "@/shared/ui/LinkButton";
import { permissionPath } from "@/shared/lib/routePaths";
import { serverErrorMessage } from "@/shared/lib/server-error";

type PermissionParams = {
  page: number;
  pageSize: number;
  search?: string;
  organizationId?: string;
  effect?: string;
  isEnabled?: boolean;
};

/**
 * 列表行来自 `Record<string, unknown>`；只有 owner/name 都是非空字符串时
 * 才能构造注册路由 `/permissions/:organizationName/:permissionName`。
 */
function permissionRowPath(row: Record<string, unknown>): string | null {
  const { owner, name } = row;
  if (typeof owner !== "string" || typeof name !== "string" || !owner || !name) return null;
  return permissionPath(owner, name);
}

export default function PermissionsPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);
  const organization = useSessionStore((state) => state.organization);
  const [uploadModalOpen, setUploadModalOpen] = useState(false);
  const [orgFilter, setOrgFilter] = useState(organization?.id ?? "");
  const queryClient = useQueryClient();

  const list = useCrudList<Record<string, unknown>, PermissionParams, string>({
    enabled: account !== null,
    buildQueryKey: (p) =>
      [
        "permissions",
        "paginated",
        p.page,
        p.pageSize,
        p.search,
        p.organizationId,
        p.effect,
        p.isEnabled,
      ] as const,
    buildParams: ({ page, pageSize, search, filters }) => {
      const isEnabledRaw = filters.isEnabled as string | undefined;
      const isEnabled =
        isEnabledRaw === "true" ? true : isEnabledRaw === "false" ? false : undefined;
      return {
        page,
        pageSize,
        search: search || undefined,
        organizationId: orgFilter || undefined,
        effect: (filters.effect as string | undefined) || undefined,
        isEnabled,
      };
    },
    paginatedFn: getPermissionsPaginated,
    deleteFn: deletePermission,
    invalidateKeysOnDelete: [["permissions"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),
  });

  const importMutation = useMutation({
    mutationFn: ({ orgId, permissions }: { orgId: string; permissions: ImportPermissionItem[] }) =>
      importPermissions(orgId, permissions),
    onSuccess: (result) => {
      message.success(
        tc("importSuccessWithCount", { succeeded: result.succeeded, total: result.total }),
      );
      if (result.failed > 0) {
        message.warning(`${tc("importPartial")}: ${result.errors?.slice(0, 3).join("; ")}`);
      }
      void queryClient.invalidateQueries({ queryKey: ["permissions"] });
    },
    onError: (error) => {
      message.error(serverErrorMessage(error, tc, "common:operationFailed"));
    },
  });

  const handleImport = (file: File) => {
    const reader = new FileReader();
    reader.onload = (e) => {
      try {
        const data = JSON.parse(e.target?.result as string) as ImportPermissionItem[];
        const orgId = account?.organization?.id ?? "";
        importMutation.mutate({ orgId, permissions: data });
      } catch {
        message.error(tc("invalidJsonFile"));
      }
    };
    reader.readAsText(file);
    return false;
  };

  return (
    <>
      {renderCrudListView<Record<string, unknown>, PermissionParams, string>(
        {
          // ── UI ──
          title: t("permission.title"),
          description: t("permission.description"),
          searchPlaceholder: t("permission.searchPlaceholder"),
          rowKey: (record) => `${record.owner}/${record.name}`,
          searchableColumns: [
            { dataIndex: "name", label: tc("name") },
            { dataIndex: "displayName", label: tc("displayName") },
          ],
          filters: [
            {
              key: "effect",
              label: t("permission.effect"),
              options: [
                { label: t("permission.effectAllow"), value: "Allow" },
                { label: t("permission.effectDeny"), value: "Deny" },
              ],
            },
            {
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
              <PermissionGuard action="permission:write" resource="permissions" fallback={null}>
                <Upload accept=".json" showUploadList={false} beforeUpload={handleImport}>
                  <Button icon={<UploadOutlined />} loading={importMutation.isPending}>
                    {tc("import")}
                  </Button>
                </Upload>
              </PermissionGuard>
              <PermissionGuard action="permission:write" resource="permissions" fallback={null}>
                <Button icon={<UploadOutlined />} onClick={() => setUploadModalOpen(true)}>
                  {tc("uploadExcel")}
                </Button>
              </PermissionGuard>
              <PermissionGuard action="permission:write" resource="permissions" fallback={null}>
                <LinkButton type="primary" to="/permissions/new">
                  {t("permission.newPermission")}
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
              render: (v: string, r: Record<string, unknown>) => {
                const to = permissionRowPath(r);
                return r.id && to ? <Link to={to}>{v}</Link> : v;
              },
            },
            {
              title: tc("displayName"),
              dataIndex: "displayName",
              key: "displayName",
            },
            {
              title: t("permission.resourceType"),
              dataIndex: "resourceType",
              key: "resourceType",
              render: (v: string) => <Tag>{v}</Tag>,
            },
            {
              title: t("permission.effect"),
              dataIndex: "effect",
              key: "effect",
              render: (v: string) => (
                <Tag color={v === "Allow" ? "green" : "red"}>{v || "Allow"}</Tag>
              ),
            },
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
              render: (_, record) => {
                const to = permissionRowPath(record);
                return (
                  <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
                    {to && (
                      <LinkButton type="link" size="small" to={to}>
                        {tc("edit")}
                      </LinkButton>
                    )}
                    <PermissionGuard
                      action="permission:write"
                      resource="permissions"
                      fallback={null}
                    >
                      <Popconfirm
                        title={tc("confirmDelete")}
                        onConfirm={() => list.deleteMutation.mutate(record.id as string)}
                      >
                        <Button type="link" size="small" danger>
                          {tc("delete")}
                        </Button>
                      </Popconfirm>
                    </PermissionGuard>
                  </div>
                );
              },
            },
          ],

          // ── Bulk Delete ──
          enableBulkDelete: true,
          bulkDeletePermission: { permission: "permission:write", resource: "permissions" },
        },
        list,
      )}
      <BatchUploadModal
        open={uploadModalOpen}
        onClose={() => setUploadModalOpen(false)}
        onUpload={async (file) => {
          const result = await importPermissionsFromExcel(
            file,
            orgFilter || account?.organization?.id,
          );
          await queryClient.invalidateQueries({ queryKey: ["permissions"] });
          return result;
        }}
        title={t("permission.uploadTitle")}
      />
    </>
  );
}
