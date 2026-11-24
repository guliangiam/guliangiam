import { Button, Space, Typography } from "antd";
import { LinkOutlined, DisconnectOutlined } from "@ant-design/icons";
import { type FC, useCallback, useMemo } from "react";
import { ProviderIcon } from "@/shared/ui/ProviderIcon";

type ProviderInfo = {
  type: string;
  name: string;
  displayName: string;
  category: string;
};

type LinkedIdentity = {
  providerId: string;
  identityType: string;
  subject: string;
  providerAccount?: string;
  profileSnapshot?: Record<string, unknown>;
};

type OAuthWidgetProps = {
  provider: ProviderInfo;
  linkedIdentity?: LinkedIdentity | null;
  canUnlink?: boolean;
  canLink?: boolean;
  onLink?: () => void;
  onUnlink?: () => void;
  loading?: boolean;
};

function getLinkedName(identity: LinkedIdentity): string {
  const snapshot = identity.profileSnapshot ?? {};
  return (
    (snapshot.displayName as string) ??
    (snapshot.username as string) ??
    identity.providerAccount ??
    identity.subject ??
    ""
  );
}

export const OAuthWidget: FC<OAuthWidgetProps> = ({
  provider,
  linkedIdentity,
  canUnlink = true,
  canLink = true,
  onLink,
  onUnlink,
  loading,
}) => {
  const isLinked = !!linkedIdentity;
  const linkedName = useMemo(
    () => (linkedIdentity ? getLinkedName(linkedIdentity) : ""),
    [linkedIdentity],
  );

  const handleLink = useCallback(() => {
    onLink?.();
  }, [onLink]);

  const handleUnlink = useCallback(() => {
    onUnlink?.();
  }, [onUnlink]);

  return (
    <Space
      style={{
        width: "100%",
        padding: "8px 0",
        borderBottom: "1px solid var(--ant-color-border-secondary)",
      }}
    >
      <ProviderIcon type={provider.type} size={32} />
      <Typography.Text strong style={{ minWidth: 80 }}>
        {provider.displayName || provider.type}
      </Typography.Text>

      {isLinked ? (
        <>
          <Typography.Text type="secondary">{linkedName}</Typography.Text>
          <Button
            size="small"
            danger
            icon={<DisconnectOutlined />}
            disabled={!canUnlink}
            loading={loading}
            onClick={handleUnlink}
          >
            Unlink
          </Button>
        </>
      ) : (
        <>
          <Typography.Text type="secondary">(not linked)</Typography.Text>
          <Button
            size="small"
            type="primary"
            icon={<LinkOutlined />}
            disabled={!canLink}
            loading={loading}
            onClick={handleLink}
          >
            Link
          </Button>
        </>
      )}
    </Space>
  );
};
