import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Result,
  Row,
  Select,
  Skeleton,
  Switch,
  Tag,
} from "antd";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import type { AdapterDetail, AdapterFormValues } from "./useAdapterDetailPageController";
import { useAdapterDetailPageController } from "./useAdapterDetailPageController";

type Props = { ctrl: ReturnType<typeof useAdapterDetailPageController> };

export function AdapterDetailPageView({ ctrl }: Props) {
  const {
    id,
    tc,
    tr,
    navigate,
    form,
    useSameDb,
    detailQuery,
    updateMutation,
    deleteMutation,
    DATABASE_TYPE_OPTIONS,
  } = ctrl;

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
          title={tr("adapter.notFound")}
          description={tr("adapter.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/adapters")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  const data = detailQuery.data as AdapterDetail;

  return (
    <ResourceFormPage
      title={tr("adapter.detailTitle", { name: data.displayName || data.name })}
      description={tr("adapter.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      submitPermission={{ permission: "adapter:write", resource: "adapters" }}
      onCancel={() => navigate("/adapters")}
      onFinish={async (values: AdapterFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <PermissionGuard action="adapter:delete" fallback={null}>
          <Popconfirm
            title={tc("deleteConfirm")}
            description={tr("adapter.deleteDesc")}
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
      <FormSection title={tc("overview", "Overview")}>
        <Descriptions bordered column={{ xs: 1, sm: 2, lg: 3 }} size="small">
          <Descriptions.Item label={tc("name")}>{data.name}</Descriptions.Item>
          <Descriptions.Item label={tc("type")}>{data.type}</Descriptions.Item>
          <Descriptions.Item label={tc("statusLabel")}>
            {data.isEnabled ? (
              <Tag color="green">{tc("enabled")}</Tag>
            ) : (
              <Tag color="red">{tc("disabled")}</Tag>
            )}
          </Descriptions.Item>
        </Descriptions>
      </FormSection>

      <FormSection title={tc("adapterBasicInfo")}>
        <Form.Item label={tc("organization")} name="owner">
          <OrganizationSelect placeholder={tc("selectOrganization")} />
        </Form.Item>
        <Form.Item
          label={tc("displayName")}
          name="displayName"
          rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
        >
          <Input />
        </Form.Item>
        <Form.Item label={tc("adapterTable")} name="tableName">
          <Input placeholder="e.g. casbin_rule" />
        </Form.Item>
        <Form.Item label={tc("enabled")} name="isEnabled" valuePropName="checked">
          <Switch />
        </Form.Item>
      </FormSection>

      <FormSection title={tc("adapterDbConnection")}>
        <Form.Item label={tc("adapterUseSameDb")} name="useSameDb" valuePropName="checked">
          <Switch />
        </Form.Item>
        {!useSameDb && (
          <>
            <Form.Item label={tc("adapterDatabaseType")} name="databaseType">
              <Select options={DATABASE_TYPE_OPTIONS} placeholder={tc("selectDatabaseType")} />
            </Form.Item>
            <Row gutter={16}>
              <Col span={16}>
                <Form.Item label={tc("adapterHost")} name="host">
                  <Input placeholder="e.g. localhost" />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item label={tc("adapterPort")} name="port">
                  <InputNumber min={1} max={65535} style={{ width: "100%" }} placeholder="5432" />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label={tc("adapterUser")} name="dbUser">
                  <Input />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label={tc("adapterPassword")}
                  name="dbPassword"
                  extra={tc("leaveEmptyToKeepCurrent")}
                >
                  <Input.Password autoComplete="new-password" />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item label={tc("adapterDatabase")} name="databaseName">
              <Input placeholder="e.g. casbin" />
            </Form.Item>
          </>
        )}
      </FormSection>

      <FormSection title={tr("adapter.policy")}>
        <Form.Item label={tr("adapter.policy")} name="policy" extra={tc("policyHint")}>
          <Input.TextArea
            rows={8}
            style={{ fontFamily: "monospace", fontSize: 12 }}
            placeholder="p, alice, data1, read&#10;p, bob, data2, write"
          />
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}
