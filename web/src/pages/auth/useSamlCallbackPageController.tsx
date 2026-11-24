import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useLocation, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { queryKeys } from "@/shared/api/query-keys";
import {
  createSamlAcsPayload,
  getSamlCallbackError,
  resolveSamlCallbackContext,
  submitSamlAcs,
  type SamlCallbackContext,
} from "@/shared/lib/auth-saml-callback";
import { localizedError } from "@/shared/lib/server-error";

export type SamlCallbackPageLoadingState = {
  status: "loading";
  isSubmitting: boolean;
};

export type SamlCallbackPageErrorState = {
  status: "error";
  errorMessage: string;
};

export type SamlCallbackPageControllerState =
  | SamlCallbackPageLoadingState
  | SamlCallbackPageErrorState;

/**
 * Completes the SP-initiated SAML browser callback.
 *
 * The old implementation posted `samlResponse` to `/api/v1/auth/login`, whose
 * request type has no such field, so the payload was silently dropped. It now
 * forwards the IdP form fields to the dedicated SP ACS, which verifies the
 * assertion, consumes the one-time state and answers 303 with the session
 * cookie.
 */
export function useSamlCallbackPageController(): SamlCallbackPageControllerState {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  const startedRef = useRef(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [callbackContext, setCallbackContext] = useState<SamlCallbackContext | null>(null);

  useEffect(() => {
    let cancelled = false;
    resolveSamlCallbackContext(location.search).then((ctx) => {
      if (!cancelled) {
        setCallbackContext(ctx);
      }
      return undefined;
    });
    return () => {
      cancelled = true;
    };
  }, [location.search]);

  const callbackError = callbackContext ? getSamlCallbackError(callbackContext) : null;

  useEffect(() => {
    if (!callbackContext || callbackError || startedRef.current) {
      return;
    }

    const payload = createSamlAcsPayload(callbackContext);
    if (!payload) {
      // getSamlCallbackError already rejects a context without org/provider
      // binding, so this path is defensive only.
      return;
    }

    startedRef.current = true;
    void Promise.resolve()
      .then(() => {
        setIsSubmitting(true);
        return submitSamlAcs(payload);
      })
      .then(() =>
        queryClient.invalidateQueries({ queryKey: queryKeys.session.all }).then(() => {
          navigate("/", { replace: true });
          return undefined;
        }),
      )
      .catch((error) => {
        setSubmitError(localizedError(error, t, "saml.loginFailed"));
      })
      .finally(() => {
        setIsSubmitting(false);
      });
  }, [callbackContext, callbackError, navigate, queryClient, t]);

  const errorMessage = callbackError || submitError;
  if (errorMessage) {
    return {
      status: "error",
      errorMessage,
    };
  }

  return {
    status: "loading",
    isSubmitting,
  };
}
