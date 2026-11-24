import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Radio,
  Result,
  Select,
  Skeleton,
  Switch,
  Tag,
  Typography,
} from "antd";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { SyncerColumnMappingEditor } from "@/shared/ui/SyncerColumnMappingEditor";
import { GlFormRow } from "@/shared/ui/GlForm";
import {
  useSyncerDetailPageController,
  type SyncerFormValues,
} from "./useSyncerDetailPageController";

function getHostLabel(type: string, tr: (key: string) => string) {
  const keyMap: Record<string, string> = {
    "Azure AD": "syncer.hostLabel.azureAd",
    "Google Workspace": "syncer.hostLabel.googleWorkspace",
    "Active Directory": "syncer.hostLabel.activeDirectory",
    SCIM: "syncer.hostLabel.scim",
    "AWS IAM": "syncer.hostLabel.awsIam",
  };
  return tr(keyMap[type] ?? "syncer.hostLabel.default");
}
function getUserLabel(type: string, tr: (key: string) => string) {
  const keyMap: Record<string, string> = {
    WeCom: "syncer.userLabel.wecom",
    DingTalk: "syncer.userLabel.dingtalk",
    Lark: "syncer.userLabel.lark",
    "Azure AD": "syncer.userLabel.azureAd",
    "Active Directory": "syncer.userLabel.activeDirectory",
    SCIM: "syncer.userLabel.scim",
    "AWS IAM": "syncer.userLabel.awsIam",
  };
  return tr(keyMap[type] ?? "syncer.userLabel.default");
}
function getPasswordLabel(type: string, tr: (key: string) => string) {
  const keyMap: Record<string, string> = {
    WeCom: "syncer.passwordLabel.wecom",
    DingTalk: "syncer.passwordLabel.dingtalk",
    Lark: "syncer.passwordLabel.lark",
    "Azure AD": "syncer.passwordLabel.azureAd",
    "Active Directory": "syncer.passwordLabel.activeDirectory",
    SCIM: "syncer.passwordLabel.scim",
    "AWS IAM": "syncer.passwordLabel.awsIam",
  };
  return tr(keyMap[type] ?? "syncer.passwordLabel.default");
}

