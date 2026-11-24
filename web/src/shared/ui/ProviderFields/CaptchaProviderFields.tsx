import { Form, Input, InputNumber, Select } from "antd";
import type { FormInstance } from "antd";
import { useTranslation } from "react-i18next";

import { captchaUITypeToKind } from "../../hooks/useProviderCategories";
import { CAPTCHA_CONFIG_FIELDS, CAPTCHA_CREDENTIAL_LABELS } from "./captcha-constants";

export function CaptchaProviderFields({ form }: { form?: FormInstance }) {
  const { t } = useTranslation("common");
  const watchedType = Form.useWatch("providerType", form);
  const kind = captchaUITypeToKind(String(watchedType ?? ""));
  const labels = CAPTCHA_CREDENTIAL_LABELS[kind] ?? CAPTCHA_CREDENTIAL_LABELS.default;
  const specs = CAPTCHA_CONFIG_FIELDS[kind] ?? [];

  return (
    <>
      <Form.Item label={t(labels.idLabelKey)} name="clientId">
        <Input placeholder={t(labels.idLabelKey)} autoComplete="off" />
      </Form.Item>
      <Form.Item label={t(labels.secretLabelKey)} name="clientSecret">
        <Input.Password placeholder={t(labels.secretLabelKey)} autoComplete="new-password" />
      </Form.Item>
      {specs.map((spec) => (
        <Form.Item key={spec.formName} label={t(spec.labelKey)} name={spec.formName}>
          {spec.input === "number" ? (
            <InputNumber min={0} max={1} step={0.05} style={{ width: "100%" }} />
          ) : spec.input === "password" ? (
            <Input.Password placeholder={t(spec.labelKey)} autoComplete="new-password" />
          ) : spec.input === "select" ? (
            <Select
              placeholder={t(spec.labelKey)}
              options={(spec.options ?? []).map((option) => ({
                label: t(option.labelKey),
                value: option.value,
              }))}
            />
          ) : (
            <Input placeholder={t(spec.labelKey)} autoComplete="off" />
          )}
        </Form.Item>
      ))}
    </>
  );
}
