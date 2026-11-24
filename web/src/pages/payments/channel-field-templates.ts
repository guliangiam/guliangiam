// Structured credential templates for the channel-config create form.
//
// Keys mirror internal/infra/payment/factory.go exactly — the assembled JSON is
// sealed by ChannelConfigService and later decrypted into the string map the
// gateway factory reads. Credential specs follow the vendors' official docs:
// Square Bearer access token + location, Checkout.com secret key + webhook
// signing secret, WeChat Pay APIv3 merchant certificate key pair.

export type ChannelFieldDef = {
  /** Top-level key in the sealed config JSON. */
  key: string;
  labelKey: string;
  labelFallback: string;
  control: "text" | "password" | "textarea";
  required: boolean;
  placeholder?: string;
  extraKey?: string;
  extraFallback?: string;
};

export const CHANNEL_FIELD_TEMPLATES: Readonly<Record<string, readonly ChannelFieldDef[]>> = {
  square: [
    {
      key: "accessToken",
      labelKey: "paymentsAdmin.field.accessToken",
      labelFallback: "Access Token",
      control: "password",
      required: true,
      placeholder: "EAAA...",
    },
    {
      key: "locationId",
      labelKey: "paymentsAdmin.field.locationId",
      labelFallback: "Location ID",
      control: "text",
      required: true,
      placeholder: "L...",
    },
    {
      key: "webhookKey",
      labelKey: "paymentsAdmin.field.webhookKey",
      labelFallback: "Webhook Signature Key",
      control: "password",
      required: false,
      extraKey: "paymentsAdmin.field.webhookKeyExtra",
      extraFallback: "Optional; verifies Square webhook notification signatures.",
    },
    {
      key: "apiVersion",
      labelKey: "paymentsAdmin.field.apiVersion",
      labelFallback: "API Version",
      control: "text",
      required: false,
      placeholder: "2024-12-18",
      extraKey: "paymentsAdmin.field.apiVersionExtra",
      extraFallback: "Square-Version date header; the adapter defaults to 2024-12-18 when empty.",
    },
  ],
  checkout: [
    {
      key: "apiKey",
      labelKey: "paymentsAdmin.field.apiKey",
      labelFallback: "Secret Key",
      control: "password",
      required: true,
      placeholder: "sk_...",
    },
    {
      key: "webhookSecret",
      labelKey: "paymentsAdmin.field.webhookSecret",
      labelFallback: "Webhook Signing Secret",
      control: "password",
      required: false,
      extraKey: "paymentsAdmin.field.webhookSecretExtra",
      extraFallback: "Optional; verifies the Cko-Signature HMAC on webhook notifications.",
    },
  ],
  wechat_pay: [
    {
      key: "appId",
      labelKey: "paymentsAdmin.field.appId",
      labelFallback: "App ID",
      control: "text",
      required: true,
      placeholder: "wx8888...",
    },
    {
      key: "mchId",
      labelKey: "paymentsAdmin.field.mchId",
      labelFallback: "Merchant ID",
      control: "text",
      required: true,
      placeholder: "1900000109",
    },
    {
      key: "serialNo",
      labelKey: "paymentsAdmin.field.serialNo",
      labelFallback: "Merchant Certificate Serial No",
      control: "text",
      required: true,
      extraKey: "paymentsAdmin.field.serialNoExtra",
      extraFallback:
        "Serial number of the merchant API certificate (apiclient_cert.pem), not the platform certificate.",
    },
    {
      key: "privateKey",
      labelKey: "paymentsAdmin.field.privateKey",
      labelFallback: "Merchant API Private Key (PEM)",
      control: "textarea",
      required: true,
      placeholder: "-----BEGIN PRIVATE KEY-----",
      extraKey: "paymentsAdmin.field.privateKeyExtra",
      extraFallback:
        "Contents of apiclient_key.pem; signs every WeChat Pay APIv3 call with SHA256-RSA.",
    },
    {
      key: "platformCertPEM",
      labelKey: "paymentsAdmin.field.platformCertPEM",
      labelFallback: "Platform Certificate (PEM)",
      control: "textarea",
      required: true,
      placeholder: "-----BEGIN CERTIFICATE-----",
      extraKey: "paymentsAdmin.field.platformCertPEMExtra",
      extraFallback: "WeChat Pay platform certificate bundle used to verify responses.",
    },
  ],
};

export function getChannelTemplate(
  channel: string | undefined,
): readonly ChannelFieldDef[] | undefined {
  if (!channel) return undefined;
  return CHANNEL_FIELD_TEMPLATES[channel];
}

/**
 * Assemble template field values into the plaintext config JSON the existing
 * payload path expects. Empty optional fields are omitted (the factory treats
 * missing keys as empty strings); sandbox environments inject isSandbox="true",
 * the only spelling sandboxEnabled accepts.
 */
export function assembleChannelTemplateConfig(
  template: readonly ChannelFieldDef[],
  environment: string,
  values: Record<string, string | undefined>,
): string {
  const config: Record<string, string> = {};
  for (const field of template) {
    const value = (values[field.key] ?? "").trim();
    if (field.required && !value) {
      throw new Error(`missing required channel config field: ${field.key}`);
    }
    if (value) config[field.key] = value;
  }
  if (environment === "sandbox") config.isSandbox = "true";
  return JSON.stringify(config);
}
