import { Form, Input, Select, Switch } from "antd";
import type { FormInstance } from "antd";
import { useTranslation } from "react-i18next";
import { PAYMENT_CURRENCY_OPTIONS } from "@/shared/lib/currency-utils";

/**
 * Field configuration for each payment provider type.
 * Maps a provider type value to an array of field definitions
 * that should be rendered in the form.
 */
type FieldDef = {
  formField: string;
  labelKey: string;
  placeholder?: string;
  type: "input" | "password" | "select" | "switch";
  options?: { label: string; value: string }[];
};

const PAYMENT_FIELDS: Record<string, FieldDef[]> = {
  Stripe: [
    {
      formField: "password",
      labelKey: "provider.payment.apiKey",
      placeholder: "sk_live_...",
      type: "password",
    },
    {
      formField: "contentType",
      labelKey: "provider.payment.currency",
      type: "select",
      options: [...PAYMENT_CURRENCY_OPTIONS],
    },
  ],
  Alipay: [
    {
      formField: "appId",
      labelKey: "provider.payment.appId",
      placeholder: "2021...",
      type: "input",
    },
    {
      formField: "clientSecret",
      labelKey: "provider.payment.privateKey",
      placeholder: "MIIEvQ...",
      type: "password",
    },
    {
      formField: "clientSecret2",
      labelKey: "provider.payment.publicKey",
      placeholder: "MIIBIjAN...",
      type: "password",
    },
    { formField: "subType", labelKey: "provider.payment.sandbox", type: "switch" },
    {
      formField: "contentType",
      labelKey: "provider.payment.currency",
      type: "select",
      options: [...PAYMENT_CURRENCY_OPTIONS],
    },
  ],
  WeChatPay: [
    {
      formField: "appId",
      labelKey: "provider.payment.appId",
      placeholder: "wx8888...",
      type: "input",
    },
    {
      formField: "clientId2",
      labelKey: "provider.payment.mchId",
      placeholder: "1900000109",
      type: "input",
    },
    {
      formField: "region",
      labelKey: "provider.payment.certSerialNo",
      placeholder: "Serial number of merchant certificate",
      type: "input",
    },
    {
      formField: "host",
      labelKey: "provider.payment.host",
      placeholder: "api.mch.weixin.qq.com",
      type: "input",
    },
    { formField: "subType", labelKey: "provider.payment.sandbox", type: "switch" },
    {
      formField: "contentType",
      labelKey: "provider.payment.currency",
      type: "select",
      options: [...PAYMENT_CURRENCY_OPTIONS],
    },
  ],
  PayPal: [
    {
      formField: "clientId",
      labelKey: "provider.payment.clientId",
      placeholder: "AXs1xR...",
      type: "input",
    },
    {
      formField: "clientSecret",
      labelKey: "provider.payment.clientSecret",
      placeholder: "EBvjA...",
      type: "password",
    },
    { formField: "subType", labelKey: "provider.payment.sandbox", type: "switch" },
    {
      formField: "contentType",
      labelKey: "provider.payment.currency",
      type: "select",
      options: [...PAYMENT_CURRENCY_OPTIONS],
    },
  ],
  Adyen: [
    {
      formField: "password",
      labelKey: "provider.payment.apiKey",
      placeholder: "AQEyhm...",
      type: "password",
    },
    {
      formField: "admin",
      labelKey: "provider.payment.merchantId",
      placeholder: "YourMerchantAccount",
      type: "input",
    },
    {
      formField: "clientSecret2",
      labelKey: "provider.payment.hmacKey",
      placeholder: "Hex-encoded HMAC key",
      type: "password",
    },
    {
      formField: "host",
      labelKey: "provider.payment.host",
      placeholder: "checkout-live.adyen.com",
      type: "input",
    },
    { formField: "subType", labelKey: "provider.payment.sandbox", type: "switch" },
    {
      formField: "contentType",
      labelKey: "provider.payment.currency",
      type: "select",
      options: [...PAYMENT_CURRENCY_OPTIONS],
    },
  ],
  AirWallex: [
    {
      formField: "clientId",
      labelKey: "provider.payment.clientId",
      placeholder: "T2h4bXl...",
      type: "input",
    },
    {
      formField: "password",
      labelKey: "provider.payment.apiKey",
      placeholder: "Your AirWallex API key",
      type: "password",
    },
    {
      formField: "clientSecret2",
      labelKey: "provider.payment.webhookSecret",
      placeholder: "Webhook signing secret",
      type: "password",
    },
    { formField: "subType", labelKey: "provider.payment.sandbox", type: "switch" },
    {
      formField: "contentType",
      labelKey: "provider.payment.currency",
      type: "select",
      options: [...PAYMENT_CURRENCY_OPTIONS],
    },
  ],
  Paddle: [
    {
      formField: "password",
      labelKey: "provider.payment.apiToken",
      placeholder: "Paddle API token",
      type: "password",
    },
    {
      formField: "clientSecret2",
      labelKey: "provider.payment.webhookSecret",
      placeholder: "Webhook signing secret",
      type: "password",
    },
    { formField: "subType", labelKey: "provider.payment.sandbox", type: "switch" },
    {
      formField: "contentType",
      labelKey: "provider.payment.currency",
      type: "select",
      options: [...PAYMENT_CURRENCY_OPTIONS],
    },
  ],
  Polar: [
    {
      formField: "password",
      labelKey: "provider.payment.accessToken",
      placeholder: "Polar access token",
      type: "password",
    },
    {
      formField: "clientSecret2",
      labelKey: "provider.payment.webhookSecret",
      placeholder: "Webhook signing secret",
      type: "password",
    },
    { formField: "subType", labelKey: "provider.payment.sandbox", type: "switch" },
    {
      formField: "contentType",
      labelKey: "provider.payment.currency",
      type: "select",
      options: [...PAYMENT_CURRENCY_OPTIONS],
    },
  ],
  FastSpring: [
    {
      formField: "admin",
      labelKey: "provider.payment.apiUsername",
      placeholder: "FastSpring API username",
      type: "input",
    },
    {
      formField: "password",
      labelKey: "provider.payment.apiPassword",
      placeholder: "FastSpring API password",
      type: "password",
    },
    {
      formField: "appId",
      labelKey: "provider.payment.storefrontPath",
      placeholder: "/shop/example",
      type: "input",
    },
  ],
  LemonSqueezy: [
    {
      formField: "password",
      labelKey: "provider.payment.apiKey",
      placeholder: "LemonSqueezy API key",
      type: "password",
    },
    {
      formField: "clientId2",
      labelKey: "provider.payment.storeId",
      placeholder: "Store ID",
      type: "input",
    },
    {
      formField: "clientSecret2",
      labelKey: "provider.payment.webhookSecret",
      placeholder: "Webhook signing secret",
      type: "password",
    },
  ],
  GC: [
    {
      formField: "clientId2",
      labelKey: "provider.payment.xmpch",
      placeholder: "Merchant ID",
      type: "input",
    },
    {
      formField: "password",
      labelKey: "provider.payment.secretKey",
      placeholder: "Signing secret key",
      type: "password",
    },
    {
      formField: "host",
      labelKey: "provider.payment.host",
      placeholder: "Payment gateway endpoint",
      type: "input",
    },
  ],
};

