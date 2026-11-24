import { App, Form, Input } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamModel } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { localizedError } from "@/shared/lib/server-error";

type ModelFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  description?: string;
  modelText: string;
};

export default function ModelCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<ModelFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: ModelFormValues) => {
      return createIamModel({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        description: values.description,
        modelText: values.modelText,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.models.all });
      message.success(tr("model.created"));
      navigate("/models");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "model.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("model.createTitle")}
      description={tr("model.createDesc")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/models")}
      onFinish={async (values: ModelFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tr("model.orgIdRequired") }]}
      >
        <GlFormRow label={tc("organizationId")} required>
          <OrganizationSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="name" rules={[{ required: true, message: tr("model.nameRequired") }]}>
        <GlFormRow label={tc("name")} required>
          <Input placeholder="e.g. model-rbac" />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[{ required: true, message: tr("model.displayNameRequired") }]}
      >
        <GlFormRow label={tc("displayName")} required>
          <Input placeholder={tr("model.displayNamePlaceholder")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="description">
        <GlFormRow label={tc("description")}>
          <Input.TextArea rows={2} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="modelText">
        <GlFormRow label={tr("model.modelText")}>
          <Input.TextArea rows={8} placeholder={tr("model.modelTextPlaceholder")} />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
