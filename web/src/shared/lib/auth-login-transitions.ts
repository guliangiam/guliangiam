import { t } from "i18next";
import type { LoginMfaRequired, LoginResult } from "@/shared/api/iam/auth";
import type { ResponseEnvelope } from "@/shared/api/http";
import { buildMfaSetupUrl, type PendingMfaSetupContext } from "@/shared/lib/auth-mfa-setup";
import type { MfaPropsItem } from "@/shared/types/domain";

type AuthTransitionPayloadBase = {
  organization: string;
  username: string;
  application: string;
  password?: string;
  applicationId?: string;
  challengeId?: string;
};

export type SamlPendingMfaStateDraft<TPayload> = {
  source: "legacy";
  loginValues: TPayload;
  mfaProps: MfaPropsItem[];
};

export type PendingMfaStateDraft<TPayload extends AuthTransitionPayloadBase> = {
  source: "auth";
  loginValues: TPayload & {
    applicationId?: string;
    password: string;
    challengeId: string;
  };
  mfaProps: MfaPropsItem[];
  challengeState?: string;
};

export type LoginTransition<TPayload extends AuthTransitionPayloadBase> =
  | {
      kind: "pending-mfa";
      pendingMfa: PendingMfaStateDraft<TPayload>;
      infoMessage: string;
    }
  | {
      kind: "required-mfa-setup";
      pendingSetup: PendingMfaSetupContext;
      targetUrl: string;
    }
  | {
      kind: "success";
      finishContext: {
        organization: string;
        application: string;
      };
    };

export type SamlLoginTransition<TPayload extends AuthTransitionPayloadBase> =
  | {
      kind: "error";
      message: string;
      shouldReloadCaptcha: boolean;
    }
  | {
      kind: "pending-mfa";
      pendingMfa: SamlPendingMfaStateDraft<TPayload>;
      infoMessage: string;
    }
  | {
      kind: "required-mfa-setup";
      targetUrl: string;
      shouldInvalidateSession: boolean;
    }
  | {
      kind: "consent";
      targetUrl: string;
    }
  | {
      kind: "finish";
    };

type NavigateFn = (to: string, options?: { replace?: boolean }) => void;

export function buildConsentRoute(values: { applicationName: string; locationSearch: string }) {
  return `/consent/${encodeURIComponent(values.applicationName)}${values.locationSearch}`;
}

function isConsentRequiredResponse(response: ResponseEnvelope) {
  return Boolean(
    response.data &&
    typeof response.data === "object" &&
    "required" in response.data &&
    response.data.required === true,
  );
}

export function toMfaProps(result: LoginMfaRequired) {
  return result.availableMfaMethods.map((item) => ({
    mfaType: item.methodType,
    isPreferred: item.methodType === result.nextMfa,
    secret: item.target ?? null,
    countryCode: item.countryCode ?? null,
    mfaDisplayName: item.displayName ?? null,
  }));
}

function createPendingMfaSetupContext<TPayload extends AuthTransitionPayloadBase>(values: {
  payload: TPayload;
  response: Extract<LoginResult, { status: "required_mfa_setup" }>;
  returnTo: string;
}): PendingMfaSetupContext {
  return {
    organization: values.payload.organization,
    application: values.payload.application,
    username: values.payload.username,
    returnTo: values.returnTo,
    challengeId: values.response.challengeId,
    requiredMfa: values.response.requiredMfa,
  };
}

function createSamlPendingMfaState<TPayload extends AuthTransitionPayloadBase>(values: {
  payload: TPayload;
  response: ResponseEnvelope;
}): SamlPendingMfaStateDraft<TPayload> {
  // Clear password from the payload to avoid keeping it in React state.
  const { password: _password, ..._safePayload } = values.payload as AuthTransitionPayloadBase & {
    password?: string;
  };
  return {
    source: "legacy",
    loginValues: { ...values.payload, password: "" } as TPayload,
    mfaProps: Array.isArray(values.response.data2) ? (values.response.data2 as MfaPropsItem[]) : [],
  };
}

function createPendingMfaState<TPayload extends AuthTransitionPayloadBase>(values: {
  payload: TPayload;
  applicationId?: string;
  response: LoginMfaRequired;
}): PendingMfaStateDraft<TPayload> {
  return {
    source: "auth",
    loginValues: {
      ...values.payload,
      applicationId: values.applicationId ?? values.payload.applicationId,
      password: "",
      challengeId: values.response.challengeId,
    },
    mfaProps: toMfaProps(values.response),
    challengeState: values.response.challengeState,
  };
}

export function resolveLoginTransition<TPayload extends AuthTransitionPayloadBase>(values: {
  payload: TPayload;
  applicationId?: string;
  response: LoginResult;
  returnTo: string;
}): LoginTransition<TPayload> {
  switch (values.response.status) {
    case "mfa_required":
      return {
        kind: "pending-mfa",
        pendingMfa: createPendingMfaState({
          payload: values.payload,
          applicationId: values.applicationId,
          response: values.response,
        }),
        infoMessage: t("auth:signin.mfaRequiredInfo"),
      };
    case "required_mfa_setup": {
      const pendingSetup = createPendingMfaSetupContext({
        payload: values.payload,
        response: values.response,
        returnTo: values.returnTo,
      });
      return {
        kind: "required-mfa-setup",
        targetUrl: buildMfaSetupUrl({
          owner: pendingSetup.organization,
          name: pendingSetup.username,
          returnTo: pendingSetup.returnTo,
          requiredMfa: pendingSetup.requiredMfa,
        }),
        pendingSetup,
      };
    }
    default:
      return {
        kind: "success",
        finishContext: {
          organization: values.payload.organization,
          application: values.payload.application,
        },
      };
  }
}

