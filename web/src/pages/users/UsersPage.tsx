import { useMemo, useCallback } from "react";
import { Avatar, Button, Space, Switch, Tag, Tooltip } from "antd";
import {
  DownloadOutlined,
  EditOutlined,
  UploadOutlined,
  UserSwitchOutlined,
} from "@ant-design/icons";
import { Link, useNavigate } from "react-router-dom";
import { renderCrudListView, type CrudListViewConfig } from "@/shared/ui/CrudListPage";
import { ConfirmButton } from "@/shared/ui/ConfirmButton";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { useUsersPageController, type UserDeleteKey } from "./useUsersPageController";
import { userDetailPath, userRouteSlug } from "./user-route";
import type { UserListItem, UserListParams } from "@/shared/api/iam/user";

/** 用户批量操作按钮组 — 文案、loading 与回调由调用方传入，避免在页面组件内定义内嵌组件。 */
function renderUserBulkActions({
  enableLabel,
  disableLabel,
  resetLabel,
  loading,
  onEnable,
  onDisable,
  onReset,
}: {
  enableLabel: string;
  disableLabel: string;
  resetLabel: string;
  loading: boolean;
  onEnable: () => void;
  onDisable: () => void;
  onReset: () => void;
}) {
  return (
    <Space size="small">
      <PermissionGuard action="users:write" resource="users" fallback={null}>
        <Button size="small" loading={loading} onClick={onEnable}>
          {enableLabel}
        </Button>
      </PermissionGuard>
      <PermissionGuard action="users:write" resource="users" fallback={null}>
        <Button size="small" loading={loading} onClick={onDisable}>
          {disableLabel}
        </Button>
      </PermissionGuard>
      <Button size="small" onClick={onReset}>
        {resetLabel}
      </Button>
    </Space>
  );
}

