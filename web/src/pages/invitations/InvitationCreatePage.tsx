import { Form, Input, InputNumber, Select } from "antd";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { UserPaginateSelect } from "@/shared/ui/UserPaginateSelect";
import {
  useInvitationCreateController,
  type InvitationFormValues,
} from "./useInvitationCreateController";

/** 单选适配器：表单值为单个用户 ID，内部桥接多选型 UserPaginateSelect。 */
function InvitedBySelect({
  value,
  onChange,
  organizationId,
  placeholder,
}: {
  value?: string;
  onChange?: (value?: string) => void;
  organizationId?: string;
  placeholder?: string;
}) {
  return (
    <UserPaginateSelect
      organizationId={organizationId ?? ""}
      value={value ? [value] : []}
      onChange={(values) => onChange?.(values[values.length - 1])}
      placeholder={placeholder}
      allowClear
    />
  );
}

export default function InvitationCreatePage() {
  const {
    form,
    tc,
    tr,
    navigate,
    mutation,
    roleOptions,
    groupOptions,
    handleOrganizationChange,
    defaultInvitedByLabel,
    EMAIL_PATTERN,
  } = useInvitationCreateController();
  const organizationId = Form.useWatch("organizationId", form);

  return (
    <ResourceFormPage
      title={tr("invitation.createTitle")}
      description={tr("invitation.createDesc")}
      saving={mutation.isPending}
      form={form}
      fieldLabels={{
        organizationId: tc("organizationId"),
        emails: tr("invitation.emails"),
        roleIds: tr("invitation.roles"),
        groupIds: tr("invitation.groups"),
        invitedBy: tr("invitation.invitedBy"),
        validDays: tr("invitation.validDays"),
      }}
      onCancel={() => navigate("/invitations")}
      onFinish={async (values: InvitationFormValues) => {
        const result = await mutation.mutateAsync(values);
        // 全部失败返回 false：壳保留草稿不导航，用户可修正后重试。
        return result.succeeded > 0;
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tr("invitation.orgIdRequired") }]}
      >
        <GlFormRow label={tc("organizationId")} required>
          <OrganizationSelect
            placeholder={tr("invitation.orgIdPlaceholder")}
            onChange={handleOrganizationChange}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="emails"
        rules={[
          { required: true, message: tr("invitation.emailRequired") },
          {
            validator: (_rule, value: string) => {
              const emails = (value ?? "")
                .split(/[\n,;、\s]+/)
                .map((item) => item.trim())
                .filter(Boolean);
              if (emails.length === 0) {
                return Promise.reject(new Error(tr("invitation.emailRequired")));
              }
              const invalid = emails.filter((email) => !EMAIL_PATTERN.test(email));
              if (invalid.length > 0) {
                return Promise.reject(
                  new Error(tr("invitation.emailsInvalid", { emails: invalid.join(", ") })),
                );
              }
              return Promise.resolve();
            },
          },
        ]}
      >
        <GlFormRow label={tr("invitation.emails")} required extra={tr("invitation.emailsHint")}>
          <Input.TextArea rows={3} placeholder="user1@example.com\nuser2@example.com" />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="roleIds">
        <GlFormRow label={tr("invitation.roles")}>
          <Select
            mode="multiple"
            allowClear
            placeholder={tr("invitation.rolesPlaceholder")}
            options={roleOptions.map((role) => ({ label: role.name, value: role.id }))}
            disabled={!form.getFieldValue("organizationId")}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="groupIds">
        <GlFormRow label={tr("invitation.groups")}>
          <Select
            mode="multiple"
            allowClear
            placeholder={tr("invitation.groupsPlaceholder")}
            options={groupOptions.map((group) => ({ label: group.displayName, value: group.id }))}
            disabled={!form.getFieldValue("organizationId")}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="invitedBy">
        <GlFormRow label={tr("invitation.invitedBy")} extra={defaultInvitedByLabel}>
          <InvitedBySelect organizationId={organizationId} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="validDays">
        <GlFormRow label={tr("invitation.validDays")} extra={tr("invitation.validDaysHint")}>
          <InputNumber min={1} max={365} style={{ width: "100%" }} />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
