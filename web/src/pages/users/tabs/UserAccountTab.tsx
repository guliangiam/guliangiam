import { useMemo, useState } from "react";
import { Button, Descriptions, Popconfirm, Select, Space, Table, Tag, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import { PlusOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import { FormSection } from "@/shared/ui/FormSection";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import type { UserEffectivePermission } from "@/shared/api/iam/user";
import type { UserDetail } from "./types";

interface UserAccountTabProps {
  user: UserDetail | undefined;
  /** signupApplicationId 反查到的应用显示名（无则回退展示 ID）。 */
  signupApplicationName?: string;
  effectivePermissions: UserEffectivePermission[];
  onRemoveRole: (roleId: string) => void;
  onAddRole: () => void;
  onRemoveGroup: (groupId: string) => void;
  onAddGroup: () => void;
}

export default function UserAccountTab({
  user,
  signupApplicationName,
  effectivePermissions,
  onRemoveRole,
  onAddRole,
  onRemoveGroup,
  onAddGroup,
}: UserAccountTabProps) {
  const { t } = useTranslation();
  const [roleFilter, setRoleFilter] = useState<string | undefined>(undefined);

  const roleNames = useMemo(
    () => Array.from(new Set(effectivePermissions.map((item) => item.role))).sort(),
    [effectivePermissions],
  );

  const filteredPermissions = useMemo(
    () =>
      roleFilter
        ? effectivePermissions.filter((item) => item.role === roleFilter)
        : effectivePermissions,
    [effectivePermissions, roleFilter],
  );

  const permissionColumns: ColumnsType<UserEffectivePermission> = [
    {
      title: t("resource:user.permissionResource", "Resource"),
      dataIndex: "resource",
      key: "resource",
      render: (value: string) => <Typography.Text code>{value}</Typography.Text>,
    },
    {
      title: t("resource:user.permissionAction", "Action"),
      dataIndex: "action",
      key: "action",
      render: (value: string) => <Tag color={value === "*" ? "gold" : "blue"}>{value}</Tag>,
    },
    {
      title: t("resource:user.permissionSource", "Source role"),
      dataIndex: "role",
      key: "role",
      render: (value: string) => <Tag color="geekblue">{value}</Tag>,
    },
  ];

  return (
    <>
      <FormSection title={t("resource:user.roleSection", "Roles")}>
        <Space wrap style={{ marginBottom: 8 }}>
          {(user?.roleBindings ?? []).map((rb: Record<string, unknown>, i: number) => {
            const roleId = String(rb.roleId ?? rb.id ?? "unknown");
            const roleName = String(rb.roleName ?? rb.displayName ?? roleId);
            return (
              <PermissionGuard
                key={i}
                action="users:write"
                resource="users"
                fallback={<Tag color="blue">{roleName}</Tag>}
              >
                <Popconfirm
                  title={t("removeRoleConfirm")}
                  onConfirm={() => onRemoveRole(roleId)}
                  okText={t("common:delete")}
                  cancelText={t("common:cancel")}
                >
                  <Tag color="blue" closable>
                    {roleName}
                  </Tag>
                </Popconfirm>
              </PermissionGuard>
            );
          })}
          {(!user?.roleBindings || user.roleBindings.length === 0) && (
            <Typography.Text type="secondary">
              {t("resource:user.noRoles", "No roles assigned")}
            </Typography.Text>
          )}
          <PermissionGuard action="users:write" resource="users" fallback={null}>
            <Button size="small" type="dashed" icon={<PlusOutlined />} onClick={onAddRole}>
              {t("resource:user.addRole", "Add Role")}
            </Button>
          </PermissionGuard>
        </Space>
      </FormSection>

      <FormSection title={t("resource:user.groupSection", "Groups")}>
        <Space wrap style={{ marginBottom: 8 }}>
          {(user?.groupMemberships ?? []).map((gm: Record<string, unknown>, i: number) => {
            const groupId = String(gm.groupId ?? gm.id ?? "unknown");
            const groupName = String(gm.groupName ?? gm.displayName ?? groupId);
            return (
              <PermissionGuard
                key={i}
                action="users:write"
                resource="users"
                fallback={<Tag color="green">{groupName}</Tag>}
              >
                <Popconfirm
                  title={t("removeGroupConfirm")}
                  onConfirm={() => onRemoveGroup(groupId)}
                  okText={t("common:delete")}
                  cancelText={t("common:cancel")}
                >
                  <Tag color="green" closable>
                    {groupName}
                  </Tag>
                </Popconfirm>
              </PermissionGuard>
            );
          })}
          {(!user?.groupMemberships || user.groupMemberships.length === 0) && (
            <Typography.Text type="secondary">
              {t("resource:user.noGroups", "No groups assigned")}
            </Typography.Text>
          )}
          <PermissionGuard action="users:write" resource="users" fallback={null}>
            <Button size="small" type="dashed" icon={<PlusOutlined />} onClick={onAddGroup}>
              {t("resource:user.addGroup", "Add Group")}
            </Button>
          </PermissionGuard>
        </Space>
      </FormSection>

      <FormSection title={t("resource:user.permissions", "Permissions")}>
        <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
          {t(
            "resource:user.effectivePermissionsHint",
            "Grants expanded from the user's bound roles, including role inheritance.",
          )}
        </Typography.Paragraph>
        <Space style={{ marginBottom: 8 }}>
          <Select
            allowClear
            size="small"
            style={{ minWidth: 200 }}
            placeholder={t("resource:user.permissionSource", "Source role")}
            options={roleNames.map((name) => ({ label: name, value: name }))}
            value={roleFilter}
            onChange={setRoleFilter}
          />
        </Space>
        <Table
          size="small"
          rowKey={(record) => `${record.resource}|${record.action}|${record.role}`}
          columns={permissionColumns}
          dataSource={filteredPermissions}
          pagination={false}
          locale={{
            emptyText: t("resource:user.noPermissions", "No explicit permissions"),
          }}
        />
      </FormSection>

      {(user?.signupApplicationId || user?.registerType || user?.registerSource) && (
        <FormSection title={t("resource:user.signupInfoSection", "Signup Information")}>
          <Descriptions column={{ xs: 1, sm: 3 }} size="small" bordered>
            <Descriptions.Item label={t("resource:user.signupApplication")}>
              {signupApplicationName || user?.signupApplicationId || "-"}
            </Descriptions.Item>
            <Descriptions.Item label={t("resource:user.registerType")}>
              {user?.registerType || "-"}
            </Descriptions.Item>
            <Descriptions.Item label={t("resource:user.registerSource")}>
              {user?.registerSource || "-"}
            </Descriptions.Item>
          </Descriptions>
        </FormSection>
      )}
    </>
  );
}
