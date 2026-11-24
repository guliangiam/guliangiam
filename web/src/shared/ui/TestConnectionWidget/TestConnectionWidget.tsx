import { Alert, App, Button, Input, Space, Typography } from "antd";
import { SendOutlined } from "@ant-design/icons";
import { type FC, useState, useCallback } from "react";
import { useTranslation } from "react-i18next";

type TestConnectionWidgetProps = {
  type: "email" | "sms" | "notification";
  providerId?: string;
  onTest: (
    providerId: string,
    recipient: string,
  ) => Promise<{ success: boolean; message?: string }>;
  recipientLabel?: string;
  recipientPlaceholder?: string;
};

/**
 * Generic widget for testing Email/SMS/Notification provider connections.
 * Allows sending a test message to verify configuration is correct.
 */
export const TestConnectionWidget: FC<TestConnectionWidgetProps> = ({
  type,
  providerId,
  onTest,
  recipientPlaceholder,
}) => {
  const { t } = useTranslation("common");
  const { message } = App.useApp();
  const [recipient, setRecipient] = useState("");
  const [sending, setSending] = useState(false);
  const [result, setResult] = useState<{ success: boolean; message?: string } | null>(null);

  const defaultPlaceholder =
    type === "email"
      ? "user@example.com"
      : type === "sms"
        ? "+1234567890"
        : "https://hooks.example.com/test";

  const handleSend = useCallback(async () => {
    if (!providerId || !recipient.trim()) {
      message.warning(t("testConnection.enterRecipient"));
      return;
    }
    setSending(true);
    setResult(null);
    try {
      const res = await onTest(providerId, recipient.trim());
      setResult(res);
      if (res.success) {
        message.success(t("testConnection.sent"));
      } else {
        message.error(res.message ?? t("testConnection.failed"));
      }
    } catch {
      setResult({ success: false, message: "Connection error" });
      message.error(t("testConnection.failed"));
    } finally {
      setSending(false);
    }
  }, [providerId, recipient, onTest, message, t]);

  return (
    <Space orientation="vertical" style={{ width: "100%", marginTop: 8 }}>
      <Typography.Text strong style={{ fontSize: 13 }}>
        {t(`testConnection.${type}Test`, {
          defaultValue: `Test ${type.charAt(0).toUpperCase() + type.slice(1)}`,
        })}
      </Typography.Text>
      <Space.Compact style={{ width: "100%" }}>
        <Input
          value={recipient}
          onChange={(e) => setRecipient(e.target.value)}
          placeholder={recipientPlaceholder ?? defaultPlaceholder}
          style={{ flex: 1 }}
          onPressEnter={handleSend}
        />
        <Button
          type="primary"
          icon={<SendOutlined />}
          loading={sending}
          disabled={!providerId || !recipient.trim()}
          onClick={handleSend}
        >
          {t("testConnection.send")}
        </Button>
      </Space.Compact>
      {result && (
        <Alert
          type={result.success ? "success" : "error"}
          title={result.message ?? (result.success ? "OK" : "Failed")}
          showIcon
          closable
          onClose={() => setResult(null)}
        />
      )}
    </Space>
  );
};
