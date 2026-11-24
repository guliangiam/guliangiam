import { App, Button, Modal, Progress } from "antd";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { localizedError } from "@/shared/lib/server-error";

type FaceRecognitionCommonModalProps = {
  open: boolean;
  onOk: (faceIdImages: string[]) => void;
  onCancel: () => void;
};

export function FaceRecognitionCommonModal({
  open,
  onOk,
  onCancel,
}: FaceRecognitionCommonModalProps) {
  const { message } = App.useApp();
  const { t } = useTranslation("auth");
  const { t: tc } = useTranslation("common");
  const [cameraReady, setCameraReady] = useState(false);
  const [percent, setPercent] = useState(0);
  const [capturedImages, setCapturedImages] = useState<string[]>([]);
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const mediaStreamRef = useRef<MediaStream | null>(null);
  const captureTimerRef = useRef<number | null>(null);

  // Closing the modal resets the capture UI. The reset is adjusted during render
  // (React's documented alternative to a sync effect) so the camera effect below
  // only deals with the external MediaStream.
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (!open) {
      setCameraReady(false);
      setPercent(0);
      setCapturedImages([]);
    }
  }

  useEffect(() => {
    if (!open) {
      return;
    }

    let cancelled = false;
    const videoElement = videoRef.current;

    async function startCamera() {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({ video: { facingMode: "user" } });
        if (cancelled) {
          stream.getTracks().forEach((track) => track.stop());
          return;
        }
        mediaStreamRef.current = stream;
        setCameraReady(true);
      } catch (error) {
        if (error instanceof DOMException) {
          if (error.name === "NotFoundError" || error.name === "DevicesNotFoundError") {
            message.error(t("cameraNotFound"));
          } else if (error.name === "NotAllowedError" || error.name === "PermissionDeniedError") {
            message.error(t("cameraPermissionDenied"));
          } else if (error.name === "NotReadableError" || error.name === "TrackStartError") {
            message.error(t("cameraInUse"));
          } else if (error.name === "TypeError") {
            message.error(t("cameraHttpsRequired"));
          } else {
            message.error(localizedError(error, tc, "common:error"));
          }
        } else {
          message.error(t("cameraOpenFailed"));
        }
        onCancel();
      }
    }

    void startCamera();

    return () => {
      cancelled = true;
      if (captureTimerRef.current !== null) {
        window.clearInterval(captureTimerRef.current);
        captureTimerRef.current = null;
      }
      mediaStreamRef.current?.getTracks().forEach((track) => track.stop());
      mediaStreamRef.current = null;
      if (videoElement) {
        videoElement.srcObject = null;
      }
    };
  }, [message, onCancel, open, t, tc]);

  useEffect(() => {
    if (!open || !cameraReady || !videoRef.current || !mediaStreamRef.current) {
      return;
    }

    const video = videoRef.current;
    video.srcObject = mediaStreamRef.current;
    void video.play();

    let ticks = 0;
    const snapshots: string[] = [];

    captureTimerRef.current = window.setInterval(() => {
      if (!video.videoWidth || !video.videoHeight) {
        return;
      }

      ticks += 1;
      if (ticks <= 3) {
        return;
      }

      const canvas = document.createElement("canvas");
      canvas.width = video.videoWidth;
      canvas.height = video.videoHeight;
      const context = canvas.getContext("2d");
      if (!context) {
        return;
      }

      context.drawImage(video, 0, 0, canvas.width, canvas.height);
      snapshots.push(canvas.toDataURL("image/png"));
      setCapturedImages([...snapshots]);
      setPercent((snapshots.length / 4) * 100);

      if (snapshots.length >= 4) {
        if (captureTimerRef.current !== null) {
          window.clearInterval(captureTimerRef.current);
          captureTimerRef.current = null;
        }
        onOk(snapshots);
      }
    }, 1000);

    return () => {
      if (captureTimerRef.current !== null) {
        window.clearInterval(captureTimerRef.current);
        captureTimerRef.current = null;
      }
    };
  }, [cameraReady, onOk, open]);

  return (
    <Modal
      closable={false}
      mask={{ closable: false }}
      destroyOnHidden
      open={open}
      title={t("faceRecognitionTitle")}
      width={350}
      footer={[
        <Button
          key="ok"
          type="primary"
          disabled={capturedImages.length === 0}
          onClick={() => onOk(capturedImages)}
        >
          {tc("confirm")}
        </Button>,
        <Button key="cancel" onClick={onCancel}>
          {tc("cancel")}
        </Button>,
      ]}
    >
      <Progress percent={percent} />
      <div
        style={{
          marginTop: 20,
          marginBottom: 50,
          justifyContent: "center",
          alignContent: "center",
          position: "relative",
          flexDirection: "column",
        }}
      >
        <div style={{ display: "flex", justifyContent: "center", alignContent: "center" }}>
          <video
            ref={videoRef}
            style={{
              borderRadius: "50%",
              height: 220,
              verticalAlign: "middle",
              width: 220,
              objectFit: "cover",
            }}
          >
            <track kind="captions" />
          </video>
          <div
            style={{
              position: "absolute",
              width: 240,
              height: 240,
              top: "50%",
              left: "50%",
              transform: "translate(-50%, -50%)",
            }}
          >
            {/* WCAG 1.1.1 — decorative SVG, hide from screen readers (A-01) */}
            <svg width="240" height="240" fill="none" aria-hidden="true">
              <circle
                strokeDasharray="700"
                strokeDashoffset={700 - 6.9115 * percent}
                strokeWidth="4"
                cx="120"
                cy="120"
                r="110"
                stroke="var(--gl-color-primary)"
                transform="rotate(-90, 120, 120)"
                strokeLinecap="round"
                style={{ transition: "all .2s linear" }}
              />
            </svg>
          </div>
        </div>
      </div>
    </Modal>
  );
}
