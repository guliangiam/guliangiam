import { useCallback } from "react";
import { useTranslation } from "react-i18next";
import type { ResponseEnvelope } from "@/shared/api/http";
import type { OAuthQueryContext } from "@/shared/lib/auth-oauth-context";
import { runFaceIdLogin, runWebAuthnLogin } from "@/shared/lib/auth-signin-providers";
import type { ApplicationItem } from "@/shared/types/domain";

type FaceRecognitionRequest = (mode: "vector" | "images") => Promise<{
  faceId?: number[];
  faceIdImage?: string[];
}>;

type LoginValuesBase = {
  organization: string;
  application: string;
  username: string;
  captchaToken?: string;
};

type BiometricPayloadBase = {
  organization: string;
  application: string;
  // 与 Face ID 的 faceId/faceIdImage 同理：payload 上的字段必须在这里声明，
  // createPayload 的字面量才能安全地断言成 TPayload。
  applicationId?: string;
  username: string;
  password?: string;
  code?: string;
  signinMethod?: string;
  faceId?: number[];
  faceIdImage?: string[];
};

type UseBiometricSigninActionsValues<TPayload extends BiometricPayloadBase> = {
  application: ApplicationItem | null;
  oauthContext: OAuthQueryContext | null;
  isCasLogin: boolean;
  isSamlLogin: boolean;
  hasFaceIdProvider: boolean;
  requestFaceRecognition: FaceRecognitionRequest;
  createProtocolContextPayload: () => Omit<TPayload, keyof BiometricPayloadBase>;
  createCaptchaPayload: (captchaToken?: string) => Partial<TPayload>;
  onSuccess: (values: { response: ResponseEnvelope; payload: TPayload }) => Promise<void>;
};

export function useBiometricSigninActions<
  TValues extends LoginValuesBase,
  TPayload extends BiometricPayloadBase,
>(values: UseBiometricSigninActionsValues<TPayload>) {
  // isCasLogin / isSamlLogin / oauthContext 仍留在 values 类型里（调用点兼容），
  // 但 WebAuthn 已改从 payload 上读协议上下文，这里不再解构。
  const {
    application,
    createCaptchaPayload,
    createProtocolContextPayload,
    hasFaceIdProvider,
    onSuccess,
    requestFaceRecognition,
  } = values;
  const { t } = useTranslation();

  const signInWithWebAuthn = useCallback(
    async (formValues: TValues) => {
      const owner = application?.organization || application?.owner || formValues.organization;
      if (!owner) {
        throw new Error(t("biometric.missingOrganization"));
      }

      await runWebAuthnLogin({
        owner,
        username: formValues.username?.trim() ?? "",
        createPayload: (username) =>
          ({
            ...createProtocolContextPayload(),
            signinMethod: "WebAuthn",
            organization: formValues.organization,
            username,
            password: "",
            application: formValues.application,
            // finish 的协议上下文（含 applicationId）从 payload 上读，
            // 与 Face ID / 扫码登录同一条路径。
            applicationId: application?.id ?? undefined,
          }) as TPayload,
        onSuccess,
      });
    },
    [application, createProtocolContextPayload, onSuccess, t],
  );

  const signInWithFaceId = useCallback(
    async (formValues: TValues) => {
      const faceOwner = application?.organization || formValues.organization;
      await runFaceIdLogin({
        owner: faceOwner,
        username: formValues.username?.trim() ?? "",
        useImageModal: hasFaceIdProvider,
        loadFaceRecognitionCommonModalModule: () =>
          import("@/pages/auth/FaceRecognitionCommonModal"),
        loadFaceRecognitionModalModule: () => import("@/pages/auth/FaceRecognitionModal"),
        requestFaceRecognition,
        createPayload: ({ username, faceId, faceIdImage }) =>
          ({
            ...createProtocolContextPayload(),
            signinMethod: "Face ID",
            organization: formValues.organization,
            username,
            password: "",
            code: undefined,
            faceId,
            faceIdImage,
            application: formValues.application,
            // complete 的协议上下文（含 applicationId）从 payload 上读，
            // 与 WebAuthn / 扫码登录同一条路径。
            applicationId: application?.id ?? undefined,
            ...createCaptchaPayload(formValues.captchaToken),
          }) as TPayload,
        onSuccess,
      });
    },
    [
      application?.id,
      application?.organization,
      createCaptchaPayload,
      createProtocolContextPayload,
      hasFaceIdProvider,
      onSuccess,
      requestFaceRecognition,
    ],
  );

  return {
    signInWithFaceId,
    signInWithWebAuthn,
  };
}
