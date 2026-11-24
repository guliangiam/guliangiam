import { Form, Input, InputNumber, Select } from "antd";
import { useTranslation } from "react-i18next";

export type EmailProviderFieldsProps = {
  value: Record<string, string>;
  onChange: (field: string, val: string) => void;
  subType?: string;
};

const sslModeOptions = [
  { label: "None", value: "none" },
  { label: "SSL/TLS", value: "ssl" },
  { label: "STARTTLS", value: "starttls" },
];

/** Provider types that use a user-defined HTTP endpoint instead of a vendor API. */
const HTTP_TYPES = new Set(["CustomHTTP", "Custom", "Custom HTTP"]);

type FieldSpec = {
  /** Component-side key; mapped to a providers column via DynamicProviderFields. */
  key: string;
  labelKey: string;
  secret?: boolean;
  placeholder?: string;
  placeholderKey?: string;
};

/**
 * Per-vendor credential layouts, mirroring the backend email builder's column
 * mapping (admin/password/appId/sender/host/region/customApi). Keys follow
 * each vendor's official naming (e.g. SendCloud apiUser/apiKey, Zoho
 * accountId), while the component key stays one of the generic bridge keys so
 * values land on the native columns.
 */
const PROVIDER_FIELDS: Record<string, FieldSpec[]> = {
  SendGrid: [
    { key: "apiKey", labelKey: "apiKey", secret: true },
    { key: "sender", labelKey: "sender", placeholder: "from@verified-domain.com" },
  ],
  AzureACS: [
    {
      key: "endpoint",
      labelKey: "endpoint",
      placeholder: "https://<resource>.communication.azure.com",
    },
    { key: "apiKey", labelKey: "apiKey", secret: true },
    { key: "sender", labelKey: "sender", placeholder: "from@verified-domain.com" },
  ],
  Resend: [
    { key: "apiKey", labelKey: "apiKey", secret: true },
    { key: "sender", labelKey: "sender", placeholder: "onboarding@resend.dev" },
  ],
  "SUBMAIL Email": [
    { key: "appId", labelKey: "appId" },
    { key: "appKey", labelKey: "appKey", secret: true },
  ],
  Mailtrap: [
    { key: "apiToken", labelKey: "apiKey", secret: true },
    { key: "sender", labelKey: "sender", placeholder: "from@your-verified-domain" },
  ],
  DirectMail: [
    { key: "accessKeyId", labelKey: "accessKeyId", placeholder: "AccessKeyId" },
    { key: "accessKeySecret", labelKey: "accessKeySecret", secret: true },
    {
      key: "sender",
      labelKey: "sender",
      placeholder: "AccountName (verified sender address)",
    },
    { key: "region", labelKey: "region", placeholder: "cn-hangzhou" },
    { key: "endpoint", labelKey: "endpoint", placeholder: "https://dm.aliyuncs.com/" },
  ],
  TencentSES: [
    { key: "accessKeyId", labelKey: "accessKeyId", placeholder: "SecretId" },
    { key: "accessKeySecret", labelKey: "accessKeySecret", secret: true, placeholder: "SecretKey" },
    { key: "sender", labelKey: "sender", placeholder: "FromEmailAddress" },
    { key: "region", labelKey: "region", placeholder: "ap-guangzhou" },
    { key: "endpoint", labelKey: "endpoint", placeholder: "https://ses.tencentcloudapi.com/" },
  ],
  SendCloud: [
    { key: "accessKeyId", labelKey: "apiUser", placeholder: "API_USER" },
    { key: "accessKeySecret", labelKey: "apiKey", secret: true, placeholder: "API_KEY" },
    { key: "sender", labelKey: "sender", placeholder: "from@your-domain.com" },
  ],
  Mailgun: [
    { key: "apiKey", labelKey: "apiKey", secret: true, placeholder: "Private API key" },
    { key: "domain", labelKey: "domain", placeholder: "mg.example.com" },
    { key: "sender", labelKey: "sender", placeholder: "defaults to postmaster@domain" },
  ],
  AmazonSES: [
    { key: "accessKeyId", labelKey: "accessKeyId", placeholder: "AccessKeyId" },
    {
      key: "accessKeySecret",
      labelKey: "accessKeySecret",
      secret: true,
      placeholder: "SecretAccessKey",
    },
    { key: "region", labelKey: "region", placeholder: "us-east-1 / cn-north-1" },
    { key: "sender", labelKey: "sender", placeholder: "Verified identity (email/domain)" },
  ],
  Postmark: [
    { key: "serverToken", labelKey: "serverToken", secret: true },
    { key: "sender", labelKey: "sender", placeholder: "from@your-domain.com" },
  ],
  MailerSend: [
    { key: "apiToken", labelKey: "apiKey", secret: true },
    { key: "sender", labelKey: "sender", placeholder: "from@verified-domain.com" },
  ],
  Brevo: [
    { key: "apiKey", labelKey: "apiKey", secret: true },
    { key: "sender", labelKey: "sender", placeholder: "from@your-domain.com" },
  ],
  ZohoMail: [
    {
      key: "accessToken",
      labelKey: "accessToken",
      secret: true,
      placeholder: "Zoho-oauthtoken value",
    },
    {
      key: "accountId",
      labelKey: "accountId",
      placeholder: "accountId from Get All User Accounts",
    },
    { key: "sender", labelKey: "sender", placeholder: "fromAddress" },
    {
      key: "endpoint",
      labelKey: "endpoint",
      placeholder: "https://mail.zoho.com (data-center override)",
    },
  ],
};

