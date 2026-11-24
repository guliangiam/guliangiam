import { useEffect } from "react";
import {
  Alert,
  App,
  Button,
  Card,
  Form,
  Input,
  Popconfirm,
  Result,
  Skeleton,
  Switch,
  Tag,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  deleteIamNotificationProvider,
  getIamNotificationProvider,
  patchIamNotificationProvider,
} from "@/shared/api/iam";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { GlFormRow } from "@/shared/ui/GlForm";
import { queryKeys } from "@/shared/api/query-keys";
import { localizedError } from "@/shared/lib/server-error";

type NotificationProviderFormValues = {
  webhookUrl: string;
  accessToken?: string;
  isEnabled: boolean;
  corpId?: string;
  agentId?: string;
  appId?: string;
  templateId?: string;
  smtpHost?: string;
  smtpPort?: string;
  fromAddress?: string;
  smtpUsername?: string;
  accessKeyId?: string;
  signName?: string;
  templateCode?: string;
};

/** 渠道专属非敏感参数 → provider.config（与新建页保持一致）。 */
function buildChannelConfig(
  providerType: string,
  values: NotificationProviderFormValues,
): Record<string, unknown> | undefined {
  if (providerType === "wecom_app") {
    return { corp_id: values.corpId ?? "", agent_id: values.agentId ?? "" };
  }
  if (providerType === "wechat_official") {
    return { app_id: values.appId ?? "", template_id: values.templateId ?? "" };
  }
  if (providerType === "email") {
    return {
      host: values.smtpHost ?? "",
      port: values.smtpPort ?? "",
      from: values.fromAddress ?? "",
      username: values.smtpUsername ?? "",
    };
  }
  if (providerType === "sms") {
    return {
      access_key_id: values.accessKeyId ?? "",
      sign_name: values.signName ?? "",
      template_code: values.templateCode ?? "",
    };
  }
  return undefined;
}

function configString(config: Record<string, unknown> | undefined, key: string): string {
  const value = config?.[key];
  return typeof value === "string" ? value : value == null ? "" : String(value);
}

export default function NotificationProviderDetailPage() {
  return (
    <PermissionGuard action="notification-provider:read" resource="notification-providers">
      <NotificationProviderDetailContent />
    </PermissionGuard>
  );
}

