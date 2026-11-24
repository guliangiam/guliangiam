import { App, Button, Form, Input, Typography } from "antd";
import { requestText } from "@/shared/api/http";
import { AUTH_SAML_ACS } from "@/shared/api/paths";
import { useTranslation } from "react-i18next";

export type SamlProviderFieldsProps = {
  value: Record<string, string>;
  onChange: (field: string, val: string) => void;
};

export function SamlProviderFields({ value, onChange }: SamlProviderFieldsProps) {
  const { t } = useTranslation("common");
  const { message } = App.useApp();
  // The only external-IdP callback is the SP ACS; replace the two placeholders
  // with this provider's organization slug and provider ID. The old
  // /api/v1/saml/acs path does not exist (it is the IdP-side response endpoint).
  const defaultSpAcsUrl = `${window.location.origin}${AUTH_SAML_ACS}/{organizationSlug}/{providerId}`;
  // spEntityId 后端必填且无默认值，SP metadata 端点当前未暴露；旧回退
  // /api/v1/saml/metadata 是 IdP metadata 路由形状（缺 applicationId 段），
  // 对 SP 不成立。以站点 origin 为建议值，实际由管理员确定并须与 IdP 侧注册一致。
  const defaultSpEntityId = window.location.origin;

  const handleFetchMetadata = async () => {
    const url = value.metadataUrl?.trim();
    if (!url) {
      message.warning("Please enter a metadata URL first");
      return;
    }
    try {
      const xml = await requestText(url);
      onChange("metadataXml", xml);

      // Try to extract SSO URL from XML
      const ssoMatch = xml.match(/<md:SingleSignOnService[^>]*Location="([^"]+)"/);
      if (ssoMatch) onChange("ssoUrl", ssoMatch[1]);

      // Try to extract SLO URL from XML
      const sloMatch = xml.match(/<md:SingleLogoutService[^>]*Location="([^"]+)"/);
      if (sloMatch) onChange("sloUrl", sloMatch[1]);

      message.success("Metadata fetched and parsed successfully");
    } catch {
      message.error("Failed to fetch metadata from URL");
    }
  };

  return (
    <>
      <Form.Item
        label={t("metadataUrl")}
        extra="IdP metadata URL - click Fetch to auto-populate fields"
      >
        <Input
          value={value.metadataUrl ?? ""}
          onChange={(e) => onChange("metadataUrl", e.target.value)}
          placeholder="https://idp.example.com/metadata"
          suffix={
            <Button type="link" size="small" onClick={handleFetchMetadata}>
              Fetch
            </Button>
          }
        />
      </Form.Item>
      <Form.Item
        label={t("metadataXml")}
        extra="SAML XML metadata (auto-populated from URL or paste manually)"
      >
        <Input.TextArea
          value={value.metadataXml ?? ""}
          onChange={(e) => onChange("metadataXml", e.target.value)}
          rows={6}
          placeholder="SAML XML metadata"
        />
      </Form.Item>
      <Form.Item label={t("ssoUrl")}>
        <Input
          value={value.ssoUrl ?? ""}
          onChange={(e) => onChange("ssoUrl", e.target.value)}
          placeholder="https://idp.example.com/sso"
        />
      </Form.Item>
      <Form.Item label={t("sloUrl")}>
        <Input
          value={value.sloUrl ?? ""}
          onChange={(e) => onChange("sloUrl", e.target.value)}
          placeholder="https://idp.example.com/slo"
        />
      </Form.Item>
      {/* SP endpoints display - read-only info */}
      <Form.Item
        label="SP ACS URL"
        extra="Service Provider Assertion Consumer Service URL (copy to IdP configuration). Replace {organizationSlug} and {providerId} with this provider's organization slug and provider ID."
      >
        <Input
          value={value.spAcsUrl ?? defaultSpAcsUrl}
          readOnly
          suffix={
            <Typography.Link
              onClick={() => {
                navigator.clipboard.writeText(value.spAcsUrl ?? defaultSpAcsUrl);
              }}
            >
              Copy
            </Typography.Link>
          }
        />
      </Form.Item>
      <Form.Item
        label="SP Entity ID"
        extra="Service Provider Entity ID (copy to IdP configuration)"
      >
        <Input
          value={value.spEntityId ?? defaultSpEntityId}
          readOnly
          suffix={
            <Typography.Link
              onClick={() => {
                navigator.clipboard.writeText(value.spEntityId ?? defaultSpEntityId);
              }}
            >
              Copy
            </Typography.Link>
          }
        />
      </Form.Item>
    </>
  );
}
