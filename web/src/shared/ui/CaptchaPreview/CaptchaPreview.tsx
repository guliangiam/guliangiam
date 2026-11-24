import { Alert, App, Button, Card, Space, Typography } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import { type FC, useState, useCallback } from "react";
import { useTranslation } from "react-i18next";

type CaptchaPreviewProps = {
  providerId?: string;
  providerType?: string;
  siteKey?: string;
  providerUrl?: string;
  onTest?: (providerId: string) => Promise<{ success: boolean; message?: string }>;
};

/**
 * Renders a preview of a captcha provider configuration.
 * For reCAPTCHA/hCaptcha/Turnstile, shows the widget if siteKey is configured.
 * For custom providers, shows a connection test button.
 */
export const CaptchaPreview: FC<CaptchaPreviewProps> = ({
  providerId,
  providerType,
  siteKey,
  providerUrl,
  onTest,
}) => {
  const { t } = useTranslation("common");
  const { message } = App.useApp();
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; message?: string } | null>(null);

  const handleTest = useCallback(async () => {
    if (!providerId || !onTest) return;
    setTesting(true);
    setTestResult(null);
    try {
      const result = await onTest(providerId);
      setTestResult(result);
      if (result.success) {
        message.success(t("captcha.testSuccess"));
      } else {
        message.error(result.message ?? t("captcha.testFailed"));
      }
    } catch {
      setTestResult({ success: false, message: "Connection error" });
      message.error(t("captcha.testFailed"));
    } finally {
      setTesting(false);
    }
  }, [providerId, onTest, message, t]);

  const isWellKnownType = ["reCAPTCHA", "hCaptcha", "Turnstile", "GeeTest"].includes(
    providerType ?? "",
  );

  return (
    <Card size="small" title={t("captcha.preview")} style={{ marginTop: 8 }}>
      {isWellKnownType && siteKey ? (
        <Space orientation="vertical" style={{ width: "100%" }}>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {t("captcha.siteKeyLabel")}: {siteKey.slice(0, 8)}...
          </Typography.Text>
          <div
            style={{
              width: 304,
              height: 78,
              border: "1px dashed rgba(15, 23, 42, 0.12)",
              borderRadius: 8,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              background: "rgba(248, 250, 252, 0.9)",
            }}
          >
            <Typography.Text type="secondary">
              {providerType} {t("captcha.widget")}
            </Typography.Text>
          </div>
        </Space>
      ) : providerUrl ? (
        <Space orientation="vertical" style={{ width: "100%" }}>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {t("captcha.endpointLabel")}: {providerUrl}
          </Typography.Text>
        </Space>
      ) : (
        <Typography.Text type="secondary">{t("captcha.noConfig")}</Typography.Text>
      )}

      {providerId && onTest && (
        <div style={{ marginTop: 12 }}>
          <Button size="small" icon={<ReloadOutlined />} loading={testing} onClick={handleTest}>
            {t("captcha.testConnection")}
          </Button>
          {testResult && (
            <Alert
              type={testResult.success ? "success" : "error"}
              title={testResult.message ?? (testResult.success ? "OK" : "Failed")}
              showIcon
              style={{ marginTop: 8 }}
            />
          )}
        </div>
      )}
    </Card>
  );
};
