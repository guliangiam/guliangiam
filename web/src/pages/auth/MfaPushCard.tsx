import React, { useState, useEffect, useCallback, useRef } from "react";
import { Card, Typography, Button, Space, Alert, Skeleton, Result } from "antd";
import {
  MobileOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import { useTranslation } from "react-i18next";

const { Text, Title } = Typography;

type PushStatus = "pending" | "approved" | "denied" | "expired" | "error";

interface MfaPushCardProps {
  onRequestPush: () => Promise<{ requestId: string; expiresAt: string }>;
  onPollStatus: (requestId: string) => Promise<PushStatus>;
  autoStart?: boolean;
}

const MfaPushCard: React.FC<MfaPushCardProps> = ({
  onRequestPush,
  onPollStatus,
  autoStart = true,
}) => {
  const { t } = useTranslation("auth");
  const [status, setStatus] = useState<PushStatus | "idle">("idle");
  const [_requestId, setRequestId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [countdown, setCountdown] = useState(0);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const countdownRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const clearTimers = useCallback(() => {
    if (pollRef.current) clearInterval(pollRef.current);
    if (countdownRef.current) clearInterval(countdownRef.current);
    pollRef.current = null;
    countdownRef.current = null;
  }, []);

  useEffect(() => {
    if (autoStart) {
      handleRequest();
    }
    return () => clearTimers();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleRequest = useCallback(async () => {
    setError(null);
    setStatus("idle");
    clearTimers();
    try {
      const result = await onRequestPush();
      setRequestId(result.requestId);
      setStatus("pending");
      const expires = new Date(result.expiresAt).getTime();
      const remaining = Math.max(0, Math.floor((expires - Date.now()) / 1000));
      setCountdown(remaining);

      countdownRef.current = setInterval(() => {
        setCountdown((prev) => {
          if (prev <= 1) {
            clearTimers();
            setStatus("expired");
            return 0;
          }
          return prev - 1;
        });
      }, 1000);

      pollRef.current = setInterval(async () => {
        try {
          const s = await onPollStatus(result.requestId);
          if (s === "approved" || s === "denied" || s === "expired") {
            clearTimers();
            setStatus(s);
          }
        } catch {
          setStatus("error");
          clearTimers();
        }
      }, 3000);
    } catch (e) {
      setStatus("error");
      setError(t("mfa.push.requestFailed", { error: String(e) }));
    }
  }, [onRequestPush, onPollStatus, clearTimers, t]);

  const handleRetry = useCallback(() => {
    handleRequest();
  }, [handleRequest]);

  const formatCountdown = (seconds: number) => {
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, "0")}`;
  };

  return (
    <Card style={{ textAlign: "center", maxWidth: 400, margin: "0 auto" }}>
      <Title level={4}>{t("mfa.push.title")}</Title>

      {error && status !== "pending" && (
        <Alert
          type="error"
          title={error}
          style={{ marginBottom: 16 }}
          closable
          onClose={() => setError(null)}
        />
      )}

      {status === "idle" && (
        <Space orientation="vertical" size="large">
          <MobileOutlined style={{ fontSize: 48, color: "var(--gl-color-primary)" }} />
          <Text type="secondary">{t("mfa.push.idlePrompt")}</Text>
          <Button type="primary" size="large" onClick={handleRequest}>
            {t("mfa.push.send")}
          </Button>
        </Space>
      )}

      {status === "pending" && (
        <Space orientation="vertical" size="large">
          <Skeleton active paragraph={{ rows: 1 }} />
          <MobileOutlined style={{ fontSize: 32, color: "var(--gl-color-primary)" }} />
          <Text>{t("mfa.push.waiting")}</Text>
          <Text type="secondary">{formatCountdown(countdown)}</Text>
          <Button type="link" icon={<ReloadOutlined />} onClick={handleRetry}>
            {t("mfa.push.resend")}
          </Button>
        </Space>
      )}

      {status === "approved" && (
        <Result
          icon={<CheckCircleOutlined style={{ color: "var(--gl-color-success)" }} />}
          title={t("mfa.push.approved")}
          subTitle={t("mfa.push.redirecting")}
        />
      )}

      {status === "denied" && (
        <Space orientation="vertical" size="large">
          <CloseCircleOutlined style={{ fontSize: 48, color: "var(--gl-color-error)" }} />
          <Text type="danger">{t("mfa.push.denied")}</Text>
          <Button type="primary" onClick={handleRetry}>
            {t("mfa.push.tryAgain")}
          </Button>
        </Space>
      )}

      {status === "expired" && (
        <Space orientation="vertical" size="large">
          <CloseCircleOutlined style={{ fontSize: 48, color: "var(--gl-color-warning)" }} />
          <Text type="warning">{t("mfa.push.expired")}</Text>
          <Button type="primary" onClick={handleRetry}>
            {t("mfa.push.resend")}
          </Button>
        </Space>
      )}

      {status === "error" && (
        <Space orientation="vertical" size="large">
          <CloseCircleOutlined style={{ fontSize: 48, color: "var(--gl-color-error)" }} />
          <Text type="danger">{t("mfa.push.error")}</Text>
          <Button type="primary" onClick={handleRetry}>
            {t("mfa.push.tryAgain")}
          </Button>
        </Space>
      )}
    </Card>
  );
};

export default MfaPushCard;
