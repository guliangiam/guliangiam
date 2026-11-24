import { useState } from "react";
import type { FormInstance } from "antd";
import type { NavigateFunction } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import type { ResponseEnvelope } from "@/shared/api/http";
import { queryKeys } from "@/shared/api/query-keys";
import { useAutoSigninFlow } from "@/shared/hooks/useAutoSigninFlow";
import { useBiometricSigninActions } from "@/shared/hooks/useBiometricSigninActions";
import { useQrSigninFlow } from "@/shared/hooks/useQrSigninFlow";
import { useLoginPayloadBuilders } from "@/shared/hooks/useLoginPayloadBuilders";
import { useLoginSubmissionFlow } from "@/shared/hooks/useLoginSubmissionFlow";
import { useLoginTransitionFlow } from "@/shared/hooks/useLoginTransitionFlow";
import { useVerificationCodeFlow } from "@/shared/hooks/useVerificationCodeFlow";
import {
  executeSamlLoginTransition,
  resolveSamlLoginTransition,
  type PendingMfaStateDraft,
  type SamlPendingMfaStateDraft,
} from "@/shared/lib/auth-login-transitions";
import { DEFAULT_ORGANIZATION_NAME, type LoginMethodKey } from "@/shared/lib/auth-signin-items";
import type { LoginPageAuthContextState } from "./useLoginPageAuthContext";
import type { LoginPageFormFlowsState } from "./useLoginPageFormFlows";

export type LoginPageFormValues = {
  organization: string;
  application: string;
  countryCode?: string;
  username: string;
  password?: string;
  code?: string;
  captchaToken?: string;
  agreement?: boolean;
  autoSignin?: boolean;
};

type LoginPagePayload = {
  type?: string;
  signinMethod?: string;
  organization: string;
  username: string;
  application: string;
  password?: string;
  code?: string;
  clientId?: string;
  redirectUri?: string;
  state?: string;
  scope?: string;
  service?: string;
  relayState?: string;
  samlRequest?: string;
  captchaType?: string;
  captchaToken?: string;
  clientSecret?: string;
  faceId?: number[];
  faceIdImage?: string[];
  mfaType?: string;
  passcode?: string;
  recoveryCode?: string;
  enableMfaRemember?: boolean;
  challengeId?: string;
  applicationId?: string;
};

type PendingMfaState = PendingMfaStateDraft<LoginPagePayload>;

type SessionAccountLike = {
  owner?: string | null;
  name?: string | null;
} | null;

type FaceRecognitionRequest = (mode: "vector" | "images") => Promise<{
  faceId?: number[];
  faceIdImage?: string[];
}>;

type UseLoginPageAuthActionsValues = {
  form: FormInstance<LoginPageFormValues>;
  loginMethod: LoginMethodKey;
  pathname: string;
  search: string;
  navigate: NavigateFunction;
  sessionAccount: SessionAccountLike;
  authContext: LoginPageAuthContextState;
  formFlows: LoginPageFormFlowsState;
  requestFaceRecognition: FaceRecognitionRequest;
  isSessionLoading: boolean;
  showSuccess: (message: string) => void;
  showError: (message: string) => void;
  showInfo: (message: string) => void;
};