export function resolveSamlLoginTransition<TPayload extends AuthTransitionPayloadBase>(values: {
  payload: TPayload;
  response: ResponseEnvelope;
  returnTo: string;
  isOauthAuthorize: boolean;
  consentApplicationName?: string | null;
  locationSearch: string;
}): SamlLoginTransition<TPayload> {
  if (values.response.status !== "ok") {
    return {
      kind: "error",
      message: values.response.msg || t("auth:signin.loginFailed"),
      shouldReloadCaptcha: true,
    };
  }

  if (values.response.data === "NextMfa") {
    return {
      kind: "pending-mfa",
      pendingMfa: createSamlPendingMfaState(values),
      infoMessage: t("auth:signin.mfaRequiredInfo"),
    };
  }

  if (values.response.data === "RequiredMfa") {
    return {
      kind: "required-mfa-setup",
      targetUrl: buildMfaSetupUrl({
        owner: values.payload.organization,
        name: values.payload.username,
        returnTo: values.returnTo,
      }),
      shouldInvalidateSession: true,
    };
  }

  if (values.isOauthAuthorize && isConsentRequiredResponse(values.response)) {
    if (!values.consentApplicationName) {
      return {
        kind: "error",
        message: t("auth:signin.samlConsentMissingApp"),
        shouldReloadCaptcha: false,
      };
    }

    return {
      kind: "consent",
      targetUrl: buildConsentRoute({
        applicationName: values.consentApplicationName,
        locationSearch: values.locationSearch,
      }),
    };
  }

  return { kind: "finish" };
}

export async function executeLoginTransition<TPayload extends AuthTransitionPayloadBase>(values: {
  transition: LoginTransition<TPayload>;
  setPendingMfa: (pendingMfa: PendingMfaStateDraft<TPayload>) => void;
  showInfo: (message: string) => void;
  storePendingSetup: (context: PendingMfaSetupContext) => void;
  navigate: NavigateFn;
  finishConsoleLogin: (context: { organization: string; application: string }) => Promise<void>;
  onPendingMfa?: (pendingMfa: PendingMfaStateDraft<TPayload>) => Promise<void> | void;
}) {
  switch (values.transition.kind) {
    case "pending-mfa":
      if (values.onPendingMfa) {
        await values.onPendingMfa(values.transition.pendingMfa);
        return;
      }

      values.setPendingMfa(values.transition.pendingMfa);
      values.showInfo(values.transition.infoMessage);
      return;
    case "required-mfa-setup":
      values.storePendingSetup(values.transition.pendingSetup);
      values.navigate(values.transition.targetUrl);
      return;
    case "success":
      await values.finishConsoleLogin(values.transition.finishContext);
      return;
  }
}

export async function executeSamlLoginTransition<
  TPayload extends AuthTransitionPayloadBase,
>(values: {
  transition: SamlLoginTransition<TPayload>;
  response: ResponseEnvelope;
  payload: TPayload;
  captchaRequired?: boolean;
  reloadCaptcha?: () => unknown;
  showError: (message: string) => void;
  showInfo: (message: string) => void;
  setPendingMfa: (pendingMfa: SamlPendingMfaStateDraft<TPayload>) => void;
  invalidateSession: () => Promise<void>;
  navigate: NavigateFn;
  finishLogin: (response: ResponseEnvelope, payload: TPayload) => Promise<void>;
}) {
  switch (values.transition.kind) {
    case "error":
      values.showError(values.transition.message);
      if (values.transition.shouldReloadCaptcha && values.captchaRequired) {
        values.reloadCaptcha?.();
      }
      return;
    case "pending-mfa":
      values.setPendingMfa(values.transition.pendingMfa);
      values.showInfo(values.transition.infoMessage);
      return;
    case "required-mfa-setup":
      if (values.transition.shouldInvalidateSession) {
        await values.invalidateSession();
      }
      values.navigate(values.transition.targetUrl);
      return;
    case "consent":
      values.navigate(values.transition.targetUrl, { replace: true });
      return;
    case "finish":
      await values.finishLogin(values.response, values.payload);
      return;
  }
}

export async function resolveAndExecuteSamlLoginTransition<
  TPayload extends AuthTransitionPayloadBase,
>(values: {
  payload: TPayload;
  response: ResponseEnvelope;
  returnTo: string;
  isOauthAuthorize: boolean;
  consentApplicationName?: string | null;
  locationSearch: string;
  captchaRequired?: boolean;
  reloadCaptcha?: () => unknown;
  showError: (message: string) => void;
  showInfo: (message: string) => void;
  setPendingMfa: (pendingMfa: SamlPendingMfaStateDraft<TPayload>) => void;
  invalidateSession: () => Promise<void>;
  navigate: NavigateFn;
  finishLogin: (response: ResponseEnvelope, payload: TPayload) => Promise<void>;
}) {
  const transition = resolveSamlLoginTransition({
    payload: values.payload,
    response: values.response,
    returnTo: values.returnTo,
    isOauthAuthorize: values.isOauthAuthorize,
    consentApplicationName: values.consentApplicationName,
    locationSearch: values.locationSearch,
  });

  await executeSamlLoginTransition({
    transition,
    response: values.response,
    payload: values.payload,
    captchaRequired: values.captchaRequired,
    reloadCaptcha: values.reloadCaptcha,
    showError: values.showError,
    showInfo: values.showInfo,
    setPendingMfa: values.setPendingMfa,
    invalidateSession: values.invalidateSession,
    navigate: values.navigate,
    finishLogin: values.finishLogin,
  });
}
