import React, { useState, useEffect, useCallback, useRef } from "react";
import { Card, Typography, Input, Button, Space, Alert, Progress, Steps } from "antd";
import { LaptopOutlined, CheckCircleOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";

const { Text, Title } = Typography;

interface DeviceFlowCardProps {
  onAuthorize: (userCode: string) => Promise<{
    deviceCode: string;
    userCode: string;
    verificationUri: string;
    expiresIn: number;
    interval: number;
  }>;
  onPollToken: (deviceCode: string) => Promise<{ accessToken: string } | null>;
  onManualSubmit?: (userCode: string) => Promise<boolean>;
}

const DeviceFlowCard: React.FC<DeviceFlowCardProps> = ({
  onAuthorize,
  onPollToken,
  onManualSubmit,
}) => {
  const { t } = useTranslation("auth");
  const [step, setStep] = useState<0 | 1 | 2>(0);
  const [_deviceCode, setDeviceCode] = useState("");
  const [userCode, setUserCode] = useState("");
  const [verificationUri, setVerificationUri] = useState("");
  const [expiresIn, setExpiresIn] = useState(0);
  const [_pollInterval, setPollInterval] = useState(5);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [remainingSeconds, setRemainingSeconds] = useState(0);
  const [inputUserCode, setInputUserCode] = useState("");
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const countdownRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const clearTimers = useCallback(() => {
    if (pollRef.current) clearInterval(pollRef.current);
    if (countdownRef.current) clearInterval(countdownRef.current);
    pollRef.current = null;
    countdownRef.current = null;
  }, []);

  useEffect(() => () => clearTimers(), [clearTimers]);

  const handleAuthorize = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const result = await onAuthorize(userCode || crypto.randomUUID().slice(0, 8).toUpperCase());
      setDeviceCode(result.deviceCode);
      setUserCode(result.userCode);
      setVerificationUri(result.verificationUri);
      setExpiresIn(result.expiresIn);
      setPollInterval(result.interval || 5);
      setRemainingSeconds(result.expiresIn);
      setStep(1);

      countdownRef.current = setInterval(() => {
        setRemainingSeconds((prev) => {
          if (prev <= 1) {
            clearTimers();
            setError(t("login.deviceFlow.expired"));
            setStep(0);
            return 0;
          }
          return prev - 1;
        });
      }, 1000);

      pollRef.current = setInterval(
        async () => {
          try {
            const token = await onPollToken(result.deviceCode);
            if (token) {
              clearTimers();
              setStep(2);
            }
          } catch {
            // authorization_pending - ignore and retry
          }
        },
        (result.interval || 5) * 1000,
      );
    } catch (e) {
      setError(t("login.deviceFlow.authorizeFailed", { error: String(e) }));
    } finally {
      setLoading(false);
    }
  }, [onAuthorize, onPollToken, userCode, clearTimers, t]);

  const handleManualVerify = useCallback(async () => {
    if (!onManualSubmit) return;
    setError(null);
    setLoading(true);
    try {
      const ok = await onManualSubmit(inputUserCode);
      if (ok) {
        setStep(2);
      } else {
        setError(t("login.deviceFlow.invalidCode"));
      }
    } catch (e) {
      setError(t("login.deviceFlow.verifyFailed", { error: String(e) }));
    } finally {
      setLoading(false);
    }
  }, [onManualSubmit, inputUserCode, t]);

  const progressPercent = expiresIn > 0 ? ((expiresIn - remainingSeconds) / expiresIn) * 100 : 0;

  return (
    <Card style={{ textAlign: "center", maxWidth: 460, margin: "0 auto" }}>
      <Title level={4}>{t("login.deviceFlow.title")}</Title>

      <Steps
        current={step}
        size="small"
        items={[
          { title: t("login.deviceFlow.stepInitiate") },
          { title: t("login.deviceFlow.stepAuthorize") },
          { title: t("login.deviceFlow.stepComplete") },
        ]}
        style={{ marginBottom: 24 }}
      />

      {error && (
        <Alert
          type="error"
          title={error}
          style={{ marginBottom: 16 }}
          closable
          onClose={() => setError(null)}
        />
      )}

      {step === 0 && (
        <Space orientation="vertical" size="large">
          <LaptopOutlined style={{ fontSize: 48, color: "var(--gl-color-primary)" }} />
          <Text type="secondary">{t("login.deviceFlow.initiatePrompt")}</Text>
          <Button type="primary" size="large" loading={loading} onClick={handleAuthorize}>
            {t("login.deviceFlow.initiate")}
          </Button>
        </Space>
      )}

      {step === 1 && (
        <Space orientation="vertical" size="middle" style={{ width: "100%" }}>
          <Text type="secondary">{t("login.deviceFlow.visitUri")}</Text>
          <Text code copyable style={{ fontSize: 14 }}>
            {verificationUri}
          </Text>
          <Text type="secondary">{t("login.deviceFlow.enterCode")}</Text>
          <Text code copyable style={{ fontSize: 24, letterSpacing: 4, padding: "8px 16px" }}>
            {userCode}
          </Text>
          <Progress
            percent={progressPercent}
            showInfo={false}
            strokeColor="var(--gl-color-primary)"
          />
          <Text type="secondary">
            {t("login.deviceFlow.remaining", { seconds: remainingSeconds })}
          </Text>

          {onManualSubmit && (
            <>
              <Text type="secondary" style={{ marginTop: 16 }}>
                {t("login.deviceFlow.manualVerify")}
              </Text>
              <Space.Compact style={{ width: "100%" }}>
                <Input
                  value={inputUserCode}
                  onChange={(e) => setInputUserCode(e.target.value.toUpperCase())}
                  placeholder={t("login.deviceFlow.codePlaceholder")}
                  style={{ textAlign: "center", letterSpacing: 2, textTransform: "uppercase" }}
                />
                <Button type="primary" loading={loading} onClick={handleManualVerify}>
                  {t("login.deviceFlow.verify")}
                </Button>
              </Space.Compact>
            </>
          )}
        </Space>
      )}

      {step === 2 && (
        <Space orientation="vertical" size="large">
          <CheckCircleOutlined style={{ fontSize: 48, color: "var(--gl-color-success)" }} />
          <Text type="success">{t("login.deviceFlow.success")}</Text>
        </Space>
      )}
    </Card>
  );
};

export default DeviceFlowCard;
