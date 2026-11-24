import { Alert, Card, Skeleton, Space, Typography } from "antd";
import { useTranslation } from "react-i18next";
import type { SamlCallbackPageControllerState } from "./useSamlCallbackPageController";

export function SamlCallbackPageCard(controller: SamlCallbackPageControllerState) {
  const { t } = useTranslation("auth");

  if (controller.status === "error") {
    return (
      <div
        style={{
          minHeight: "100vh",
          display: "flex",
          justifyContent: "center",
          alignItems: "center",
          padding: 24,
          background:
            "radial-gradient(circle at top left, color-mix(in srgb, var(--gl-color-primary) 15%, transparent), transparent 32%), linear-gradient(180deg, var(--gl-color-bg) 0%, var(--gl-color-bg-elevated) 100%)",
        }}
      >
        <Card className="gl-panel" variant="borderless" style={{ width: "100%", maxWidth: 520 }}>
          <Space orientation="vertical" size={16} style={{ width: "100%" }}>
            <div>
              <Typography.Title level={3} style={{ marginBottom: 8 }}>
                {t("samlCallbackTitle")}
              </Typography.Title>
              <Typography.Paragraph style={{ marginBottom: 0 }}>
                {t("samlCallbackDesc")}
              </Typography.Paragraph>
            </div>

            <Alert showIcon type="error" title={controller.errorMessage} />
          </Space>
        </Card>
      </div>
    );
  }

  return (
    <div
      style={{
        minHeight: "100vh",
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        padding: 24,
        background:
          "radial-gradient(circle at top left, color-mix(in srgb, var(--gl-color-primary) 15%, transparent), transparent 32%), linear-gradient(180deg, var(--gl-color-bg) 0%, var(--gl-color-bg-elevated) 100%)",
      }}
    >
      <Card className="gl-panel" variant="borderless" style={{ width: "100%", maxWidth: 520 }}>
        <Space orientation="vertical" size={16} style={{ width: "100%" }}>
          <div>
            <Typography.Title level={3} style={{ marginBottom: 8 }}>
              {t("samlCallbackTitle")}
            </Typography.Title>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              {t("samlCallbackDesc")}
            </Typography.Paragraph>
          </div>

          <div style={{ display: "flex", justifyContent: "center", padding: "24px 0" }}>
            <Skeleton active paragraph={{ rows: 2 }} title={false} />
          </div>
        </Space>
      </Card>
    </div>
  );
}
