import { Form, Input } from "antd";
import { useTranslation } from "react-i18next";

/**
 * Face ID providers. The admin face-id-providers page offers Alibaba Cloud
 * Facebody and Local UniFace; this generic Provider form keeps the vendor
 * fields used by either adapter.
 */
export function FaceIdProviderFields() {
  const { t } = useTranslation("common");
  return (
    <>
      <Form.Item label={t("providerUrl")} name="providerUrl">
        <Input placeholder="https://faceid.example.com/api" />
      </Form.Item>
      <Form.Item label={t("appId")} name="appId">
        <Input placeholder="App ID" />
      </Form.Item>
      <Form.Item label={t("accessKey")} name="admin">
        <Input placeholder="Access Key ID" />
      </Form.Item>
      <Form.Item label={t("accessSecret")} name="password">
        <Input.Password placeholder="Access Key Secret" />
      </Form.Item>
      <Form.Item label={t("endpoint")} name="host">
        <Input placeholder="Endpoint URL (optional override)" />
      </Form.Item>
    </>
  );
}
