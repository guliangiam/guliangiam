import { App, Form, Input, InputNumber } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamOrder } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { ProductSelect } from "@/shared/ui/ProductSelect";
import { UserSelect } from "@/shared/ui/UserSelect";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { localizedError } from "@/shared/lib/server-error";

type OrderFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  user?: string;
  product?: string;
  provider?: string;
  price?: number;
  currency?: string;
};

export default function OrderCreatePage() {
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<OrderFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: OrderFormValues) => {
      return createIamOrder({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        user: values.user,
        // 后端 orders 契约是 products: string[] 与 payment，不接受 product/provider。
        products: values.product ? [values.product] : undefined,
        payment: values.provider,
        price: values.price,
        currency: values.currency,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.orders.all });
      message.success(tr("order.created", "Order created"));
      navigate("/orders");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "order.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("order.createTitle", "New Order")}
      description={tr(
        "order.createDesc",
        "Create a new order, associate user, product and payment method.",
      )}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/orders")}
      onFinish={async (values: OrderFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[
          { required: true, message: tr("order.orgIdRequired", "Please enter organization ID") },
        ]}
      >
        <GlFormRow label={tr("order.orgId", "Organization ID")} required>
          <OrganizationSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="name"
        rules={[{ required: true, message: tr("order.nameRequired", "Please enter order name") }]}
      >
        <GlFormRow label={tr("order.name", "Name")} required>
          <Input placeholder={tr("order.namePlaceholder", "e.g. order-1")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[
          { required: true, message: tr("order.displayNameRequired", "Please enter display name") },
        ]}
      >
        <GlFormRow label={tr("order.displayName", "Display Name")} required>
          <Input placeholder={tr("order.displayNamePlaceholder", "e.g. Order One")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="user">
        <GlFormRow label={tr("order.user", "User")}>
          <UserSelect allowClear />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="product">
        <GlFormRow label={tr("order.product", "Product")}>
          <ProductSelect allowClear />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="provider">
        <GlFormRow label={tr("order.paymentProvider", "Payment Provider")}>
          <Input placeholder={tr("order.paymentProviderPlaceholder", "Payment provider")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="price">
        <GlFormRow label={tr("order.price", "Price")}>
          <InputNumber min={0} style={{ width: "100%" }} placeholder="0.00" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="currency">
        <GlFormRow label={tr("order.currency", "Currency")}>
          <Input placeholder={tr("order.currencyPlaceholder", "e.g. CNY")} />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
