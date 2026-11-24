import { App, Form, Input, InputNumber, Select, Alert } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamPayment } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { CURRENCY_OPTIONS } from "@/shared/lib/currency-utils";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { ProductSelect } from "@/shared/ui/ProductSelect";
import { UserSelect } from "@/shared/ui/UserSelect";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { localizedError } from "@/shared/lib/server-error";

/**
 * Payment provider options — MUST stay in sync with backend factory.go.
 * Keys match the providerType strings used in internal/infra/payment/factory.go.
 * Label strings now sourced from i18n resource.json via the `providers` block.
 */
const PAYMENT_PROVIDER_OPTIONS = [
  // ── Asian local ──
  { value: "alipay", group: "asianLocal" },
  { value: "wechat_pay", group: "asianLocal" },
  // ── Global gateways ──
  { value: "stripe", group: "globalGateway" },
  { value: "paypal", group: "globalGateway" },
  { value: "adyen", group: "globalGateway" },
  { value: "airwallex", group: "globalGateway" },
  { value: "paddle", group: "globalGateway" },
  { value: "polar", group: "globalGateway" },
  { value: "fastspring", group: "globalGateway" },
  { value: "lemonsqueezy", group: "globalGateway" },
  { value: "gc", group: "regional" },
  // ── Internal / test ──
  { value: "balance", group: "internal" },
  { value: "dummy", group: "internal" },
] as const;

type PaymentFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  user?: string;
  provider?: string;
  product?: string;
  price?: number;
  currency?: string;
};

export default function PaymentCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<PaymentFormValues>();
  const selectedProvider = Form.useWatch("provider", form);

  const mutation = useMutation({
    mutationFn: async (values: PaymentFormValues) => {
      return createIamPayment({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        user: values.user,
        provider: values.provider,
        // 后端 payments 契约使用 products: string[]，不接受 product 字段。
        products: values.product ? [values.product] : undefined,
        price: values.price,
        currency: values.currency,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.payments.all });
      message.success(tr("payment.created"));
      navigate("/payments");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "payment.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("payment.createTitle")}
      description={tr("payment.createDesc")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/payments")}
      onFinish={async (values: PaymentFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tr("payment.orgIdRequired") }]}
      >
        <GlFormRow label={tc("organizationId")} required>
          <OrganizationSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="name" rules={[{ required: true, message: tr("payment.nameRequired") }]}>
        <GlFormRow label={tc("name")} required>
          <Input placeholder="e.g. payment-1" />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[{ required: true, message: tr("payment.displayNameRequired") }]}
      >
        <GlFormRow label={tc("displayName")} required>
          <Input placeholder="e.g. My Payment" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="user">
        <GlFormRow label={tr("payment.user")}>
          <UserSelect allowClear />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="provider"
        rules={[{ required: true, message: tr("payment.paymentMethodRequired", "请选择支付渠道") }]}
      >
        <GlFormRow label={tr("payment.paymentMethod")} required>
          <Select
            placeholder={tr("payment.paymentMethodPlaceholder")}
            showSearch
            optionFilterProp="label"
            options={[
              {
                label: tr("payment.providerGroup.asianLocal"),
                options: PAYMENT_PROVIDER_OPTIONS.filter((o) => o.group === "asianLocal").map(
                  (o) => ({
                    value: o.value,
                    label: tr(`payment.providers.${o.value}`),
                  }),
                ),
              },
              {
                label: tr("payment.providerGroup.globalGateway"),
                options: PAYMENT_PROVIDER_OPTIONS.filter((o) => o.group === "globalGateway").map(
                  (o) => ({
                    value: o.value,
                    label: tr(`payment.providers.${o.value}`),
                  }),
                ),
              },
              {
                label: tr("payment.providerGroup.regional"),
                options: PAYMENT_PROVIDER_OPTIONS.filter((o) => o.group === "regional").map(
                  (o) => ({
                    value: o.value,
                    label: tr(`payment.providers.${o.value}`),
                  }),
                ),
              },
              {
                label: tr("payment.providerGroup.internal"),
                options: PAYMENT_PROVIDER_OPTIONS.filter((o) => o.group === "internal").map(
                  (o) => ({
                    value: o.value,
                    label: tr(`payment.providers.${o.value}`),
                  }),
                ),
              },
            ]}
          />
        </GlFormRow>
      </Form.Item>
      {selectedProvider === "wechat_pay" && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 24 }}
          title={tr("payment.wechatCertWarning")}
          description={tr("payment.wechatCertDesc")}
        />
      )}
      <Form.Item name="product">
        <GlFormRow label={tr("payment.product")}>
          <ProductSelect allowClear />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="price">
        <GlFormRow label={tr("payment.price")}>
          <InputNumber min={0} style={{ width: "100%" }} placeholder="0.00" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="currency">
        <GlFormRow label={tc("currency.label")}>
          <Select
            placeholder={tc("placeholder.selectType")}
            allowClear
            showSearch
            options={CURRENCY_OPTIONS}
          />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