export function useLoginPageAuthActions(values: UseLoginPageAuthActionsValues) {
  const {
    authContext,
    form,
    formFlows,
    isSessionLoading,
    loginMethod,
    navigate,
    pathname,
    requestFaceRecognition,
    search,
    sessionAccount,
    showError,
    showInfo,
    showSuccess,
  } = values;
  const {
    authApplicationQuery,
    autoSigninEnabled,
    autoSigninOrganization,
    canAutoSigninWithCurrentSession,
    casContext,
    hasFaceIdProvider,
    isCasLogin,
    isOauthAuthorize,
    isSamlLogin,
    mfaSetupCompleted,
    oauthContext,
    resolvedApplication,
    returnTo,
    samlContext,
    silentSignin,
    watchedApplication,
    watchedOrganization,
    brandedApplicationQuery,
  } = authContext;
  const { captchaRequired, captchaState } = formFlows;
  const [pendingMfa, setPendingMfa] = useState<PendingMfaState | null>(null);
  const queryClient = useQueryClient();

  const invalidateSession = () =>
    queryClient.invalidateQueries({ queryKey: queryKeys.session.all });

  const { applyLoginTransition, createLoginTransition, verifyPendingMfa } = useLoginTransitionFlow({
    mfaSetupCompleted,
    pendingMfa,
    setPendingMfa: (nextPendingMfa) => {
      setPendingMfa(nextPendingMfa as PendingMfaState | null);
    },
    returnTo,
    application: resolvedApplication,
    navigate,
    showSuccess,
    showError,
    showInfo,
  });

  const { createProtocolContextPayload, createCaptchaPayload } = useLoginPayloadBuilders({
    isSamlLogin,
    isCasLogin,
    oauthContext,
    casService: casContext?.service,
    samlRelayState: samlContext?.relayState,
    samlRequest: samlContext?.samlRequest,
    captchaRequired,
    captchaState,
  });

  const handleSamlOnSuccess = async ({
    response,
    payload,
  }: {
    response: ResponseEnvelope;
    payload: LoginPagePayload;
  }) => {
    const transition = resolveSamlLoginTransition({
      payload,
      response,
      returnTo,
      isOauthAuthorize,
      consentApplicationName: resolvedApplication?.name,
      locationSearch: search,
    });

    await executeSamlLoginTransition({
      transition,
      response,
      payload,
      captchaRequired,
      showError,
      showInfo,
      setPendingMfa: (nextPendingMfa: SamlPendingMfaStateDraft<LoginPagePayload>) => {
        setPendingMfa(nextPendingMfa as unknown as PendingMfaState);
      },
      invalidateSession,
      navigate,
      finishLogin: async (_response, finishPayload) => {
        await applyLoginTransition({
          kind: "success",
          finishContext: {
            organization: finishPayload.organization,
            application: finishPayload.application,
          },
        });
      },
    });
  };

  const { signInWithFaceId, signInWithWebAuthn } = useBiometricSigninActions<
    LoginPageFormValues,
    LoginPagePayload
  >({
    application: resolvedApplication,
    oauthContext,
    isCasLogin,
    isSamlLogin,
    hasFaceIdProvider,
    requestFaceRecognition,
    createProtocolContextPayload,
    createCaptchaPayload,
    onSuccess: handleSamlOnSuccess,
  });

  // 扫码登录：选中 tab 即创建会话并轮询；手机确认后由 exchange 落地会话 cookie，
  // 再把令牌文档交给与 Face ID complete 相同的统一后处理。
  const { qrSigninState, reloadQrSignin } = useQrSigninFlow({
    active: loginMethod === "qrcode",
    organizationId: resolvedApplication?.organizationId,
    applicationId: resolvedApplication?.id,
    onConfirmed: async (loginResult) => {
      const payload = {
        ...createProtocolContextPayload(),
        signinMethod: "QR Code",
        organization: watchedOrganization,
        application: watchedApplication,
        username: "",
        password: "",
        applicationId: resolvedApplication?.id,
      } as LoginPagePayload;
      // exchange 的响应就是登录结果（与 /faceid/complete 同形）：直接交给统一后处理，
      // 协议上下文（授权页）下其中的授权码/回调字段由它消费。
      await handleSamlOnSuccess({ response: { status: "ok", data: loginResult }, payload });
    },
  });

  const loginMutation = useLoginSubmissionFlow<LoginPageFormValues, LoginPagePayload>({
    loginMethod,
    captchaRequired,
    captchaProviderType: captchaState?.type,
    captchaId: captchaState?.captchaId,
    resolvedApplicationId: resolvedApplication?.id,
    createProtocolContextPayload,
    createCaptchaPayload,
    signInWithWebAuthn,
    signInWithFaceId,
    onSuccess: async ({ response, formPayload }) => {
      const transition = createLoginTransition({
        payload: formPayload,
        applicationId: formPayload.applicationId,
        response,
      });
      await applyLoginTransition(transition);
    },
    showError,
  });

  const { autoSigninMutation, onContinue, onUseOther, shouldShowSignedInContinuation } =
    useAutoSigninFlow({
      pathname,
      search,
      watchedOrganization,
      watchedApplication,
      silentSignin,
      autoSigninEnabled,
      resolvedApplicationName: resolvedApplication?.name,
      autoSigninOrganization,
      defaultOrganizationName: DEFAULT_ORGANIZATION_NAME,
      canAutoSigninWithCurrentSession,
      isSessionLoading,
      hasPendingMfa: Boolean(pendingMfa),
      isAuthApplicationLoading: authApplicationQuery.isLoading,
      isBrandedApplicationLoading: brandedApplicationQuery.isLoading,
      isLoginPending: loginMutation.isPending,
      sessionAccount,
      createProtocolContextPayload,
      onSuccess: handleSamlOnSuccess,
      showError,
      clearManualSigninFields: () => {
        form.setFieldValue("username", "");
        form.setFieldValue("password", "");
        form.setFieldValue("code", "");
      },
    });

  const sendCodeMutation = useVerificationCodeFlow<LoginPageFormValues>({
    form,
    loginMethod,
    application: resolvedApplication,
    captchaRequired,
    captchaState,
    showSuccess,
    showError,
  });

  return {
    pendingMfa,
    clearPendingMfa: () => {
      setPendingMfa(null);
    },
    verifyPendingMfa,

    onContinue,
    onUseOther,
    shouldShowSignedInContinuation,
    loadButtonLoading: loginMutation.isPending || autoSigninMutation.isPending,
    sendCodePending: sendCodeMutation.isPending,
    qrSigninState,
    reloadQrSignin,
    submitLogin: (formValues: LoginPageFormValues) => {
      loginMutation.mutate(formValues);
    },
    sendVerificationCode: () => {
      void sendCodeMutation.mutateAsync();
    },
  };
}

export type LoginPageAuthActionsState = ReturnType<typeof useLoginPageAuthActions>;
