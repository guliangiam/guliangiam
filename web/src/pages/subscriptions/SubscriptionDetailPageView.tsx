import {
  Alert,
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  Popconfirm,
  Result,
  Select,
  Skeleton,
} from "antd";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { StateTag } from "@/shared/ui/StateTag";
import type { SubscriptionFormValues } from "./useSubscriptionDetailPageController";
import { useSubscriptionDetailPageController } from "./useSubscriptionDetailPageController";

type Props = { ctrl: ReturnType<typeof useSubscriptionDetailPageController> };

export function SubscriptionDetailPageView({ ctrl }: Props) {
  const {
    id,
    tc,
    tr,
    navigate,
    form,
    orgNameMap,
    detailQuery,
    updateMutation,
    deleteMutation,
    SUBSCRIPTION_STATES,
    SUBSCRIPTION_TRANSITIONS,
    allowedTargets,
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
          title={tr("subscription.notFound")}
          description={tr("subscription.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/subscriptions")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  const data = detailQuery.data;

  return (
    <ResourceFormPage
      title={tr("subscription.detailTitle", { name: data.displayName || data.name })}
      description={tr("subscription.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/subscriptions")}
      onFinish={async (values: SubscriptionFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <Popconfirm
          title={tc("deleteConfirm")}
          description={tr("subscription.deleteDesc")}
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
          <Descriptions.Item label={tc("statusLabel")}>
            <StateTag value={data.state} />
          </Descriptions.Item>
          <Descriptions.Item label={tc("user")}>{data.user || tc("none")}</Descriptions.Item>
          <Descriptions.Item label={tr("subscription.plan")}>
            {data.plan || tc("none")}
          </Descriptions.Item>
          <Descriptions.Item label={tr("subscription.pricing")}>
            {data.pricing || tc("none")}
          </Descriptions.Item>
          <Descriptions.Item label={tr("subscription.startTime")}>
            {data.startAt || tc("none")}
          </Descriptions.Item>
          <Descriptions.Item label={tr("subscription.endTime")}>
            {data.endAt || tc("none")}
          </Descriptions.Item>
          <Descriptions.Item label={tc("expireTime")} span={{ xs: 1, sm: 2 }}>
            {data.expireAt || tc("none")}
          </Descriptions.Item>
        </Descriptions>
      </FormSection>

      <FormSection title={tr("subscription.editSection", "Edit")}>
        <Form.Item
          label={tc("displayName")}
          name="displayName"
          rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
        >
          <Input />
        </Form.Item>
        <Form.Item label={tc("statusLabel")} name="state">
          <Select
            options={allowedTargets(SUBSCRIPTION_STATES, data.state, SUBSCRIPTION_TRANSITIONS, tc)}
          />
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}
