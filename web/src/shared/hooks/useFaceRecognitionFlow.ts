import { useCallback, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

export type FaceRecognitionMode = "vector" | "images";

export type FaceRecognitionResult = {
  faceId?: number[];
  faceIdImage?: string[];
};

export function useFaceRecognitionFlow() {
  const [faceRecognitionMode, setFaceRecognitionMode] = useState<FaceRecognitionMode | null>(null);
  const faceRecognitionResolverRef = useRef<{
    resolve: (result: FaceRecognitionResult) => void;
    reject: (error: Error) => void;
  } | null>(null);
  const { t } = useTranslation();

  const requestFaceRecognition = useCallback((mode: FaceRecognitionMode) => {
    return new Promise<FaceRecognitionResult>((resolve, reject) => {
      faceRecognitionResolverRef.current = { resolve, reject };
      setFaceRecognitionMode(mode);
    });
  }, []);

  const handleFaceRecognitionCancel = useCallback(() => {
    faceRecognitionResolverRef.current?.reject(new Error(t("faceId.loginCancelled")));
    faceRecognitionResolverRef.current = null;
    setFaceRecognitionMode(null);
  }, [t]);

  const handleFaceRecognitionVectorOk = useCallback((faceId: number[]) => {
    faceRecognitionResolverRef.current?.resolve({ faceId });
    faceRecognitionResolverRef.current = null;
    setFaceRecognitionMode(null);
  }, []);

  const handleFaceRecognitionImagesOk = useCallback((faceIdImage: string[]) => {
    faceRecognitionResolverRef.current?.resolve({ faceIdImage });
    faceRecognitionResolverRef.current = null;
    setFaceRecognitionMode(null);
  }, []);

  return {
    faceRecognitionMode,
    requestFaceRecognition,
    handleFaceRecognitionCancel,
    handleFaceRecognitionVectorOk,
    handleFaceRecognitionImagesOk,
  };
}
