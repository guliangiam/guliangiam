import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  Popconfirm,
  Result,
  Select,
  Skeleton,
  Space,
  Switch,
  Tag,
} from "antd";
import { SectionNavShell } from "@/features/section-nav/SectionNavShell";
import type { SectionNavItem } from "@/features/section-nav/types";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { FormSection } from "@/shared/ui/FormSection";
import { SubTable } from "@/shared/ui/SubTable";
import { UserPaginateSelect } from "@/shared/ui/UserPaginateSelect";
import { GroupPaginateSelect } from "@/shared/ui/GroupPaginateSelect";
import { RolePaginateSelect } from "@/shared/ui/RolePaginateSelect";
import { CasbinPolicyEditor } from "@/shared/ui/CasbinPolicyEditor";
import { GlFormRow } from "@/shared/ui/GlForm";
import { useRoleDetailPageController, type RoleFormValues } from "./useRoleDetailPageController";
import type { Policy } from "@/shared/api/iam/enforcer";

export default function RoleDetailPage() {
  const {
    organizationName,
    roleName,
    matchedRole,
    tc,
    tr,
    navigate,
    form,
    advancedPolicies,
    setAdvancedPolicies,
    listQuery,
    detailQuery,
    policiesQuery,
    enforcerId,
    updateMutation,
    deleteMutation,
    savePoliciesMutation,
    users,
    groups,
    roleIds,
  } = useRoleDetailPageController();

  if (listQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }
  if (listQuery.isError) {
    return (
      <Result
        status="error"
        title={tc("loadFailed")}
        subTitle={String(listQuery.error)}
        extra={
          <Button type="primary" onClick={() => listQuery.refetch()}>
            {tc("retry")}
          </Button>
        }
      />
    );
  }
  if (!matchedRole) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Alert
          type="error"
          showIcon
          title={tr("role.roleNotFound")}
          description={tr("role.roleNotFoundDesc", { org: organizationName, role: roleName })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/roles")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  const configTab = (
    <>
      <FormSection title={tr("basicInfo")}>
        <Form.Item label={tc("name")}>
          <span>{matchedRole.name}</span>
        </Form.Item>
        <Form.Item label={tr("role.organization")}>
          <span>{matchedRole.owner}</span>
        </Form.Item>
        <Form.Item label={tc("statusLabel")}>
          {matchedRole.isEnabled ? (
            <Tag color="green">{tc("enabled")}</Tag>
          ) : (
            <Tag color="red">{tc("disabled")}</Tag>
          )}
        </Form.Item>
        <Form.Item
          name="displayName"
          rules={[{ required: true, message: tr("role.validation.displayNameRequired") }]}
        >
          <GlFormRow label={tc("displayName")} required>
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="description">
          <GlFormRow label={tc("description")}>
            <Input.TextArea rows={3} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="isEnabled" valuePropName="checked">
          <GlFormRow label={tc("enabled")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <FormSection title={tr("role.subResources")}>
        <Form.Item name="users">
          <GlFormRow label={tr("role.subUsers")}>
            <UserPaginateSelect placeholder={tr("role.searchUsers")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="groups">
          <GlFormRow label={tr("role.subGroups")}>
            <GroupPaginateSelect placeholder={tr("role.searchGroups")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="roleIds">
          <GlFormRow label={tr("role.subRoles")}>
            <RolePaginateSelect placeholder={tr("role.searchRoles")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="domains">
          <GlFormRow label={tr("role.subDomains")}>
            <Select mode="tags" placeholder={tr("role.domainPlaceholder")} />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      {users.length > 0 && (
        <FormSection title={tr("role.assignedUsers")}>
          <SubTable
            title={tr("role.assignedUsers")}
            columns={[{ title: tc("user", "User"), dataIndex: "value" }]}
            dataSource={users.map((u, i) => ({ key: i, value: u }))}
            onChange={() => {}}
            addable={false}
            deletable={false}
            editable={false}
          />
        </FormSection>
      )}
      {groups.length > 0 && (
        <FormSection title={tr("role.assignedGroups")}>
          <SubTable
            title={tr("role.assignedGroups")}
            columns={[{ title: tr("group.title"), dataIndex: "value" }]}
            dataSource={groups.map((g, i) => ({ key: i, value: g }))}
            onChange={() => {}}
            addable={false}
            deletable={false}
            editable={false}
          />
        </FormSection>
      )}
      {roleIds.length > 0 && (
        <FormSection title={tr("role.nestedRoles")}>
          <SubTable
            title={tr("role.nestedRoles")}
            columns={[{ title: tc("roleId", "Role ID"), dataIndex: "value" }]}
            dataSource={roleIds.map((r, i) => ({ key: i, value: r }))}
            onChange={() => {}}
            addable={false}
            deletable={false}
            editable={false}
          />
        </FormSection>
      )}
    </>
  );

  const policiesTab = enforcerId ? (
    <FormSection title={tr("role.policiesTab")}>
      {policiesQuery.isLoading ? (
        <Skeleton active paragraph={{ rows: 6 }} />
      ) : (
        <CasbinPolicyEditor
          policies={advancedPolicies}
          onAdd={(policy: Policy) => setAdvancedPolicies([...advancedPolicies, policy])}
          onRemove={(index: number) => {
            const next = [...advancedPolicies];
            next.splice(index, 1);
            setAdvancedPolicies(next);
          }}
          onUpdate={(index: number, policy: Policy) => {
            const next = [...advancedPolicies];
            next[index] = policy;
            setAdvancedPolicies(next);
          }}
        />
      )}
      <Space style={{ marginTop: 12 }}>
        <PermissionGuard action="role:write" resource="roles" fallback={null}>
          <Button
            type="primary"
            disabled={!enforcerId}
            loading={savePoliciesMutation.isPending}
            onClick={() => savePoliciesMutation.mutate()}
          >
            {tr("role.savePolicies")}
          </Button>
        </PermissionGuard>
      </Space>
    </FormSection>
  ) : (
    <Alert type="info" title={tr("role.noEnforcer")} description={tr("role.noEnforcerDesc")} />
  );

  const sectionItems: SectionNavItem[] = [
    { key: "config", anchor: "role-section-config", label: tr("role.configTab") },
    { key: "policies", anchor: "role-section-policies", label: tr("role.policiesTab") },
  ];

  return (
    <ResourceFormPage
      title={tr("role.detailTitle", { name: matchedRole.displayName || roleName })}
      description={tr("role.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/roles")}
      submitPermission={{ permission: "role:write", resource: "roles" }}
      onFinish={async (values: RoleFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <PermissionGuard action="role:write" resource="roles" fallback={null}>
          <Popconfirm
            title={tc("deleteConfirm")}
            description={tr("role.deleteDesc")}
            onConfirm={() => deleteMutation.mutate()}
            okText={tc("delete")}
            cancelText={tc("cancel")}
            okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
          >
            <Button danger>{tc("delete")}</Button>
          </Popconfirm>
        </PermissionGuard>
      }
      enableDraft
      draftRoute="role"
      draftEntityId={`${organizationName}/${roleName}`}
    >
      <SectionNavShell items={sectionItems}>
        <div className="gl-section-anchors">
          <div id="role-section-config" className="gl-section-anchor">
            {configTab}
          </div>
          <div id="role-section-policies" className="gl-section-anchor">
            {policiesTab}
          </div>
        </div>
      </SectionNavShell>
    </ResourceFormPage>
  );
}
