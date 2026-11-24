import { Form, Input, Select } from "antd";
import { useTranslation } from "react-i18next";

export function NotificationProviderFields() {
  const { t } = useTranslation("common");
  return (
    <>
      <Form.Item label={t("method")} name="method">
        <Select
          allowClear
          options={[
            { label: "POST", value: "POST" },
            { label: "PUT", value: "PUT" },
          ]}
          placeholder="HTTP method"
        />
      </Form.Item>
      <Form.Item label={t("endpoint")} name="customApi" extra="Webhook URL or API endpoint">
        <Input placeholder="https://api.example.com/notify" />
      </Form.Item>
      <Form.Item label={t("customHeader")} name="customHeader" extra="Custom HTTP headers (JSON)">
        <Input.TextArea rows={2} placeholder='{"Authorization":"Bearer ..."}' />
      </Form.Item>
      <Form.Item
        label={t("customBody")}
        name="customBody"
        extra="Custom notification body template (JSON with {{title}}, {{content}}, {{receiver}} placeholders)"
      >
        <Input.TextArea
          rows={4}
          placeholder='{"title":"{{title}}","content":"{{content}}","receiver":"{{receiver}}"}'
        />
      </Form.Item>
      <Form.Item label="Title" name="appId" extra="Default notification title / subject template">
        <Input placeholder="Notification from GuliangIAM" />
      </Form.Item>
      <Form.Item label="Content" name="template" extra="Default notification content template">
        <Input.TextArea rows={3} placeholder="Notification content with {{variables}}" />
      </Form.Item>
      <Form.Item
        label={t("recipient")}
        name="sender"
        extra="Default receiver ID (channel, chat ID, user)"
      >
        <Input placeholder={t("channelIdPlaceholder")} />
      </Form.Item>
      <Form.Item label={t("admin")} name="admin">
        <Input placeholder={t("usernameBotTokenPlaceholder")} />
      </Form.Item>
      <Form.Item label={t("password")} name="password">
        <Input.Password placeholder="API Key / Secret" />
      </Form.Item>
      <Form.Item label={t("contentType")} name="contentType">
        <Select
          allowClear
          options={[
            { label: "JSON", value: "application/json" },
            { label: "Form", value: "application/x-www-form-urlencoded" },
          ]}
          placeholder="Content type"
        />
      </Form.Item>
    </>
  );
}
