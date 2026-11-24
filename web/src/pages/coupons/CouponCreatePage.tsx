import { App, Form, Input, InputNumber, Select } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamCoupon } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { localizedError } from "@/shared/lib/server-error";
import { COUPON_STATES, stateOptions } from "@/shared/constants/enums";

type CouponFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  code: string;
  discountType?: string;
  discount?: number;
  maxDiscount?: number;
  scope?: string;
  quantity?: number;
  maxUsagePerUser?: number;
  currency?: string;
  state?: string;
};

export default function CouponCreatePage() {
  const { t: tr } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<CouponFormValues>();
  const discountType = Form.useWatch("discountType", form);

  const mutation = useMutation({
    mutationFn: async (values: CouponFormValues) => {
      return createIamCoupon({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        code: values.code,
        discountType: values.discountType,
        discount: values.discount,
        maxDiscount: values.maxDiscount,
        scope: values.scope,
        quantity: values.quantity,
        maxUsagePerUser: values.maxUsagePerUser,
        currency: values.currency,
        state: values.state,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.coupons.all });
      message.success(tr("coupon.created", "Coupon created"));
      navigate("/coupons");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "coupon.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("coupon.createTitle", "New Coupon")}
      description={tr("coupon.createDesc", "Create a new coupon, define discount type and scope.")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/coupons")}
      onFinish={async (values: CouponFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[
          { required: true, message: tr("coupon.orgIdRequired", "Please enter organization ID") },
        ]}
      >
        <GlFormRow label={tr("coupon.orgId", "Organization ID")} required>
          <OrganizationSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="name"
        rules={[{ required: true, message: tr("coupon.nameRequired", "Please enter coupon name") }]}
      >
        <GlFormRow label={tr("coupon.name", "Name")} required>
          <Input placeholder={tr("coupon.namePlaceholder", "e.g. coupon-1")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[
          {
            required: true,
            message: tr("coupon.displayNameRequired", "Please enter display name"),
          },
        ]}
      >
        <GlFormRow label={tr("coupon.displayName", "Display Name")} required>
          <Input placeholder={tr("coupon.displayNamePlaceholder", "e.g. New Year Coupon")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="code"
        rules={[
          { required: true, message: tr("coupon.redeemCodeRequired", "Please enter redeem code") },
        ]}
      >
        <GlFormRow label={tr("coupon.redeemCode", "Redeem Code")} required>
          <Input placeholder={tr("coupon.codePlaceholder", "e.g. NEWYEAR2024")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="discountType">
        <GlFormRow label={tr("coupon.discountType", "Discount Type")}>
          <Select
            placeholder={tr("coupon.discountTypePlaceholder", "Select discount type")}
            allowClear
          >
            <Select.Option value="percentage">{tr("coupon.percent", "Percentage")}</Select.Option>
            <Select.Option value="fixed">{tr("coupon.fixed", "Fixed Amount")}</Select.Option>
          </Select>
        </GlFormRow>
      </Form.Item>
      <Form.Item name="discount">
        <GlFormRow label={tr("coupon.discountValue", "Discount Value")}>
          <InputNumber
            min={0}
            max={discountType === "percentage" ? 100 : undefined}
            style={{ width: "100%" }}
            placeholder="0"
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="maxDiscount">
        <GlFormRow label={tr("coupon.maxDiscount", "Max Discount")}>
          <InputNumber min={0} style={{ width: "100%" }} placeholder="0" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="scope">
        <GlFormRow label={tr("coupon.scope", "Scope")}>
          <Select
            allowClear
            placeholder={tr("coupon.scopePlaceholder", "Scope")}
            options={[
              { label: tc("scopeGlobal"), value: "global" },
              { label: tc("scopeOrganization"), value: "organization" },
              { label: tc("scopeProduct"), value: "product" },
            ]}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="quantity">
        <GlFormRow label={tr("coupon.quantity", "Quantity")}>
          <InputNumber min={0} style={{ width: "100%" }} placeholder="0" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="maxUsagePerUser">
        <GlFormRow label={tr("coupon.maxPerUser", "Max Per User")}>
          <InputNumber min={0} style={{ width: "100%" }} placeholder="0" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="currency">
        <GlFormRow label={tr("coupon.currency", "Currency")}>
          <Input placeholder={tr("coupon.currencyPlaceholder", "e.g. CNY")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="state">
        <GlFormRow label={tr("coupon.status", "Status")}>
          <Select
            allowClear
            placeholder={tr("coupon.statusPlaceholder", "e.g. Active")}
            options={stateOptions(COUPON_STATES, tc)}
          />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
