import { App, Form, Input } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamResource } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { ApplicationSelect } from "@/shared/ui/ApplicationSelect";
import { GlFormRow } from "@/shared/ui/GlForm";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { localizedError } from "@/shared/lib/server-error";

// Mirrors `CreateResourceRequest`: `applicationId`, `tag`, `filePath`,
// `fileSize`, `mimeType`. The form previously also collected `url` and
// `description`, which the API has no fields for, so both were discarded on
// submit while appearing to be saved.
type ResourceFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  applicationId: string;
  tag: string;
};

export default function ResourceCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<ResourceFormValues>();
  const organizationId = Form.useWatch("organizationId", form) ?? "";

  const mutation = useMutation({
    mutationFn: async (values: ResourceFormValues) => {
      return createIamResource({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        type: values.type,
        applicationId: values.applicationId,
        tag: values.tag,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.resources.all });
      message.success(tr("iamResource.created"));
      navigate("/resources");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "iamResource.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("iamResource.createTitle")}
      description={tr("iamResource.createDesc")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/resources")}
      onFinish={async (values: ResourceFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tr("iamResource.orgIdRequired") }]}
      >
        <GlFormRow label={tc("organizationId")} required>
          <OrganizationSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="name" rules={[{ required: true, message: tr("iamResource.nameRequired") }]}>
        <GlFormRow label={tc("name")} required>
          <Input placeholder="e.g. resource-api" />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[{ required: true, message: tr("iamResource.displayNameRequired") }]}
      >
        <GlFormRow label={tc("displayName")} required>
          <Input placeholder="e.g. API Resource" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="type" rules={[{ required: true, message: tr("iamResource.typeRequired") }]}>
        <GlFormRow label={tc("type")} required>
          <Input placeholder={tr("iamResource.typePlaceholder")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="applicationId"
        tooltip="The application the resource belongs to; PATCH/POST store it as applicationId."
      >
        <GlFormRow label={tr("iamResource.application")}>
          <ApplicationSelect allowClear organizationId={organizationId || undefined} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="tag">
        <GlFormRow label={tr("iamResource.tag")}>
          <Input placeholder="e.g. avatar" />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
