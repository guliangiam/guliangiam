import { Button, Card, List, Space, Typography } from "antd";
import { CheckOutlined, LockOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import type { ConsentPageReadyState } from "./useConsentPageController";

export function ConsentPageCard({
  application,
  consentScopes,
  canGrant,
  isSubmitting,
  handleGrant,
  handleDeny,
}: ConsentPageReadyState) {
  const { t } = useTranslation("auth");

  return (
    <div className="login-content" style={{ margin: "auto" }}>
      <div style={{ marginBottom: 10, textAlign: "center" }}>
        <Card
          style={{
            width: 460,
            maxWidth: "calc(100vw - 32px)",
            margin: "72px auto",
            borderRadius: 16,
          }}
          styles={{ body: { padding: 32 } }}
        >
          <div style={{ textAlign: "center", marginBottom: 24 }}>
            {application.logo ? (
              <div style={{ marginBottom: 16 }}>
                <img
                  src={application.logo}
                  alt={application.displayName || application.name}
                  style={{ height: 56, objectFit: "contain" }}
                />
              </div>
            ) : null}
            <Typography.Title level={3} style={{ marginBottom: 8 }}>
              {t("consentTitle")}
            </Typography.Title>
            <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
              <strong style={{ color: "var(--gl-color-text)" }}>
                {application.displayName || application.name}
              </strong>
              {t("consentDesc")}
            </Typography.Paragraph>
            {application.homepageUrl ? (
              <Typography.Link href={application.homepageUrl} target="_blank" rel="noreferrer">
                {application.homepageUrl}
              </Typography.Link>
            ) : null}
          </div>

          <div style={{ marginBottom: 24, textAlign: "left" }}>
            <Typography.Text type="secondary">
              <LockOutlined style={{ marginRight: 8 }} />
              {t("consentScopes")}
            </Typography.Text>
          </div>

          <List
            size="small"
            dataSource={consentScopes}
            locale={{ emptyText: t("noScopes") }}
            renderItem={(item) => (
              <List.Item style={{ alignItems: "flex-start", paddingInline: 0 }}>
                <div
                  style={{
                    display: "grid",
                    gridTemplateColumns: "18px 1fr",
                    columnGap: 10,
                    width: "100%",
                  }}
                >
                  <CheckOutlined style={{ color: "var(--gl-color-success)", marginTop: 4 }} />
                  <div>
                    <div style={{ fontWeight: 600 }}>{item.displayName}</div>
                    {item.description ? (
                      <Typography.Paragraph type="secondary" style={{ margin: "4px 0 0" }}>
                        {item.description}
                      </Typography.Paragraph>
                    ) : null}
                  </div>
                </div>
              </List.Item>
            )}
          />

          <div style={{ textAlign: "center", marginTop: 28 }}>
            <Space size={16}>
              <Button
                type="primary"
                size="large"
                shape="round"
                disabled={!canGrant}
                loading={isSubmitting}
                onClick={() => void handleGrant()}
              >
                {t("allow")}
              </Button>
              <Button size="large" shape="round" onClick={handleDeny} disabled={!canGrant}>
                {t("deny")}
              </Button>
            </Space>
          </div>

          <div
            style={{
              marginTop: 24,
              padding: 16,
              borderRadius: 12,
              background: "var(--gl-color-bg)",
              border: "1px solid var(--gl-color-border)",
            }}
          >
            <Typography.Paragraph type="secondary" style={{ margin: 0, textAlign: "center" }}>
              {t("consentNote")}
            </Typography.Paragraph>
          </div>
        </Card>
      </div>
    </div>
  );
}
