import {
  Alert,
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Result,
  Skeleton,
  Switch,
  Tag,
} from "antd";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import type { AgentFormValues } from "./useAgentDetailPageController";
import { useAgentDetailPageController } from "./useAgentDetailPageController";

type Props = { ctrl: ReturnType<typeof useAgentDetailPageController> };

export function AgentDetailPageView({ ctrl }: Props) {
  const { id, tc, tr, navigate, form, orgNameMap, detailQuery, updateMutation, deleteMutation } =
    ctrl;

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
          title={tr("agent.notFound")}
          description={tr("agent.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/ai/agents")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  const data = detailQuery.data;

  return (
    <ResourceFormPage
      title={tr("agent.detailTitle", { name: data.displayName || data.name })}
      description={tr("agent.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/ai/agents")}
      onFinish={async (values: AgentFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <Popconfirm
          title={tc("deleteConfirm")}
          description={tr("agent.deleteDesc")}
          onConfirm={() => deleteMutation.mutate()}
          okText={tc("delete")}
          cancelText={tc("cancel")}
          okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
        >
          <Button danger>{tc("delete")}</Button>
        </Popconfirm>
      }
    >
      <FormSection title={tc("overview", "Overview")}>
        <Descriptions bordered column={{ xs: 1, sm: 2 }} size="small">
          <Descriptions.Item label={tc("name")}>{data.name}</Descriptions.Item>
          <Descriptions.Item label={tc("organization")}>
            {orgNameMap.get(data.organizationId) || data.organizationId}
          </Descriptions.Item>
          <Descriptions.Item label={tr("agent.type")}>{data.type}</Descriptions.Item>
          <Descriptions.Item label={tr("agent.application")}>
            {data.applicationId || tc("notSet", "—")}
          </Descriptions.Item>
          <Descriptions.Item label={tr("agent.url")} span={{ xs: 1, sm: 2 }}>
            {data.url || tc("notSet", "—")}
          </Descriptions.Item>
          <Descriptions.Item label={tc("statusLabel")}>
            {data.isEnabled ? (
              <Tag color="green">{tc("enabled")}</Tag>
            ) : (
              <Tag color="red">{tc("disabled")}</Tag>
            )}
          </Descriptions.Item>
          <Descriptions.Item label={tr("agent.lastPing")}>
            {data.lastPingAt || tc("never", "—")}
          </Descriptions.Item>
        </Descriptions>
      </FormSection>

      <FormSection title={tr("agent.editSection", "Edit")}>
        <Form.Item
          label={tc("displayName")}
          name="displayName"
          rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
        >
          <Input />
        </Form.Item>
        <Form.Item label={tc("host")} name="host">
          <Input placeholder="agent-host.example.com" />
        </Form.Item>
        <Form.Item label={tc("port")} name="port">
          <InputNumber min={1} max={65535} style={{ width: "100%" }} placeholder="443" />
        </Form.Item>
        <Form.Item label={tc("enabled")} name="isEnabled" valuePropName="checked">
          <Switch />
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}
