import { Card, Descriptions, Typography, Skeleton, Tag, Button, message } from "antd";
import { useQuery } from "@tanstack/react-query";
import { requestDocumentJson } from "@/shared/api/http";
import { useTranslation } from "react-i18next";
import { CopyOutlined } from "@ant-design/icons";

const { Title, Text, Paragraph } = Typography;

type OidcConfig = {
  issuer: string;
  authorization_endpoint: string;
  token_endpoint: string;
  userinfo_endpoint: string;
  jwks_uri: string;
  registration_endpoint: string;
  scopes_supported: string[];
  response_types_supported: string[];
  code_challenge_methods_supported: string[];
  token_endpoint_auth_methods_supported: string[];
  subject_types_supported: string[];
  id_token_signing_alg_values_supported: string[];
  grant_types_supported: string[];
  end_session_endpoint: string;
  backchannel_logout_supported: boolean;
  backchannel_logout_session_supported: boolean;
};

export default function OidcDiscoveryPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");

  const query = useQuery({
    queryKey: ["oidc-discovery"],
    queryFn: async () => {
      const cfg = await requestDocumentJson<OidcConfig>("/.well-known/openid-configuration");
      if (!cfg) throw new Error("Failed to fetch OIDC configuration");
      return cfg;
    },
  });

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text).then(
      () => message.success(tc("copySuccess")),
      () => message.error(tc("copyFailed")),
    );
  };

  if (query.isLoading) {
    return (
      <div style={{ padding: 24, textAlign: "center" }}>
        <Skeleton active paragraph={{ rows: 6 }} />
      </div>
    );
  }

  if (query.error) {
    return (
      <Card>
        <Title level={4}>{t("oidcDiscovery.title")}</Title>
        <Paragraph type="danger">{t("oidcDiscovery.fetchError")}</Paragraph>
      </Card>
    );
  }

  const config = query.data;
  if (!config) return null;

  const endpointKeys = [
    "issuer",
    "authorization_endpoint",
    "token_endpoint",
    "userinfo_endpoint",
    "jwks_uri",
    "registration_endpoint",
    "end_session_endpoint",
  ] as const;

  const endpointValues: Record<string, string> = {
    issuer: config.issuer,
    authorization_endpoint: config.authorization_endpoint,
    token_endpoint: config.token_endpoint,
    userinfo_endpoint: config.userinfo_endpoint,
    jwks_uri: config.jwks_uri,
    registration_endpoint: config.registration_endpoint,
    end_session_endpoint: config.end_session_endpoint,
  };

  const capabilityKeys = [
    "scopes_supported",
    "response_types_supported",
    "grant_types_supported",
    "code_challenge_methods_supported",
    "id_token_signing_alg_values_supported",
  ] as const;

  const capabilityArrays: Record<string, string[] | undefined> = {
    scopes_supported: config.scopes_supported,
    response_types_supported: config.response_types_supported,
    grant_types_supported: config.grant_types_supported,
    code_challenge_methods_supported: config.code_challenge_methods_supported,
    id_token_signing_alg_values_supported: config.id_token_signing_alg_values_supported,
  };

  return (
    <div style={{ padding: 24 }}>
      <Title level={4}>{t("oidcDiscovery.title")}</Title>
      <Paragraph type="secondary">{t("oidcDiscovery.description")}</Paragraph>

      <Card title={t("oidcDiscovery.endpoints")} style={{ marginBottom: 16 }}>
        <Descriptions column={1} bordered size="small">
          {endpointKeys.map((key) => (
            <Descriptions.Item key={key} label={t(`oidcDiscovery.endpoint.${key}`)}>
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <Text code>{endpointValues[key]}</Text>
                <Button
                  type="text"
                  size="small"
                  icon={<CopyOutlined />}
                  onClick={() => copyToClipboard(endpointValues[key])}
                />
              </div>
            </Descriptions.Item>
          ))}
        </Descriptions>
      </Card>

      <Card title={t("oidcDiscovery.capabilities")} style={{ marginBottom: 16 }}>
        <Descriptions column={1} bordered size="small">
          {capabilityKeys.map((key) => (
            <Descriptions.Item key={key} label={t(`oidcDiscovery.capability.${key}`)}>
              {capabilityArrays[key]?.map((s) => (
                <Tag key={s}>{s}</Tag>
              ))}
            </Descriptions.Item>
          ))}
          <Descriptions.Item label={t("oidcDiscovery.capability.backchannel_logout_supported")}>
            <Tag color={config.backchannel_logout_supported ? "green" : "red"}>
              {config.backchannel_logout_supported ? t("oidcDiscovery.yes") : t("oidcDiscovery.no")}
            </Tag>
          </Descriptions.Item>
        </Descriptions>
      </Card>
    </div>
  );
}
