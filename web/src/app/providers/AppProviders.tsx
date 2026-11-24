import message from "antd/es/message";
import {
  MutationCache,
  QueryClient,
  QueryClientProvider,
  keepPreviousData,
} from "@tanstack/react-query";
import type { PropsWithChildren } from "react";
import { useState, useEffect } from "react";
import { t } from "i18next";
import type { TFunction } from "i18next";
import { setQueryClient } from "@/shared/api/query-client";
import { OrganizationThemeProvider } from "@/features/theme/OrganizationThemeProvider";
import { isHttpError } from "@/shared/api/http";
import { resolveCacheStrategy, CACHE_DYNAMIC } from "@/shared/api/query-keys";
import { serverErrorMessage } from "@/shared/lib/server-error";
import { initWebVitals } from "@/shared/lib/web-vitals";
import "@/shared/i18n";

function useOnlineStatus() {
  const [isOnline, setIsOnline] = useState(
    typeof navigator !== "undefined" ? navigator.onLine : true,
  );

  useEffect(() => {
    const goOnline = () => setIsOnline(true);
    const goOffline = () => setIsOnline(false);

    window.addEventListener("online", goOnline);
    window.addEventListener("offline", goOffline);
    return () => {
      window.removeEventListener("online", goOnline);
      window.removeEventListener("offline", goOffline);
    };
  }, []);

  return isOnline;
}

function OfflineBanner() {
  const isOnline = useOnlineStatus();
  /* WCAG 3.1.1 Language of Page — use i18n instead of hardcoded English (E-01) */
  if (isOnline) return null;
  return (
    <div
      style={{
        position: "fixed",
        bottom: 0,
        left: 0,
        right: 0,
        padding: "8px 16px",
        background: "var(--gl-color-warning)",
        color: "#fff",
        textAlign: "center",
        fontSize: 14,
        fontWeight: 500,
        zIndex: 9999,
      }}
    >
      {t(
        "common:offlineWarning",
        "Network disconnected. Changes will not be saved until connection is restored.",
      )}
    </div>
  );
}

export type MutationErrorNotifier = (error: unknown) => void;

/**
 * Localized sentence for a failed mutation.
 *
 * `serverErrorMessage` maps the HTTP status to an existing fourteen-locale
 * key; the backend's English operator prose never reaches the console.
 */
// oxlint-disable-next-line react/only-export-components -- provider module also exports its mutation-error policy for tests
export function mutationErrorText(error: unknown, translate: TFunction = t): string {
  return serverErrorMessage(error, translate, "common:operationFailed");
}

function defaultMutationErrorNotifier(error: unknown): void {
  message.error(mutationErrorText(error));
}

/**
 * Query client shared by the app and its tests.
 *
 * Mutation error presentation has one dedup rule:
 *  - a mutation that declares its own `onError` owns the presentation;
 *  - a mutation whose caller renders the failure inline (form `Alert`, field
 *    mapping) opts out with `meta: { suppressGlobalError: true }`;
 *  - everything else is an unconsumed error and gets one global fallback toast.
 * 401 stays silent because `http.ts` owns the redirect.
 */
// oxlint-disable-next-line react/only-export-components -- provider module also exports its query client for tests
export function createAppQueryClient(
  options: { notifyMutationError?: MutationErrorNotifier } = {},
): QueryClient {
  const notify = options.notifyMutationError ?? defaultMutationErrorNotifier;

  return new QueryClient({
    mutationCache: new MutationCache({
      onError: (error, _variables, _context, mutation) => {
        if (isHttpError(error) && error.status === 401) return;
        if (mutation.options.onError) return;
        if (mutation.meta?.suppressGlobalError === true) return;
        notify(error);
      },
    }),
    defaultOptions: {
      queries: {
        // Default staleTime/gcTime for dynamic data (Tier 3).
        // Per-query overrides are applied via resolveCacheStrategy in the
        // queryCache subscribe callback below.
        staleTime: CACHE_DYNAMIC.staleTime,
        gcTime: CACHE_DYNAMIC.gcTime,
        refetchOnWindowFocus: false,
        structuralSharing: true,
        placeholderData: keepPreviousData,
        retry: (failureCount, error) => {
          // Auth/session requests: never retry (avoid repeated login redirects)
          if (isHttpError(error) && error.status === 401) {
            return false;
          }
          // Don't retry on client errors (4xx)
          if (isHttpError(error) && error.status >= 400 && error.status < 500) {
            return false;
          }
          // Retry up to 2 times for server errors (5xx) and network failures
          return failureCount < 2;
        },
      },
      mutations: {
        // Mutations never retry by default
        retry: false,
      },
    },
  });
}

export function AppProviders({ children }: PropsWithChildren) {
  // Initialize Web Vitals performance monitoring
  useEffect(() => {
    initWebVitals();
  }, []);

  const [queryClient] = useState(() => createAppQueryClient());

  // Bridge: expose queryClient to route loaders (outside React scope).
  // Must come after the useState declaration: referring to queryClient earlier
  // is a temporal-dead-zone error that tsc rejects.
  useEffect(() => {
    setQueryClient(queryClient);
  }, [queryClient]);

  // Apply per-query cache strategy (staleTime/gcTime) based on queryKey prefix.
  // We subscribe to the queryCache and override staleTime/gcTime when a query is added.
  useEffect(() => {
    const seen = new WeakSet<object>();
    const subscription = queryClient.getQueryCache().subscribe((event) => {
      if (event.type === "added") {
        const query = event.query;
        if (!seen.has(query)) {
          seen.add(query);
          const strategy = resolveCacheStrategy(query.queryKey);
          // Apply the resolved cache strategy by setting query defaults
          queryClient.setQueryDefaults(query.queryKey, {
            staleTime: strategy.staleTime,
            gcTime: strategy.gcTime,
            retry: (failureCount: number, error: unknown) => {
              // Auth/session requests: never retry (avoid repeated login redirects)
              if (isHttpError(error) && error.status === 401) {
                return false;
              }
              // Don't retry on client errors (4xx)
              if (isHttpError(error) && error.status >= 400 && error.status < 500) {
                return false;
              }
              // Retry up to strategy.retry times for server errors and network failures
              return failureCount < strategy.retry;
            },
          });
        }
      }
    });
    return () => {
      if (typeof subscription === "function") subscription();
    };
  }, [queryClient]);

  return (
    <QueryClientProvider client={queryClient}>
      <OrganizationThemeProvider>
        {children}
        <OfflineBanner />
      </OrganizationThemeProvider>
    </QueryClientProvider>
  );
}
