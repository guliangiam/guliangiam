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
  Tag,
  Timeline,
  Typography,
} from "antd";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { StateTag } from "@/shared/ui/StateTag";
import { stateCaptionKey } from "@/shared/constants/enums";
import type { OrderFormValues } from "./useOrderDetailPageController";
import { useOrderDetailPageController } from "./useOrderDetailPageController";

type Props = { ctrl: ReturnType<typeof useOrderDetailPageController> };

export function OrderDetailPageView({ ctrl }: Props) {
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
    cancelMutation,
    payMutation,
    ORDER_STATES,
    ORDER_TRANSITIONS,
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
          title={tr("order.notFound")}
          description={tr("order.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/orders")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  const data = detailQuery.data;

  // 服务端枚举为小写（ORDER_STATES）。可支付=等待支付(created/payment_pending)+failed 重试；
  // 可取消=尚未收款且存在到 cancelled 的合法迁移（created/payment_pending）。
  const payable =
    data.state === "created" || data.state === "payment_pending" || data.state === "failed";
  const cancellable = data.state === "created" || data.state === "payment_pending";

  const flowSteps = ["created", "payment_pending", "paid", "completed"] as const;
  const flowIndex = flowSteps.indexOf(data.state as (typeof flowSteps)[number]);
  const settled = flowIndex >= 2 || data.state === "refunded" || data.state === "closed";
  const stepStatus = (step: (typeof flowSteps)[number]) => {
    if (data.state === step) return "current";
    if (step === "created") return "done";
    if (step === "payment_pending") return settled ? "done" : "todo";
    if (step === "paid")
      return flowIndex >= 3 || data.state === "refunded" || data.state === "closed"
        ? "done"
        : "todo";
    return data.state === "refunded" || data.state === "closed" ? "done" : "todo";
  };
  const currentTag = (
    <Tag color="blue" style={{ marginLeft: 8 }}>
      {tc("current")}
    </Tag>
  );

  return (
    <ResourceFormPage
      title={tr("order.detailTitle", { name: data.displayName || data.name })}
      description={tr("order.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/orders")}
      onFinish={async (values: OrderFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      submitPermission={{ permission: "order:write", resource: "orders" }}
      extra={
        <>
          {payable && (
            <PermissionGuard action="order:write" resource="orders" fallback={null}>
              <Popconfirm
                title={tr("order.confirmPay")}
                onConfirm={() => payMutation.mutate()}
                okText={tc("confirm")}
                cancelText={tc("cancel")}
              >
                <Button loading={payMutation.isPending}>{tr("order.pay.payNow")}</Button>
              </Popconfirm>
            </PermissionGuard>
          )}
          {cancellable && (
            <PermissionGuard action="order:write" resource="orders" fallback={null}>
              <Popconfirm
                title={tr("order.confirmCancel")}
                onConfirm={() => cancelMutation.mutate()}
                okText={tc("confirm")}
                cancelText={tc("cancel")}
              >
                <Button danger loading={cancelMutation.isPending}>
                  {tr("order.cancel")}
                </Button>
              </Popconfirm>
            </PermissionGuard>
          )}
          <PermissionGuard action="order:delete" resource="orders" fallback={null}>
            <Popconfirm
              title={tc("deleteConfirm")}
              description={tr("order.deleteDesc")}
              onConfirm={() => deleteMutation.mutate()}
              okText={tc("delete")}
              cancelText={tc("cancel")}
              okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
            >
              <Button danger>{tc("delete")}</Button>
            </Popconfirm>
          </PermissionGuard>
        </>
      }
    >
      <FormSection title={tc("overview", "Overview")}>
        <Descriptions bordered column={{ xs: 1, sm: 2 }} size="small">
          <Descriptions.Item label={tc("name")}>{data.name}</Descriptions.Item>
          <Descriptions.Item label={tc("organization")}>
            {orgNameMap.get(data.organizationId) || data.organizationId}
          </Descriptions.Item>
          <Descriptions.Item label={tc("type")}>{data.type}</Descriptions.Item>
          <Descriptions.Item label={tr("order.currentStatus")}>
            <StateTag value={data.state} />
          </Descriptions.Item>
          <Descriptions.Item label={tc("user")}>{data.user || tc("none")}</Descriptions.Item>
          <Descriptions.Item label={tc("product")}>
            {data.productName || data.product || tc("none")}
          </Descriptions.Item>
          <Descriptions.Item label={tc("paymentMethod")}>
            {data.providerName || data.provider || tc("none")}
          </Descriptions.Item>
          <Descriptions.Item label={tc("price")}>
            {data.price} {data.currency}
          </Descriptions.Item>
          {data.paymentUrl && (
            <Descriptions.Item label={tr("order.paymentUrl")} span={{ xs: 1, sm: 2 }}>
              <Typography.Link href={data.paymentUrl} target="_blank" rel="noopener noreferrer">
                {data.paymentUrl}
              </Typography.Link>
            </Descriptions.Item>
          )}
          {data.tradeNo && (
            <Descriptions.Item label={tr("order.tradeNo")} span={{ xs: 1, sm: 2 }}>
              {data.tradeNo}
            </Descriptions.Item>
          )}
        </Descriptions>
      </FormSection>

      <FormSection title={tc("editSection", "Edit")}>
        <Form.Item
          label={tc("displayName")}
          name="displayName"
          rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
        >
          <Input />
        </Form.Item>
        <Form.Item label={tc("statusLabel")} name="state">
          <Select options={allowedTargets(ORDER_STATES, data.state, ORDER_TRANSITIONS, tc)} />
        </Form.Item>
        <Form.Item label={tr("order.relatedProducts")}>
          <span>{data.products?.join(", ") || tc("none")}</span>
        </Form.Item>
        <Form.Item label={tc("payUrl")}>
          <span>{data.payUrl || tc("none")}</span>
        </Form.Item>
      </FormSection>

      <FormSection title={tr("order.statusTimeline")}>
        <Timeline
          items={[
            ...flowSteps.map((step) => {
              const status = stepStatus(step);
              return {
                color: status === "current" ? "blue" : status === "done" ? "green" : "gray",
                children: (
                  <>
                    <strong>{tc(stateCaptionKey(step), { defaultValue: step })}</strong>
                    {status === "current" && currentTag}
                    {step === "created" && (
                      <>
                        <br />
                        <span style={{ fontSize: 12, color: "var(--ant-color-text-secondary)" }}>
                          {data.createdAt}
                        </span>
                      </>
                    )}
                  </>
                ),
              };
            }),
            ...(["failed", "cancelled", "refunded", "closed"].includes(data.state)
              ? [
                  {
                    color:
                      data.state === "refunded"
                        ? "purple"
                        : data.state === "closed"
                          ? "gray"
                          : "red",
                    children: (
                      <>
                        <strong>
                          {tc(stateCaptionKey(data.state), { defaultValue: data.state })}
                        </strong>
                        <Tag
                          color={
                            data.state === "refunded"
                              ? "purple"
                              : data.state === "closed"
                                ? "default"
                                : "red"
                          }
                          style={{ marginLeft: 8 }}
                        >
                          {tc("current")}
                        </Tag>
                      </>
                    ),
                  },
                ]
              : []),
          ]}
        />
      </FormSection>
    </ResourceFormPage>
  );
}
