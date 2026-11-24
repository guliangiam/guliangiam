import React, { useState, useCallback, useRef } from "react";
import { Card, Typography, Button, Space, Alert, Skeleton, Steps } from "antd";
import { ScanOutlined, CheckCircleOutlined, CameraOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";

const { Text, Title } = Typography;

interface FaceIdCardProps {
  onInitialize: () => Promise<{ sessionId: string; challenge: string }>;
  onCapture: (
    sessionId: string,
    imageBlob: Blob,
  ) => Promise<{ verified: boolean; livenessPassed: boolean }>;
  onComplete: (sessionId: string) => Promise<void>;
}

const FaceIdCard: React.FC<FaceIdCardProps> = ({ onInitialize, onCapture, onComplete }) => {
  const { t } = useTranslation("auth");
  const [step, setStep] = useState<0 | 1 | 2 | 3>(0);
  const [sessionId, setSessionId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [livenessPassed, setLivenessPassed] = useState(false);
  const videoRef = useRef<HTMLVideoElement>(null);
  const streamRef = useRef<MediaStream | null>(null);

  const stopCamera = useCallback(() => {
    if (streamRef.current) {
      streamRef.current.getTracks().forEach((track) => track.stop());
      streamRef.current = null;
    }
  }, []);

  const handleInitialize = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const result = await onInitialize();
      setSessionId(result.sessionId);
      setStep(1);

      const stream = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: "user", width: 640, height: 480 },
      });
      streamRef.current = stream;
      if (videoRef.current) {
        videoRef.current.srcObject = stream;
      }
    } catch (e) {
      setError(t("login.faceId.initFailed", { error: String(e) }));
    } finally {
      setLoading(false);
    }
  }, [onInitialize, t]);

  const handleCapture = useCallback(async () => {
    if (!videoRef.current) return;
    setError(null);
    setLoading(true);
    try {
      const canvas = document.createElement("canvas");
      canvas.width = videoRef.current.videoWidth;
      canvas.height = videoRef.current.videoHeight;
      const ctx = canvas.getContext("2d");
      if (!ctx) throw new Error("Canvas context unavailable");
      ctx.drawImage(videoRef.current, 0, 0);
      stopCamera();

      const blob = await new Promise<Blob>((resolve, reject) => {
        let settled = false;
        canvas.toBlob(
          (b) => {
            if (settled) return;
            settled = true;
            if (b) {
              resolve(b);
            } else {
              reject(new Error("Blob conversion failed"));
            }
          },
          "image/jpeg",
          0.9,
        );
      });

      const result = await onCapture(sessionId, blob);
      if (result.verified) {
        setLivenessPassed(result.livenessPassed);
        setStep(2);
        await onComplete(sessionId);
        setStep(3);
      } else {
        setError(t("login.faceId.verificationFailed"));
        setStep(1);
      }
    } catch (e) {
      setError(t("login.faceId.captureFailed", { error: String(e) }));
    } finally {
      setLoading(false);
    }
  }, [sessionId, onCapture, onComplete, stopCamera, t]);

  React.useEffect(() => () => stopCamera(), [stopCamera]);

  return (
    <Card style={{ textAlign: "center", maxWidth: 440, margin: "0 auto" }}>
      <Title level={4}>{t("login.faceId.title")}</Title>

      <Steps
        current={step}
        size="small"
        items={[
          { title: t("login.faceId.stepInit") },
          { title: t("login.faceId.stepCapture") },
          { title: t("login.faceId.stepVerify") },
          { title: t("login.faceId.stepDone") },
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
          <CameraOutlined style={{ fontSize: 48, color: "var(--gl-color-primary)" }} />
          <Text type="secondary">{t("login.faceId.initPrompt")}</Text>
          <Button type="primary" size="large" loading={loading} onClick={handleInitialize}>
            {t("login.faceId.start")}
          </Button>
        </Space>
      )}

      {step === 1 && (
        <Space orientation="vertical" size="middle" style={{ width: "100%" }}>
          <div
            style={{
              position: "relative",
              width: 320,
              height: 240,
              margin: "0 auto",
              borderRadius: 12,
              overflow: "hidden",
              background: "#000",
            }}
          >
            <video
              ref={videoRef}
              autoPlay
              playsInline
              muted
              aria-label={t("login.faceId.videoLabel")}
              style={{ width: "100%", height: "100%", objectFit: "cover" }}
            />
            <div
              style={{
                position: "absolute",
                top: "50%",
                left: "50%",
                transform: "translate(-50%, -50%)",
                width: 160,
                height: 200,
                border: "2px dashed rgba(255,255,255,0.6)",
                borderRadius: "50%",
              }}
            />
          </div>
          <Text type="secondary">{t("login.faceId.alignFace")}</Text>
          <Button
            type="primary"
            size="large"
            loading={loading}
            onClick={handleCapture}
            icon={<ScanOutlined />}
          >
            {t("login.faceId.capture")}
          </Button>
        </Space>
      )}

      {step === 2 && (
        <Space orientation="vertical" size="large">
          <Skeleton active paragraph={{ rows: 1 }} />
          <Text>{t("login.faceId.verifying")}</Text>
        </Space>
      )}

      {step === 3 && (
        <Space orientation="vertical" size="large">
          <CheckCircleOutlined style={{ fontSize: 48, color: "var(--gl-color-success)" }} />
          <Text type="success">{t("login.faceId.success")}</Text>
          {livenessPassed && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              {t("login.faceId.livenessPassed")}
            </Text>
          )}
        </Space>
      )}
    </Card>
  );
};

export default FaceIdCard;
