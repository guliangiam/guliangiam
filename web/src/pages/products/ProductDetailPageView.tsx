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
  Switch,
  Tag,
} from "antd";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { ImageCropUpload } from "@/shared/ui/ImageCropUpload";
import { StateTag } from "@/shared/ui/StateTag";
import type { ProductFormValues } from "./useProductDetailPageController";
import { useProductDetailPageController } from "./useProductDetailPageController";

type Props = { ctrl: ReturnType<typeof useProductDetailPageController> };

export function ProductDetailPageView({ ctrl }: Props) {
  const {
    id,
    tc,
    tr,
    navigate,
    form,
    orgNameMap,
    detailQuery,
    paymentProvidersQuery,
    updateMutation,
    deleteMutation,
    buyMutation,
    PRODUCT_STATES,
    PRODUCT_TRANSITIONS,
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
          title={tr("product.notFound")}
          description={tr("product.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/products")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  const data = detailQuery.data;

  return (
    <ResourceFormPage
      title={tr("product.detailTitle", {
        name: data.displayName || data.name,
      })}
      description={tr("product.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/products")}
      onFinish={async (values: ProductFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <>
          {data.state === "active" && (
            <Popconfirm
              title={tr("product.confirmBuy")}
              onConfirm={() => buyMutation.mutate()}
              okText={tc("confirm")}
              cancelText={tc("cancel")}
            >
              <Button type="primary" loading={buyMutation.isPending}>
                {tr("product.buy.title")}
              </Button>
            </Popconfirm>
          )}
          <Popconfirm
            title={tc("deleteConfirm")}
            description={tr("product.deleteDesc")}
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
          <Descriptions.Item label={tc("statusLabel")}>
            <StateTag value={data.state} />
          </Descriptions.Item>
          <Descriptions.Item label={tr("product.recharge")}>
            {data.isRecharge ? (
              <Tag color="green">{tc("yes")}</Tag>
            ) : (
              <Tag color="default">{tc("no")}</Tag>
            )}
          </Descriptions.Item>
        </Descriptions>
      </FormSection>

      <FormSection title={tr("product.basicInfo")}>
        <Form.Item
          label={tc("displayName")}
          name="displayName"
          rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
        >
          <Input />
        </Form.Item>
        <Form.Item label={tr("product.currency")} name="currency">
          <Input />
        </Form.Item>
        <Form.Item label={tc("price")} name="price">
          <InputNumber min={0} style={{ width: "100%" }} />
        </Form.Item>
        <Form.Item label={tc("statusLabel")} name="state">
          <Select options={allowedTargets(PRODUCT_STATES, data.state, PRODUCT_TRANSITIONS, tc)} />
        </Form.Item>
        <Form.Item label={tr("product.recharge")} name="isRecharge" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item label={tr("product.quantity")} name="quantity">
          <InputNumber min={0} style={{ width: "100%" }} />
        </Form.Item>
      </FormSection>

      <FormSection title={tr("product.mediaAndDetails")}>
        <Form.Item label={tr("product.image")} name="image">
          <ImageCropUpload
            tag="product"
            owner={data.organizationId}
            name={data.name}
            aspectRatio={1}
            shape="square"
            allowUrlInput
          />
        </Form.Item>
        <Form.Item label={tc("detail")} name="detail">
          <Input.TextArea rows={3} />
        </Form.Item>
        <Form.Item label={tc("description")} name="description">
          <Input.TextArea rows={3} />
        </Form.Item>
        <Form.Item label={tr("product.tag")} name="tag">
          <Input />
        </Form.Item>
      </FormSection>

      <FormSection title={tr("product.paymentInfo")}>
        <Form.Item label={tr("product.paymentProviders")} name="paymentProviders">
          <Select
            mode="multiple"
            placeholder={tc("selectPaymentProviders")}
            options={
              paymentProvidersQuery.data?.map((p) => ({
                label: p.displayName || p.name,
                value: p.name,
              })) ?? []
            }
          />
        </Form.Item>
        <Form.Item label={tr("product.successUrl")} name="successUrl">
          <Input placeholder={tc("successUrlPlaceholder")} />
        </Form.Item>
        <Form.Item label={tr("product.rechargeOptions")} name="rechargeOptions">
          <Select mode="tags" placeholder={tc("rechargeOptionsPlaceholder")} />
        </Form.Item>
        <Form.Item
          label={tr("product.disableCustomRecharge")}
          name="disableCustomRecharge"
          valuePropName="checked"
        >
          <Switch />
        </Form.Item>
        <Form.Item label={tc("sold")} name="sold">
          <InputNumber min={0} disabled style={{ width: "100%" }} />
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}
