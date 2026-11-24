import { Avatar, Space, Typography } from "antd";
import { type FC, useMemo } from "react";

type ProviderInfo = {
  type: string;
  name: string;
  displayName: string;
};

type LinkedIdentity = {
  providerId: string;
  identityType: string;
  subject: string;
  providerAccount?: string;
};

type SamlWidgetProps = {
  provider: ProviderInfo;
  linkedIdentity?: LinkedIdentity | null;
  username?: string;
};

export const SamlWidget: FC<SamlWidgetProps> = ({ provider, linkedIdentity, username }) => {
  const displayName = useMemo(() => {
    if (!linkedIdentity) return "";
    return linkedIdentity.providerAccount ?? linkedIdentity.subject ?? username ?? "";
  }, [linkedIdentity, username]);

  return (
    <Space
      style={{
        width: "100%",
        padding: "8px 0",
        borderBottom: "1px solid var(--ant-color-border-secondary)",
      }}
    >
      <Avatar size="small" style={{ backgroundColor: "#722ed1" }}>
        SAML
      </Avatar>
      <Typography.Text strong style={{ minWidth: 80 }}>
        {provider.displayName || provider.type}
      </Typography.Text>
      {linkedIdentity ? (
        <Typography.Text type="secondary">{displayName}</Typography.Text>
      ) : (
        <Typography.Text type="secondary">(not linked)</Typography.Text>
      )}
    </Space>
  );
};
