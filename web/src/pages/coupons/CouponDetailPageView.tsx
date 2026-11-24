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
  Select,
  Skeleton,
} from "antd";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { StateTag } from "@/shared/ui/StateTag";
import type { CouponFormValues } from "./useCouponDetailPageController";
import { useCouponDetailPageController } from "./useCouponDetailPageController";

type Props = { ctrl: ReturnType<typeof useCouponDetailPageController> };

export function CouponDetailPageView({ ctrl }: Props) {
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
    validateMutation,
    CURRENCY_OPTIONS,
    COUPON_STATES,
    COUPON_TRANSITIONS,
    allowedTargets,
  } = ctrl;

  if (detailQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 4 }} />
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
          title={tr("coupon.notFound")}
          description={tr("coupon.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/coupons")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  const data = detailQuery.data;

  return (
    <ResourceFormPage
      title={tr("coupon.detailTitle", { name: data.displayName || data.name })}
      description={tr("coupon.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/coupons")}
      onFinish={async (values: CouponFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <>
          <Button loading={validateMutation.isPending} onClick={() => validateMutation.mutate()}>
            {tr("coupon.validate")}
          </Button>
          <Popconfirm
            title={tc("deleteConfirm")}
            description={tr("coupon.deleteDesc")}
            onConfirm={() => deleteMutation.mutate()}
            okText={tc("delete")}
            cancelText={tc("cancel")}
            okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
          >
            <Button danger>{tc("delete")}</Button>
          </Popconfirm>
        </>
      }
    >
      <FormSection title={tc("overview", "Overview")}>
        <Descriptions bordered column={{ xs: 1, sm: 2 }} size="small">
          <Descriptions.Item label={tc("name")}>{data.name}</Descriptions.Item>
          <Descriptions.Item label={tc("organization")}>
            {orgNameMap.get(data.organizationId) || data.organizationId}
          </Descriptions.Item>
          <Descriptions.Item label={tr("coupon.redeemCode")}>{data.code}</Descriptions.Item>
          <Descriptions.Item label={tr("coupon.discountType")}>
            {data.discountType === "percentage"
              ? tr("coupon.percentage")
              : data.discountType === "fixed"
                ? tr("coupon.fixedAmount")
                : data.discountType}
          </Descriptions.Item>
          <Descriptions.Item label={tc("statusLabel")}>
            <StateTag value={data.state} />
          </Descriptions.Item>
          <Descriptions.Item label={tr("coupon.usedCount")}>
            {data.usedCount ?? 0}
          </Descriptions.Item>
          <Descriptions.Item label={tr("coupon.applicableProducts")} span={{ xs: 1, sm: 2 }}>
            {data.products?.join(", ") || tc("none")}
          </Descriptions.Item>
          <Descriptions.Item label={tr("coupon.applicableUsers")} span={{ xs: 1, sm: 2 }}>
            {data.users?.join(", ") || tc("none")}
          </Descriptions.Item>
          <Descriptions.Item label={tr("coupon.startTime")} span={1}>
            {data.startTime || tc("none")}
          </Descriptions.Item>
          <Descriptions.Item label={tr("expireTime")} span={1}>
            {data.expireTime || tc("none")}
          </Descriptions.Item>
        </Descriptions>
      </FormSection>

      <FormSection title={tr("coupon.editSection", "Edit")}>
        <Form.Item
          label={tc("displayName")}
          name="displayName"
          rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
        >
          <Input />
        </Form.Item>
        <Form.Item label={tc("statusLabel")} name="state">
          <Select options={allowedTargets(COUPON_STATES, data.state, COUPON_TRANSITIONS, tc)} />
        </Form.Item>
        <Form.Item label={tc("scope")} name="scope">
          <Select
            options={[
              { label: tc("scopeGlobal"), value: "global" },
              { label: tc("scopeOrganization"), value: "organization" },
              { label: tc("scopeProduct"), value: "product" },
            ]}
          />
        </Form.Item>
        <Form.Item label={tr("coupon.discountValue")} name="discount">
          <InputNumber min={0} style={{ width: "100%" }} />
        </Form.Item>
        <Form.Item label={tr("coupon.maxDiscount")} name="maxDiscount">
          <InputNumber min={0} style={{ width: "100%" }} />
        </Form.Item>
        <Form.Item label={tr("coupon.minOrderAmount")} name="minOrderAmount">
          <InputNumber min={0} style={{ width: "100%" }} />
        </Form.Item>
        <Form.Item label={tr("coupon.currency")} name="currency">
          <Select showSearch options={CURRENCY_OPTIONS} />
        </Form.Item>
        <Form.Item label={tr("coupon.quantity")} name="quantity">
          <InputNumber min={0} style={{ width: "100%" }} />
        </Form.Item>
        <Form.Item label={tr("coupon.maxUsagePerUser")} name="maxUsagePerUser">
          <InputNumber min={0} style={{ width: "100%" }} />
        </Form.Item>
        <Form.Item label={tc("description")} name="description">
          <Input.TextArea rows={3} />
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}
