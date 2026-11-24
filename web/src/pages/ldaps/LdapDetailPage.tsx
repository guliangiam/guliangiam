import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Result,
  Select,
  Skeleton,
  Switch,
  Table,
  Tag,
} from "antd";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { serverErrorMessage } from "@/shared/lib/server-error";
import { FormSection } from "@/shared/ui/FormSection";
import { JsonEditor } from "@/shared/ui/JsonEditor";
import { jsonObjectFieldRule } from "@/shared/lib/json-field-rules";
import { GlFormRow } from "@/shared/ui/GlForm";
import { useLdapDetailPageController, type LdapFormValues } from "./useLdapDetailPageController";
import { FILTER_FIELD_OPTIONS, LDAP_TYPE_OPTIONS, PASSWORD_TYPE_OPTIONS } from "./ldap-options";
import { LinkButton } from "@/shared/ui/LinkButton";

export default function LdapDetailPage() {
  const navigate = useNavigate();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const {
    id,
    form,
    previewOpen,
    setPreviewOpen,
    detailQuery,
    previewQuery,
    updateMutation,
    deleteMutation,
  } = useLdapDetailPageController();

  const autoSync = Form.useWatch("autoSync", form) ?? false;

  const fieldLabels: Record<string, string> = {
    displayName: tc("displayName"),
    serverName: tr("ldap.serverName"),
    type: tr("ldap.type"),
    serverHost: tc("serverHost"),
    port: tc("port"),
    enableSsl: tr("ldap.enableSsl"),
    allowSelfSignedCert: tr("ldap.allowSelfSignedCert"),
    baseDn: tc("baseDN"),
    admin: tr("ldap.admin"),
    password: tc("password"),
    passwordType: tr("ldap.passwordType"),
    attributeName: tr("ldap.attributeName"),
    autoSync: tc("autoSync"),
    syncInterval: tc("syncInterval"),
    filter: tc("ldapFilter"),
    filterFields: tr("ldap.filterFields"),
    enableGroups: tr("ldap.enableGroups"),
    defaultGroups: tr("ldap.defaultGroups"),
    customAttributes: tr("ldap.customAttributes"),
    isEnabled: tc("enabled"),
  };

  if (detailQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }
  if (detailQuery.isError) {
    return (
      <Result
        status="error"
        title={tc("loadFailed")}
        subTitle={serverErrorMessage(detailQuery.error, tc, "common:loadFailed")}
        extra={
          <Button type="primary" onClick={() => detailQuery.refetch()}>
            {tc("retry")}
          </Button>
        }
      />
    );
  }
  if (!detailQuery.data) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Alert
          type="error"
          showIcon
          title={tr("ldap.notFound")}
          description={tr("ldap.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/ldaps")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <>
      <ResourceFormPage
        title={tr("ldap.detailTitle", {
          name: detailQuery.data.displayName || detailQuery.data.name,
        })}
        description={tr("ldap.editDescription")}
        loading={detailQuery.isLoading}
        saving={updateMutation.isPending}
        form={form}
        fieldLabels={fieldLabels}
        onCancel={() => navigate("/ldaps")}
        onFinish={async (values: LdapFormValues) => {
          await updateMutation.mutateAsync(values);
        }}
        submitPermission={{ permission: "ldaps:write", resource: "ldaps" }}
        extra={
          <>
            <Button style={{ marginRight: 8 }} onClick={() => setPreviewOpen(true)}>
              {tr("ldap.previewUsers")}
            </Button>
            <LinkButton style={{ marginRight: 8 }} to={`/ldaps/${id}/sync`}>
              {tr("ldap.syncAction")}
            </LinkButton>
            <PermissionGuard action="ldaps:write" resource="ldaps" fallback={null}>
              <Popconfirm
                title={tc("deleteConfirm")}
                description={tr("ldap.deleteDesc")}
                onConfirm={() => deleteMutation.mutate()}
                okText={tc("delete")}
                cancelText={tc("cancel")}
                okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
              >
                <Button danger>{tc("delete")}</Button>
              </Popconfirm>
            </PermissionGuard>
          </>
        }
      >
        <FormSection title={tc("overview", "Overview")}>
          <Form.Item label={tc("name")}>
            <span>{detailQuery.data.name}</span>
          </Form.Item>
          <Form.Item label={tc("statusLabel")}>
            {detailQuery.data.isEnabled ? (
              <Tag color="green">{tc("enabled")}</Tag>
            ) : (
              <Tag color="red">{tc("disabled")}</Tag>
            )}
          </Form.Item>
        </FormSection>

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
          <Form.Item name="type">
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
          <Form.Item name="port">
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
            <GlFormRow label={tc("password")} extra={tc("leaveEmptyToKeepCurrent")}>
              <Input.Password />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="passwordType">
            <GlFormRow label={tr("ldap.passwordType")}>
              <Select options={PASSWORD_TYPE_OPTIONS} />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="attributeName">
            <GlFormRow label={tr("ldap.attributeName")}>
              <Input placeholder="uid" />
            </GlFormRow>
          </Form.Item>
        </FormSection>

        <FormSection title={tc("ldapSyncConfig")}>
          <Form.Item name="autoSync" valuePropName="checked">
            <GlFormRow label={tc("autoSync")} inline>
              <Switch />
            </GlFormRow>
          </Form.Item>
          {autoSync && (
            <Form.Item name="syncInterval">
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
          <Form.Item name="isEnabled" valuePropName="checked">
            <GlFormRow label={tc("enabled")} inline>
              <Switch />
            </GlFormRow>
          </Form.Item>
        </FormSection>
      </ResourceFormPage>

      <Modal
        title={tr("ldap.previewUsers")}
        open={previewOpen}
        onCancel={() => setPreviewOpen(false)}
        footer={null}
        width={720}
      >
        {previewQuery.isLoading ? (
          <Skeleton active paragraph={{ rows: 6 }} />
        ) : previewQuery.isError ? (
          <Alert
            type="error"
            showIcon
            title={tc("loadFailed")}
            description={serverErrorMessage(previewQuery.error, tc, "common:loadFailed")}
            action={
              <Button size="small" onClick={() => previewQuery.refetch()}>
                {tc("retry")}
              </Button>
            }
          />
        ) : previewQuery.data?.users?.length ? (
          <Table
            size="small"
            dataSource={previewQuery.data.users}
            rowKey="dn"
            pagination={{ pageSize: 10 }}
            columns={[
              { title: "DN", dataIndex: "dn", key: "dn", ellipsis: true },
              { title: "UID", dataIndex: "uid", key: "uid" },
              { title: "CN", dataIndex: "cn", key: "cn" },
              { title: tr("ldap.email"), dataIndex: "email", key: "email" },
              {
                title: tc("statusLabel"),
                dataIndex: "existsLocal",
                key: "existsLocal",
                render: (v: boolean) =>
                  v ? (
                    <Tag color="green">{tr("ldap.exists")}</Tag>
                  ) : (
                    <Tag color="blue">{tr("ldap.new")}</Tag>
                  ),
              },
            ]}
          />
        ) : (
          <Alert type="info" title={tr("ldap.noPreviewUsers")} />
        )}
      </Modal>
    </>
  );
}