function NotificationProviderDetailContent() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<NotificationProviderFormValues>();
  // 平台管理员跨组织打开详情时，列表链接携带 ?organizationId=…（对照 useProviderDetail）。
  const [searchParams] = useSearchParams();
  const orgIdFromQuery = searchParams.get("organizationId") || undefined;

  const detailQuery = useQuery({
    queryKey: queryKeys.notificationProviders.detail(id!),
    queryFn: () => getIamNotificationProvider(id!, orgIdFromQuery),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({
        webhookUrl: detailQuery.data.webhookUrl,
        // 凭据不回显（读接口只返回 accessTokenSet/accessTokenMasked）：留空表示保持原值
        isEnabled: detailQuery.data.isEnabled,
        corpId: configString(detailQuery.data.config, "corp_id"),
        agentId: configString(detailQuery.data.config, "agent_id"),
        appId: configString(detailQuery.data.config, "app_id"),
        templateId: configString(detailQuery.data.config, "template_id"),
        smtpHost: configString(detailQuery.data.config, "host"),
        smtpPort: configString(detailQuery.data.config, "port"),
        fromAddress: configString(detailQuery.data.config, "from"),
        smtpUsername: configString(detailQuery.data.config, "username"),
        accessKeyId: configString(detailQuery.data.config, "access_key_id"),
        signName: configString(detailQuery.data.config, "sign_name"),
        templateCode: configString(detailQuery.data.config, "template_code"),
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: NotificationProviderFormValues) => {
      if (!detailQuery.data) throw new Error(tr("notificationProvider.dataNotLoaded"));
      const accessToken = values.accessToken?.trim();
      return patchIamNotificationProvider(
        detailQuery.data.id,
        {
          webhookUrl: values.webhookUrl,
          ...(accessToken ? { accessToken } : {}),
          ...(buildChannelConfig(detailQuery.data.providerType, values)
            ? { config: buildChannelConfig(detailQuery.data.providerType, values) }
            : {}),
          isEnabled: values.isEnabled,
          revision: detailQuery.data.revision,
        },
        detailQuery.data.organizationId,
      );
    },
    onSuccess: () => {
      // 保存成功后清空输入框：明文凭据不回填，也不应继续留在表单状态里。
      form.resetFields(["accessToken"]);
      message.success(tr("notificationProvider.updated"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.notificationProviders.all });
      void queryClient.invalidateQueries({ queryKey: queryKeys.providers.all });
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "notificationProvider.updateFailed"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tr("notificationProvider.dataNotLoaded"));
      return deleteIamNotificationProvider(detailQuery.data.id, detailQuery.data.organizationId);
    },
    onSuccess: () => {
      message.success(tr("notificationProvider.deleted"));
      void queryClient.invalidateQueries({ queryKey: ["notification-providers"] });
      void queryClient.invalidateQueries({ queryKey: queryKeys.providers.all });
      navigate("/notification-providers");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "notificationProvider.deleteFailed"));
    },
  });

  if (detailQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 4 }} />
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
          description={tc("notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/notification-providers")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <ResourceFormPage
      title={tr("notificationProvider.detailTitle", { name: detailQuery.data.name })}
      description={tr("notificationProvider.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/notification-providers")}
      submitPermission={{
        permission: "notification-provider:write",
        resource: "notification-providers",
      }}
      onFinish={async (values: NotificationProviderFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <PermissionGuard
          action="notification-provider:write"
          resource="notification-providers"
          fallback={null}
        >
          <Popconfirm
            title={tr("notificationProvider.deleteConfirmTitle")}
            description={tr("notificationProvider.deleteConfirmDesc")}
            onConfirm={() => deleteMutation.mutate()}
            okText={tc("delete")}
            cancelText={tc("cancel")}
            okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
          >
            <Button danger>{tc("delete")}</Button>
          </Popconfirm>
        </PermissionGuard>
      }
    >
      <Form.Item label={tc("name")}>
        <span>{detailQuery.data.name}</span>
      </Form.Item>
      <Form.Item label={tr("notificationProvider.providerType")}>
        <span>{detailQuery.data.providerType}</span>
      </Form.Item>
      <Form.Item label={tc("statusLabel")}>
        {detailQuery.data.isEnabled ? (
          <Tag color="green">{tc("enabled")}</Tag>
        ) : (
          <Tag color="red">{tc("disabled")}</Tag>
        )}
      </Form.Item>
      <Form.Item name="webhookUrl">
        <GlFormRow label={tr("notificationProvider.webhookUrl")}>
          <Input placeholder={tc("webhookUrl")} />
        </GlFormRow>
      </Form.Item>
      {detailQuery.data.providerType === "wecom_app" && (
        <>
          <Form.Item name="corpId" rules={[{ required: true, message: "请填写企业微信 CorpID" }]}>
            <GlFormRow label={tc("corpId", "企业微信 CorpID")} required>
              <Input placeholder="ww1234567890abcdef" />
            </GlFormRow>
          </Form.Item>
          <Form.Item
            name="agentId"
            rules={[{ required: true, message: "请填写企业微信应用 AgentID" }]}
          >
            <GlFormRow label={tc("agentId", "企业微信应用 AgentID")} required>
              <Input placeholder="1000002" />
            </GlFormRow>
          </Form.Item>
        </>
      )}
      {detailQuery.data.providerType === "wechat_official" && (
        <>
          <Form.Item name="appId" rules={[{ required: true, message: "请填写公众号 AppID" }]}>
            <GlFormRow label={tc("appId")} required>
              <Input placeholder="wx1234567890abcdef" />
            </GlFormRow>
          </Form.Item>
          <Form.Item
            name="templateId"
            rules={[{ required: true, message: "请填写模板消息 Template ID" }]}
          >
            <GlFormRow label={tc("templateId")} required>
              <Input placeholder="-abc123..." />
            </GlFormRow>
          </Form.Item>
        </>
      )}
      {detailQuery.data.providerType === "email" && (
        <>
          <Form.Item
            name="smtpHost"
            rules={[{ required: true, message: "请填写 SMTP 服务器地址" }]}
          >
            <GlFormRow label={tc("smtpHost", "SMTP 服务器")} required>
              <Input placeholder="smtp.example.com" />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="smtpPort">
            <GlFormRow label={tc("smtpPort", "SMTP 端口")}>
              <Input placeholder="587" />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="fromAddress" rules={[{ required: true, message: "请填写发件人地址" }]}>
            <GlFormRow label={tc("fromAddress", "发件人地址")} required>
              <Input placeholder="noreply@example.com" />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="smtpUsername">
            <GlFormRow label={tc("smtpUsername", "SMTP 用户名")}>
              <Input placeholder="noreply@example.com" />
            </GlFormRow>
          </Form.Item>
        </>
      )}
      {detailQuery.data.providerType === "sms" && (
        <>
          <Form.Item
            name="accessKeyId"
            rules={[{ required: true, message: "请填写短信服务 AccessKey ID" }]}
          >
            <GlFormRow label={tc("accessKeyId", "AccessKey ID")} required>
              <Input placeholder="LTAI..." />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="signName" rules={[{ required: true, message: "请填写短信签名" }]}>
            <GlFormRow label={tc("signName", "短信签名")} required>
              <Input placeholder="签名名称" />
            </GlFormRow>
          </Form.Item>
          <Form.Item
            name="templateCode"
            rules={[{ required: true, message: "请填写短信模板 Code" }]}
          >
            <GlFormRow label={tc("templateCode", "短信模板 Code")} required>
              <Input placeholder="SMS_123456789" />
            </GlFormRow>
          </Form.Item>
        </>
      )}
      <Form.Item name="accessToken">
        <GlFormRow
          label={tr("notificationProvider.accessToken")}
          help={tc(
            "notificationProviderSecretHint",
            "密钥字段：SMTP 密码 / 短信 AccessKey Secret / 企业微信应用 Secret 等",
          )}
        >
          <Input.Password placeholder={detailQuery.data.accessTokenMasked ?? tc("accessToken")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="isEnabled" valuePropName="checked">
        <GlFormRow label={tc("isEnabled")} inline>
          <Switch />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
