import { App, Form, Input, Select, Switch } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamNotificationProvider } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { notificationProviderTypeOptions } from "@/shared/constants/enums";
import { ConditionalField } from "@/shared/ui/ConditionalField";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { localizedError } from "@/shared/lib/server-error";

type NotificationProviderFormValues = {
  organizationId: string;
  name: string;
  providerType: string;
  webhookUrl: string;
  accessToken: string;
  isEnabled?: boolean;
  // wecom_app
  corpId?: string;
  agentId?: string;
  // wechat_official
  appId?: string;
  templateId?: string;
  // email
  smtpHost?: string;
  smtpPort?: string;
  fromAddress?: string;
  smtpUsername?: string;
  // sms
  accessKeyId?: string;
  signName?: string;
  templateCode?: string;
};

/**
 * 渠道专属的非敏感参数落 provider.config；密钥统一走 accessToken（服务端加密
 * 存储，读接口只回 masked）。notifier builder 从 metadata 读取同名键。
 */
function buildChannelConfig(
  values: NotificationProviderFormValues,
): Record<string, unknown> | undefined {
  if (values.providerType === "wecom_app") {
    return { corp_id: values.corpId ?? "", agent_id: values.agentId ?? "" };
  }
  if (values.providerType === "wechat_official") {
    return { app_id: values.appId ?? "", template_id: values.templateId ?? "" };
  }
  if (values.providerType === "email") {
    return {
      host: values.smtpHost ?? "",
      port: values.smtpPort ?? "",
      from: values.fromAddress ?? "",
      username: values.smtpUsername ?? "",
    };
  }
  if (values.providerType === "sms") {
    return {
      access_key_id: values.accessKeyId ?? "",
      sign_name: values.signName ?? "",
      template_code: values.templateCode ?? "",
    };
  }
  return undefined;
}

// 新建同样是写操作：只读权限不能创建（对齐统一 ProviderDetailPage 的守卫策略）。
export default function NotificationProviderCreatePage() {
  return (
    <PermissionGuard action="notification-provider:write" resource="notification-providers">
      <NotificationProviderCreateContent />
    </PermissionGuard>
  );
}

function NotificationProviderCreateContent() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const [form] = Form.useForm<NotificationProviderFormValues>();

  const mutation = useMutation({
    mutationFn: async (values: NotificationProviderFormValues) => {
      return createIamNotificationProvider({
        organizationId: values.organizationId,
        name: values.name,
        providerType: values.providerType,
        webhookUrl: values.webhookUrl,
        accessToken: values.accessToken,
        config: buildChannelConfig(values),
        isEnabled: values.isEnabled ?? true,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.notificationProviders.all });
      await queryClient.invalidateQueries({ queryKey: queryKeys.providers.all });
      message.success(tr("notificationProvider.created"));
      navigate("/notification-providers");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "notificationProvider.createFailed"));
    },
  });

  return (
    <ResourceFormPage
      title={tr("notificationProvider.createTitle")}
      description={tr("notificationProvider.createDesc")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/notification-providers")}
      onFinish={async (values: NotificationProviderFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tc("validation.organizationIdRequired") }]}
      >
        <GlFormRow label={tc("organizationId")} required>
          <OrganizationSelect ariaLabel={tc("organizationId")} style={{ width: "100%" }} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="name"
        rules={[{ required: true, message: tr("notificationProvider.nameRequired") }]}
      >
        <GlFormRow label={tc("name")} required>
          <Input placeholder={tc("namePlaceholder")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item
        name="providerType"
        rules={[{ required: true, message: tr("notificationProvider.providerTypeRequired") }]}
      >
        <GlFormRow label={tr("notificationProvider.providerType")} required>
          <Select
            placeholder={tr("notificationProvider.providerTypePlaceholder")}
            options={notificationProviderTypeOptions()}
          />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="webhookUrl">
        <GlFormRow label={tr("notificationProvider.webhookUrl")}>
          <Input placeholder={tr("notificationProvider.webhookUrlPlaceholder")} />
        </GlFormRow>
      </Form.Item>
      <ConditionalField
        dependencies={["providerType"]}
        when={(values) => values.providerType === "wecom_app"}
      >
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
      </ConditionalField>
      <ConditionalField
        dependencies={["providerType"]}
        when={(values) => values.providerType === "wechat_official"}
      >
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
      </ConditionalField>
      <ConditionalField
        dependencies={["providerType"]}
        when={(values) => values.providerType === "email"}
      >
        <Form.Item name="smtpHost" rules={[{ required: true, message: "请填写 SMTP 服务器地址" }]}>
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
      </ConditionalField>
      <ConditionalField
        dependencies={["providerType"]}
        when={(values) => values.providerType === "sms"}
      >
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
        <Form.Item name="templateCode" rules={[{ required: true, message: "请填写短信模板 Code" }]}>
          <GlFormRow label={tc("templateCode", "短信模板 Code")} required>
            <Input placeholder="SMS_123456789" />
          </GlFormRow>
        </Form.Item>
      </ConditionalField>
      <Form.Item name="accessToken">
        <GlFormRow
          label={tr("notificationProvider.accessToken")}
          help={tc(
            "notificationProviderSecretHint",
            "密钥字段：SMTP 密码 / 短信 AccessKey Secret / 企业微信应用 Secret 等",
          )}
        >
          <Input.Password placeholder={tc("accessToken")} />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="isEnabled" valuePropName="checked" initialValue={true}>
        <GlFormRow label={tc("isEnabled")} inline>
          <Switch />
        </GlFormRow>
      </Form.Item>
    </ResourceFormPage>
  );
}