/** SMTP remains the default layout for the SMTP kind and unknown provider types. */
const SMTP_KEYS = new Set(["SMTP"]);

export function EmailProviderFields({ value, onChange, subType }: EmailProviderFieldsProps) {
  const { t } = useTranslation("common");
  const type = subType ?? "";
  const isHttp = HTTP_TYPES.has(type);
  const isSmtp = SMTP_KEYS.has(type) || (!PROVIDER_FIELDS[type] && !isHttp);
  const fields = PROVIDER_FIELDS[type] ?? [];

  if (isSmtp) {
    return (
      <>
        <Form.Item label={t("host")}>
          <Input
            value={value.host ?? ""}
            onChange={(e) => onChange("host", e.target.value)}
            placeholder="smtp.example.com"
          />
        </Form.Item>
        <Form.Item label={t("port")}>
          <InputNumber
            value={value.port ? Number(value.port) : undefined}
            onChange={(val) => onChange("port", String(val ?? ""))}
            min={1}
            max={65535}
            precision={0}
            style={{ width: "100%" }}
            placeholder="465 / 587 / 25"
          />
        </Form.Item>
        <Form.Item label={t("username")}>
          <Input
            value={value.username ?? ""}
            onChange={(e) => onChange("username", e.target.value)}
            placeholder="admin@example.com"
          />
        </Form.Item>
        <Form.Item label={t("password")}>
          <Input.Password
            value={value.password ?? ""}
            onChange={(e) => onChange("password", e.target.value)}
            placeholder={t("leaveEmptyToKeepCurrent")}
          />
        </Form.Item>
        <Form.Item label={t("sslMode")} extra="SSL/TLS for port 465, STARTTLS for port 587">
          <Select
            value={value.sslMode || "ssl"}
            onChange={(val) => onChange("sslMode", val)}
            options={sslModeOptions}
          />
        </Form.Item>
      </>
    );
  }

  if (isHttp) {
    return (
      <>
        <Form.Item label={t("endpoint")} extra="HTTP endpoint URL for sending emails">
          <Input
            value={value.endpoint ?? ""}
            onChange={(e) => onChange("endpoint", e.target.value)}
            placeholder="https://email-api.example.com/send"
          />
        </Form.Item>
        <Form.Item label={t("method")}>
          <Select
            value={value.method || "POST"}
            onChange={(val) => onChange("method", val)}
            options={[
              { label: "POST", value: "POST" },
              { label: "PUT", value: "PUT" },
            ]}
          />
        </Form.Item>
        <Form.Item label={t("contentType")}>
          <Select
            value={value.contentType || "application/json"}
            onChange={(val) => onChange("contentType", val)}
            options={[
              { label: "JSON", value: "application/json" },
              { label: "Form", value: "application/x-www-form-urlencoded" },
            ]}
          />
        </Form.Item>
        <Form.Item label={t("customHeader")} extra="Custom HTTP headers (JSON)">
          <Input.TextArea
            value={value.httpHeaders ?? ""}
            onChange={(e) => onChange("httpHeaders", e.target.value)}
            rows={2}
            placeholder='{"Authorization":"Bearer xxx"}'
          />
        </Form.Item>
      </>
    );
  }

  return (
    <>
      {fields.map((field) => {
        const placeholder = field.placeholderKey ? t(field.placeholderKey) : field.placeholder;
        return (
          <Form.Item key={field.key} label={t(field.labelKey)}>
            {field.secret ? (
              <Input.Password
                value={value[field.key] ?? ""}
                onChange={(e) => onChange(field.key, e.target.value)}
                placeholder={placeholder}
              />
            ) : (
              <Input
                value={value[field.key] ?? ""}
                onChange={(e) => onChange(field.key, e.target.value)}
                placeholder={placeholder}
              />
            )}
          </Form.Item>
        );
      })}
    </>
  );
}
