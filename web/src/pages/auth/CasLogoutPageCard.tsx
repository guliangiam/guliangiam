import { Alert, Button, Card, Skeleton, Space, Statistic, Typography } from "antd";
import type { CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import type { CasLogoutPageControllerState } from "./useCasLogoutPageController";

const shellStyle: CSSProperties = {
  minHeight: "100vh",
  display: "flex",
  justifyContent: "center",
  alignItems: "center",
  padding: 24,
  background:
    "radial-gradient(circle at top left, color-mix(in srgb, var(--gl-color-primary) 15%, transparent), transparent 32%), linear-gradient(180deg, var(--gl-color-bg) 0%, var(--gl-color-bg-elevated) 100%)",
};

export function CasLogoutPageCard(controller: CasLogoutPageControllerState) {
  const { t } = useTranslation("auth");

  return (
    <div style={shellStyle}>
      <Card className="gl-panel" variant="borderless" style={{ width: "100%", maxWidth: 520 }}>
        <Space orientation="vertical" size={16} style={{ width: "100%" }}>
          <div>
            <Typography.Title level={3} style={{ marginBottom: 8 }}>
              {t("casLogoutTitle")}
            </Typography.Title>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              {t("casLogoutDesc")}
            </Typography.Paragraph>
          </div>

          {controller.status === "pending" ? (
            <div style={{ display: "flex", justifyContent: "center", padding: "24px 0" }}>
              <Skeleton active paragraph={{ rows: 2 }} title={false} />
            </div>
          ) : null}

          {controller.status === "error" ? (
            <>
              <Alert showIcon type="error" title={controller.errorMessage} />
              <Space>
                <Button type="primary" onClick={controller.onRetry}>
                  {t("casLogoutRetry")}
                </Button>
                <Button onClick={controller.onReturnToSignIn}>{t("casLogoutBackToSignIn")}</Button>
              </Space>
            </>
          ) : null}

          {controller.status === "settled" ? (
            <>
              <Statistic title={t("casLogoutRevokedSessions")} value={controller.revokedSessions} />
              {controller.redirectRejected ? (
                <Alert
                  showIcon
                  type="warning"
                  title={t("casLogoutRedirectRejected")}
                  description={t("casLogoutRedirectRejectedDesc")}
                />
              ) : (
                <Alert
                  showIcon
                  type="success"
                  title={t("casLogoutSettled")}
                  description={t("casLogoutSettledDesc")}
                />
              )}
              <Button type="primary" onClick={controller.onReturnToSignIn}>
                {t("casLogoutBackToSignIn")}
              </Button>
            </>
          ) : null}
        </Space>
      </Card>
    </div>
  );
}
