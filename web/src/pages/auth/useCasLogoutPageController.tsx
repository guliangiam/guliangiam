import { useCallback, useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { casLogout } from "@/shared/api/iam/auth";
import { queryKeys } from "@/shared/api/query-keys";
import { localizedError } from "@/shared/lib/server-error";

export type CasLogoutSettledState = {
  status: "settled";
  /** Sessions the server revoked across every application in the organization. */
  revokedSessions: number;
  /** True when the requested return target was not a registered redirect URI. */
  redirectRejected: boolean;
  onReturnToSignIn: () => void;
};

export type CasLogoutErrorState = {
  status: "error";
  errorMessage: string;
  onRetry: () => void;
  onReturnToSignIn: () => void;
};

export type CasLogoutPendingState = {
  status: "pending";
};

export type CasLogoutPageControllerState =
  | CasLogoutSettledState
  | CasLogoutErrorState
  | CasLogoutPendingState;

/**
 * Controller for the CAS single-logout entry point (设计报告 28.10.5 FE-014).
 *
 * The page is entered by redirect from a relying party at
 * `/cas/:owner/:casApplicationName/logout?service=…`. It does not decide
 * anything: it posts the request to the protocol endpoint and renders or follows
 * whatever verdict comes back. The `service` value is forwarded but never used
 * as a navigation target — only the server's `redirectUri` is (42.1.2).
 */
export function useCasLogoutPageController(): CasLogoutPageControllerState {
  const { t } = useTranslation("auth");
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { owner, casApplicationName } = useParams<{
    owner: string;
    casApplicationName: string;
  }>();

  const [settled, setSettled] = useState<CasLogoutSettledState | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  // The service the relying party asked to return to, taken from the entry URL.
  const service = new URLSearchParams(location.search).get("service") ?? "";

  // AC-FE-014: the only two landing pages are the whitelisted target and the CAS
  // login page for this application. The `service` value is deliberately not
  // forwarded: it was refused as a return target, so it is not a safe input for
  // the next hop either.
  const returnToLogin =
    owner && casApplicationName ? `/cas/${owner}/${casApplicationName}/login` : "/login";

  // Guards against the double invocation of an effect in React strict mode:
  // a second POST is harmless (the endpoint is idempotent) but it would revoke
  // twice within the same navigation and re-enter the redirect.
  const startedRef = useRef(false);

  useEffect(() => {
    if (startedRef.current) {
      return;
    }
    startedRef.current = true;

    let cancelled = false;

    void casLogout(service)
      .then(async (result) => {
        if (cancelled) return undefined;

        await queryClient.invalidateQueries({ queryKey: queryKeys.session.all });
        if (cancelled) return undefined;

        if (result.redirectUri) {
          // Server-adjudicated absolute URL of the relying party: a top-level
          // navigation, deliberately not a router push.
          window.location.replace(result.redirectUri);
          return undefined;
        }

        setSettled({
          status: "settled",
          revokedSessions: result.revokedSessions,
          redirectRejected: result.redirectRejected,
          onReturnToSignIn: () => navigate(returnToLogin, { replace: true }),
        });
        return undefined;
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        setErrorMessage(localizedError(error, t, "casLogoutFailed"));
      });

    return () => {
      cancelled = true;
    };
    // oxlint-disable-next-line react/exhaustive-effect-dependencies -- attempt 是「重试」的重跑键：onRetry 先重置 startedRef 再递增它来重新发起登出请求，effect 体内不读取该值
  }, [attempt, navigate, queryClient, returnToLogin, service, t]);

  const onReturnToSignIn = useCallback(() => {
    navigate(returnToLogin, { replace: true });
  }, [navigate, returnToLogin]);

  const onRetry = useCallback(() => {
    setErrorMessage(null);
    setSettled(null);
    startedRef.current = false;
    setAttempt((value) => value + 1);
  }, []);

  if (errorMessage) {
    return { status: "error", errorMessage, onRetry, onReturnToSignIn };
  }

  if (settled) {
    return settled;
  }

  return { status: "pending" };
}
