import {
  Alert,
  Button,
  Card,
  DatePicker,
  Form,
  Input,
  Popconfirm,
  Result,
  Select,
  Skeleton,
  Switch,
  Tag,
} from "antd";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import {
  KEY_ALGORITHM_OPTIONS,
  KEY_TYPE_OPTIONS,
  type KeyFormValues,
  type useKeyDetailPageController,
} from "./useKeyDetailPageController";

type KeyDetailPageController = ReturnType<typeof useKeyDetailPageController>;

export function KeyDetailPageView({ ctrl }: { ctrl: KeyDetailPageController }) {
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
          title={tr("key.notFound")}
          description={tr("key.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/keys")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <ResourceFormPage
      title={tr("key.detailTitle", {
        name: detailQuery.data.displayName || detailQuery.data.name,
      })}
      description={tr("key.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/keys")}
      onFinish={async (values: KeyFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <Popconfirm
          title={tc("deleteConfirm")}
          description={tr("key.deleteDesc")}
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
        <Form.Item label={tc("name")}>
          <span>{detailQuery.data.name}</span>
        </Form.Item>
        <Form.Item label={tc("organization")}>
          <span>
            {orgNameMap.get(detailQuery.data.organizationId) || detailQuery.data.organizationId}
          </span>
        </Form.Item>
        <Form.Item label={tc("statusLabel")}>
          <Tag color={detailQuery.data.isEnabled ? "green" : "default"}>
            {detailQuery.data.isEnabled ? tc("enabled") : tc("disabled")}
          </Tag>
        </Form.Item>
      </FormSection>

      <FormSection title={tc("keyBasicInfo")}>
        {/* PatchKeyRequest 不含 keyType：类型创建后不可变，仅展示。 */}
        <Form.Item label={tc("type")} name="keyType">
          <Select options={KEY_TYPE_OPTIONS} disabled />
        </Form.Item>
        <Form.Item label={tc("algorithm")} name="algorithm">
          <Select options={KEY_ALGORITHM_OPTIONS} allowClear />
        </Form.Item>
        <Form.Item
          label={tc("displayName")}
          name="displayName"
          rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
        >
          <Input />
        </Form.Item>
        <Form.Item label={tc("expireTime")} name="expiresAt">
          {/* 后端 PatchKey 只在 expiresAt 非 null 时更新：清空是静默 no-op，禁止清除。 */}
          <DatePicker showTime allowClear={false} style={{ width: "100%" }} />
        </Form.Item>
        <Form.Item label={tc("enabled")} name="isEnabled" valuePropName="checked">
          <Switch />
        </Form.Item>
      </FormSection>

      <FormSection title={tc("keyMaterial")}>
        <Form.Item
          label={tc("publicKey")}
          name="publicKey"
          rules={[{ required: true, message: tc("publicKeyRequired", "Please enter public key") }]}
        >
          <Input.TextArea rows={4} style={{ fontFamily: "monospace", fontSize: 12 }} />
        </Form.Item>
        <Form.Item label={tc("privateKey")} name="privateKey" extra={tc("privateKeyKeepHint")}>
          {/* 留空 = 保持已存储私钥不变；仅输入新值时才提交（BE-B-6）。 */}
          <Input.TextArea
            rows={4}
            style={{ fontFamily: "monospace", fontSize: 12 }}
            placeholder={tc("privateKeyKeepPlaceholder")}
          />
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}
