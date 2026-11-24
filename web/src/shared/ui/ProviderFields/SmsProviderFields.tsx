import { Form, Input, Select } from "antd";
import { useTranslation } from "react-i18next";

export type SmsProviderFieldsProps = {
  value: Record<string, string>;
  onChange: (field: string, val: string) => void;
  subType?: string;
};

/** Provider types that use HTTP endpoint instead of access keys */
const HTTP_TYPES = new Set(["CustomHTTP", "Custom", "Custom HTTP SMS"]);

type FieldSpec = {
  /** Component-side key; mapped to a providers column via DynamicProviderFields. */
  key: string;
  labelKey: string;
  secret?: boolean;
  placeholder?: string;
  placeholderKey?: string;
};

/**
 * Per-vendor credential layouts. Keys follow each vendor's official naming
 * (e.g. 联麓 MchId/AppKey, 云片 API Key, Vonage API Key/Secret), while the
 * component key itself stays one of the generic bridge keys so values land on
 * the native columns the backend builder expects.
 */
const PROVIDER_FIELDS: Record<string, FieldSpec[]> = {
  TwilioSMS: [
    {
      key: "accessKeyId",
      labelKey: "accountSid",
      placeholder: "ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    },
    { key: "accessKeySecret", labelKey: "authToken", secret: true },
    { key: "signName", labelKey: "fromNumber", placeholder: "+15017122661" },
  ],
  "Twilio Verify": [
    {
      key: "accessKeyId",
      labelKey: "accountSid",
      placeholder: "ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    },
    { key: "accessKeySecret", labelKey: "authToken", secret: true },
    { key: "appId", labelKey: "serviceSid", placeholder: "VAxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" },
  ],
  AlibabaSMS: [
    { key: "accessKeyId", labelKey: "accessKeyId" },
    { key: "accessKeySecret", labelKey: "accessKeySecret", secret: true },
    { key: "signName", labelKey: "signName" },
    { key: "templateCode", labelKey: "templateCode", placeholder: "SMS_123456789" },
  ],
  "Alibaba Cloud PNVS SMS": [
    { key: "accessKeyId", labelKey: "accessKeyId" },
    { key: "accessKeySecret", labelKey: "accessKeySecret", secret: true },
    { key: "signName", labelKey: "signName" },
    { key: "templateCode", labelKey: "templateCode", placeholder: "SMS_123456789" },
  ],
  TencentSMS: [
    {
      key: "accessKeyId",
      labelKey: "secretId",
      placeholder: "AKIDxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    },
    { key: "accessKeySecret", labelKey: "secretKey", secret: true },
    { key: "appId", labelKey: "sdkAppId", placeholder: "1400000000" },
    { key: "signName", labelKey: "signName" },
    { key: "templateCode", labelKey: "templateId" },
    { key: "region", labelKey: "region", placeholder: "ap-guangzhou" },
  ],
  VolcengineSMS: [
    { key: "accessKeyId", labelKey: "accessKeyId" },
    { key: "accessKeySecret", labelKey: "accessKeySecret", secret: true },
    { key: "appId", labelKey: "smsAccount", placeholder: "8a1b2c3d" },
    { key: "signName", labelKey: "signName" },
    { key: "templateCode", labelKey: "templateId", placeholder: "ST_8a1b2c3d" },
  ],
  HuaweiSMS: [
    {
      key: "endpoint",
      labelKey: "endpoint",
      placeholder: "https://smsapi.cn-north-4.myhuaweicloud.com:443/sms/batchSendSms/v1",
    },
    { key: "accessKeyId", labelKey: "appKey" },
    { key: "accessKeySecret", labelKey: "appSecret", secret: true },
    { key: "signName", labelKey: "signature" },
    { key: "templateCode", labelKey: "templateId" },
  ],
  AmazonSNS: [
    { key: "accessKeyId", labelKey: "accessKeyId" },
    { key: "accessKeySecret", labelKey: "secretKey", secret: true },
    { key: "region", labelKey: "region", placeholder: "us-east-1" },
    { key: "signName", labelKey: "senderId" },
  ],
  AzureACSSMS: [
    {
      key: "accessKeySecret",
      labelKey: "connectionString",
      secret: true,
      placeholder: "endpoint=https://xxx.communication.azure.com/;accesskey=xxx",
    },
    { key: "signName", labelKey: "fromNumber", placeholder: "+15017122661" },
  ],
  Infobip: [
    { key: "endpoint", labelKey: "baseUrl", placeholder: "https://api.infobip.com" },
    { key: "accessKeySecret", labelKey: "apiKey", secret: true },
    { key: "signName", labelKey: "senderName" },
  ],
  UCloudSMS: [
    { key: "accessKeyId", labelKey: "publicKey" },
    { key: "accessKeySecret", labelKey: "privateKey", secret: true },
    { key: "appId", labelKey: "projectId" },
    { key: "signName", labelKey: "signName" },
    { key: "templateCode", labelKey: "templateId" },
  ],
  EmaySMS: [
    { key: "endpoint", labelKey: "endpoint", placeholder: "https://www.btom.cn:8088" },
    { key: "accessKeyId", labelKey: "serialNumber" },
    { key: "accessKeySecret", labelKey: "password", secret: true },
    { key: "signName", labelKey: "signName" },
  ],
  BaiduSMS: [
    { key: "accessKeyId", labelKey: "accessKey" },
    { key: "accessKeySecret", labelKey: "secretKey", secret: true },
    { key: "signName", labelKey: "signId", placeholder: "sms-sign-xxxxx" },
    { key: "templateCode", labelKey: "templateId", placeholder: "sms-tmpl-xxxxx" },
    { key: "httpHeaders", labelKey: "invokeId", placeholder: "sms-invoke-xxxxx" },
  ],
  Submail: [
    { key: "appId", labelKey: "appId" },
    { key: "accessKeySecret", labelKey: "appKey", secret: true },
    { key: "signName", labelKey: "signName" },
    { key: "templateCode", labelKey: "projectId" },
  ],
  MailtrapSMS: [
    { key: "accessKeySecret", labelKey: "apiKey", secret: true },
    { key: "signName", labelKey: "fromNumber", placeholder: "+15017122661" },
  ],
  OsonSMS: [
    { key: "endpoint", labelKey: "endpoint", placeholder: "https://api.osonsms.com" },
    { key: "accessKeySecret", labelKey: "apiKey", secret: true },
    { key: "signName", labelKey: "senderName" },
  ],
  "SmsBao SMS": [
    { key: "accessKeyId", labelKey: "username" },
    { key: "accessKeySecret", labelKey: "apiKey", secret: true },
    { key: "signName", labelKey: "signName" },
    { key: "templateCode", labelKey: "template" },
    { key: "httpHeaders", labelKey: "goodsId" },
  ],
  "Msg91 SMS": [
    { key: "accessKeySecret", labelKey: "authKey", secret: true },
    { key: "signName", labelKey: "senderId" },
    { key: "templateCode", labelKey: "templateId" },
  ],
  // Mock sender needs no credentials; rendering generic key fields would be
  // misleading (the builder ignores them entirely).
  MockSMS: [],
  UnismsSMS: [
    { key: "accessKeyId", labelKey: "mchId", placeholder: "MchId" },
    { key: "appId", labelKey: "appId", placeholder: "App Id" },
    { key: "accessKeySecret", labelKey: "appKey", secret: true },
    { key: "templateCode", labelKey: "templateId" },
  ],
  YunpianSMS: [
    { key: "accessKeySecret", labelKey: "apiKey", secret: true },
    { key: "signName", labelKey: "signName", placeholder: "Sign Name" },
    { key: "templateCode", labelKey: "templateCode" },
  ],
  JDCloudSMS: [
    { key: "accessKeyId", labelKey: "accessKeyId" },
    { key: "accessKeySecret", labelKey: "secretKey", secret: true },
    { key: "signName", labelKey: "signId", placeholder: "Sign ID" },
    { key: "templateCode", labelKey: "templateId" },
    { key: "region", labelKey: "region", placeholder: "cn-north-1" },
  ],
  VonageSMS: [
    { key: "accessKeyId", labelKey: "apiKey" },
    { key: "accessKeySecret", labelKey: "secretKey", secret: true },
    { key: "signName", labelKey: "fromNumber", placeholder: "Sender ID / MSISDN" },
  ],
  PlivoSMS: [
    { key: "accessKeyId", labelKey: "authId" },
    { key: "accessKeySecret", labelKey: "secretKey", secret: true },
    { key: "signName", labelKey: "fromNumber", placeholder: "Source Number" },
  ],
  TelnyxSMS: [
    { key: "accessKeySecret", labelKey: "apiKey", secret: true },
    { key: "signName", labelKey: "fromNumber", placeholder: "Sender Number" },
  ],
  SinchSMS: [
    { key: "accessKeyId", labelKey: "keyId" },
    { key: "accessKeySecret", labelKey: "secretKey", secret: true },
    { key: "appId", labelKey: "projectId", placeholder: "Project ID / Service Plan ID" },
    { key: "signName", labelKey: "fromNumber", placeholder: "Sender ID" },
  ],
  BirdSMS: [
    { key: "accessKeySecret", labelKey: "accessKey", secret: true },
    { key: "signName", labelKey: "fromNumber", placeholder: "Originator" },
  ],
  ClickatellSMS: [
    { key: "accessKeySecret", labelKey: "apiKey", secret: true },
    { key: "signName", labelKey: "fromNumber", placeholder: "Sender ID" },
  ],
  AfricastalkingSMS: [
    { key: "accessKeyId", labelKey: "username" },
    { key: "accessKeySecret", labelKey: "apiKey", secret: true },
    { key: "signName", labelKey: "fromNumber", placeholder: "Sender ID / Short Code" },
  ],
};

const DEFAULT_FIELDS: FieldSpec[] = [
  { key: "accessKeyId", labelKey: "accessKeyId", placeholder: "Access Key ID / Account SID" },
  {
    key: "accessKeySecret",
    labelKey: "accessKeySecret",
    secret: true,
    placeholderKey: "accessKeySecretOrAuthToken",
  },
  { key: "signName", labelKey: "signName", placeholder: "Sign Name / Sender ID" },
  { key: "templateCode", labelKey: "templateCode", placeholder: "Template Code" },
];

export function SmsProviderFields({ value, onChange, subType }: SmsProviderFieldsProps) {
  const { t } = useTranslation("common");
  const isHttp = HTTP_TYPES.has(subType ?? "");
  const fields = PROVIDER_FIELDS[subType ?? ""] ?? DEFAULT_FIELDS;

  return (
    <>
      {isHttp ? (
        <>
          <Form.Item label={t("endpoint")} extra="HTTP endpoint URL for sending SMS">
            <Input
              value={value.endpoint ?? ""}
              onChange={(e) => onChange("endpoint", e.target.value)}
              placeholder="https://sms-api.example.com/send"
            />
          </Form.Item>
          <Form.Item label={t("method")}>
            <Select
              value={value.method || "POST"}
              onChange={(val) => onChange("method", val)}
              options={[
                { label: "POST", value: "POST" },
                { label: "PUT", value: "PUT" },
                { label: "GET", value: "GET" },
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
              placeholder='{"X-API-Key":"xxx"}'
            />
          </Form.Item>
          <Form.Item
            label={t("customBody")}
            extra="Custom request body template (JSON with {{code}}, {{phone}} placeholders)"
          >
            <Input.TextArea
              value={value.customBody ?? ""}
              onChange={(e) => onChange("customBody", e.target.value)}
              rows={4}
              placeholder='{"to":"{{phone}}","message":"Your code is {{code}}"}'
            />
          </Form.Item>
        </>
      ) : (
        fields.map((field) => {
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
        })
      )}
    </>
  );
}
