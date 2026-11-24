import { App, Form, Input } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamAdapter } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { localizedError } from "@/shared/lib/server-error";

type AdapterFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  policy: string;
};

export default function AdapterCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<AdapterFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: AdapterFormValues) => {
      return createIamAdapter({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        type: values.type,
        policy: values.policy,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.adapters.all });
      message.success(tr("adapter.created"));
      navigate("/adapters");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "adapter.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("adapter.createTitle")}
      description={tr("adapter.createDesc")}
      saving={mutation.isPending}
      form={form}
      submitPermission={{ permission: "adapter:write", resource: "adapters" }}
      onCancel={() => navigate("/adapters")}
      onFinish={async (values: AdapterFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tr("adapter.orgIdRequired") }]}
      >
        <GlFormRow label={tc("organizationId")} required>
          <OrganizationSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="name" rules={[{ required: true, message: tr("adapter.nameRequired") }]}>
        <GlFormRow label={tc("name")} required>
          <Input placeholder="e.g. adapter-mysql" />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[{ required: true, message: tr("adapter.displayNameRequired") }]}
      >
        <GlFormRow label={tc("displayName")} required>
          <Input placeholder={tr("adapter.displayNamePlaceholder")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="type" rules={[{ required: true, message: tr("adapter.typeRequired") }]}>
        <GlFormRow label={tc("type")} required>
          <Input placeholder="e.g. database" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="policy">
        <GlFormRow label={tr("adapter.policy")}>
          <Input.TextArea rows={6} placeholder={tr("adapter.policyPlaceholder")} />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
