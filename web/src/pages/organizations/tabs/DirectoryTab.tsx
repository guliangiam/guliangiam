/**
 * Organization 详情页 · Tab 8「LDAP」与 Tab 9「Kerberos」
 *
 * 两者同属目录服务接入配置，故合并为一个模块导出两个 Tab 组件，
 * 避免产生两个各只有 20 余行的碎片文件。
 */
import { Form, Input, Select } from "antd";
import { useTranslation } from "react-i18next";
import { FormSection } from "@/shared/ui/FormSection";
import { OrgLdapSubTable } from "../OrgSubTables";

type DirectoryTabProps = {
  organizationId: string;
};

export function LdapTab({ organizationId }: DirectoryTabProps) {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");

  return (
    <>
      <Form.Item label={tr("organization.ldapAttributes")} name="ldapAttributes">
        <Select
          mode="multiple"
          tokenSeparators={[","]}
          placeholder={t("selectLdapAttrs")}
          options={[
            { label: "uid", value: "uid" },
            { label: "cn", value: "cn" },
            { label: "mail", value: "mail" },
            { label: "email", value: "email" },
            { label: "mobile", value: "mobile" },
            { label: "displayName", value: "displayName" },
            { label: "givenName", value: "givenName" },
            { label: "sn", value: "sn" },
            { label: "gecos", value: "gecos" },
            { label: "uidNumber", value: "uidNumber" },
            { label: "gidNumber", value: "gidNumber" },
            { label: "homeDirectory", value: "homeDirectory" },
            { label: "loginShell", value: "loginShell" },
            { label: "title", value: "title" },
            { label: "userPassword", value: "userPassword" },
            { label: "memberOf", value: "memberOf" },
            { label: "sshPublicKey", value: "sshPublicKey" },
          ]}
        />
      </Form.Item>
      <FormSection
        title={tr("organization.ldapServers")}
        description={tr("organization.ldapServersDesc")}
      >
        <OrgLdapSubTable organizationId={organizationId} />
      </FormSection>
    </>
  );
}

export function KerberosTab() {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");

  return (
    <>
      <Form.Item label={tr("organization.kerberosRealm")} name="kerberosRealm">
        <Input placeholder="EXAMPLE.COM" />
      </Form.Item>
      <Form.Item label={tr("organization.kerberosKdcHost")} name="kerberosKdcHost">
        <Input placeholder="kdc.example.com" />
      </Form.Item>
      <Form.Item label={tr("organization.kerberosKeytab")} name="kerberosKeytab">
        <Input.TextArea
          rows={3}
          placeholder={`${t("keytabData")} · ${t("leaveEmptyToKeepCurrent")}`}
        />
      </Form.Item>
      <Form.Item label={tr("organization.kerberosServiceName")} name="kerberosServiceName">
        <Input placeholder="HTTP" />
      </Form.Item>
    </>
  );
}

// DirectoryTab 是 LdapTab + KerberosTab 的组合容器，供 OrganizationDetailPage 使用
export function DirectoryTab({ organizationId }: DirectoryTabProps) {
  return (
    <>
      <LdapTab organizationId={organizationId} />
      <KerberosTab />
    </>
  );
}
