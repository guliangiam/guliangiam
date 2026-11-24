import { App, Form, Input, Switch } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamEnforcer } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { localizedError } from "@/shared/lib/server-error";

type EnforcerFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  description?: string;
  isEnabled?: boolean;
};

export default function EnforcerCreatePage() {
  const { message } = App.useApp();
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<EnforcerFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: EnforcerFormValues) => {
      return createIamEnforcer({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        description: values.description,
        isEnabled: values.isEnabled ?? true,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.enforcers.all });
      message.success(t("createSuccess"));
      navigate("/enforcers");
    },
    onError: (error) => {
      message.error(localizedError(error, t, "error"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("enforcer.createTitle", "New Enforcer")}
      description={tr(
        "enforcer.createDesc",
        "Create a new Casbin enforcer, define policy enforcement engine.",
      )}
      saving={mutation.isPending}
      form={form}
      submitPermission={{ permission: "enforcer:write", resource: "enforcers" }}
      onCancel={() => navigate("/enforcers")}
      onFinish={async (values: EnforcerFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[
          { required: true, message: tr("enforcer.orgIdRequired", "Please enter organization ID") },
        ]}
      >
        <GlFormRow label={t("organizationId")} required>
          <OrganizationSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="name"
        rules={[
          { required: true, message: tr("enforcer.nameRequired", "Please enter enforcer name") },
        ]}
      >
        <GlFormRow label={t("name")} required>
          <Input placeholder={tr("enforcer.namePlaceholder", "e.g. enforcer-rbac")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[
          {
            required: true,
            message: tr("enforcer.displayNameRequired", "Please enter display name"),
          },
        ]}
      >
        <GlFormRow label={t("displayName")} required>
          <Input placeholder={tr("enforcer.displayNamePlaceholder", "e.g. RBAC Enforcer")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="description">
        <GlFormRow label={tr("enforcer.description", "Description")}>
          <Input.TextArea rows={3} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="isEnabled" valuePropName="checked" initialValue={true}>
        <GlFormRow label={t("enabled")} inline>
          <Switch />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
