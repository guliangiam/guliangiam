import { useCallback, useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { LoginResult } from "@/shared/api/iam/auth";
import { AUTH_AUTHORIZE } from "@/shared/api/paths";
import { queryKeys } from "@/shared/api/query-keys";
import {
  consumePendingMfaSetup,
  createMfaSetupResumeContext,
  storePendingMfaSetup,
} from "@/shared/lib/auth-mfa-setup";
import {
  executeLoginTransition,
  resolveLoginTransition,
  type LoginTransition,
  type PendingMfaStateDraft,
} from "@/shared/lib/auth-login-transitions";
import {
  finishConsoleLogin as runFinishConsoleLogin,
  hasOauthAuthorizeParams,
  persistLastLogin,
} from "@/shared/lib/auth-finishers";
import {
  verifyPendingMfa as runVerifyPendingMfa,
  type PendingMfaVerifyValues,
} from "@/shared/lib/auth-mfa-verify";
import type { ApplicationItem } from "@/shared/types/domain";

type LoginTransitionPayload = {
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
  applicationId?: string;
  challengeId?: string;
};

type PendingMfaState = PendingMfaStateDraft<LoginTransitionPayload>;

type UseLoginTransitionFlowValues = {
  mfaSetupCompleted: boolean;
  pendingMfa: PendingMfaState | null;
  setPendingMfa: (pendingMfa: PendingMfaState | null) => void;
  returnTo: string;
  application: ApplicationItem | null;
  navigate: (path: string) => void;
  showSuccess: (message: string) => void;
  showError: (message: string) => void;
  showInfo: (message: string) => void;
};

export function useLoginTransitionFlow(values: UseLoginTransitionFlowValues) {
  const {
    application: _application,
    mfaSetupCompleted,
    navigate,
    pendingMfa,
    returnTo,
    setPendingMfa,
    showError: _showError,
    showInfo,
    showSuccess,
  } = values;
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  useEffect(() => {
    if (!mfaSetupCompleted || pendingMfa) {
      return;
    }

    const pendingSetup = consumePendingMfaSetup();
    if (!pendingSetup) {
      return;
    }

    const resumeContext = createMfaSetupResumeContext(pendingSetup);
    setPendingMfa({
      source: "auth",
      ...resumeContext,
    });
  }, [mfaSetupCompleted, pendingMfa, setPendingMfa]);

  const invalidateSession = useCallback(
    () => queryClient.invalidateQueries({ queryKey: queryKeys.session.all }),
    [queryClient],
  );

  const finishConsoleLogin = useCallback(
    async (payload: { organization: string; application: string }) => {
      // OAuth 授权页上的密码登录：登录接口只建会话、不签发授权码，
      // 若按控制台登录流程 navigate(returnTo) 会停留在同一个授权页。
      // 会话 Cookie 此时已建立，直接把原始授权参数交给标准 authorize 端点：
      // 由它校验 PKCE、签发 code 并 302 回 redirect_uri。
      // INC-FE-05：仅当 URL 携带真实授权参数才早退，无参手访回落控制台流；
      // INC-FE-06：早退前同样持久化 lastLogin，与控制台流保持一致。
      if (
        window.location.pathname === "/login/oauth/authorize" &&
        hasOauthAuthorizeParams(window.location.search)
      ) {
        persistLastLogin(payload);
        await invalidateSession();
        window.location.assign(`${AUTH_AUTHORIZE}${window.location.search}`);
        return;
      }

      await runFinishConsoleLogin(payload, {
        invalidateSession,
        navigate,
        returnTo,
        showSuccess,
      });
    },
    [invalidateSession, navigate, returnTo, showSuccess],
  );

  const createLoginTransition = useCallback(
    (transitionValues: {
      payload: LoginTransitionPayload;
      applicationId?: string;
      response: LoginResult;
    }) =>
      resolveLoginTransition({
        payload: transitionValues.payload,
        applicationId: transitionValues.applicationId,
        response: transitionValues.response,
        returnTo,
      }),
    [returnTo],
  );

  const applyLoginTransition = useCallback(
    async (
      transition: LoginTransition<LoginTransitionPayload>,
      options?: {
        onPendingMfa?: (
          pendingMfa: PendingMfaStateDraft<LoginTransitionPayload>,
        ) => Promise<void> | void;
      },
    ) => {
      await executeLoginTransition({
        transition,
        setPendingMfa: (nextPendingMfa) => {
          setPendingMfa(nextPendingMfa);
        },
        showInfo,
        storePendingSetup: storePendingMfaSetup,
        navigate,
        finishConsoleLogin: finishConsoleLogin,
        onPendingMfa: options?.onPendingMfa,
      });
    },
    [finishConsoleLogin, navigate, setPendingMfa, showInfo],
  );

  const verifyPendingMfa = useCallback(
    async (verifyValues: PendingMfaVerifyValues) => {
      if (!pendingMfa) {
        throw new Error(t("mfa.noPendingChallenge"));
      }

      await runVerifyPendingMfa({
        pendingMfa,
        verifyValues,
        createTransition: (response: LoginResult) =>
          createLoginTransition({
            payload: pendingMfa.loginValues,
            applicationId: pendingMfa.loginValues.applicationId,
            response,
          }),
        applyTransition: applyLoginTransition,
        setPendingMfa: (nextPendingMfa: PendingMfaStateDraft<LoginTransitionPayload>) => {
          setPendingMfa(nextPendingMfa);
        },
        showInfo,
      });
    },
    [applyLoginTransition, createLoginTransition, pendingMfa, t, setPendingMfa, showInfo],
  );

  return {
    applyLoginTransition,
    createLoginTransition,
    verifyPendingMfa,
  };
}
