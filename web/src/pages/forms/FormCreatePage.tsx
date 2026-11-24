import { App, Form, Input, Switch } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { createIamForm } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { localizedError } from "@/shared/lib/server-error";

type FormFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  type?: string;
  properties?: string;
  isEnabled?: boolean;
};

export default function FormCreatePage() {
  const account = useSessionStore((state) => state.account);
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const [form] = Form.useForm<FormFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: FormFormValues) => {
      return createIamForm({
        ...values,
        organizationId: values.organizationId || account?.organization?.id || "",
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.forms.all });
      message.success(tr("formResource.created"));
      navigate("/forms");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "formResource.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("formResource.createTitle")}
      description={tr("formResource.createDesc")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/forms")}
      onFinish={async (values) => {
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
      <Form.Item name="name" rules={[{ required: true, message: tr("formResource.nameRequired") }]}>
        <GlFormRow label={tc("name")} required>
          <Input />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[{ required: true, message: tr("formResource.displayNameRequired") }]}
      >
        <GlFormRow label={tc("displayName")} required>
          <Input />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="type">
        <GlFormRow label={tc("type")}>
          <Input />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="properties"
        rules={[
          {
            validator: (_rule, value?: string) => {
              if (!value || !value.trim()) return Promise.resolve();
              try {
                JSON.parse(value);
                return Promise.resolve();
              } catch {
                return Promise.reject(new Error(tr("formResource.propertiesInvalid")));
              }
            },
          },
        ]}
      >
        <GlFormRow label={tr("formResource.properties")}>
          <Input.TextArea rows={4} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="isEnabled" valuePropName="checked" initialValue={false}>
        <GlFormRow label={tc("isEnabled")} inline>
          <Switch />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
