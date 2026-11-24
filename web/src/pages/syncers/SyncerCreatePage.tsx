import { App, Form, Input, InputNumber, Radio, Select, Switch } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamSyncer, serializeSyncerColumnMapping } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { getCertOptions } from "@/shared/api/iam/cert";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import {
  SyncerColumnMappingEditor,
  type SyncerColumnMapping,
} from "@/shared/ui/SyncerColumnMappingEditor";
import { localizedError } from "@/shared/lib/server-error";

const SYNCER_TYPE_VALUES = [
  "Database",
  "Keycloak",
  "WeCom",
  "Azure AD",
  "Active Directory",
  "Google Workspace",
  "DingTalk",
  "Lark",
  "Okta",
  "SCIM",
  "AWS IAM",
] as const;

const DATABASE_TYPE_VALUES = ["postgres", "mysql", "mssql", "oracle", "sqlite3"] as const;

const SSL_MODE_VALUES = ["disable", "require", "verify-ca", "verify-full"] as const;

const SSH_TYPE_VALUES = ["None", "Password", "Cert"] as const;

const CLOUD_SYNCER_TYPES = new Set([
  "Keycloak",
  "WeCom",
  "Azure AD",
  "Google Workspace",
  "DingTalk",
  "Lark",
  "Okta",
  "SCIM",
  "AWS IAM",
]);

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
    "Google Workspace": "syncer.passwordLabel.googleWorkspace",
    "Active Directory": "syncer.passwordLabel.activeDirectory",
    SCIM: "syncer.passwordLabel.scim",
    "AWS IAM": "syncer.passwordLabel.awsIam",
  };
  return tr(keyMap[type] ?? "syncer.passwordLabel.default");
}

type SyncerFormValues = {
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  databaseType: string;
  sslMode: string;
  host: string;
  port: number;
  username: string;
  password: string;
  database: string;
  tableName: string;
  syncInterval: number;
  isReadOnly: boolean;
  affinity: string;
  columnMapping: SyncerColumnMapping[];
  sshType: string;
  sshHost: string;
  sshPort: number;
  sshUsername: string;
  sshPassword: string;
  sshCertId: string;
  affiliationTable: string;
  avatarBaseUrl: string;
  isEnabled: boolean;
};

function SyncerColumnMappingEditorInput({
  value,
  onChange,
}: {
  value?: SyncerColumnMapping[];
  onChange?: (value: SyncerColumnMapping[]) => void;
}) {
  return <SyncerColumnMappingEditor value={value ?? []} onChange={(v) => onChange?.(v)} />;
}

