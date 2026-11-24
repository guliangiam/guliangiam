import { App, Form, Input, Select } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamEntry } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { localizedError } from "@/shared/lib/server-error";

type EntryFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  type?: string;
  provider?: string;
  application?: string;
  message?: string;
};

export default function EntryCreatePage() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const [form] = Form.useForm<EntryFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: EntryFormValues) => {
      return createIamEntry({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        type: values.type,
        provider: values.provider,
        application: values.application,
        message: values.message,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.entries.all });
      message.success(tr("entry.created"));
      navigate("/entries");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "entry.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("entry.createTitle")}
      description={tr("entry.createDesc")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/entries")}
      onFinish={async (values: EntryFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tr("entry.orgIdRequired") }]}
      >
        <GlFormRow label={tc("organizationId")} required>
          <OrganizationSelect placeholder={tc("orgIdPlaceholder")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="name" rules={[{ required: true, message: tr("entry.nameRequired") }]}>
        <GlFormRow label={tc("name")} required>
          <Input placeholder={tc("namePlaceholder")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[{ required: true, message: tr("entry.displayNameRequired") }]}
      >
        <GlFormRow label={tc("displayName")} required>
          <Input placeholder={tr("entry.displayNamePlaceholder")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="type">
        <GlFormRow label={tc("type")}>
          <Select
            placeholder={tr("entry.typePlaceholder")}
            allowClear
            options={[
              { value: "login", label: "Login" },
              { value: "logout", label: "Logout" },
              { value: "signup", label: "Signup" },
              { value: "update", label: "Update" },
              { value: "delete", label: "Delete" },
              { value: "create", label: "Create" },
              { value: "other", label: "Other" },
            ]}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="provider">
        <GlFormRow label={tr("entry.provider")}>
          <Input placeholder={tr("entry.providerPlaceholder")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="application">
        <GlFormRow label={tr("entry.application")}>
          <Input placeholder={tr("entry.applicationPlaceholder")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="message">
        <GlFormRow label={tr("entry.message")}>
          <Input.TextArea rows={4} placeholder={tr("entry.messagePlaceholder")} />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
