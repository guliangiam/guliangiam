import { useCallback, useEffect, useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { ResponseEnvelope } from "@/shared/api/http";
import { localizedError } from "@/shared/lib/server-error";

type SessionAccountLike = {
  owner?: string | null;
  name?: string | null;
};

export type AutoSigninFinishPayload = {
  type?: string;
  signinMethod: "Password";
  organization: string;
  username: string;
  password: string;
  application: string;
  clientId?: string;
  redirectUri?: string;
  state?: string;
  scope?: string;
  service?: string;
  relayState?: string;
  samlRequest?: string;
};

type UseAutoSigninFlowValues = {
  pathname: string;
  search: string;
  watchedOrganization: string;
  watchedApplication: string;
  silentSignin: string | null;
  autoSigninEnabled: boolean;
  resolvedApplicationName?: string | null;
  autoSigninOrganization?: string | null;
  defaultOrganizationName: string;
  canAutoSigninWithCurrentSession: boolean;
  isSessionLoading: boolean;
  hasPendingMfa: boolean;
  isAuthApplicationLoading: boolean;
  isBrandedApplicationLoading: boolean;
  isLoginPending: boolean;
  sessionAccount: SessionAccountLike | null;
  createProtocolContextPayload: () => Pick<
    AutoSigninFinishPayload,
    | "type"
    | "clientId"
    | "redirectUri"
    | "state"
    | "scope"
    | "service"
    | "relayState"
    | "samlRequest"
  >;
  onSuccess: (values: {
    response: ResponseEnvelope;
    payload: AutoSigninFinishPayload;
  }) => Promise<void>;
  showError: (message: string) => void;
  clearManualSigninFields: () => void;
};

function isInIframe() {
  try {
    return window.self !== window.top;
  } catch {
    return true;
  }
}

function sendSilentSigninData(data: string) {
  if (typeof window === "undefined" || !isInIframe()) {
    return;
  }

  window.parent.postMessage({ tag: "GuliangIAM", type: "SilentSignin", data }, "*");
}

export function useAutoSigninFlow(values: UseAutoSigninFlowValues) {
  const {
    autoSigninEnabled,
    autoSigninOrganization,
    canAutoSigninWithCurrentSession,
    clearManualSigninFields,
    createProtocolContextPayload,
    defaultOrganizationName,
    hasPendingMfa,
    isAuthApplicationLoading,
    isBrandedApplicationLoading,
    isLoginPending,
    isSessionLoading,
    onSuccess,
    pathname,
    resolvedApplicationName,
    search,
    sessionAccount,
    showError,
    silentSignin,
    watchedApplication,
    watchedOrganization,
  } = values;
  const [showManualSignin, setShowManualSignin] = useState(false);
  const autoSigninAttemptedRef = useRef<string | null>(null);
  const silentSigninNoSessionReportedRef = useRef<string | null>(null);
  const { t } = useTranslation();

  const autoSigninMutation = useMutation({
    mutationFn: async () => {
      if (!resolvedApplicationName) {
        throw new Error(t("autoSignin.appNotResolved"));
      }

      const finishPayload: AutoSigninFinishPayload = {
        ...createProtocolContextPayload(),
        signinMethod: "Password",
        organization: autoSigninOrganization || defaultOrganizationName,
        username: sessionAccount?.name || "",
        password: "",
        application: resolvedApplicationName,
      };

      const response: ResponseEnvelope = {
        status: "ok",
        data: "AutoSignin",
      };

      return { response, finishPayload };
    },
    onSuccess: async ({ response, finishPayload }) => {
      await onSuccess({
        response,
        payload: finishPayload,
      });
    },
    onError: (error) => {
      if (silentSignin !== null) {
        sendSilentSigninData("sign-in-failed");
      }

      showError(localizedError(error, t, "autoSignin.failed"));
    },
  });

  // Both resets below are adjusted during render instead of from effects: they
  // only depend on props/derived values, so an effect would add an extra render
  // pass and briefly expose a stale value.
  const routeKey = `${pathname}|${watchedOrganization}|${watchedApplication}`;
  const [syncedRouteKey, setSyncedRouteKey] = useState(routeKey);
  if (routeKey !== syncedRouteKey) {
    setSyncedRouteKey(routeKey);
    setShowManualSignin(false);
  }
  if (!canAutoSigninWithCurrentSession && showManualSignin) {
    setShowManualSignin(false);
  }

  useEffect(() => {
    if (silentSignin === null) {
      silentSigninNoSessionReportedRef.current = null;
      return;
    }

    const noSessionReportKey = [pathname, search, watchedOrganization, watchedApplication].join(
      "|",
    );

    if (
      isSessionLoading ||
      sessionAccount ||
      silentSigninNoSessionReportedRef.current === noSessionReportKey
    ) {
      return;
    }

    sendSilentSigninData("user-not-logged-in");
    silentSigninNoSessionReportedRef.current = noSessionReportKey;
  }, [
    isSessionLoading,
    pathname,
    search,
    sessionAccount,
    silentSignin,
    watchedApplication,
    watchedOrganization,
  ]);

  const autoSigninMutateRef = useRef(autoSigninMutation.mutate);
  // Kept up to date from an effect rather than during render: writing a ref
  // while rendering is not safe with concurrent rendering.
  useEffect(() => {
    autoSigninMutateRef.current = autoSigninMutation.mutate;
  }, [autoSigninMutation.mutate]);

  useEffect(() => {
    const shouldAutoSignin = silentSignin !== null || autoSigninEnabled;
    if (
      !shouldAutoSignin ||
      !resolvedApplicationName ||
      !canAutoSigninWithCurrentSession ||
      isSessionLoading ||
      hasPendingMfa ||
      isAuthApplicationLoading ||
      isBrandedApplicationLoading ||
      isLoginPending ||
      autoSigninMutation.isPending
    ) {
      return;
    }

    const attemptKey = [
      pathname,
      search,
      resolvedApplicationName,
      sessionAccount?.owner || "",
      sessionAccount?.name || "",
    ].join("|");

    if (autoSigninAttemptedRef.current === attemptKey) {
      return;
    }

    autoSigninAttemptedRef.current = attemptKey;
    if (silentSignin !== null) {
      sendSilentSigninData("signing-in");
    }

    autoSigninMutateRef.current();
  }, [
    autoSigninEnabled,
    autoSigninMutation.isPending,
    canAutoSigninWithCurrentSession,
    hasPendingMfa,
    isAuthApplicationLoading,
    isBrandedApplicationLoading,
    isLoginPending,
    isSessionLoading,
    pathname,
    resolvedApplicationName,
    search,
    sessionAccount?.name,
    sessionAccount?.owner,
    silentSignin,
  ]);

  const onUseOther = useCallback(() => {
    setShowManualSignin(true);
    clearManualSigninFields();
  }, [clearManualSigninFields]);

  return {
    autoSigninMutation,
    onContinue: autoSigninMutation.mutate,
    onUseOther,
    shouldShowSignedInContinuation:
      canAutoSigninWithCurrentSession &&
      silentSignin === null &&
      !hasPendingMfa &&
      !showManualSignin &&
      !autoSigninMutation.isPending,
  };
}
