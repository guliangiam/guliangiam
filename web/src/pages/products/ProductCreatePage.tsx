import { App, Form, Input, InputNumber, Select, Switch } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamProduct } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ImageCropUpload } from "@/shared/ui/ImageCropUpload";
import { CURRENCY_OPTIONS } from "@/shared/constants/currency";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { PRODUCT_STATES, stateOptions } from "@/shared/constants/enums";
import { localizedError } from "@/shared/lib/server-error";

type ProductFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  currency?: string;
  price?: number;
  state?: string;
  isRecharge?: boolean;
  image?: string;
  detail?: string;
  description?: string;
  tag?: string;
  quantity?: number;
};

export default function ProductCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<ProductFormValues>();
  const orgId = Form.useWatch("organizationId", form) ?? "";

  const mutation = useMutation({
    mutationFn: async (values: ProductFormValues) => {
      return createIamProduct({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        currency: values.currency,
        price: values.price,
        state: values.state,
        isRecharge: values.isRecharge ?? false,
        image: values.image,
        detail: values.detail,
        description: values.description,
        tag: values.tag,
        quantity: values.quantity,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.products.all });
      message.success(tr("product.created"));
      navigate("/products");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "product.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("product.createTitle")}
      description={tr("product.createDesc")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/products")}
      onFinish={async (values: ProductFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tc("validation.organizationIdRequired") }]}
      >
        <GlFormRow label={tc("organizationId")} required>
          <OrganizationSelect ariaLabel={tc("organizationId")} style={{ width: "100%" }} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="name" rules={[{ required: true, message: tr("product.nameRequired") }]}>
        <GlFormRow label={tc("name")} required>
          <Input placeholder={tr("product.namePlaceholder")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[{ required: true, message: tr("product.displayNameRequired") }]}
      >
        <GlFormRow label={tc("displayName")} required>
          <Input placeholder={tr("product.displayNamePlaceholder")} />
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
      <Form.Item name="price">
        <GlFormRow label={tr("product.price")}>
          <InputNumber min={0} style={{ width: "100%" }} placeholder="0.00" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="state">
        <GlFormRow label={tr("product.state")}>
          <Select
            placeholder={tc("placeholder.selectStatus")}
            allowClear
            options={stateOptions(PRODUCT_STATES, tc)}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="isRecharge" valuePropName="checked" initialValue={false}>
        <GlFormRow label={tr("product.isRecharge")} inline>
          <Switch />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="image">
        <GlFormRow label={tr("product.image")}>
          <ImageCropUpload tag="product" owner={orgId} allowUrlInput />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="detail">
        <GlFormRow label={tr("product.detail")}>
          <Input.TextArea rows={3} placeholder={tr("product.detailPlaceholder")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="description">
        <GlFormRow label={tc("description")}>
          <Input.TextArea rows={3} placeholder={tc("description")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="tag">
        <GlFormRow label={tr("product.tag")}>
          <Input placeholder={tr("product.tagPlaceholder")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="quantity">
        <GlFormRow label={tr("product.quantity")}>
          <InputNumber min={0} style={{ width: "100%" }} placeholder="0" />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
