import { App, Form, Input, Select, Switch } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createPermission } from "@/shared/api/iam/permission";
import { queryKeys } from "@/shared/api/query-keys";
import { permissionCreateSchema } from "@/shared/api/iam/schemas";
import { applyZodErrorsToForm } from "@/shared/utils/zod-form";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { ApplicationSelect } from "@/shared/ui/ApplicationSelect";
import { UserPaginateSelect } from "@/shared/ui/UserPaginateSelect";
import { GroupPaginateSelect } from "@/shared/ui/GroupPaginateSelect";
import { RolePaginateSelect } from "@/shared/ui/RolePaginateSelect";
import { DATABASE_ADAPTER_OPTIONS } from "@/shared/constants/enums";
import { PERMISSION_MODEL_OPTIONS } from "./permission-options";
import { localizedError } from "@/shared/lib/server-error";

const RESOURCE_TYPE_OPTIONS = [
  { label: "Application", value: "Application" },
  { label: "TreeNode", value: "TreeNode" },
  { label: "Custom", value: "Custom" },
  { label: "API", value: "API" },
  { label: "User", value: "User" },
  { label: "Organization", value: "Organization" },
  { label: "Provider", value: "Provider" },
  { label: "Role", value: "Role" },
  { label: "Permission", value: "Permission" },
  { label: "Group", value: "Group" },
  { label: "Resource", value: "Resource" },
  { label: "Session", value: "Session" },
];

const APPLICATION_ACTIONS = [
  { label: "Read", value: "Read" },
  { label: "Write", value: "Write" },
  { label: "Admin", value: "Admin" },
];

const API_ACTIONS = [
  { label: "GET", value: "GET" },
  { label: "POST", value: "POST" },
  { label: "PUT", value: "PUT" },
  { label: "DELETE", value: "DELETE" },
  { label: "PATCH", value: "PATCH" },
];

const EFFECT_OPTIONS = [
  { label: "Allow", value: "allow" },
  { label: "Deny", value: "deny" },
];

const STATE_OPTIONS = [
  { label: "Approved", value: "Approved" },
  { label: "Pending", value: "Pending" },
];

type PermissionFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  description: string;
  model: string;
  adapter: string;
  resourceType: string;
  resources: string[];
  actions: string[];
  effect: string;
  domains: string[];
  users: string[];
  groups: string[];
  roles: string[];
  sorter: string;
  application: string;
  state: string;
  isEnabled: boolean;
};

export default function PermissionCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<PermissionFormValues>();

  const resourceType = Form.useWatch("resourceType", form) ?? "Application";
  const organizationId = Form.useWatch("organizationId", form) ?? "";
  const isApi = resourceType === "API";
  const actionOptions = isApi ? API_ACTIONS : APPLICATION_ACTIONS;

  const mutation = useMutation({
    mutationFn: async (values: PermissionFormValues) => {
      return createPermission({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        description: values.description,
        model: values.model,
        adapter: values.adapter,
        resourceType: values.resourceType,
        resources: values.resources,
        actions: values.actions,
        effect: values.effect,
        domains: values.domains,
        users: values.users,
        groups: values.groups,
        roles: values.roles,
        sorter: values.sorter,
        application: values.application,
        state: values.state,
        isEnabled: values.isEnabled,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.permissions.all });
      message.success(tc("createSuccess"));
      navigate("/permissions");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("permission.createTitle")}
      description={tr("permission.createDescription")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/permissions")}
      onFinish={async (values: PermissionFormValues) => {
        const result = permissionCreateSchema.safeParse(values);
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
          <Input placeholder="e.g. read-users" />
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

      <FormSection title={tr("permission.casbinConfig")}>
        <Form.Item name="model">
          <GlFormRow label={tr("permission.model")}>
            <Select
              allowClear
              showSearch
              optionFilterProp="label"
              options={PERMISSION_MODEL_OPTIONS}
              placeholder={tr("permission.selectModel")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="adapter">
          <GlFormRow label={tr("permission.adapter")}>
            <Select
              allowClear
              showSearch
              optionFilterProp="label"
              options={DATABASE_ADAPTER_OPTIONS}
              placeholder={tr("permission.selectAdapter")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="application">
          <GlFormRow label={tr("permission.application")}>
            <ApplicationSelect
              allowClear
              organizationId={organizationId || undefined}
              placeholder={tr("permission.selectApplication")}
            />
          </GlFormRow>
        </Form.Item>
        {/* permissions.sorter is a text column and the DTO accepts *string. */}
        <Form.Item name="sorter" initialValue="">
          <GlFormRow label={tr("permission.sorter")}>
            <Input />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <FormSection title={tr("permission.policyRules")}>
        <Form.Item name="resourceType" initialValue="Application">
          <GlFormRow label={tr("permission.resourceType")}>
            <Select
              showSearch
              allowClear
              options={RESOURCE_TYPE_OPTIONS}
              placeholder={tr("permission.selectResourceType")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="resources">
          <GlFormRow label={tr("permission.resources")}>
            <Select
              mode={isApi ? "multiple" : "tags"}
              allowClear
              placeholder={tr("permission.resourcesPlaceholder")}
              tokenSeparators={[","]}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="actions">
          <GlFormRow label={tr("permission.actions")}>
            <Select
              mode="multiple"
              allowClear
              placeholder={tr("permission.selectActions")}
              options={actionOptions}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="effect" initialValue="allow">
          <GlFormRow label={tr("permission.effect")}>
            <Select options={EFFECT_OPTIONS} />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <FormSection title={tr("permission.subResources")}>
        <Form.Item name="users">
          <GlFormRow label={tr("permission.subUsers")}>
            <UserPaginateSelect
              placeholder={tr("permission.searchUsers")}
              allOptionValue="*"
              allOptionLabel={tr("permission.allUsers")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="groups">
          <GlFormRow label={tr("permission.subGroups")}>
            <GroupPaginateSelect
              placeholder={tr("permission.searchGroups")}
              allOptionValue="*"
              allOptionLabel={tr("permission.allGroups")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="roles">
          <GlFormRow label={tr("permission.subRoles")}>
            <RolePaginateSelect placeholder={tr("permission.searchRoles")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="domains">
          <GlFormRow label={tr("permission.subDomains")}>
            <Select mode="tags" placeholder={tr("permission.domainPlaceholder")} />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <FormSection title={tr("permission.approval")}>
        <Form.Item name="state" initialValue="Approved">
          <GlFormRow label={tr("permission.state")}>
            <Select options={STATE_OPTIONS} />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <Form.Item name="isEnabled" valuePropName="checked" initialValue={true}>
        <GlFormRow label={tc("enabled")} inline>
          <Switch />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
