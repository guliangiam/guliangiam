import { t } from "i18next";
import { type LoginResult, loginWithPassword } from "@/shared/api/iam/auth";
import type { ResponseEnvelope } from "@/shared/api/http";
import { toVerificationMfaType } from "@/shared/lib/auth-mfa-setup";
import { submitCasChallenge, type ChallengePendingState } from "@/shared/lib/auth-challenge";
import type { LoginTransition, PendingMfaStateDraft } from "@/shared/lib/auth-login-transitions";

export type PendingMfaVerifyValues = {
  mfaType: string;
  passcode?: string;
  recoveryCode?: string;
  enableMfaRemember?: boolean;
};

type PendingMfaPayloadBase = {
  organization: string;
  username: string;
  application: string;
  challengeId?: string;
  applicationId?: string;
};

export type SamlPendingMfaVerifyPayload<TPayload> = TPayload & {
  mfaType: string;
  passcode: string;
  recoveryCode: string;
  enableMfaRemember: boolean;
};

export function createSamlPendingMfaVerifyPayload<TPayload>(values: {
  loginValues: TPayload;
  verifyValues: PendingMfaVerifyValues;
}): SamlPendingMfaVerifyPayload<TPayload> {
  return {
    ...values.loginValues,
    mfaType: values.verifyValues.mfaType,
    passcode: values.verifyValues.passcode ?? "",
    recoveryCode: values.verifyValues.recoveryCode ?? "",
    enableMfaRemember: values.verifyValues.enableMfaRemember ?? false,
  };
}

export async function verifySamlPendingMfa<TPayload>(values: {
  loginValues: TPayload;
  verifyValues: PendingMfaVerifyValues;
  submit: (payload: SamlPendingMfaVerifyPayload<TPayload>) => Promise<ResponseEnvelope>;
  onPendingMfa: (
    pending: ChallengePendingState<SamlPendingMfaVerifyPayload<TPayload>>,
  ) => void | Promise<void>;
  showInfo: (message: string) => void;
  onComplete: (
    response: ResponseEnvelope,
    payload: SamlPendingMfaVerifyPayload<TPayload>,
  ) => Promise<void>;
  errorMessage?: string;
}) {
  const payload = createSamlPendingMfaVerifyPayload({
    loginValues: values.loginValues,
    verifyValues: values.verifyValues,
  });

  await submitCasChallenge({
    payload,
    submit: values.submit,
    onPendingMfa: values.onPendingMfa,
    showInfo: values.showInfo,
    onComplete: values.onComplete,
    errorMessage: values.errorMessage,
  });
}

export async function verifyPendingMfa<TPayload extends PendingMfaPayloadBase>(values: {
  pendingMfa: PendingMfaStateDraft<TPayload>;
  verifyValues: PendingMfaVerifyValues;
  createTransition: (response: LoginResult) => LoginTransition<TPayload>;
  applyTransition: (
    transition: LoginTransition<TPayload>,
    options?: {
      onPendingMfa?: (pendingMfa: PendingMfaStateDraft<TPayload>) => Promise<void> | void;
    },
  ) => Promise<void>;
  setPendingMfa: (pendingMfa: PendingMfaStateDraft<TPayload>) => void;
  showInfo: (message: string) => void;
  verifyImpl?: typeof loginWithPassword;
  toVerificationMfaTypeImpl?: typeof toVerificationMfaType;
  pushPendingMessage?: string;
  errorMessage?: string;
}) {
  const challengeId = values.pendingMfa.loginValues.challengeId;
  if (!challengeId) {
    throw new Error(t("auth:signin.mfa.missingChallengeId"));
  }

  const verifyImpl = values.verifyImpl ?? loginWithPassword;
  const toVerificationMfaTypeImpl = values.toVerificationMfaTypeImpl ?? toVerificationMfaType;
  const response = await verifyImpl({
    challengeId,
    mfaType: toVerificationMfaTypeImpl(values.verifyValues.mfaType),
    passcode: values.verifyValues.passcode,
    recoveryCode: values.verifyValues.recoveryCode,
    enableMfaRemember: values.verifyValues.enableMfaRemember,
  });

  const transition = values.createTransition(response);
  await values.applyTransition(transition, {
    onPendingMfa: (nextPendingMfa) => {
      values.setPendingMfa(nextPendingMfa);
      if (nextPendingMfa.challengeState === "push_pending") {
        values.showInfo(values.pushPendingMessage ?? t("auth:signin.mfa.pushPendingMessage"));
        return;
      }
      throw new Error(values.errorMessage ?? t("auth:signin.mfa.verifyFailed"));
    },
  });
}
