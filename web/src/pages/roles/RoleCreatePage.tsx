import { App, Form, Input, Select, Switch } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createRole } from "@/shared/api/iam/role";
import { queryKeys } from "@/shared/api/query-keys";
import { roleCreateSchema } from "@/shared/api/iam/schemas";
import { applyZodErrorsToForm } from "@/shared/utils/zod-form";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { GlFormRow } from "@/shared/ui/GlForm";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { UserPaginateSelect } from "@/shared/ui/UserPaginateSelect";
import { GroupPaginateSelect } from "@/shared/ui/GroupPaginateSelect";
import { RolePaginateSelect } from "@/shared/ui/RolePaginateSelect";

type RoleFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  description: string;
  users: string[];
  groups: string[];
  roleIds: string[];
  domains: string[];
  isEnabled: boolean;
};

export default function RoleCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<RoleFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: RoleFormValues) => {
      return createRole({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        description: values.description,
        users: values.users,
        groupIds: values.groups,
        roleIds: values.roleIds,
        domains: values.domains,
        isEnabled: values.isEnabled,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.roles.all });
      message.success(tc("createSuccess"));
      navigate("/roles");
    },
  });

  const fieldLabels: Record<string, string> = {
    organizationId: tc("organizationId"),
    name: tc("name"),
    displayName: tc("displayName"),
    description: tc("description"),
    isEnabled: tc("enabled"),
    users: tr("role.subUsers"),
    groups: tr("role.subGroups"),
    roleIds: tr("role.subRoles"),
    domains: tr("role.subDomains"),
  };

  return (
    <ResourceFormPage
      title={tr("role.createTitle")}
      description={tr("role.createDescription")}
      saving={mutation.isPending}
      form={form}
      fieldLabels={fieldLabels}
      onCancel={() => navigate("/roles")}
      onFinish={async (values: RoleFormValues) => {
        const result = roleCreateSchema.safeParse(values);
        if (!result.success) {
          applyZodErrorsToForm(form, result.error);
          return false;
        }
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tc("validation.organizationIdRequired") }]}
      >
        <GlFormRow label={tc("organizationId")} required>
          <OrganizationSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="name" rules={[{ required: true, message: tc("validation.nameRequired") }]}>
        <GlFormRow label={tc("name")} required>
          <Input placeholder="e.g. admin" />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="displayName"
        rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
      >
        <GlFormRow label={tc("displayName")} required>
          <Input />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="description">
        <GlFormRow label={tc("description")}>
          <Input.TextArea rows={3} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="isEnabled" valuePropName="checked" initialValue={true}>
        <GlFormRow label={tc("enabled")} inline>
          <Switch />
        </GlFormRow>
      </Form.Item>

      <FormSection title={tr("role.subResources")}>
        <Form.Item name="users">
          <GlFormRow label={tr("role.subUsers")}>
            <UserPaginateSelect placeholder={tr("role.searchUsers")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="groups">
          <GlFormRow label={tr("role.subGroups")}>
            <GroupPaginateSelect placeholder={tr("role.searchGroups")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="roleIds">
          <GlFormRow label={tr("role.subRoles")}>
            <RolePaginateSelect placeholder={tr("role.searchRoles")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="domains">
          <GlFormRow label={tr("role.subDomains")}>
            <Select mode="tags" placeholder={tr("role.domainPlaceholder")} />
          </GlFormRow>
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}
