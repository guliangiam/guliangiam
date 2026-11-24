import { useState } from "react";
import { Button, Col, Popconfirm, Row, Space, Table, Tag, Tooltip, Typography } from "antd";
import { DeleteOutlined, LinkOutlined, PlusOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { FormSection } from "@/shared/ui/FormSection";
import { OAuthWidget } from "@/shared/ui/OAuthWidget";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { SamlWidget } from "@/shared/ui/SamlWidget";
import { ProviderInlineCreateModal } from "@/shared/ui/ProviderInlineCreateModal";
import { formatDateTime } from "./types";
import type { IdentityFormItem, ProviderItem, UserDetail } from "./types";

interface UserIdentityTabProps {
  user: UserDetail | undefined;
  identities: IdentityFormItem[];
  providers: ProviderItem[] | undefined;
  onDeleteIdentity: (index: number) => void;
  onAddIdentity: () => void;
  onLinkIdentity: (providerId: string) => void;
}

export default function UserIdentityTab({
  user: _user,
  identities,
  providers,
  onDeleteIdentity,
  onAddIdentity,
  onLinkIdentity,
}: UserIdentityTabProps) {
  const { t } = useTranslation();

  // 提供商名称统一从 provider 列表解析；找不到时回退到 id，避免空白。
  const providerName = (providerId: string) =>
    providers?.find((p) => p.id === providerId)?.displayName ?? providerId;
  const providerCategory = (providerId: string) =>
    providers?.find((p) => p.id === providerId)?.category ?? "oauth";

  const oauthOidcSamlProviders = (providers ?? []).filter((p) => {
    const cat = p.category?.toLowerCase() ?? "";
    return cat === "oauth" || cat === "oidc" || cat === "saml";
  });

  // 内联新建第三方登录 Provider：创建后 ["providers"] 失效，本 Tab 的 provider 列表自动刷新。
  const [createProviderOpen, setCreateProviderOpen] = useState(false);

  return (
    <Space orientation="vertical" style={{ width: "100%" }} size="large">
      {/* 1. 身份列表（可增删） */}
      <FormSection
        title={t("resource:user.identitySection")}
        extra={
          <Button type="primary" size="small" icon={<PlusOutlined />} onClick={onAddIdentity}>
            {t("resource:user.addIdentity")}
          </Button>
        }
      >
        {identities.length > 0 ? (
          <Table
            dataSource={identities.map((item, i) => ({ ...item, rowIndex: i }))}
            columns={[
              {
                title: "Provider",
                dataIndex: "providerId",
                key: "providerId",
                render: (v: string) => {
                  const resolvedName = providers?.find((p) => p.id === v)?.displayName ?? v;
                  return <span>{resolvedName}</span>;
                },
              },
              {
                title: t("resource:user.identityType"),
                dataIndex: "identityType",
                key: "identityType",
              },
              { title: "Subject", dataIndex: "subject", key: "subject" },
              {
                title: t("resource:user.providerAccount"),
                dataIndex: "providerAccount",
                key: "providerAccount",
              },
              {
                title: t("resource:user.linkedAt"),
                dataIndex: "linkedAt",
                key: "linkedAt",
                render: (v: string | null) => formatDateTime(v),
              },
              {
                title: "",
                key: "actions",
                width: 48,
                render: (_: unknown, record: IdentityFormItem & { rowIndex: number }) => (
                  <Popconfirm
                    title={t("resource:user.unlinkIdentityConfirm", "Unlink this identity?")}
                    onConfirm={() => onDeleteIdentity(record.rowIndex)}
                  >
                    <Button type="text" danger size="small" icon={<DeleteOutlined />} />
                  </Popconfirm>
                ),
              },
            ]}
            pagination={false}
            size="small"
            rowKey="rowIndex"
          />
        ) : (
          <Typography.Text type="secondary">{t("resource:user.emptyIdentities")}</Typography.Text>
        )}
      </FormSection>

      {/* 2. 3rd-party login linking（无登录提供商时也渲染，保证内联新建入口可达） */}
      <FormSection
        title={t("resource:user.thirdPartyLogin", "3rd-Party Login Linking")}
        extra={
          <Space>
            <PermissionGuard action="provider:write" resource="providers" fallback={null}>
              <Button
                size="small"
                type="link"
                icon={<PlusOutlined />}
                onClick={() => setCreateProviderOpen(true)}
              >
                {t("resource:provider.newProvider")}
              </Button>
            </PermissionGuard>
            <Link to="/providers">{t("resource:user.manageProviders", "Manage providers")}</Link>
          </Space>
        }
      >
        {oauthOidcSamlProviders.length === 0 ? (
          <Typography.Text type="secondary">
            {t(
              "resource:user.noLoginProviders",
              "No sign-in providers yet. Create one inline to enable third-party login.",
            )}
          </Typography.Text>
        ) : (
          <Space orientation="vertical" style={{ width: "100%" }} size="middle">
            {oauthOidcSamlProviders.map((provider) => {
              const linkedIdentity = identities.find((i) => i.providerId === provider.id);
              const isLinked = !!linkedIdentity;
              return (
                <Row key={provider.id} align="middle" justify="space-between" wrap={false}>
                  <Col>
                    <Space>
                      <Tag color={isLinked ? "green" : "default"}>
                        {provider.category ?? "OAuth"}
                      </Tag>
                      <Typography.Text>{provider.displayName ?? provider.id}</Typography.Text>
                      {isLinked && (
                        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                          ({linkedIdentity.subject})
                        </Typography.Text>
                      )}
                    </Space>
                  </Col>
                  <Col>
                    {isLinked ? (
                      <Popconfirm
                        title={t("resource:user.unlinkIdentityConfirm", "Unlink this provider?")}
                        onConfirm={() => {
                          const idx = identities.findIndex((i) => i.providerId === provider.id);
                          if (idx >= 0) onDeleteIdentity(idx);
                        }}
                      >
                        <Button size="small" danger>
                          {t("resource:user.unlink", "Unlink")}
                        </Button>
                      </Popconfirm>
                    ) : (
                      <Tooltip
                        title={t("resource:user.linkViaAuthFlow", "Link via auth redirect flow")}
                      >
                        <Button
                          size="small"
                          type="primary"
                          icon={<LinkOutlined />}
                          onClick={() => onLinkIdentity(provider.id)}
                        >
                          {t("resource:user.link", "Link")}
                        </Button>
                      </Tooltip>
                    )}
                  </Col>
                </Row>
              );
            })}
          </Space>
        )}
      </FormSection>

      {/* 3. OAuth Widgets */}
      {identities.filter((i) => i.identityType === "oauth2" || i.identityType === "oidc").length >
        0 && (
        <FormSection title={t("oauthConnections", "OAuth Connections")}>
          <Space orientation="vertical" style={{ width: "100%" }}>
            {identities
              .filter((i) => i.identityType === "oauth2" || i.identityType === "oidc")
              .map((i) => (
                <OAuthWidget
                  key={i.identityType + ":" + i.providerId}
                  provider={{
                    type: i.identityType,
                    name: i.providerId,
                    displayName: providerName(i.providerId),
                    category: providerCategory(i.providerId),
                  }}
                  linkedIdentity={{
                    providerId: i.providerId,
                    identityType: i.identityType,
                    subject: i.subject,
                  }}
                />
              ))}
          </Space>
        </FormSection>
      )}

      {/* 4. SAML Widgets */}
      {identities.filter((i) => i.identityType === "saml").length > 0 && (
        <FormSection title={t("samlConnections", "SAML Connections")}>
          <Space orientation="vertical" style={{ width: "100%" }}>
            {identities
              .filter((i) => i.identityType === "saml")
              .map((i) => (
                <SamlWidget
                  key={i.identityType + ":" + i.providerId}
                  provider={{
                    type: i.identityType,
                    name: i.providerId,
                    displayName: providerName(i.providerId),
                  }}
                  linkedIdentity={{
                    providerId: i.providerId,
                    identityType: i.identityType,
                    subject: i.subject,
                  }}
                />
              ))}
          </Space>
        </FormSection>
      )}

      <ProviderInlineCreateModal
        open={createProviderOpen}
        onClose={() => setCreateProviderOpen(false)}
        allowedCategories={["OAuth", "OIDC", "SAML"]}
        defaultCategory="OAuth"
      />
    </Space>
  );
}
