import { App, Avatar, Button, Tag, Popconfirm } from "antd";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getOrganizationsPaginated, deleteOrganization } from "@/shared/api/iam/organization";
import { getApplicationOptions } from "@/shared/api/iam/application";
import { defaultApplicationLabel } from "./default-application";
import { normalizeOrganizationStatus } from "./organization-status";
import { queryKeys } from "@/shared/api/query-keys";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { LinkButton } from "@/shared/ui/LinkButton";
import { serverErrorMessage } from "@/shared/lib/server-error";

type OrgParams = {
  page: number;
  pageSize: number;
  search?: string;
  status?: string;
};

export default function OrganizationsPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [deletingId, setDeletingId] = useState<string | null>(null);
  // One batch options query (platform-wide list) maps default application ids to
  // names; the column never triggers a per-row request.
  const { data: applicationOptions } = useQuery({
    queryKey: [...queryKeys.applications.auth(), "options", "all"],
    queryFn: () => getApplicationOptions(),
    enabled: account !== null,
    staleTime: 5 * 60_000,
  });
  const applicationNameById = new Map<string, string>();
  for (const application of applicationOptions ?? []) {
    applicationNameById.set(application.id, application.displayName || application.slug);
  }

  return renderCrudListPage<
    {
      id: string;
      name: string;
      slug: string;
      displayName: string;
      status: string;
      logoUrl?: string;
      defaultApplication?: string;
      websiteUrl?: string;
      passwordType?: string;
      balance?: number;
      currency?: string;
      createdAt?: string;
    },
    OrgParams,
    string
  >({
    // ── UI ──
    title: t("organization.title"),
    description: t("organization.description"),
    searchPlaceholder: t("organization.searchPlaceholder"),
    rowKey: "id",
    columns: [
      {
        title: "",
        dataIndex: "logoUrl",
        key: "logoUrl",
        width: 40,
        render: (v: string, record: { name: string; displayName?: string }) =>
          v ? (
            <Avatar src={v} alt={record.displayName || record.name} size="small" shape="square" />
          ) : (
            <Avatar
              size="small"
              shape="square"
              style={{ backgroundColor: "var(--gl-color-primary)" }}
            >
              O
            </Avatar>
          ),
      },
      {
        title: tc("name"),
        dataIndex: "name",
        key: "name",
        fixed: "left",
        render: (v: string, r: { id: string; slug?: string; name: string }) =>
          r.id ? <Link to={`/organizations/${r.slug ?? r.name}`}>{v}</Link> : v,
      },
      { title: tc("displayName"), dataIndex: "displayName", key: "displayName" },
      {
        title: t("organization.defaultApplication"),
        dataIndex: "defaultApplication",
        key: "defaultApplication",
        render: (value: string | undefined) => {
          const resolved = defaultApplicationLabel(value, applicationNameById);
          if (!resolved) return "-";
          return <span title={resolved.title}>{resolved.label}</span>;
        },
      },
      {
        title: t("organization.passwordType"),
        dataIndex: "passwordType",
        key: "passwordType",
        width: 120,
        render: (v: string) => (v ? <Tag>{v}</Tag> : "-"),
      },
      {
        title: tc("statusLabel"),
        dataIndex: "status",
        key: "status",
        width: 80,
        render: (v: string) => {
          const status = normalizeOrganizationStatus(v);
          if (!status) return <Tag>-</Tag>;
          const label = tc(`enumValues.${status}`, { defaultValue: status });
          return <Tag color={status === "active" ? "green" : "default"}>{label}</Tag>;
        },
      },
      {
        title: tc("balance"),
        dataIndex: "balance",
        key: "balance",
        width: 100,
        align: "right",
        render: (v: number | null | undefined) =>
          v !== null && v !== undefined ? v.toFixed(2) : "-",
      },
      {
        title: tc("currency.label"),
        dataIndex: "currency",
        key: "currency",
        width: 80,
        render: (v: string) => (v ? <Tag>{v}</Tag> : "-"),
      },
      {
        title: t("organization.website"),
        dataIndex: "websiteUrl",
        key: "websiteUrl",
        render: (v: string) =>
          v ? (
            <a href={v} target="_blank" rel="noopener noreferrer">
              {v}
            </a>
          ) : (
            "-"
          ),
      },
      {
        title: tc("createdAt"),
        dataIndex: "createdAt",
        key: "createdAt",
        render: (v: string) => (v ? new Date(v).toLocaleDateString() : "-"),
      },
      {
        title: tc("actions"),
        key: "actions",
        width: 280,
        fixed: "right",
        render: (_, record) => {
          const isBuiltIn = record.slug === "built-in" || record.name === "built-in";
          const orgIdent = record.slug ?? record.name;
          return (
            <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
              <LinkButton
                className="gl-table-action-link"
                type="link"
                size="small"
                to={`/organizations/${orgIdent}`}
              >
                {tc("edit")}
              </LinkButton>
              {/* Casdoor OrganizationListPage.tsx#L108-111 — Groups/Users quick-link row actions.
                  GLIAM 用 session + orgFilter 架构（无嵌套路径），这里显式携带组织上下文，
                  便于目标页按 organizationId 过滤。*/}
              <LinkButton
                className="gl-table-action-link"
                type="link"
                size="small"
                to={`/groups?organizationId=${encodeURIComponent(record.id)}`}
              >
                {t("organization.groupsLink", { defaultValue: "Groups" })}
              </LinkButton>
              <LinkButton
                className="gl-table-action-link"
                type="link"
                size="small"
                to={`/users?organizationId=${encodeURIComponent(record.id)}`}
              >
                {t("organization.usersLink", { defaultValue: "Users" })}
              </LinkButton>
              {/* DELETE /api/v1/organizations/{id} 的声明权限是 organizations:delete，
                  不是 organizations:write（routes.go:71 + permissions.yaml:103）。 */}
              <PermissionGuard
                action="organizations:delete"
                resource="organizations"
                fallback={null}
              >
                <Popconfirm
                  okButtonProps={{
                    loading: deletingId === record.id,
                    disabled: isBuiltIn || deletingId !== null,
                  }}
                  title={isBuiltIn ? t("organization.builtInProtected") : tc("confirmDelete")}
                  okText={isBuiltIn ? undefined : tc("delete")}
                  cancelText={tc("cancel")}
                  onConfirm={async () => {
                    setDeletingId(record.id);
                    try {
                      await deleteOrganization(record.id);
                      await queryClient.invalidateQueries({
                        queryKey: queryKeys.organizations.all,
                      });
                      message.success(tc("deleteSuccess"));
                    } catch (error) {
                      message.error(serverErrorMessage(error, tc, "common:deleteFailed"));
                    } finally {
                      setDeletingId(null);
                    }
                  }}
                >
                  <Button type="link" size="small" danger disabled={isBuiltIn}>
                    {tc("delete")}
                  </Button>
                </Popconfirm>
              </PermissionGuard>
            </div>
          );
        },
      },
    ],
    filters: [
      {
        key: "status",
        label: tc("statusLabel"),
        options: [
          { label: tc("enabled"), value: "active" },
          { label: tc("enumValues.suspended"), value: "suspended" },
        ],
      },
    ],
    // 工具栏：新建按钮
    toolbar: (
      <PermissionGuard action="organizations:write" resource="organizations" fallback={null}>
        <Button type="primary" onClick={() => navigate("/organizations/new")}>
          {t("organization.newOrganization")}
        </Button>
      </PermissionGuard>
    ),

    // ── Hooks ──
    enabled: account !== null,
    buildQueryKey: (p) => queryKeys.organizations.paginatedWithParams(p),
    buildParams: ({ page, pageSize, search, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      status: filters.status || undefined,
    }),
    paginatedFn: getOrganizationsPaginated,
    deleteFn: deleteOrganization,
    invalidateKeysOnDelete: [queryKeys.organizations.all],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),

    // ── Bulk Delete (built-in org is not selectable) ──
    enableBulkDelete: true,
    bulkDeletePermission: { permission: "organizations:delete", resource: "organizations" },
    rowSelectable: (record) => record.slug !== "built-in" && record.name !== "built-in",
  });
}
