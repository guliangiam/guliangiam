import { Alert, Button, Col, Popconfirm, Row, Space, Table, Tag, Tooltip, Typography } from "antd";
import { LinkOutlined } from "@ant-design/icons";
import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { UserIdentity } from "@/shared/api/iam/user-types";

type ProviderItem = {
  id?: string;
  name?: string;
  displayName?: string;
  category?: string;
  type?: string;
  clientId?: string;
};

type AccountLoginsSectionProps = {
  identities: UserIdentity[];
  providers: ProviderItem[] | undefined;
  /** 自助解绑（DELETE /auth/identities/{id}）。传入时每行展示解绑入口。 */
  deleteIdentityMutation?: UseMutationResult<void, Error, string>;
  /** 自助绑定：跳转托管认证重定向流程。传入时展示 provider 绑定列表。 */
  onLinkIdentity?: (providerId: string) => void;
};

export function AccountLoginsSection({
  identities,
  providers,
  deleteIdentityMutation,
  onLinkIdentity,
}: AccountLoginsSectionProps) {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");

  const providerName = (providerId: string) => {
    const provider = providers?.find((p) => p.id === providerId);
    return provider ? (provider.displayName ?? provider.name ?? providerId) : providerId;
  };

  const loginProviders = (providers ?? []).filter((p) => {
    const cat = p.category?.toLowerCase() ?? "";
    return cat === "oauth" || cat === "oidc" || cat === "saml";
  });

  const unlinkButton = (identity: UserIdentity) => {
    // identity.id 缺失（后端未返回持久化 id）时无法定位自助解绑端点，禁用入口。
    const disabled = !identity.id || !deleteIdentityMutation;
    return (
      <Popconfirm
        title={t("user.unlinkIdentityConfirm")}
        onConfirm={() => {
          if (identity.id) deleteIdentityMutation?.mutate(identity.id);
        }}
        okText={t("user.unlink")}
        cancelText={t("common:cancel")}
        okButtonProps={{ danger: true, loading: deleteIdentityMutation?.isPending }}
        disabled={disabled}
      >
        <Button size="small" danger disabled={disabled}>
          {t("user.unlink")}
        </Button>
      </Popconfirm>
    );
  };

  const columns = [
    {
      title: tc("providerName"),
      dataIndex: "providerId",
      render: (v: string) => providerName(v),
    },
    {
      title: t("user.identityType"),
      dataIndex: "identityType",
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: t("user.providerAccount"),
      dataIndex: "providerAccount",
      render: (v: string) => v || "-",
    },
    {
      title: t("user.linkedAt"),
      dataIndex: "linkedAt",
      render: (v: string) => (v ? new Date(v).toLocaleDateString() : "-"),
    },
    ...(deleteIdentityMutation
      ? [
          {
            title: t("account.actions"),
            key: "actions",
            render: (_: unknown, record: UserIdentity) => unlinkButton(record),
          },
        ]
      : []),
  ];

  return (
    <>
      {identities.length === 0 ? (
        <Alert
          type="info"
          title={t("account.noLinkedProviders")}
          description={t("account.noLinkedProvidersDesc")}
        />
      ) : (
        <Table
          size="small"
          pagination={false}
          dataSource={identities.map((id, i: number) => ({ key: i, ...id }))}
          columns={columns}
        />
      )}

      {onLinkIdentity && loginProviders.length > 0 && (
        <Space orientation="vertical" style={{ width: "100%", marginTop: 16 }} size="middle">
          {loginProviders.map((provider) => {
            const linkedIdentity = identities.find((i) => i.providerId === provider.id);
            const isLinked = !!linkedIdentity;
            return (
              <Row key={provider.id} align="middle" justify="space-between" wrap={false}>
                <Col>
                  <Space>
                    <Tag color={isLinked ? "green" : "default"}>{provider.category ?? "OAuth"}</Tag>
                    <Typography.Text>
                      {provider.displayName ?? provider.name ?? provider.id}
                    </Typography.Text>
                    {isLinked && linkedIdentity?.subject && (
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        ({linkedIdentity.subject})
                      </Typography.Text>
                    )}
                  </Space>
                </Col>
                <Col>
                  {isLinked && linkedIdentity ? (
                    unlinkButton(linkedIdentity)
                  ) : (
                    <Tooltip title={t("user.linkViaAuthFlow")}>
                      <Button
                        size="small"
                        type="primary"
                        icon={<LinkOutlined />}
                        onClick={() => provider.id && onLinkIdentity(provider.id)}
                      >
                        {t("user.link")}
                      </Button>
                    </Tooltip>
                  )}
                </Col>
              </Row>
            );
          })}
        </Space>
      )}

      <Typography.Paragraph type="secondary" style={{ marginTop: 16 }}>
        {t("account.linkedProvidersNote")}
      </Typography.Paragraph>
    </>
  );
}