export default function UsersPage() {
  const navigate = useNavigate();
  const {
    t,
    tr,
    ta,
    account,
    organizationName,
    orgFilter,
    orgContext,
    orgNameMap,
    orgSlugById,
    appNameMap,
    optimisticUserStatus,
    hoverPrefetch,
    prefetchUserDetail,
    impersonateMutation,
    exportMutation,
    toggleAdminMutation,
    bulkStatusMutation,
    crud,
    handleImport,
    handleOrgChange,
    handleReset,
    DEFAULT_SORT_FIELD,
    DEFAULT_SORT_ORDER,
  } = useUsersPageController();

  // Casdoor UserListPage.tsx#L36-38 — protect self + built-in admin from UI deletion
  const isProtected = useCallback(
    (record: UserListItem) => {
      const isSelf = record.username === account?.name;
      const isBuiltInAdmin = record.organizationId === "built-in" && record.username === "admin";
      return isSelf || isBuiltInAdmin;
    },
    [account?.name],
  );

  const renderUserLink = useCallback(
    (username: string, organizationId?: string) => {
      // Detail route param is the org *slug*; display names resolve to 404.
      const orgSlug = (organizationId && orgSlugById.get(organizationId)) || organizationName;
      return <Link to={`/users/${orgSlug}/${username}`}>{username}</Link>;
    },
    [orgSlugById, organizationName],
  );

  const columns = useMemo(
    () => [
      {
        title: "",
        key: "avatar",
        width: 48,
        fixed: "left" as const,
        render: (_: unknown, record: UserListItem) => {
          const dn = record.displayName || record.username;
          return record.avatarUrl ? (
            <Avatar size="small" src={record.avatarUrl} alt={dn}>
              {dn.charAt(0).toUpperCase()}
            </Avatar>
          ) : (
            <Avatar size="small" style={{ backgroundColor: "var(--gl-color-primary)" }}>
              {dn.charAt(0).toUpperCase()}
            </Avatar>
          );
        },
      },
      {
        title: t("name"),
        key: "username",
        dataIndex: "username",
        fixed: "left" as const,
        width: 140,
        sorter: true,
        render: (_username: string, record: UserListItem) =>
          renderUserLink(record.username, record.organizationId),
      },
      {
        title: t("organization"),
        key: "organizationId",
        dataIndex: "organizationId",
        width: 120,
        render: (orgId: string) =>
          orgId ? (
            <Link to={`/organizations/${orgId}`}>{orgNameMap.get(orgId) || orgId}</Link>
          ) : (
            "-"
          ),
      },
      { title: t("displayName"), key: "displayName", dataIndex: "displayName", width: 140 },
      {
        title: ta("email"),
        key: "email",
        dataIndex: "email",
        width: 180,
        render: (email: string) => (email ? <a href={`mailto:${email}`}>{email}</a> : "-"),
      },
      { title: t("auth:phone"), key: "phone", dataIndex: "phone", width: 130 },
      { title: t("realName"), key: "realName", dataIndex: "realName", width: 100 },
      {
        title: tr("user.affiliation"),
        key: "affiliation",
        dataIndex: "affiliation",
        width: 140,
        render: (v: string) => v || "-",
      },
      {
        title: tr("user.registerType"),
        key: "registerType",
        dataIndex: "registerType",
        width: 130,
        render: (v: string) => v || "-",
      },
      {
        title: tr("user.registerSource"),
        key: "registerSource",
        dataIndex: "registerSource",
        width: 160,
        render: (v: string) => v || "-",
      },
      {
        title: t("userType"),
        key: "userType",
        dataIndex: "userType",
        width: 90,
        render: (ut: string) => (ut ? <Tag>{ut}</Tag> : "-"),
      },
      {
        title: tr("user.tag"),
        key: "tag",
        dataIndex: "tag",
        width: 120,
        render: (v: string) => (v ? <Tag>{v}</Tag> : "-"),
      },
      {
        title: t("signupApp"),
        key: "signupApplicationId",
        dataIndex: "signupApplicationId",
        width: 110,
        render: (appId: string) =>
          appId ? <Link to={`/applications/${appId}`}>{appNameMap.get(appId) || appId}</Link> : "-",
      },
      {
        title: t("statusLabel"),
        key: "status",
        dataIndex: "status",
        width: 90,
        render: (status: string, record: UserListItem) => {
          if (!status) return "-";
          const isActive = status === "active";
          return (
            <PermissionGuard action="users:write" resource="users" fallback={null}>
              <Switch
                size="small"
                checked={isActive}
                title={`${tr("user.status")}: ${record.username}`}
                loading={optimisticUserStatus.isLoading(record.id)}
                checkedChildren="✓"
                unCheckedChildren="✕"
                onChange={(checked) =>
                  optimisticUserStatus.mutate({
                    userId: record.id,
                    status: checked ? "active" : "disabled",
                    revision: record.revision ?? 0,
                    organizationId: record.organizationId,
                  })
                }
              />
            </PermissionGuard>
          );
        },
      },
      {
        title: tr("user.isAdmin"),
        key: "isAdmin",
        dataIndex: "isAdmin",
        width: 80,
        render: (isAdmin: boolean, record: UserListItem) => (
          <PermissionGuard action="users:write" resource="users" fallback={null}>
            <Switch
              size="small"
              checked={isAdmin}
              title={`${tr("user.isAdmin")}: ${record.username}`}
              loading={
                toggleAdminMutation.isPending && toggleAdminMutation.variables?.userId === record.id
              }
              onChange={(checked) =>
                toggleAdminMutation.mutate({
                  userId: record.id,
                  isAdmin: checked,
                  revision: record.revision ?? 0,
                  organizationId: record.organizationId,
                })
              }
            />
          </PermissionGuard>
        ),
      },
      {
        title: t("verified"),
        key: "isVerified",
        dataIndex: "isVerified",
        width: 80,
        render: (v: boolean) => (v ? <Tag color="green">{t("yes")}</Tag> : "-"),
      },
      {
        title: tr("user.forbidden"),
        key: "isForbidden",
        dataIndex: "isForbidden",
        width: 80,
        render: (v: boolean) => (v ? <Tag color="red">{t("yes")}</Tag> : "-"),
      },
      {
        title: t("createdAt"),
        key: DEFAULT_SORT_FIELD,
        dataIndex: DEFAULT_SORT_FIELD,
        width: 110,
        sorter: true,
        defaultSortOrder: DEFAULT_SORT_ORDER,
        render: (value: string) => {
          if (!value) return "-";
          const parsed = new Date(value);
          return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleDateString();
        },
      },
      {
        title: "",
        width: 160,
        key: "actions",
        fixed: "right" as const,
        render: (_: unknown, record: UserListItem) => {
          const protectedUser = isProtected(record);
          return (
            <Space
              size="small"
              {...hoverPrefetch(() =>
                prefetchUserDetail(
                  userRouteSlug(record.organizationId, orgSlugById, organizationName),
                  record.username,
                ),
              )}
            >
              <Tooltip title={t("edit")}>
                <Button
                  type="text"
                  size="small"
                  icon={<EditOutlined />}
                  onClick={() => {
                    const orgId = record.organizationId;
                    navigate(userDetailPath(orgId, record.username, orgSlugById, organizationName));
                  }}
                />
              </Tooltip>
              {/* Casdoor L204: impersonate 排除 self */}
              <PermissionGuard action="users:write" resource="users" fallback={null}>
                <Tooltip title={t("action.impersonate")}>
                  <Button
                    type="text"
                    size="small"
                    icon={<UserSwitchOutlined />}
                    disabled={record.username === account?.name}
                    loading={
                      impersonateMutation.isPending && impersonateMutation.variables === record.id
                    }
                    onClick={() => impersonateMutation.mutate(record.id)}
                  />
                </Tooltip>
              </PermissionGuard>
              {/* DELETE /api/v1/users/{id} 的声明权限是 users:delete，不是 users:write。 */}
              <PermissionGuard action="users:delete" resource="users" fallback={null}>
                <ConfirmButton
                  onConfirm={() =>
                    crud.deleteMutation.mutate({
                      id: record.id,
                      organizationId: record.organizationId,
                    })
                  }
                  disabled={protectedUser || crud.deleteMutation.isPending}
                  confirmLabel={
                    protectedUser
                      ? t("action.protectedUser", { defaultValue: "Protected user" })
                      : t("action.confirmDelete", { defaultValue: "Confirm delete" })
                  }
                >
                  {t("delete")}
                </ConfirmButton>
              </PermissionGuard>
            </Space>
          );
        },
      },
    ],
    [
      DEFAULT_SORT_FIELD,
      DEFAULT_SORT_ORDER,
      t,
      tr,
      ta,
      renderUserLink,
      orgNameMap,
      orgSlugById,
      appNameMap,
      organizationName,
      navigate,
      toggleAdminMutation,
      impersonateMutation,
      crud.deleteMutation,
      hoverPrefetch,
      optimisticUserStatus,
      prefetchUserDetail,
      account?.name,
      isProtected,
    ],
  );

  const config: CrudListViewConfig<UserListItem, UserListParams, UserDeleteKey> = {
    title: tr("user.title"),
    description: tr("user.title"),
    searchPlaceholder: t("placeholder.searchByName"),
    rowKey: "id",
    rowKeyToDeleteKey: (r) => ({ id: r.id, organizationId: r.organizationId }),
    tableId: "users-list",
    // D-04：tag / registerSource 是补全列，默认收进列菜单，避免列表变宽；
    // 用户可在列菜单中开启，选择会持久化。region 只在详情 profile 中，列表 DTO 未暴露。
    defaultHiddenColumns: ["tag", "registerSource"],
    defaultSortField: DEFAULT_SORT_FIELD,
    defaultSortOrder: DEFAULT_SORT_ORDER,
    searchableColumns: [
      { dataIndex: "username", label: t("name") },
      { dataIndex: "displayName", label: t("displayName") },
      { dataIndex: "email", label: ta("email") },
    ],
    filters: [
      {
        key: "statusFilter",
        label: t("statusLabel"),
        options: [
          { label: t("enabled"), value: "active" },
          { label: t("disabled"), value: "disabled" },
        ],
      },
      {
        key: "userTypeFilter",
        label: t("userType"),
        options: [
          { label: tr("user.userTypeNormal"), value: "normal" },
          { label: tr("user.userTypeManager"), value: "manager" },
          { label: tr("user.userTypeService"), value: "service" },
        ],
      },
    ],
    columns,

    enableBulkDelete: true,
    bulkDeletePermission: { permission: "users:delete", resource: "users" },
    rowSelectable: (record) => !isProtected(record),
    bulkActions: ({ selectedKeys }) =>
      renderUserBulkActions({
        enableLabel: t("action.enable"),
        disableLabel: t("action.disable"),
        resetLabel: t("action.resetFilters"),
        loading: bulkStatusMutation.isPending,
        onEnable: () => bulkStatusMutation.mutate({ status: "active", ids: selectedKeys }),
        onDisable: () => bulkStatusMutation.mutate({ status: "disabled", ids: selectedKeys }),
        onReset: handleReset,
      }),

    toolbar: (
      <Space>
        {account?.isAdmin ? (
          <OrganizationSelect
            value={orgFilter}
            onChange={handleOrgChange}
            style={{ width: 180 }}
            allowClear
          />
        ) : orgContext ? (
          // 非平台管理员从组织列表快捷入口进入时，也要让组织上下文可见、可清除。
          <Tag closable onClose={() => handleOrgChange("")} color="blue">
            {t("organization")}: {orgNameMap.get(orgContext) ?? orgContext}
          </Tag>
        ) : null}
        <PermissionGuard action="users:write" resource="users" fallback={null}>
          {/* 不要用 <Link><Button/></Link>：交互元素嵌套会让按钮冒泡给 <a>，
              且可访问角色退化为 button（链接语义丢失）。统一用 Button 导航。 */}
          <Button type="primary" onClick={() => navigate("/users/new")}>
            {tr("user.newUser")}
          </Button>
        </PermissionGuard>
        <PermissionGuard action="users:write" resource="users" fallback={null}>
          <Button icon={<UploadOutlined />} onClick={handleImport}>
            {t("action.import")}
          </Button>
        </PermissionGuard>
        <Button
          icon={<DownloadOutlined />}
          onClick={() => exportMutation.mutate(orgFilter || undefined)}
          loading={exportMutation.isPending}
        >
          {t("action.export")}
        </Button>
      </Space>
    ),
  };

  return renderCrudListView<UserListItem, UserListParams, UserDeleteKey>(config, crud);
}