/** Default fields shown for unknown/custom payment types */
const DEFAULT_FIELDS: FieldDef[] = [
  {
    formField: "clientId2",
    labelKey: "provider.payment.merchantId",
    placeholder: "Merchant ID / Second Client ID",
    type: "input",
  },
  {
    formField: "clientSecret2",
    labelKey: "provider.payment.clientSecret2",
    placeholder: "Second Client Secret",
    type: "password",
  },
  {
    formField: "host",
    labelKey: "provider.payment.host",
    placeholder: "API endpoint URL",
    type: "input",
  },
  {
    formField: "admin",
    labelKey: "provider.payment.admin",
    placeholder: "Username / Merchant Account",
    type: "input",
  },
  {
    formField: "password",
    labelKey: "provider.payment.apiKey",
    placeholder: "API Key / Secret",
    type: "password",
  },
  {
    formField: "contentType",
    labelKey: "provider.payment.currency",
    type: "select",
    options: [...PAYMENT_CURRENCY_OPTIONS],
  },
  {
    formField: "appId",
    labelKey: "provider.payment.appId",
    placeholder: "Certificate / App ID",
    type: "input",
  },
];

interface PaymentProviderFieldsProps {
  form?: FormInstance;
}

export function PaymentProviderFields({ form }: PaymentProviderFieldsProps) {
  const { t } = useTranslation("resource");
  const providerType = Form.useWatch("providerType", form) ?? "";
  const fields = PAYMENT_FIELDS[providerType] ?? DEFAULT_FIELDS;

  return (
    <>
      {fields.map((field) => {
        if (field.type === "switch") {
          return (
            <Form.Item
              key={field.formField}
              label={t(field.labelKey)}
              name={field.formField}
              valuePropName="checked"
              getValueFromEvent={(checked: boolean) => (checked ? "true" : "")}
              getValueProps={(value: string) => ({ checked: value === "true" })}
            >
              <Switch />
            </Form.Item>
          );
        }

        if (field.type === "select") {
          return (
            <Form.Item key={field.formField} label={t(field.labelKey)} name={field.formField}>
              <Select allowClear options={field.options} />
            </Form.Item>
          );
        }

        if (field.type === "password") {
          return (
            <Form.Item key={field.formField} label={t(field.labelKey)} name={field.formField}>
              <Input.Password placeholder={field.placeholder} />
            </Form.Item>
          );
        }

        return (
          <Form.Item key={field.formField} label={t(field.labelKey)} name={field.formField}>
            <Input placeholder={field.placeholder} />
          </Form.Item>
        );
      })}
    </>
  );
}
