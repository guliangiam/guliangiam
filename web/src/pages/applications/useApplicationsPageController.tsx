import { useEffect, useMemo, useRef, useState } from "react";
import { App, Button, Space, Tag, Typography, Upload } from "antd";
import { UploadOutlined } from "@ant-design/icons";
import { Link, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import type { UploadProps } from "antd";
import {
  createApplication,
  deleteApplication,
  exportApplicationConfig,
  getApplication,
  getApplicationsPaginated,
  importApplicationConfig,
  type ApplicationListParams,
  type CreateApplicationPayload,
} from "@/shared/api/iam/application";
import { useSessionStore } from "@/features/session/session-store";
import type { CrudListViewConfig } from "@/shared/ui/CrudListPage";
import { useCrudList, type CrudListResult } from "@/shared/hooks/useCrudList";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { LinkButton } from "@/shared/ui/LinkButton";
import { queryKeys } from "@/shared/api/query-keys";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import {
  APP_STATUS_VALUES,
  CATEGORY_COLORS,
  CATEGORY_VALUES,
  type ApplicationRow,
} from "./application-list-helpers";
import { ApplicationsRowActions } from "./ApplicationsRowActions";
import { downloadApplicationExport } from "./application-helpers";
import { importRequiresClientMaterial, importRequiresSecretRotation } from "./application-payload";
import { localizedError } from "@/shared/lib/server-error";

export type ApplicationDeleteKey = { id: string; organizationId?: string };

export function useApplicationsPageController(): CrudListViewConfig<
  ApplicationRow,
  ApplicationListParams,
  ApplicationDeleteKey
> & { crud: CrudListResult<ApplicationRow, ApplicationDeleteKey> } {
  const { message } = App.useApp();
  const { t: tr } = useTranslation("resource");
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const organization = useSessionStore((state) => state.organization);
  const { nameMap: orgNameMap } = useOrganizationNames();

  // 组织筛选跟随会话组织：显式选择绑定在当时的会话上下文上，会话切换后自动
  // 失效并回落到新会话组织（对照 useUsersPageController 的 orgSelection 先例），
  // 避免一次性 useState 初值把旧组织钉死。
  const sessionOrgId = organization?.id ?? "";
  const [orgSelection, setOrgSelection] = useState<{ context: string; value: string } | null>(null);
  const orgFilter =
    orgSelection && orgSelection.context === sessionOrgId ? orgSelection.value : sessionOrgId;
  const [duplicatingId, setDuplicatingId] = useState<string | null>(null);
  const [exportingId, setExportingId] = useState<string | null>(null);

  // 页面壳需要直接操作分页（切组织重置页码），因此自持 useCrudList，
  // View 用 renderCrudListView 消费同一份 state（对照 UsersPage/SessionsPage）。
  const crud = useCrudList<ApplicationRow, ApplicationListParams, ApplicationDeleteKey>({
    buildQueryKey: (p) => queryKeys.applications.paginatedWithParams(p),
    buildParams: ({ page, pageSize, search, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      organizationId: orgFilter || undefined,
      category: filters.category || undefined,
      status: filters.status || undefined,
    }),
    paginatedFn: getApplicationsPaginated,
    deleteFn: (key) => deleteApplication(key.id, key.organizationId),
    invalidateKeysOnDelete: [queryKeys.applications.paginated(), queryKeys.applications.all],
    enabled: organization !== null,
  });

  // 会话组织切换后回到第 1 页：旧组织的页码可能超出新组织的结果范围。
  const lastSessionOrgIdRef = useRef(sessionOrgId);
  useEffect(() => {
    if (lastSessionOrgIdRef.current === sessionOrgId) return;
    lastSessionOrgIdRef.current = sessionOrgId;
    crud.serverPagination.onChange(1, crud.pagination.pageSize);
  }, [sessionOrgId, crud]);

  const handleExport = async (record: ApplicationRow) => {
    if (!record.id) return;
    try {
      setExportingId(record.id);
      const blob = await exportApplicationConfig(record.id, record.organizationId);
      downloadApplicationExport(blob, record.name);
    } catch (err) {
      message.error(localizedError(err, tr, "application.exportFailed"));
    } finally {
      setExportingId(null);
    }
  };

  const handleDuplicate = async (record: ApplicationRow) => {
    if (!record.id || !record.organizationId) return;
    try {
      setDuplicatingId(record.id);
      const detail = await getApplication(record.id);
      // Event-handler-only: never executed during render (react/purity false positive).
      // eslint-disable-next-line react/purity
      const newSlug = `${detail.slug}-copy-${Date.now().toString(36)}`;
      const payload: CreateApplicationPayload = {
        organizationId: detail.organizationId,
        slug: newSlug,
        displayName: `Copy of ${detail.displayName || detail.name}`,
        category: detail.category,
        appType: detail.appType,
        description: detail.description,
        homepageUrl: detail.homepageUrl,
        domain: detail.domain,
        status: detail.status || "active",
        clientId: "", // backend generates new
        clientSecret: "", // backend generates new
        redirectUris: detail.redirectUris?.length ? detail.redirectUris : undefined,
        grantTypes: detail.grantTypes?.length ? detail.grantTypes : undefined,
        tokenEndpointAuthMethod: detail.tokenEndpointAuthMethod,
        customScopes: detail.customScopes?.length ? detail.customScopes : undefined,
      };
      const result = await createApplication(payload);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.applications.paginated() }),
        queryClient.invalidateQueries({ queryKey: queryKeys.applications.all }),
      ]);
      message.success(tr("application.duplicated", "Application duplicated"));
      navigate(`/applications/${result.id}`);
    } catch (err) {
      message.error(localizedError(err, tr, "application.duplicateFailed"));
    } finally {
      setDuplicatingId(null);
    }
  };

  const handleImportFile = async (file: File): Promise<void> => {
    if (!organization?.id) {
      message.error(tr("application.importNoOrg", "Select an organization first"));
      return;
    }
    try {
      const text = await file.text();
      const data = JSON.parse(text);
      if (importRequiresClientMaterial(data)) {
        message.error(
          tr(
            "application.importRequiresClientMaterial",
            "This export needs a key or certificate. Import the application first, then register the client material in the console.",
          ),
        );
        return;
      }
      await importApplicationConfig(organization.id, data);
      await queryClient.invalidateQueries({ queryKey: queryKeys.applications.paginated() });
      if (importRequiresSecretRotation(data)) {
        message.warning(
          tr(
            "application.importSecretNotIncluded",
            "Application imported. The export carries no client secret; rotate the client secret in the console before using client_secret_basic or client_secret_post.",
          ),
        );
      } else {
        message.success(tr("application.imported", "Application imported"));
      }
    } catch (err) {
      message.error(localizedError(err, tr, "application.importFailed"));
      throw err; // 让 Upload 组件显示 error 状态
    }
  };

  const importUploadProps: UploadProps = {
    accept: ".json",
    showUploadList: false,
    beforeUpload: async (file) => {
      await handleImportFile(file);
      return false; // 阻止自动上传
    },
  };

  const categoryOptions = useMemo(
    () =>
      CATEGORY_VALUES.map((v) => ({ label: tr(`application.categoryOption.${v}`, v), value: v })),
    [tr],
  );
  const statusOptions = useMemo(
    () =>
      APP_STATUS_VALUES.map((v) => ({
        label: tr(`application.statusOption.${v}`, v.charAt(0).toUpperCase() + v.slice(1)),
        value: v,
      })),
    [tr],
  );

  return {
    title: tr("application.title", "Identity Access"),
    description: tr(
      "application.description",
      "Application list, details, creation and deletion have switched to IAM API. This page serves as the unified entry for application management.",
    ),
    searchPlaceholder: tr("application.searchPlaceholder", "Filter by application name"),
    rowKey: "id",
    rowKeyToDeleteKey: (r) => ({ id: r.id ?? "", organizationId: r.organizationId }),
    filters: [
      { key: "category", label: tr("application.category", "Category"), options: categoryOptions },
      { key: "status", label: tr("application.status", "Status"), options: statusOptions },
    ],
    columns: (list) => [
      {
        title: tr("application.name", "Name"),
        dataIndex: "name",
        key: "name",
        width: 180,
        fixed: "left" as const,
        ellipsis: true,
        render: (_: unknown, record: ApplicationRow) =>
          record.id ? (
            <Link to={`/applications/${record.id}`}>{record.displayName || record.name}</Link>
          ) : (
            record.name
          ),
      },
      {
        title: tr("application.displayName", "Display Name"),
        dataIndex: "displayName",
        key: "displayName",
        width: 180,
        ellipsis: true,
      },
      {
        title: tr("application.category", "Category"),
        dataIndex: "category",
        key: "category",
        width: 100,
        render: (v: string) => (v ? <Tag color={CATEGORY_COLORS[v] ?? "default"}>{v}</Tag> : "-"),
      },
      {
        title: tr("application.organization", "Organization"),
        dataIndex: "organizationId",
        key: "organizationId",
        width: 160,
        ellipsis: true,
        render: (orgId: string) => (orgId ? orgNameMap.get(orgId) || orgId : "-"),
      },
      {
        title: tr("application.clientId", "Client ID"),
        dataIndex: "clientId",
        key: "clientId",
        width: 200,
        ellipsis: true,
        render: (v: string) =>
          v ? (
            <Typography.Text copyable code style={{ fontSize: 12 }}>
              {v}
            </Typography.Text>
          ) : (
            "-"
          ),
      },
      {
        title: tr("application.redirectUris"),
        dataIndex: "redirectUris",
        key: "redirectUris",
        width: 240,
        ellipsis: true,
        render: (value: string[] | null | undefined) =>
          Array.isArray(value) && value.length > 0 ? (
            <Typography.Text style={{ fontSize: 12 }}>{value.join(", ")}</Typography.Text>
          ) : (
            "-"
          ),
      },
      {
        title: tr("application.providerCount", "Providers"),
        dataIndex: "providers",
        key: "providers",
        width: 100,
        render: (value: string[] | null | undefined) =>
          Array.isArray(value) && value.length > 0 ? (
            <Tag color="blue">{tr("application.providerLinkedCount", { count: value.length })}</Tag>
          ) : (
            <Tag>0</Tag>
          ),
      },
      {
        title: tr("application.status", "Status"),
        dataIndex: "status",
        key: "status",
        width: 80,
        render: (v: string) => (
          <Tag color={v === "active" ? "green" : "default"}>
            {v ? tr(`application.statusOption.${v}`, v) : "-"}
          </Tag>
        ),
      },
      {
        title: tr("application.createdAt"),
        dataIndex: "createdAt",
        key: "createdAt",
        width: 160,
        render: (v: string) => (v ? new Date(v).toLocaleString() : "-"),
      },
      {
        title: tr("application.actions", "Actions"),
        key: "actions",
        width: 300,
        fixed: "right" as const,
        render: (_: unknown, record: ApplicationRow) => (
          <ApplicationsRowActions
            record={record}
            duplicatingId={duplicatingId}
            exportingId={exportingId}
            onDuplicate={(item) => void handleDuplicate(item)}
            onExport={(item) => void handleExport(item)}
            onDelete={(item) =>
              list.deleteMutation.mutateAsync({
                id: item.id ?? "",
                organizationId: item.organizationId,
              })
            }
          />
        ),
      },
    ],
    crud,
    enableBulkDelete: true,
    bulkDeletePermission: { permission: "app:delete", resource: "applications" },
    bulkDeleteDescription: tr(
      "application.deleteIrreversible",
      "Deletion is irreversible and cleans up consents, tokens and bindings.",
    ),
    toolbar: (
      <Space>
        {organization && (
          <OrganizationSelect
            value={orgFilter}
            onChange={(v) => {
              setOrgSelection({ context: sessionOrgId, value: v ?? "" });
              crud.serverPagination.onChange(1, crud.pagination.pageSize);
            }}
            style={{ width: 180 }}
            allowClear
          />
        )}
        <PermissionGuard action="app:write" resource="applications" fallback={null}>
          <LinkButton type="primary" to="/applications/new">
            {tr("application.newApp", "New Application")}
          </LinkButton>
        </PermissionGuard>
        <PermissionGuard action="app:write" resource="applications" fallback={null}>
          <Upload {...importUploadProps}>
            <Button
              icon={<UploadOutlined />}
              aria-label={tr("application.import", "Import")}
              title={tr("application.importTooltip", "Import application from JSON")}
            >
              {tr("application.import", "Import")}
            </Button>
          </Upload>
        </PermissionGuard>
      </Space>
    ),
  };
}
