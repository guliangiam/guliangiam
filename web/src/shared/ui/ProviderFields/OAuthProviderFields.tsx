import { Form, Input, Typography } from "antd";
import { useTranslation } from "react-i18next";

export type OAuthProviderFieldsProps = {
  value: Record<string, string>;
  onChange: (field: string, val: string) => void;
  subType?: string;
};

/** Provider types that show Domain / Tenant ID field */
const DOMAIN_TYPES = new Set(["AzureAD", "AzureADB2C", "ADFS", "Okta", "Nextcloud"]);

/** Provider types that support certificate-based authentication */
const CERT_TYPES = new Set(["AzureAD", "AzureADB2C", "ADFS", "Okta"]);

export function OAuthProviderFields({ value, onChange, subType }: OAuthProviderFieldsProps) {
  const { t } = useTranslation("common");
  const type = subType ?? "";

  return (
    <>
      <Form.Item label={t("clientId")}>
        <Input
          value={value.clientId ?? ""}
          onChange={(e) => onChange("clientId", e.target.value)}
          placeholder="Client ID / App ID"
        />
      </Form.Item>
      <Form.Item
        label={t("clientSecret")}
        extra={
          type === "Alipay"
            ? "Alipay: enter the RSA2 application private key (PEM, PKCS#1/PKCS#8) here — not an OAuth client secret"
            : undefined
        }
      >
        <Input.Password
          value={value.clientSecret ?? ""}
          onChange={(e) => onChange("clientSecret", e.target.value)}
          placeholder={
            type === "Alipay"
              ? "RSA2 private key PEM (-----BEGIN RSA PRIVATE KEY-----)"
              : t("leaveEmptyToKeepCurrent")
          }
        />
      </Form.Item>

      {/* 证书（certId）没有落库列；页面级 cert(PEM) 已覆盖该能力，这里只做明确标注。 */}
      {CERT_TYPES.has(type) && (
        <Typography.Text type="secondary">
          {t("providerFieldNotPersisted", { field: "Certificate ID" })}
        </Typography.Text>
      )}

      {/* ── Generic OAuth fields (hidden for Custom type since they have dedicated fields) ── */}
      {type !== "Custom" && (
        <>
          <Form.Item label={t("authUrl")}>
            <Input
              value={value.authUrl ?? ""}
              onChange={(e) => onChange("authUrl", e.target.value)}
              placeholder="https://github.com/login/oauth/authorize"
            />
          </Form.Item>
          <Form.Item label={t("tokenUrl")}>
            <Input
              value={value.tokenUrl ?? ""}
              onChange={(e) => onChange("tokenUrl", e.target.value)}
              placeholder="https://github.com/login/oauth/access_token"
            />
          </Form.Item>
          <Form.Item label={t("userInfoUrl")}>
            <Input
              value={value.userInfoUrl ?? ""}
              onChange={(e) => onChange("userInfoUrl", e.target.value)}
              placeholder="https://api.github.com/user"
            />
          </Form.Item>
        </>
      )}

      <Form.Item label={t("scope")}>
        <Input
          value={value.scope ?? ""}
          onChange={(e) => onChange("scope", e.target.value)}
          placeholder="openid, profile, email"
        />
      </Form.Item>
      <Form.Item label={t("redirectUri")}>
        <Input
          value={value.redirectUri ?? ""}
          onChange={(e) => onChange("redirectUri", e.target.value)}
          placeholder="https://your-app.com/callback"
        />
      </Form.Item>

      {/* ── WeChat specific fields ── */}
      {type === "WeChat" && (
        <>
          <Form.Item
            label="AppID (Mobile)"
            extra="WeChat Mobile AppID (separate from Client ID for Open Platform)"
          >
            <Input
              value={value.clientId2 ?? ""}
              onChange={(e) => onChange("clientId2", e.target.value)}
              placeholder="wx Mobile AppID"
            />
          </Form.Item>
          <Typography.Text type="secondary">
            {t("providerFieldNotPersisted", { field: "Use Media Platform" })}
          </Typography.Text>
        </>
      )}

      {/* ── WeCom specific fields ── */}
      {type === "WeCom" && (
        <Form.Item
          label="AgentID"
          extra="WeCom self-built app AgentId — required for scan login; Client ID holds the CorpID"
        >
          <Input
            value={value.agentId ?? ""}
            onChange={(e) => onChange("agentId", e.target.value)}
            placeholder="1000002"
          />
        </Form.Item>
      )}

      {/* ── Alipay specific fields ── */}
      {type === "Alipay" && (
        <Typography.Text type="secondary">
          {t("providerFieldNotPersisted", { field: "Cert ID / Root Cert ID" })}
        </Typography.Text>
      )}

      {/* ── Google specific fields ── */}
      {type === "Google" && (
        <Typography.Text type="secondary">
          {t("providerFieldNotPersisted", { field: "Fetch Phone Number" })}
        </Typography.Text>
      )}

      {/* ── Lark specific fields ── */}
      {type === "Lark" && (
        <Typography.Text type="secondary">
          {t("providerFieldNotPersisted", { field: "Fetch Phone Number" })}
        </Typography.Text>
      )}

      {/* ── Domain / Tenant ID for enterprise IdPs ── */}
      {DOMAIN_TYPES.has(type) && (
        <Form.Item
          label="Domain / Tenant ID"
          extra="Azure AD tenant domain, Okta domain, or ADFS federation name"
        >
          <Input
            value={value.domain ?? ""}
            onChange={(e) => onChange("domain", e.target.value)}
            placeholder="contoso.onmicrosoft.com"
          />
        </Form.Item>
      )}

      {/* ── Custom OAuth fields ── */}
      {type === "Custom" && (
        <>
          <Form.Item label="Custom Auth URL" extra="Authorization endpoint URL">
            <Input
              value={value.customAuthUrl ?? ""}
              onChange={(e) => onChange("customAuthUrl", e.target.value)}
              placeholder="https://idp.example.com/authorize"
            />
          </Form.Item>
          <Form.Item label={t("customTokenUrl")} extra="Token endpoint URL">
            <Input
              value={value.customTokenUrl ?? ""}
              onChange={(e) => onChange("customTokenUrl", e.target.value)}
              placeholder="https://idp.example.com/token"
            />
          </Form.Item>
          <Form.Item label="Custom UserInfo URL" extra="User info endpoint URL">
            <Input
              value={value.customUserInfoUrl ?? ""}
              onChange={(e) => onChange("customUserInfoUrl", e.target.value)}
              placeholder="https://idp.example.com/userinfo"
            />
          </Form.Item>
          {/* Custom Logout URL / Custom Logo URL / Enable Proxy 由页面级统一字段维护，
              这里不再重复渲染，避免两个控件写同一列。 */}
        </>
      )}
    </>
  );
}
