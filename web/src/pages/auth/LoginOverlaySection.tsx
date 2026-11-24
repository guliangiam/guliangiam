import { lazy, Suspense } from "react";
import type { FaceRecognitionMode } from "@/shared/hooks/useFaceRecognitionFlow";
import { LoginTermsModal } from "./LoginTermsModal";

const FaceRecognitionCommonModal = lazy(async () => {
  const module = await import("./FaceRecognitionCommonModal");
  return { default: module.FaceRecognitionCommonModal };
});

const FaceRecognitionModal = lazy(async () => {
  const module = await import("./FaceRecognitionModal");
  return { default: module.FaceRecognitionModal };
});

type LoginOverlaySectionProps = {
  termsOpen: boolean;
  termsOfUse?: string | null;
  onCloseTerms: () => void;
  faceRecognitionMode: FaceRecognitionMode | null;
  onFaceRecognitionCancel: () => void;
  onFaceRecognitionVectorOk: (faceId: number[]) => void;
  onFaceRecognitionImagesOk: (faceIdImage: string[]) => void;
};

export function LoginOverlaySection(props: LoginOverlaySectionProps) {
  const {
    faceRecognitionMode,
    onCloseTerms,
    onFaceRecognitionCancel,
    onFaceRecognitionImagesOk,
    onFaceRecognitionVectorOk,
    termsOfUse,
    termsOpen,
  } = props;

  return (
    <>
      <LoginTermsModal open={termsOpen} termsOfUse={termsOfUse} onCancel={onCloseTerms} />

      <Suspense fallback={null}>
        <FaceRecognitionModal
          open={faceRecognitionMode === "vector"}
          onOk={onFaceRecognitionVectorOk}
          onCancel={onFaceRecognitionCancel}
        />
        <FaceRecognitionCommonModal
          open={faceRecognitionMode === "images"}
          onOk={onFaceRecognitionImagesOk}
          onCancel={onFaceRecognitionCancel}
        />
      </Suspense>
    </>
  );
}
