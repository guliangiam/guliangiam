import { useEffect, useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import { getAuthAccountContext } from "@/shared/api/iam/auth";
import { useSessionStore } from "@/features/session/session-store";
import { HttpError } from "@/shared/api/http";
import { queryKeys } from "@/shared/api/query-keys";

const SESSION_STALE_MS = 30_000; // 30s — session context rarely changes mid-session

export function useSessionBootstrap() {
  const account = useSessionStore((state) => state.account);
  const organization = useSessionStore((state) => state.organization);
  const setSession = useSessionStore((state) => state.setSession);

  // Track whether we have ever successfully bootstrapped in this SPA session.
  // Prevents a race where a 2nd AppShell mount (e.g. React Router re-render
  // after logout+login transition) kicks off a redundant 401 refetch that
  // briefly clears the store mid-navigation.
  const bootstrappedRef = useRef(false);

  const query = useQuery({
    queryKey: queryKeys.session.all,
    queryFn: async () => {
      return await getAuthAccountContext();
    },
    staleTime: SESSION_STALE_MS,
    // 401/404 on /auth/account is a normal "not logged in" state,
    // not an error that should trigger retry or global redirect.
    retry: (failureCount, error) => {
      if (error instanceof HttpError && (error.status === 401 || error.status === 404))
        return false;
      return failureCount < 2;
    },
  });

  useEffect(() => {
    if (query.data) {
      bootstrappedRef.current = true;
      setSession({
        account: query.data.account,
        organization: query.data.organization,
        passwordPolicy: query.data.passwordPolicy,
        sessionPolicy: query.data.sessionPolicy,
        needUpdatePassword: query.data.needUpdatePassword,
      });
    } else if (
      query.error instanceof HttpError &&
      (query.error.status === 401 || query.error.status === 404)
    ) {
      // Only clear if we have *never* successfully bootstrapped this session.
      // If we previously had a valid session and now see 401 (e.g. CSRF
      // expiration during an active page), leave the last-known-good state
      // alone — individual API calls will surface their own 401s and the
      // auth:unauthorized listener handles redirect.
      if (!bootstrappedRef.current) {
        setSession({
          account: null,
          organization: null,
          passwordPolicy: null,
          sessionPolicy: null,
          needUpdatePassword: null,
        });
      }
    }
  }, [query.data, query.error, setSession]);

  // 后端不可用（5xx / 网络中断）与「未登录」（401/404）必须区分：前者不应把
  // 用户静默重定向到登录页（RT-09）。非 401/404 的引导失败一律视为不可用。
  const isBackendUnavailable =
    !!query.error &&
    !(
      query.error instanceof HttpError &&
      (query.error.status === 401 || query.error.status === 404)
    );

  return {
    isLoading: query.isLoading,
    account,
    organization,
    isBackendUnavailable,
    refetch: query.refetch,
  };
}
