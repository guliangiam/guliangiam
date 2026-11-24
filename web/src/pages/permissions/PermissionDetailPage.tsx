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
  Tabs,
  Tag,
} from "antd";
import { useNavigate } from "react-router-dom";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { FormSection } from "@/shared/ui/FormSection";
import { CasbinPolicyEditor } from "@/shared/ui/CasbinPolicyEditor";
import { HelpLabel } from "@/shared/ui/FieldHelp";
import { DATABASE_ADAPTER_OPTIONS } from "@/shared/constants/enums";
import { UserPaginateSelect } from "@/shared/ui/UserPaginateSelect";
import { GroupPaginateSelect } from "@/shared/ui/GroupPaginateSelect";
import { RolePaginateSelect } from "@/shared/ui/RolePaginateSelect";
import { GlFormRow } from "@/shared/ui/GlForm";
import {
  usePermissionDetailPageController,
  RESOURCE_TYPE_OPTIONS,
  EFFECT_OPTIONS,
  STATE_OPTIONS,
} from "./usePermissionDetailPageController";

export default function PermissionDetailPage() {
  const navigate = useNavigate();
  const {
    organizationName,
    permissionName,
    tc,
    tr,
    form,
    activeTab,
    setActiveTab,
    listQuery,
    matchedPermission,
    detailQuery,
    appOptionsQuery,
    policiesQuery,
    updateMutation,
    deleteMutation,
    savePoliciesMutation,
    advancedPolicies,
    isApi,
    actionOptions,
    addPolicy,
    removePolicy,
    updatePolicy,
  } = usePermissionDetailPageController();

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

  if (!matchedPermission) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Alert
          type="error"
          showIcon
          title={tc("permissionNotFound")}
          description={tc("permissionNotFoundDesc", {
            name: permissionName,
            org: organizationName,
          })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/permissions")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <ResourceFormPage
      title={tr("permission.detailTitle", {
        name: matchedPermission.displayName || permissionName,
      })}
      description={tr("permission.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/permissions")}
      submitPermission={{ permission: "permission:write", resource: "permissions" }}
      onFinish={async (values) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <PermissionGuard action="permission:write" resource="permissions" fallback={null}>
          <Popconfirm
            title={tc("deleteConfirm")}
            description={tr("permission.deleteDesc")}
            onConfirm={() => deleteMutation.mutate()}
            okText={tc("delete")}
            cancelText={tc("cancel")}
            okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
          >
            <Button danger>{tc("delete")}</Button>
          </Popconfirm>
        </PermissionGuard>
      }
    >
      <FormSection title={tr("basicInfo")}>
        <Form.Item label={tc("name")}>
          <span>{matchedPermission.name}</span>
        </Form.Item>
        <Form.Item label={tc("permission.organization")}>
          <span>{matchedPermission.owner}</span>
        </Form.Item>
        <Form.Item label={tc("statusLabel")}>
          {matchedPermission.isEnabled ? (
            <Tag color="green">{tc("enabled")}</Tag>
          ) : (
            <Tag color="red">{tc("disabled")}</Tag>
          )}
        </Form.Item>
        <Form.Item name="displayName" rules={[{ required: true, message: tc("required") }]}>
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

      <FormSection title={tr("permission.casbinConfig")}>
        <Form.Item name="model">
          <GlFormRow label={tr("permission.model")}>
            <Select
              allowClear
              showSearch
              placeholder={tr("permission.selectModel")}
              options={[
                { label: "ACL", value: "ACL" },
                { label: "ACL with domains", value: "ACL-with-domains" },
                { label: "RBAC", value: "RBAC" },
                { label: "RBAC with domains", value: "RBAC-with-domains" },
                { label: "RBAC with resource roles", value: "RBAC-with-resource-roles" },
                { label: "ABAC", value: "ABAC" },
              ]}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="adapter">
          <GlFormRow label={tr("permission.adapter")}>
            <Select
              allowClear
              showSearch
              placeholder={tr("permission.selectAdapter")}
              options={DATABASE_ADAPTER_OPTIONS}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="application">
          <GlFormRow label={tr("permission.application")}>
            <Select
              allowClear
              showSearch
              placeholder={tr("permission.selectApplication")}
              options={appOptionsQuery.data ?? []}
              loading={appOptionsQuery.isLoading}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="sorter">
          <GlFormRow label={tr("permission.sorter")}>
            <Input />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <FormSection title={tr("permission.policyRules")}>
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          size="small"
          items={[
            {
              key: "structured",
              label: (
                <HelpLabel helpKey="tabs.permissions.structured">
                  {tr("permission.structuredEditor")}
                </HelpLabel>
              ),
              children: (
                <>
                  <Form.Item name="resourceType">
                    <GlFormRow label={tr("permission.resourceType")}>
                      <Select
                        showSearch
                        allowClear
                        options={RESOURCE_TYPE_OPTIONS}
                        placeholder={tr("permission.selectResourceType")}
                      />
                    </GlFormRow>
                  </Form.Item>
                  <Form.Item name="resources">
                    <GlFormRow label={tr("permission.resources")}>
                      <Select
                        mode={isApi ? "multiple" : "tags"}
                        allowClear
                        placeholder={tr("permission.resourcesPlaceholder")}
                        tokenSeparators={[","]}
                      />
                    </GlFormRow>
                  </Form.Item>
                  <Form.Item name="actions">
                    <GlFormRow label={tr("permission.actions")}>
                      <Select
                        mode="multiple"
                        allowClear
                        placeholder={tr("permission.selectActions")}
                        options={actionOptions}
                      />
                    </GlFormRow>
                  </Form.Item>
                  <Form.Item name="effect">
                    <GlFormRow label={tr("permission.effect")}>
                      <Select options={EFFECT_OPTIONS} />
                    </GlFormRow>
                  </Form.Item>
                </>
              ),
            },
            {
              key: "advanced",
              label: (
                <HelpLabel helpKey="tabs.permissions.advanced">
                  {tr("permission.advancedEditor")}
                </HelpLabel>
              ),
              children: (
                <>
                  {policiesQuery.isLoading ? (
                    <Skeleton active paragraph={{ rows: 6 }} />
                  ) : (
                    <CasbinPolicyEditor
                      policies={advancedPolicies}
                      onAdd={addPolicy}
                      onRemove={removePolicy}
                      onUpdate={updatePolicy}
                    />
                  )}
                  <Space style={{ marginTop: 12 }}>
                    <Button
                      type="primary"
                      loading={savePoliciesMutation.isPending}
                      onClick={() => savePoliciesMutation.mutate()}
                    >
                      {tr("permission.savePolicies")}
                    </Button>
                  </Space>
                </>
              ),
            },
          ]}
        />
      </FormSection>

      <FormSection title={tr("permission.subResources")}>
        <Form.Item name="users">
          <GlFormRow label={tr("permission.subUsers")}>
            <UserPaginateSelect
              placeholder={tr("permission.searchUsers")}
              allOptionValue="*"
              allOptionLabel={tr("permission.allUsers")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="groups">
          <GlFormRow label={tr("permission.subGroups")}>
            <GroupPaginateSelect
              placeholder={tr("permission.searchGroups")}
              allOptionValue="*"
              allOptionLabel={tr("permission.allGroups")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="roles">
          <GlFormRow label={tr("permission.subRoles")}>
            <RolePaginateSelect placeholder={tr("permission.searchRoles")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="domains">
          <GlFormRow label={tr("permission.subDomains")}>
            <Select mode="tags" placeholder={tr("permission.domainPlaceholder")} />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <FormSection title={tr("permission.approval")}>
        <Form.Item name="state">
          <GlFormRow label={tr("permission.state")}>
            <Select options={STATE_OPTIONS} />
          </GlFormRow>
        </Form.Item>
        {detailQuery.data && (
          <>
            <Form.Item label={tr("permission.submitter")}>
              <span>{detailQuery.data.submitter || "-"}</span>
            </Form.Item>
            <Form.Item label={tr("permission.approver")}>
              <span>{detailQuery.data.approver || "-"}</span>
            </Form.Item>
            <Form.Item label={tr("permission.approveTime")}>
              <span>{detailQuery.data.approveTime || "-"}</span>
            </Form.Item>
          </>
        )}
      </FormSection>
    </ResourceFormPage>
  );
}
