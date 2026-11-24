import { useEffect } from "react";
import {
  Alert,
  App,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Result,
  Skeleton,
  Switch,
  Tag,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { deleteIamRadiusConfig, getIamRadiusConfig, patchIamRadiusConfig } from "@/shared/api/iam";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { GlFormRow } from "@/shared/ui/GlForm";
import { queryKeys } from "@/shared/api/query-keys";
import {
  buildRadiusConfigPatchPayload,
  type RadiusConfigFormValues,
} from "./radius-config-patch-payload";
import { localizedError } from "@/shared/lib/server-error";

export default function RadiusConfigDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<RadiusConfigFormValues>();

  const detailQuery = useQuery({
    queryKey: queryKeys.radiusConfigs.detail(id!),
    queryFn: () => getIamRadiusConfig(id!),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({
        host: detailQuery.data.host,
        port: detailQuery.data.port,
        // 凭据不回显（读接口只返回 sharedSecretSet 标记）：留空表示保持原值
        isEnabled: detailQuery.data.isEnabled,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: RadiusConfigFormValues) => {
      if (!detailQuery.data) throw new Error(tr("radiusConfig.dataNotLoaded"));
      return patchIamRadiusConfig(
        detailQuery.data.id,
        buildRadiusConfigPatchPayload(values, detailQuery.data.revision),
      );
    },
    onSuccess: () => {
      message.success(tr("radiusConfig.updated"));
      void queryClient.invalidateQueries({ queryKey: ["radius-configs"] });
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "radiusConfig.updateFailed"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tr("radiusConfig.dataNotLoaded"));
      return deleteIamRadiusConfig(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tr("radiusConfig.deleted"));
      void queryClient.invalidateQueries({ queryKey: ["radius-configs"] });
      navigate("/radius-configs");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "radiusConfig.deleteFailed"));
    },
  });

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
        <Alert type="error" showIcon title={tc("notFound")} description={tc("notFound")} />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/radius-configs")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <ResourceFormPage
      title={tr("radiusConfig.detailTitle", { name: detailQuery.data.name })}
      description={tr("radiusConfig.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/radius-configs")}
      onFinish={async (values: RadiusConfigFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <Popconfirm
          title={tr("radiusConfig.deleteConfirmTitle")}
          description={tr("radiusConfig.deleteConfirmDesc")}
          onConfirm={() => deleteMutation.mutate()}
          okText={tc("delete")}
          cancelText={tc("cancel")}
          okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
        >
          <Button danger>{tc("delete")}</Button>
        </Popconfirm>
      }
    >
      <FormSection title={tc("overview")}>
        <Form.Item label={tr("radiusConfig.name")}>
          <span>{detailQuery.data.name}</span>
        </Form.Item>
        <Form.Item label={tr("radiusConfig.authProtocol")}>
          <span>{detailQuery.data.authProtocol}</span>
        </Form.Item>
        <Form.Item label={tr("radiusConfig.status")}>
          {detailQuery.data.isEnabled ? (
            <Tag color="green">{tc("enabled")}</Tag>
          ) : (
            <Tag color="red">{tc("disabled")}</Tag>
          )}
        </Form.Item>
      </FormSection>

      <FormSection title={tr("basicInfo")}>
        <Form.Item
          name="host"
          rules={[{ required: true, message: tr("radiusConfig.hostRequired") }]}
        >
          <GlFormRow label={tr("radiusConfig.host")} required>
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="port">
          <GlFormRow label={tr("radiusConfig.port")}>
            <InputNumber min={1} max={65535} style={{ width: "100%" }} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="sharedSecret">
          <GlFormRow label={tr("radiusConfig.sharedSecret")}>
            <Input.Password />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="isEnabled" valuePropName="checked">
          <GlFormRow label={tc("enabled")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}
