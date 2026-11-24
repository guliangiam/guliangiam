import {
  App,
  Button,
  Card,
  Descriptions,
  Form,
  InputNumber,
  Select,
  Skeleton,
  Typography,
  Image,
  Space,
  Tag,
} from "antd";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { getIamProduct } from "@/shared/api/iam";
import { placeOrder } from "@/shared/api/iam/order";
import { getProviders } from "@/shared/api/iam/provider-extended";
import { handleMutationError } from "@/shared/hooks/useMutationError";
import { isHttpError } from "@/shared/api/http";
import { applyServerFieldErrors, buildServerFieldResolver } from "@/shared/lib/server-error";
import { queryKeys } from "@/shared/api/query-keys";

type ProviderItem = {
  id: string;
  name: string;
  displayName: string;
  category: string;
  type: string;
  clientId: string;
};

type BuyFormValues = {
  quantity: number;
  payment?: string;
};

export default function ProductBuyPage() {
  const { productId = "" } = useParams<{ productId: string }>();
  const { t } = useTranslation("resource");
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [form] = Form.useForm<BuyFormValues>();

  const productQuery = useQuery({
    queryKey: queryKeys.products.detail(productId),
    queryFn: () => getIamProduct(productId),
    enabled: !!productId,
  });

  const providersQuery = useQuery({
    queryKey: queryKeys.providers.all,
    queryFn: () => getProviders(),
  });

  const buyMutation = useMutation({
    mutationFn: async (values: BuyFormValues) => {
      const product = productQuery.data;
      if (!product) throw new Error(t("product.buy.notFound"));
      const quantity = Math.max(1, values.quantity ?? 1);
      // The order contract represents quantity as one catalog entry per unit:
      // `products` is a string[] and the server prices each entry individually.
      const products = Array.from({ length: quantity }, () => product.id);
      return placeOrder({
        organizationId: product.organizationId,
        products,
        payment: values.payment,
        price: product.price * quantity,
        currency: product.currency,
      });
    },
    onSuccess: (order) => {
      message.success(t("product.buy.success"));
      if (order.paymentUrl) {
        window.location.href = order.paymentUrl;
      } else {
        // The order exists even when the payment channel could not issue a URL;
        // its detail page owns the retry/explanation instead of this form.
        navigate(`/orders/${order.id}`);
      }
    },
    onError: (error) => {
      handleMutationError(error, t("product.buy.failed"));
    },
  });

  const quantity = Form.useWatch("quantity", form) ?? 1;

  if (productQuery.isLoading) {
    return (
      <div style={{ padding: 48, textAlign: "center" }}>
        <Skeleton active paragraph={{ rows: 6 }} />
      </div>
    );
  }

  const product = productQuery.data;
  if (!product) {
    return (
      <div className="gl-content__inner--reading">
        <Card>
          <Typography.Text type="danger">{t("product.buy.notFound")}</Typography.Text>
        </Card>
      </div>
    );
  }

  const paymentProviders = ((providersQuery.data ?? []) as ProviderItem[]).filter(
    (p) => p.category === "Payment",
  );

  const providerOptions = paymentProviders.map((p) => ({
    label: p.displayName || p.name,
    value: p.name,
  }));

  const filteredProviderOptions = product.providers?.length
    ? providerOptions.filter((opt) => product.providers!.includes(opt.value))
    : providerOptions;

  const totalPrice = (product.price ?? 0) * quantity;

  return (
    <div className="gl-content__inner--reading">
      <Card>
        <Typography.Title level={4}>{t("product.buy.title")}</Typography.Title>

        <div style={{ display: "flex", gap: 24, marginBottom: 24, flexWrap: "wrap" }}>
          {product.image && (
            <Image
              src={product.image}
              alt={product.displayName}
              width={200}
              height={200}
              style={{ objectFit: "cover", borderRadius: 8 }}
              fallback="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mN88P/BfwAJhAPk2iMa1AAAAABJRU5ErkJggg=="
            />
          )}
          <div style={{ flex: 1, minWidth: 250 }}>
            <Typography.Title level={5}>{product.displayName}</Typography.Title>
            {product.tag && (
              <Tag color="blue" style={{ marginBottom: 8 }}>
                {product.tag}
              </Tag>
            )}
            <Typography.Paragraph type="secondary">{product.description}</Typography.Paragraph>
            <Typography.Text strong style={{ fontSize: 18 }}>
              {product.currency} {product.price}
            </Typography.Text>
          </div>
        </div>

        <Form
          form={form}
          layout="vertical"
          initialValues={{ quantity: 1 }}
          onFinish={async (values: BuyFormValues) => {
            // Await the purchase so a double click cannot create two orders/attempts.
            try {
              await buyMutation.mutateAsync(values);
            } catch (error) {
              // 裸 Form 没有 ResourceFormPage 的集中标红，这里自行落字段红字。
              if (isHttpError(error)) {
                applyServerFieldErrors(
                  form,
                  error,
                  buildServerFieldResolver<BuyFormValues>(form),
                  t,
                );
              }
            }
          }}
        >
          <Form.Item
            label={t("product.buy.quantity")}
            name="quantity"
            rules={[{ required: true, message: t("product.buy.quantityRequired") }]}
          >
            <InputNumber min={1} max={product.quantity || 999} style={{ width: "100%" }} />
          </Form.Item>

          {filteredProviderOptions.length > 0 && (
            <Form.Item label={t("product.buy.paymentProvider")} name="payment">
              <Select
                allowClear
                placeholder={t("product.buy.selectProvider")}
                options={filteredProviderOptions}
              />
            </Form.Item>
          )}

          <Descriptions column={1} bordered size="small" style={{ marginBottom: 24 }}>
            <Descriptions.Item label={t("product.buy.unitPrice")}>
              {product.currency} {product.price}
            </Descriptions.Item>
            <Descriptions.Item label={t("product.buy.totalPrice")}>
              <Typography.Text strong>
                {product.currency} {totalPrice.toFixed(2)}
              </Typography.Text>
            </Descriptions.Item>
          </Descriptions>

          <Space>
            <Button type="primary" size="large" htmlType="submit" loading={buyMutation.isPending}>
              {t("product.buy.confirmBuy")}
            </Button>
            <Button onClick={() => navigate(-1)}>{t("product.buy.cancel")}</Button>
          </Space>
        </Form>
      </Card>
    </div>
  );
}