export default function SyncerCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<SyncerFormValues>();

  const SYNCER_TYPE_OPTIONS = SYNCER_TYPE_VALUES.map((v) => ({
    label: tr(`syncer.typeOption.${v}`),
    value: v,
  }));
  const DATABASE_TYPE_OPTIONS = DATABASE_TYPE_VALUES.map((v) => ({
    label: tr(`syncer.databaseTypeOption.${v}`),
    value: v,
  }));
  const SSL_MODE_OPTIONS = SSL_MODE_VALUES.map((v) => ({
    label: tr(`syncer.sslModeOption.${v}`),
    value: v,
  }));
  const SSH_TYPE_OPTIONS = SSH_TYPE_VALUES.map((v) => ({
    label: tr(`syncer.sshTypeOption.${v}`),
    value: v,
  }));

  const certOptionsQuery = useQuery({
    queryKey: ["cert-options"],
    queryFn: () => getCertOptions(),
    staleTime: 60_000,
  });

  const syncerType = Form.useWatch("type", form) ?? "Database";
  const sshType = Form.useWatch("sshType", form) ?? "None";
  const isCloud = CLOUD_SYNCER_TYPES.has(syncerType);
  const isDatabase = !isCloud;
  const isPostgres = Form.useWatch("databaseType", form) === "postgres";

  const mutation = useMutation({
    mutationFn: async (values: SyncerFormValues) => {
      return createIamSyncer({
        organizationId: values.organizationId,
        name: values.name,
        displayName: values.displayName,
        type: values.type,
        databaseType: values.databaseType,
        sslMode: values.sslMode,
        host: values.host,
        port: values.port,
        username: values.username,
        password: values.password,
        database: values.database,
        tableName: values.tableName,
        syncInterval: values.syncInterval,
        isReadOnly: values.isReadOnly,
        affinity: values.affinity,
        columnMapping: serializeSyncerColumnMapping(values.columnMapping),
        sshType: values.sshType,
        sshHost: values.sshHost,
        sshPort: values.sshPort,
        sshUsername: values.sshUsername,
        sshPassword: values.sshPassword,
        sshCertId: values.sshCertId,
        affiliationTable: values.affiliationTable,
        avatarBaseUrl: values.avatarBaseUrl,
        isEnabled: values.isEnabled,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.syncers.all });
      message.success(tc("createSuccess"));
      navigate("/syncers");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  return (
    <ResourceFormPage
      title={tc("syncerCreateTitle")}
      description={tc("syncerCreateDescription")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/syncers")}
      onFinish={async (values: SyncerFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <FormSection title={tr("basicInfo")}>
        <Form.Item
          name="organizationId"
          rules={[{ required: true, message: tc("validation.organizationIdRequired") }]}
        >
          <GlFormRow label={tc("organizationId")} required>
            <OrganizationSelect ariaLabel={tc("organizationId")} style={{ width: "100%" }} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="name" rules={[{ required: true, message: tc("validation.nameRequired") }]}>
          <GlFormRow label={tc("name")} required>
            <Input placeholder="machine-readable unique identifier (e.g. sales-db-sync)" />
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
      </FormSection>

      <FormSection title={tc("syncerConnection")}>
        <Form.Item name="type" initialValue="Database" rules={[{ required: true }]}>
          <GlFormRow label={tr("syncer.type")} required>
            <Select options={SYNCER_TYPE_OPTIONS} />
          </GlFormRow>
        </Form.Item>
        {isDatabase && (
          <Form.Item name="databaseType" initialValue="postgres">
            <GlFormRow label={tr("syncer.databaseType")}>
              <Select options={DATABASE_TYPE_OPTIONS} />
            </GlFormRow>
          </Form.Item>
        )}
        {isPostgres && isDatabase && (
          <Form.Item name="sslMode" initialValue="disable">
            <GlFormRow label={tr("syncer.sslMode")}>
              <Select options={SSL_MODE_OPTIONS} />
            </GlFormRow>
          </Form.Item>
        )}
        <Form.Item name="host" rules={[{ required: true }]}>
          <GlFormRow label={getHostLabel(syncerType, tr)} required>
            <Input />
          </GlFormRow>
        </Form.Item>
        {isDatabase && (
          <Form.Item name="port">
            <GlFormRow label={tr("syncer.port")}>
              <InputNumber min={0} max={65535} style={{ width: "100%" }} />
            </GlFormRow>
          </Form.Item>
        )}
        <Form.Item name="username">
          <GlFormRow label={getUserLabel(syncerType, tr)}>
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="password">
          <GlFormRow label={getPasswordLabel(syncerType, tr)}>
            {syncerType === "Google Workspace" ? <Input.TextArea rows={4} /> : <Input.Password />}
          </GlFormRow>
        </Form.Item>
        {isDatabase && (
          <>
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
          </>
        )}
      </FormSection>

      {isDatabase && (
        <FormSection title={tr("syncer.sshTunnel")}>
          <Form.Item name="sshType" initialValue="None">
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
              <Form.Item name="sshPort" initialValue={22}>
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
        <FormSection title={tc("syncerColumnMapping")}>
          <Form.Item name="columnMapping" valuePropName="value">
            <GlFormRow label={tr("syncer.columnMapping")}>
              <SyncerColumnMappingEditorInput />
            </GlFormRow>
          </Form.Item>
        </FormSection>
      )}

      <FormSection title={tc("syncerSettings")}>
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
        <Form.Item name="affiliationTable">
          <GlFormRow label={tr("syncer.affiliationTable")}>
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="avatarBaseUrl">
          <GlFormRow label={tr("syncer.avatarBaseUrl")}>
            <Input />
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
