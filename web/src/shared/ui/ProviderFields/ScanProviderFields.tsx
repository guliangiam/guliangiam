import { Form, Input, Select } from "antd";
import { useTranslation } from "react-i18next";

/**
 * G's "Scan" category is QR code login (WeChat/DingTalk), not Casdoor's
 * Security Scan / MCP Scan code-security scanners. Casdoor's Scan providers
 * are an explicit non-target for this project.
 */
export function ScanProviderFields() {
  const { t } = useTranslation("common");
  return (
    <>
      <Form.Item label={t("appId")} name="appId" extra="WeChat / DingTalk App ID">
        <Input placeholder="WeChat / DingTalk App ID" />
      </Form.Item>
      <Form.Item label={t("appSecret")} name="clientSecret">
        <Input.Password placeholder="App Secret" />
      </Form.Item>
      <Form.Item
        label={t("endpoint")}
        name="providerUrl"
        extra="QR code generation / scan endpoint URL"
      >
        <Input placeholder="https://scan-api.example.com/generate" />
      </Form.Item>
      <Form.Item label={t("method")} name="method">
        <Select
          allowClear
          options={[
            { label: "POST", value: "POST" },
            { label: "GET", value: "GET" },
          ]}
          placeholder="HTTP method for scan endpoint"
        />
      </Form.Item>
      <Form.Item label={t("contentType")} name="contentType">
        <Select
          allowClear
          options={[
            { label: "JSON", value: "application/json" },
            { label: "Form", value: "application/x-www-form-urlencoded" },
          ]}
          placeholder="Content type for scan request"
        />
      </Form.Item>
      <Form.Item
        label={t("customHeader")}
        name="customHeader"
        extra="Custom HTTP headers for scan endpoint (JSON)"
      >
        <Input.TextArea rows={2} placeholder='{"X-App-Key":"xxx"}' />
      </Form.Item>
      <Form.Item label={t("callbackUrl")} name="callbackUrl">
        <Input placeholder="https://your-app.com/scan-callback" />
      </Form.Item>
    </>
  );
}
