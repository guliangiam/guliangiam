import { App, Form, Input, InputNumber, Select, Switch } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamLdap } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { FormSection } from "@/shared/ui/FormSection";
import { JsonEditor } from "@/shared/ui/JsonEditor";
import { jsonObjectFieldRule } from "@/shared/lib/json-field-rules";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FILTER_FIELD_OPTIONS, LDAP_TYPE_OPTIONS, PASSWORD_TYPE_OPTIONS } from "./ldap-options";
import { localizedError } from "@/shared/lib/server-error";

type LdapFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  serverName: string;
  type: string;
  serverHost: string;
  port: number;
  enableSsl: boolean;
  allowSelfSignedCert: boolean;
  baseDn: string;
  filter: string;
  filterFields: string[];
  admin: string;
  password: string;
  passwordType: string;
  attributeName: string;
  defaultGroups: string[];
  enableGroups: boolean;
  customAttributes: string;
  autoSync: boolean;
  syncInterval: number;
  isEnabled: boolean;
};

export default function LdapCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [form] = Form.useForm<LdapFormValues>();

  const autoSync = Form.useWatch("autoSync", form) ?? false;

  const mutation = useMutation({
    mutationFn: async (values: LdapFormValues) => {
      let customAttributes: Record<string, string> | undefined;
      if (values.customAttributes?.trim()) {
        try {
          customAttributes = JSON.parse(values.customAttributes);
        } catch {
          // send as-is, backend will validate
        }
      }
      return createIamLdap({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        serverName: values.serverName,
        type: values.type,
        host: values.serverHost,
        port: values.port,
        enableSsl: values.enableSsl,
        allowSelfSignedCert: values.allowSelfSignedCert,
        baseDn: values.baseDn,
        filter: values.filter,
        filterFields: values.filterFields,
        admin: values.admin,
        bindPassword: values.password,
        passwordType: values.passwordType,
        attributeName: values.attributeName,
        defaultGroups: values.defaultGroups,
        enableGroups: values.enableGroups,
        customAttributes,
        syncInterval: values.autoSync ? values.syncInterval : 0,
        isEnabled: values.isEnabled,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.ldaps.all });
      message.success(tc("createSuccess"));
      navigate("/ldaps");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  return (
    <ResourceFormPage
      title={tc("ldapCreateTitle")}
      description={tc("ldapCreateDescription")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/ldaps")}
      onFinish={async (values: LdapFormValues) => {
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
          <Input placeholder="e.g. ldap-server" />
        </GlFormRow>
      </Form.Item>

      <FormSection title={tc("ldapServerConfig")}>
        <Form.Item
          name="displayName"
          rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
        >
          <GlFormRow label={tc("displayName")} required>
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="serverName">
          <GlFormRow label={tr("ldap.serverName")}>
            <Input placeholder={tr("ldap.serverNamePlaceholder")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="type" initialValue="OpenLDAP">
          <GlFormRow label={tr("ldap.type")}>
            <Select options={LDAP_TYPE_OPTIONS} />
          </GlFormRow>
        </Form.Item>
        <Form.Item
          name="serverHost"
          rules={[{ required: true, message: tc("validation.serverHostRequired") }]}
        >
          <GlFormRow label={tc("serverHost")} required>
            <Input placeholder={tc("serverHostPlaceholder")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="port" initialValue={389}>
          <GlFormRow label={tc("port")}>
            <InputNumber min={1} max={65535} style={{ width: "100%" }} placeholder="389" />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="enableSsl" valuePropName="checked">
          <GlFormRow label={tr("ldap.enableSsl")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="allowSelfSignedCert" valuePropName="checked">
          <GlFormRow label={tr("ldap.allowSelfSignedCert")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="baseDn">
          <GlFormRow label={tc("baseDN")}>
            <Input placeholder={tc("baseDNPlaceholder")} />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <FormSection title={tc("ldapAuthConfig")}>
        <Form.Item name="admin">
          <GlFormRow label={tr("ldap.admin")}>
            <Input placeholder={tr("ldap.adminPlaceholder")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="password">
          <GlFormRow label={tc("password")}>
            <Input.Password />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="passwordType" initialValue="Plain">
          <GlFormRow label={tr("ldap.passwordType")}>
            <Select options={PASSWORD_TYPE_OPTIONS} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="attributeName" initialValue="uid">
          <GlFormRow label={tr("ldap.attributeName")}>
            <Input placeholder="uid" />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <FormSection title={tc("ldapSyncConfig")}>
        <Form.Item name="autoSync" valuePropName="checked" initialValue={false}>
          <GlFormRow label={tc("autoSync")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
        {autoSync && (
          <Form.Item name="syncInterval" initialValue={60}>
            <GlFormRow label={tc("syncInterval")}>
              <InputNumber min={1} style={{ width: "100%" }} suffix={tc("minutes")} />
            </GlFormRow>
          </Form.Item>
        )}
        <Form.Item name="filter">
          <GlFormRow label={tc("ldapFilter")}>
            <Input placeholder={tc("ldapFilterPlaceholder")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="filterFields">
          <GlFormRow label={tr("ldap.filterFields")}>
            <Select
              mode="multiple"
              options={FILTER_FIELD_OPTIONS}
              placeholder={tr("ldap.filterFieldsPlaceholder")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="enableGroups" valuePropName="checked">
          <GlFormRow label={tr("ldap.enableGroups")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="defaultGroups">
          <GlFormRow label={tr("ldap.defaultGroups")}>
            <Select mode="tags" placeholder={tr("ldap.defaultGroupsPlaceholder")} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="customAttributes" rules={[jsonObjectFieldRule(tc("invalidJson"))]}>
          <GlFormRow label={tr("ldap.customAttributes")}>
            <JsonEditor rows={4} placeholder='{"ldapAttr": "guliangiamField"}' />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="isEnabled" valuePropName="checked" initialValue={true}>
          <GlFormRow label={tc("enabled")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}
