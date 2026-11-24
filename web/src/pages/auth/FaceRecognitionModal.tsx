import { App, Button, Modal, Progress, Skeleton } from "antd";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { localizedError } from "@/shared/lib/server-error";

type FaceRecognitionModalProps = {
  open: boolean;
  onOk: (faceId: number[]) => void;
  onCancel: () => void;
};

type FaceApiDetection = {
  detection: {
    score: number;
  };
  descriptor: ArrayLike<number>;
};

type FaceApiGlobal = {
  nets: {
    tinyFaceDetector: { loadFromUri: (url: string) => Promise<void> };
    faceLandmark68Net: { loadFromUri: (url: string) => Promise<void> };
    faceRecognitionNet: { loadFromUri: (url: string) => Promise<void> };
  };
  TinyFaceDetectorOptions: new () => unknown;
  detectAllFaces: (
    input: HTMLVideoElement,
    options: unknown,
  ) => {
    withFaceLandmarks: () => {
      withFaceDescriptors: () => Promise<FaceApiDetection[]>;
    };
  };
};

const FACE_API_SCRIPT_PATH = "/vendor/face-api.min.js";

let faceApiPromise: Promise<FaceApiGlobal> | null = null;

function getFaceApiGlobal() {
  return (window as Window & { faceapi?: FaceApiGlobal }).faceapi;
}

function loadFaceApiScript() {
  if (getFaceApiGlobal()) {
    return Promise.resolve(getFaceApiGlobal()!);
  }

  if (faceApiPromise) {
    return faceApiPromise;
  }

  faceApiPromise = new Promise<FaceApiGlobal>((resolve, reject) => {
    const existingScript = document.querySelector<HTMLScriptElement>(
      `script[data-face-api-script="true"]`,
    );
    if (existingScript) {
      existingScript.addEventListener("load", () => {
        const faceapi = getFaceApiGlobal();
        if (faceapi) {
          resolve(faceapi);
          return;
        }
        reject(new Error("face-api.js loaded but global object not found"));
      });
      existingScript.addEventListener("error", () =>
        reject(new Error("face-api.js script failed to load")),
      );
      return;
    }

    const script = document.createElement("script");
    script.src = FACE_API_SCRIPT_PATH;
    script.async = true;
    script.dataset.faceApiScript = "true";
    script.onload = () => {
      const faceapi = getFaceApiGlobal();
      if (faceapi) {
        resolve(faceapi);
        return;
      }
      reject(new Error("face-api.js loaded but global object not found"));
    };
    script.onerror = () => reject(new Error("face-api.js script failed to load"));
    document.head.appendChild(script);
  }).catch((error) => {
    faceApiPromise = null;
    throw error;
  });

  return faceApiPromise;
}

// oxlint-disable-next-line react/only-export-components -- preloadFaceApiScript 由 auth-signin-providers.ts 动态导入调用，导出位置不能移动
export function preloadFaceApiScript() {
  return loadFaceApiScript();
}

const FACE_MODEL_URL = "/guliangiam/models";

export function FaceRecognitionModal({ open, onOk, onCancel }: FaceRecognitionModalProps) {
  const { message } = App.useApp();
  const { t } = useTranslation("auth");
  const { t: tc } = useTranslation("common");
  const [modelsLoaded, setModelsLoaded] = useState(false);
  const [cameraReady, setCameraReady] = useState(false);
  const [percent, setPercent] = useState(0);
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const mediaStreamRef = useRef<MediaStream | null>(null);
  const detectionRef = useRef<number | null>(null);
  const faceApiRef = useRef<FaceApiGlobal | null>(null);

  useEffect(() => {
    if (!open || modelsLoaded) {
      return;
    }

    let cancelled = false;

    async function loadModels() {
      try {
        const faceapi = await loadFaceApiScript();
        faceApiRef.current = faceapi;

        await Promise.all([
          faceapi.nets.tinyFaceDetector.loadFromUri(FACE_MODEL_URL),
          faceapi.nets.faceLandmark68Net.loadFromUri(FACE_MODEL_URL),
          faceapi.nets.faceRecognitionNet.loadFromUri(FACE_MODEL_URL),
        ]);

        if (!cancelled) {
          setModelsLoaded(true);
        }
      } catch (error) {
        message.error(localizedError(error, t, "faceModelLoadFailed"));
        onCancel();
      }
    }

    void loadModels();

    return () => {
      cancelled = true;
    };
  }, [message, modelsLoaded, onCancel, open, t]);

  useEffect(() => {
    if (!open || !modelsLoaded) {
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
      setCameraReady(false);
      setPercent(0);
      if (detectionRef.current !== null) {
        window.clearInterval(detectionRef.current);
        detectionRef.current = null;
      }
      mediaStreamRef.current?.getTracks().forEach((track) => track.stop());
      mediaStreamRef.current = null;
      if (videoElement) {
        videoElement.srcObject = null;
      }
    };
  }, [message, modelsLoaded, onCancel, open, t, tc]);

  useEffect(() => {
    const faceapi = faceApiRef.current;
    if (!open || !cameraReady || !videoRef.current || !mediaStreamRef.current || !faceapi) {
      return;
    }

    const video = videoRef.current;
    video.srcObject = mediaStreamRef.current;
    void video.play();

    let count = 0;
    let goodCount = 0;

    detectionRef.current = window.setInterval(async () => {
      const faces = await faceapi
        .detectAllFaces(video, new faceapi.TinyFaceDetectorOptions())
        .withFaceLandmarks()
        .withFaceDescriptors();

      count += 1;
      if (count % 50 === 0) {
        message.warning(t("faceRecognitionTip"));
      } else if (count > 300) {
        message.error(t("faceRecognitionFailed"));
        onCancel();
        return;
      }

      if (faces.length !== 1) {
        setPercent((current) => Math.round(current / 2));
        return;
      }

      const face = faces[0];
      setPercent(Math.round(face.detection.score * 100));
      if (face.detection.score > 0.9) {
        goodCount += 1;
        if (face.detection.score > 0.99 || goodCount > 10) {
          if (detectionRef.current !== null) {
            window.clearInterval(detectionRef.current);
            detectionRef.current = null;
          }
          onOk(Array.from(face.descriptor));
        }
      }
    }, 100);

    return () => {
      if (detectionRef.current !== null) {
        window.clearInterval(detectionRef.current);
        detectionRef.current = null;
      }
    };
  }, [cameraReady, message, onCancel, onOk, open, t]);

  return (
    <Modal
      closable={false}
      mask={{ closable: false }}
      destroyOnHidden
      open={open && cameraReady}
      title={t("faceRecognitionTitle")}
      width={350}
      footer={[
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
        {modelsLoaded ? (
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
        ) : (
          <Skeleton
            active
            paragraph={{ rows: 2 }}
            style={{ display: "flex", justifyContent: "center" }}
          />
        )}
      </div>
    </Modal>
  );
}