export default function SyncerDetailPage() {
  const navigate = useNavigate();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const {
    id,
    form,
    syncerType,
    isDatabase,
    isPostgres,
    detailQuery,
    certOptionsQuery,
    DATABASE_TYPE_OPTIONS,
    SSL_MODE_OPTIONS,
    SSH_TYPE_OPTIONS,
    updateMutation,
    deleteMutation,
    testConnectionMutation,
    runSyncMutation,
  } = useSyncerDetailPageController();

  const sshType = Form.useWatch("sshType", form) ?? "None";

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
        subTitle={String(detailQuery.error)}
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
          title={tc("notFound")}
          description={tr("syncer.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/syncers")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <ResourceFormPage
      title={detailQuery.data.displayName || detailQuery.data.name}
      description={tr("syncer.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/syncers")}
      onFinish={async (values: SyncerFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <Popconfirm
          title={tc("deleteConfirm")}
          onConfirm={() => deleteMutation.mutate()}
          okText={tc("delete")}
          cancelText={tc("cancel")}
          okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
        >
          <Button danger>{tc("delete")}</Button>
        </Popconfirm>
      }
      enableDraft
      draftRoute="syncer"
      draftEntityId={id}
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

      <FormSection title={tr("syncer.connection")}>
        <Form.Item label={tc("organization")}>
          <span>{detailQuery.data.organizationId}</span>
        </Form.Item>
        <Form.Item name="displayName" rules={[{ required: true, message: tc("required") }]}>
          <GlFormRow label={tc("displayName")} required>
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="type">
          <GlFormRow label={tr("syncer.type")}>
            <Input disabled />
          </GlFormRow>
        </Form.Item>
        {!isDatabase && (
          <>
            <Form.Item name="host" rules={[{ required: true, message: tc("required") }]}>
              <GlFormRow label={getHostLabel(syncerType, tr)} required>
                <Input />
              </GlFormRow>
            </Form.Item>
            <Form.Item name="username">
              <GlFormRow label={getUserLabel(syncerType, tr)}>
                <Input />
              </GlFormRow>
            </Form.Item>
            <Form.Item name="password">
              <GlFormRow label={getPasswordLabel(syncerType, tr)}>
                {syncerType === "Google Workspace" ? (
                  <Input.TextArea rows={4} />
                ) : (
                  <Input.Password />
                )}
              </GlFormRow>
            </Form.Item>
          </>
        )}
      </FormSection>

      {isDatabase && (
        <FormSection title={tr("syncer.databaseConfiguration")}>
          <Form.Item name="databaseType">
            <GlFormRow label={tr("syncer.databaseType")}>
              <Select options={DATABASE_TYPE_OPTIONS} />
            </GlFormRow>
          </Form.Item>
          {isPostgres && (
            <Form.Item name="sslMode">
              <GlFormRow label={tr("syncer.sslMode")}>
                <Select options={SSL_MODE_OPTIONS} />
              </GlFormRow>
            </Form.Item>
          )}
          <Form.Item name="host" rules={[{ required: true, message: tc("required") }]}>
            <GlFormRow label={tr("syncer.databaseHost")} required>
              <Input />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="port">
            <GlFormRow label={tr("syncer.databasePort")}>
              <InputNumber min={0} max={65535} style={{ width: "100%" }} />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="username">
            <GlFormRow label={tr("syncer.databaseUser")}>
              <Input />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="password">
            <GlFormRow label={tr("syncer.databasePassword")}>
              <Input.Password />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="database">
            <GlFormRow label={tr("syncer.database")}>
              <Input />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="tableName">
            <GlFormRow label={tr("syncer.table")}>
              <Input />
            </GlFormRow>
          </Form.Item>
        </FormSection>
      )}

      {isDatabase && (
        <FormSection title={tr("syncer.sshTunnel")}>
          <Form.Item name="sshType">
            <GlFormRow label={tr("syncer.sshType")}>
              <Radio.Group
                options={SSH_TYPE_OPTIONS.map((o) => ({ label: o.label, value: o.value }))}
              />
            </GlFormRow>
          </Form.Item>
          {sshType !== "None" && (
            <>
              <Form.Item name="sshHost">
                <GlFormRow label={tr("syncer.sshHost")}>
                  <Input />
                </GlFormRow>
              </Form.Item>
              <Form.Item name="sshPort">
                <GlFormRow label={tr("syncer.sshPort")}>
                  <InputNumber min={0} max={65535} style={{ width: "100%" }} />
                </GlFormRow>
              </Form.Item>
              <Form.Item name="sshUsername">
                <GlFormRow label={tr("syncer.sshUser")}>
                  <Input />
                </GlFormRow>
              </Form.Item>
              {sshType === "Password" && (
                <Form.Item name="sshPassword">
                  <GlFormRow label={tr("syncer.sshPassword")}>
                    <Input.Password />
                  </GlFormRow>
                </Form.Item>
              )}
              {sshType === "Cert" && (
                <Form.Item name="sshCertId">
                  <GlFormRow label={tr("syncer.sshCertificate")}>
                    <Select
                      allowClear
                      showSearch
                      placeholder={tr("syncer.selectCertificate")}
                      options={certOptionsQuery.data ?? []}
                      loading={certOptionsQuery.isLoading}
                    />
                  </GlFormRow>
                </Form.Item>
              )}
            </>
          )}
        </FormSection>
      )}

      {isDatabase && (
        <FormSection title={tr("syncer.columnMapping")}>
          <Form.Item name="columnMapping" valuePropName="value">
            <GlFormRow label={tr("syncer.columnMapping")}>
              <SyncerColumnMappingEditor />
            </GlFormRow>
          </Form.Item>
        </FormSection>
      )}

      <FormSection title={tr("syncer.syncSettings")}>
        <Form.Item name="syncInterval">
          <GlFormRow label={tr("syncer.syncIntervalMinutes")}>
            <InputNumber
              min={0}
              style={{ width: "100%" }}
              placeholder={tr("syncer.syncIntervalPlaceholder")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="isReadOnly" valuePropName="checked">
          <GlFormRow label={tr("syncer.readOnly")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="affinity">
          <GlFormRow label={tr("syncer.affinity")}>
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="avatarBaseUrl">
          <GlFormRow label={tr("syncer.avatarBaseUrl")}>
            <Input />
          </GlFormRow>
        </Form.Item>
        {detailQuery.data.errorText && (
          <Form.Item label={tr("syncer.error")}>
            <Typography.Text type="danger">{detailQuery.data.errorText}</Typography.Text>
          </Form.Item>
        )}
        <Form.Item name="isEnabled" valuePropName="checked">
          <GlFormRow label={tc("enabled")} inline>
            <Switch />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <FormSection
        title={tr("syncer.syncConfiguration")}
        extra={
          <div style={{ display: "flex", gap: 8 }}>
            <Button
              onClick={() => testConnectionMutation.mutate()}
              loading={testConnectionMutation.isPending}
            >
              {tr("syncer.testConnection")}
            </Button>
            <Button
              type="primary"
              onClick={() => runSyncMutation.mutate()}
              loading={runSyncMutation.isPending}
            >
              {tr("syncer.runSync")}
            </Button>
          </div>
        }
      >
        <Form.Item name="affiliationTable">
          <GlFormRow label={tr("syncer.affiliationTable")}>
            <Input />
          </GlFormRow>
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}
