import { App, Form, Input, InputNumber, Select, Switch } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamRadiusConfig } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { localizedError } from "@/shared/lib/server-error";

type RadiusConfigFormValues = {
  organizationId: string;
  name: string;
  host: string;
  port?: number;
  sharedSecret: string;
  authProtocol: string;
  isEnabled?: boolean;
};

export default function RadiusConfigCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<RadiusConfigFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: RadiusConfigFormValues) => {
      return createIamRadiusConfig({
        organizationId: values.organizationId,
        name: values.name,
        host: values.host,
        port: values.port ?? 1812,
        sharedSecret: values.sharedSecret,
        authProtocol: values.authProtocol,
        isEnabled: values.isEnabled ?? true,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.radiusConfigs.all });
      message.success(tr("radiusConfig.created"));
      navigate("/radius-configs");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "radiusConfig.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("radiusConfig.createTitle")}
      description={tr("radiusConfig.createDesc")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/radius-configs")}
      onFinish={async (values: RadiusConfigFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tr("radiusConfig.orgIdRequired") }]}
      >
        <GlFormRow label={tc("organizationId")} required>
          <OrganizationSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="name" rules={[{ required: true, message: tr("radiusConfig.nameRequired") }]}>
        <GlFormRow label={tr("radiusConfig.name")} required>
          <Input placeholder="e.g. radius-main" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="host" rules={[{ required: true, message: tr("radiusConfig.hostRequired") }]}>
        <GlFormRow label={tr("radiusConfig.host")} required>
          <Input placeholder="e.g. 192.168.1.1" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="port" initialValue={1812}>
        <GlFormRow label={tr("radiusConfig.port")}>
          <InputNumber min={1} max={65535} style={{ width: "100%" }} placeholder="Default 1812" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="sharedSecret">
        <GlFormRow label={tr("radiusConfig.sharedSecret")}>
          <Input.Password placeholder={tc("sharedSecret")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="authProtocol" initialValue="PAP">
        <GlFormRow label={tr("radiusConfig.authProtocol")}>
          <Select
            placeholder={tc("placeholder.selectType")}
            options={[
              { value: "PAP", label: "PAP" },
              { value: "CHAP", label: "CHAP" },
            ]}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="isEnabled" valuePropName="checked" initialValue={true}>
        <GlFormRow label={tc("enabled")} inline>
          <Switch />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
