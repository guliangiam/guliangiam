import {
  Alert,
  Button,
  Card,
  Col,
  Form,
  Input,
  Popconfirm,
  Result,
  Row,
  Select,
  Skeleton,
  Space,
  Switch,
  Tag,
  Tooltip,
} from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import { SectionNavShell } from "@/features/section-nav/SectionNavShell";
import type { SectionNavItem } from "@/features/section-nav/types";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { FormSection } from "@/shared/ui/FormSection";
import { CasbinPolicyEditor } from "@/shared/ui/CasbinPolicyEditor";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { GlFormRow } from "@/shared/ui/GlForm";
import {
  useEnforcerDetailPageController,
  type EnforcerFormValues,
} from "./useEnforcerDetailPageController";

export default function EnforcerDetailPage() {
  const {
    id,
    tc,
    tr,
    navigate,
    form,
    detailQuery,
    policiesQuery,
    modelOptions,
    adapterOptions,
    updateMutation,
    deleteMutation,
    refreshMutation,
    addPolicyMutation,
    removePolicyMutation,
    updatePolicyMutation,
    handleAddPolicy,
    handleRemovePolicy,
    handleUpdatePolicy,
  } = useEnforcerDetailPageController();

  if (detailQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }
  if (detailQuery.isError) {
    return (
      <Result
        status="error"
        title={tc("loadFailed")}
        subTitle={String(detailQuery.error)}
        extra={
          <Button type="primary" onClick={() => detailQuery.refetch()}>
            {tc("retry")}
          </Button>
        }
      />
    );
  }
  if (!detailQuery.data) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Alert
          type="error"
          showIcon
          title={tr("enforcer.enforcerNotFound")}
          description={tr("enforcer.enforcerNotFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/enforcers")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  const configTab = (
    <FormSection title={tr("enforcer.configTab", "Configuration")}>
      <Form.Item label={tc("name")}>
        <span>{detailQuery.data.name}</span>
      </Form.Item>
      <Form.Item label={tr("enforcer.model", "Model")}>
        <span>{modelOptions.find((o) => o.value === detailQuery.data.modelId)?.label ?? "-"}</span>
      </Form.Item>
      <Form.Item label={tr("enforcer.adapter", "Adapter")}>
        <span>
          {adapterOptions.find((o) => o.value === detailQuery.data.adapterId)?.label ?? "-"}
        </span>
      </Form.Item>
      <Form.Item label={tc("statusLabel")}>
        {detailQuery.data.isEnabled ? (
          <Tag color="green">{tc("enabled")}</Tag>
        ) : (
          <Tag color="red">{tc("disabled")}</Tag>
        )}
      </Form.Item>
      <Form.Item name="owner">
        <GlFormRow label={tc("organization")}>
          <OrganizationSelect placeholder={tc("selectOrganization")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[{ required: true, message: tr("enforcer.validation.displayNameRequired") }]}
      >
        <GlFormRow label={tc("displayName")} required>
          <Input />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="description">
        <GlFormRow label={tc("description")}>
          <Input />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="isEnabled" valuePropName="checked">
        <GlFormRow label={tc("enabled")} inline>
          <Switch />
        </GlFormRow>
      </Form.Item>
    </FormSection>
  );

  const casbinSection = (
    <FormSection title={tr("enforcer.casbinSection", "Casbin Configuration")}>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="modelId">
            <GlFormRow label={tr("enforcer.model", "Model")}>
              <Select
                allowClear
                showSearch
                optionFilterProp="label"
                placeholder={tc("selectCasbinModel")}
                options={modelOptions}
              />
            </GlFormRow>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="adapterId">
            <GlFormRow label={tr("enforcer.adapter", "Adapter")}>
              <Select
                allowClear
                showSearch
                optionFilterProp="label"
                placeholder={tc("selectCasbinAdapter")}
                options={adapterOptions}
              />
            </GlFormRow>
          </Form.Item>
        </Col>
      </Row>
      <Space>
        <Tooltip
          title={tr("enforcer.hotRefreshTip", "Reload Casbin enforcer policies from database")}
        >
          <Button
            icon={<ReloadOutlined />}
            loading={refreshMutation.isPending}
            onClick={() => refreshMutation.mutate()}
          >
            {tr("enforcer.hotRefresh", "Hot Refresh")}
          </Button>
        </Tooltip>
      </Space>
    </FormSection>
  );

  const policiesTab = (
    <FormSection
      title={tr("enforcer.policyRules", "Policies")}
      description={tr("enforcer.policyRulesDesc", "Edit Casbin policy rules for this enforcer")}
      extra={
        <Button
          icon={<ReloadOutlined />}
          loading={refreshMutation.isPending}
          onClick={() => refreshMutation.mutate()}
        >
          {tr("enforcer.hotRefresh", "Hot Refresh")}
        </Button>
      }
    >
      {policiesQuery.isLoading ? (
        <Skeleton active paragraph={{ rows: 6 }} />
      ) : (
        <CasbinPolicyEditor
          policies={policiesQuery.data ?? []}
          onAdd={handleAddPolicy}
          onRemove={handleRemovePolicy}
          onUpdate={handleUpdatePolicy}
          loading={
            addPolicyMutation.isPending ||
            removePolicyMutation.isPending ||
            updatePolicyMutation.isPending
          }
        />
      )}
    </FormSection>
  );

  const sectionItems: SectionNavItem[] = [
    {
      key: "config",
      anchor: "enforcer-section-config",
      label: tr("enforcer.configTab", "Configuration"),
    },
    {
      key: "policies",
      anchor: "enforcer-section-policies",
      label: tr("enforcer.policyRules", "Policies"),
    },
  ];

  return (
    <ResourceFormPage
      title={tr("enforcer.detailTitle", {
        name: detailQuery.data.displayName || detailQuery.data.name,
      })}
      description={tr("enforcer.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      submitPermission={{ permission: "enforcer:write", resource: "enforcers" }}
      onCancel={() => navigate("/enforcers")}
      onFinish={async (values: EnforcerFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <PermissionGuard action="enforcer:delete" resource="enforcers" fallback={null}>
          <Popconfirm
            title={tc("deleteConfirm")}
            description={tr("enforcer.deleteDesc")}
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
      <SectionNavShell items={sectionItems}>
        <div className="gl-section-anchors">
          <div id="enforcer-section-config" className="gl-section-anchor">
            {configTab}
            {casbinSection}
          </div>
          <div id="enforcer-section-policies" className="gl-section-anchor">
            {policiesTab}
          </div>
        </div>
      </SectionNavShell>
    </ResourceFormPage>
  );
}
