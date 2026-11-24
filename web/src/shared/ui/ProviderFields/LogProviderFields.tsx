import { Form, Input, Select } from "antd";
import { useTranslation } from "react-i18next";

/**
 * G's "Log" category is an outbound log sink (Elasticsearch/Loki/Custom HTTP),
 * not Casdoor's log-source providers (Permission/System/Agent/SELinux Log).
 * The two share a label but not a function; Casdoor's Log providers are an
 * explicit non-target for this project.
 */
export function LogProviderFields() {
  const { t } = useTranslation("common");
  return (
    <>
      <Form.Item label={t("endpoint")} name="host" extra="Log ingestion endpoint URL">
        <Input placeholder="https://logs.example.com/ingest" />
      </Form.Item>
      <Form.Item label={t("method")} name="method">
        <Select
          allowClear
          options={[
            { label: "POST", value: "POST" },
            { label: "PUT", value: "PUT" },
          ]}
          placeholder="HTTP method for log ingestion"
        />
      </Form.Item>
      <Form.Item label={t("contentType")} name="contentType">
        <Select
          allowClear
          options={[
            { label: "JSON", value: "application/json" },
            { label: "Plain Text", value: "text/plain" },
            { label: "NDJSON", value: "application/x-ndjson" },
          ]}
          placeholder="Log payload content type"
        />
      </Form.Item>
      <Form.Item label={t("customHeader")} name="customHeader" extra="Custom HTTP headers (JSON)">
        <Input.TextArea
          rows={2}
          placeholder='{"X-Log-Source":"guliangiam","Authorization":"Bearer xxx"}'
        />
      </Form.Item>
      <Form.Item
        label={t("customBody")}
        name="customBody"
        extra="Custom log body template (JSON with {{message}}, {{level}}, {{timestamp}} placeholders)"
      >
        <Input.TextArea
          rows={4}
          placeholder='{"message":"{{message}}","level":"{{level}}","timestamp":"{{timestamp}}"}'
        />
      </Form.Item>
      <Form.Item label={t("apiKey")} name="password">
        <Input.Password placeholder="API Key for log service" />
      </Form.Item>
    </>
  );
}
