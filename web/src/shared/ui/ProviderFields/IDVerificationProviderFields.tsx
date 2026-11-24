import { Form, Input } from "antd";
import type { FormInstance } from "antd";
import { useTranslation } from "react-i18next";

/**
 * IDVerification credentials are provider-type-specific. The runtime resolver
 * (internal/app/providers/idv_resolver.go) maps each kind onto different columns:
 *   - AliyunIdv / TencentIdv: APIKey=admin, APISecret=password, SceneID=config.idvSceneId
 *   - Jumio / Veriff:         APIKey=clientId, APISecret=clientSecret
 *   - Onfido / Persona:       APIKey=clientId, SceneID=config.idvSceneId
 * Rendering the matching controls with unambiguous labels prevents the
 * mis-mapping (AccessKeyId dropped into appId, secret left empty) that previously
 * surfaced as real-name "服务不可用" (503).
 */
export function IDVerificationProviderFields({ form }: { form?: FormInstance }) {
  const { t } = useTranslation("common");
  const providerType = (Form.useWatch("providerType", form) as string | undefined) ?? "";
  const keepHint = "Leave empty to keep current";

  const accessKeyPair = providerType === "AliyunIdv" || providerType === "TencentIdv";
  const clientSecretPair = providerType === "Jumio" || providerType === "Veriff";
  const singleToken = providerType === "Onfido" || providerType === "Persona";
  const isTencent = providerType === "TencentIdv";

  return (
    <>
      {accessKeyPair && (
        <>
          <Form.Item label={t("accessKeyId")} name="admin">
            <Input placeholder={isTencent ? "SecretId" : "AccessKey ID"} />
          </Form.Item>
          <Form.Item label={t("accessKeySecret")} name="password">
            <Input.Password placeholder={isTencent ? "SecretKey" : "AccessKey Secret"} />
          </Form.Item>
          <Form.Item label={t("idvSceneId")} name="idvSceneId">
            <Input placeholder={isTencent ? "RuleId / SceneId" : "SceneId"} />
          </Form.Item>
        </>
      )}
      {clientSecretPair && (
        <>
          <Form.Item label={t("clientId")} name="clientId">
            <Input placeholder={providerType === "Jumio" ? "API token" : "API key"} />
          </Form.Item>
          <Form.Item label={t("clientSecret")} name="clientSecret">
            <Input.Password placeholder={keepHint} />
          </Form.Item>
          <Form.Item label={t("clientSecret2")} name="clientSecret2">
            <Input.Password placeholder="Webhook secret (optional)" />
          </Form.Item>
        </>
      )}
      {singleToken && (
        <>
          <Form.Item label={t("clientId")} name="clientId">
            <Input placeholder={providerType === "Onfido" ? "API token" : "API key"} />
          </Form.Item>
          <Form.Item label={t("idvSceneId")} name="idvSceneId">
            <Input placeholder={providerType === "Onfido" ? "Workflow ID" : "Template ID"} />
          </Form.Item>
          <Form.Item label={t("clientSecret2")} name="clientSecret2">
            <Input.Password placeholder="Webhook secret (optional)" />
          </Form.Item>
        </>
      )}
      {!accessKeyPair && !clientSecretPair && !singleToken && (
        <>
          <Form.Item label={t("clientId")} name="clientId">
            <Input placeholder="API key / token" />
          </Form.Item>
          <Form.Item label={t("clientSecret")} name="clientSecret">
            <Input.Password placeholder={keepHint} />
          </Form.Item>
          <Form.Item label={t("idvSceneId")} name="idvSceneId">
            <Input placeholder="SceneId" />
          </Form.Item>
        </>
      )}
      <Form.Item label={t("providerUrl")} name="providerUrl">
        <Input placeholder="Endpoint URL (optional override)" />
      </Form.Item>
    </>
  );
}
