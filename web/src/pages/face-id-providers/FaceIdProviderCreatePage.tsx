import { App, Form, Input, Select, Switch } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamFaceIdProvider } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { localizedError } from "@/shared/lib/server-error";

type FaceIdProviderFormValues = {
  organizationId: string;
  name: string;
  providerType: string;
  accessKeyId: string;
  accessKeySecret: string;
  endpoint: string;
  isEnabled?: boolean;
};

// 新建同样是写操作：只读权限不能创建（对齐统一 ProviderDetailPage 的守卫策略）。
export default function FaceIdProviderCreatePage() {
  return (
    <PermissionGuard action="face-id-provider:write" resource="face-id-providers">
      <FaceIdProviderCreateContent />
    </PermissionGuard>
  );
}

function FaceIdProviderCreateContent() {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<FaceIdProviderFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: FaceIdProviderFormValues) => {
      return createIamFaceIdProvider({
        organizationId: values.organizationId,
        name: values.name,
        providerType: values.providerType,
        accessKeyId: values.accessKeyId,
        accessKeySecret: values.accessKeySecret,
        endpoint: values.endpoint,
        isEnabled: values.isEnabled ?? true,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.faceIdProviders.all });
      await queryClient.invalidateQueries({ queryKey: queryKeys.providers.all });
      message.success(tr("faceId.created", "Face ID provider created"));
      navigate("/face-id-providers");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "faceId.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("faceId.createTitle", "New Face ID Provider")}
      description={tr("faceId.createDesc", "Configure a new face recognition provider.")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/face-id-providers")}
      onFinish={async (values: FaceIdProviderFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tc("validation.organizationIdRequired") }]}
      >
        <GlFormRow label={tr("faceId.orgId", "Organization ID")} required>
          <OrganizationSelect ariaLabel={tc("organizationId")} style={{ width: "100%" }} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="name"
        rules={[
          { required: true, message: tr("faceId.nameRequired", "Please enter provider name") },
        ]}
      >
        <GlFormRow label={tr("faceId.name", "Name")} required>
          <Input placeholder={tr("faceId.namePlaceholder", "e.g. faceid-aliyun")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="providerType" initialValue="aliyun">
        <GlFormRow label={tr("faceId.providerType", "Provider Type")} required>
          <Select
            options={[
              { label: "Alibaba Cloud Facebody", value: "aliyun" },
              { label: "Local UniFace", value: "local_uniface" },
            ]}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="accessKeyId">
        <GlFormRow label="AccessKey ID">
          <Input placeholder={tr("faceId.accessKeyId", "AccessKey ID")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="accessKeySecret">
        <GlFormRow label="AccessKey Secret">
          <Input.Password placeholder={tr("faceId.accessKeySecret", "AccessKey Secret")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="endpoint">
        <GlFormRow label={tr("faceId.endpoint", "Endpoint")}>
          <Input
            placeholder={tr("faceId.endpointPlaceholder", "e.g. facebody.cn-shanghai.aliyuncs.com")}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="isEnabled" valuePropName="checked" initialValue={true}>
        <GlFormRow label={tr("faceId.enabled", "Enabled")} inline>
          <Switch />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
