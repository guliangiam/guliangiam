import { App, Form, Input, Select, Switch } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createGroup, getGroups } from "@/shared/api/iam/group";
import { queryKeys } from "@/shared/api/query-keys";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { GlFormRow } from "@/shared/ui/GlForm";
import { GroupTreeSelect } from "@/shared/ui/GroupTreeSelect";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { UserPaginateSelect } from "@/shared/ui/UserPaginateSelect";

type GroupFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  parentId: string;
  homepage: string;
  manager: string;
  contactEmail: string;
  isTopGroup: boolean;
  isEnabled: boolean;
  users: string[];
};

export default function GroupCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<GroupFormValues>();
  const GROUP_TYPE_OPTIONS = [
    { label: tr("group.typeVirtual"), value: "virtual" },
    { label: tr("group.typePhysical"), value: "physical" },
    { label: tr("group.typeDepartment"), value: "department" },
  ];
  const organizationId = Form.useWatch("organizationId", form);

  const groupsQuery = useQuery({
    queryKey: ["groups", "auth"],
    queryFn: () => getGroups(),
  });
  // 父组只能来自已选组织，避免跨组织挂错树。
  const parentCandidates = (groupsQuery.data ?? []).filter((g) => g.owner === organizationId);

  const mutation = useMutation({
    mutationFn: async (values: GroupFormValues) => {
      return createGroup({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        type: values.type,
        parentId: values.parentId,
        homepage: values.homepage,
        manager: values.manager,
        contactEmail: values.contactEmail,
        isTopGroup: values.isTopGroup,
        isEnabled: values.isEnabled,
        users: values.users ?? [],
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.groups.all });
      message.success(tc("createSuccess"));
      navigate("/groups");
    },
  });

  // 父组/成员只能来自已选组织：切组织必须清空旧绑定，否则携旧组织 ID 提交必 422/404。
  const handleOrganizationChange = () => {
    form.setFieldsValue({ parentId: undefined, users: [] });
  };

  return (
    <ResourceFormPage
      title={tr("group.createTitle")}
      description={tr("group.createDescription")}
      saving={mutation.isPending}
      form={form}
      enableDraft
      draftRoute="groups"
      draftEntityId={"new"}
      onCancel={() => navigate("/groups")}
      onFinish={async (values: GroupFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <FormSection title={tr("group.basicInfo")}>
        <Form.Item
          name="organizationId"
          rules={[{ required: true, message: tc("validation.organizationIdRequired") }]}
        >
          <GlFormRow label={tc("organizationId")} required>
            <OrganizationSelect onChange={handleOrganizationChange} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="name" rules={[{ required: true, message: tc("validation.nameRequired") }]}>
          <GlFormRow label={tc("name")} required>
            <Input placeholder="e.g. engineering" />
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
        <Form.Item name="type" initialValue="virtual">
          <GlFormRow label={tc("type")}>
            <Select
              options={GROUP_TYPE_OPTIONS}
              allowClear
              aria-label={tc("type")}
              placeholder={tc("selectGroupType")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="parentId">
          <GlFormRow label={tr("group.parentId")}>
            <GroupTreeSelect
              groups={parentCandidates.map((g) => ({
                id: g.id,
                name: g.name,
                displayName: g.displayName,
                parentId: g.parentId,
              }))}
              disabled={!organizationId || groupsQuery.isError}
              placeholder={
                groupsQuery.isError
                  ? tc("loadFailed")
                  : organizationId === ""
                    ? tr("user.selectOrgFirst")
                    : tc("selectParentGroup")
              }
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="isTopGroup" valuePropName="checked" initialValue={false}>
          <GlFormRow label={tr("group.isTopGroup")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="isEnabled" valuePropName="checked" initialValue={true}>
          <GlFormRow label={tc("enabled")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <FormSection title={tr("group.contactInfo")}>
        <Form.Item name="homepage" rules={[{ type: "url", message: tr("form.invalidFormat") }]}>
          <GlFormRow label={tr("group.homepage")}>
            <Input placeholder="https://example.com" />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="manager">
          <GlFormRow label={tr("group.manager")}>
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item
          name="contactEmail"
          rules={[{ type: "email", message: tr("form.invalidFormat") }]}
        >
          <GlFormRow label={tr("group.contactEmail")}>
            <Input />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <FormSection title={tr("group.memberUsers")}>
        <Form.Item name="users" initialValue={[]}>
          <GlFormRow label={tr("group.memberUsers")}>
            <UserPaginateSelect organizationId={organizationId} />
          </GlFormRow>
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}
