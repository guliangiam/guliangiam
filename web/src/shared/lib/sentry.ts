import { Component, type ErrorInfo, type ReactNode } from "react";

/**
 * Sentry SDK is loaded lazily only when VITE_SENTRY_DSN is configured.
 * This avoids bundling the large @sentry/react package (~30KB gzipped)
 * into the critical path when Sentry is not used.
 */

type SentryModule = typeof import("@sentry/react");

let sentryModule: SentryModule | null = null;

/**
 * Initialize Sentry SDK for frontend error tracking.
 * Requires VITE_SENTRY_DSN environment variable to activate.
 * When DSN is not configured, all Sentry operations become no-ops.
 * When DSN is configured, @sentry/react is dynamically imported.
 */
export function initSentry(): void {
  if (!import.meta.env.VITE_SENTRY_DSN) return;

  import("@sentry/react").then((Sentry) => {
    sentryModule = Sentry;
    Sentry.init({
      dsn: import.meta.env.VITE_SENTRY_DSN,
      environment: import.meta.env.MODE,
      release: import.meta.env.VITE_APP_VERSION,
      integrations: [Sentry.browserTracingIntegration(), Sentry.replayIntegration()],
      tracesSampleRate: 0.1,
      replaysSessionSampleRate: 0,
      replaysOnErrorSampleRate: 1.0,
    });
    return undefined;
  });
}

/**
 * Lightweight error boundary that captures errors to Sentry when available.
 * Falls back to console.error when Sentry is not configured.
 * This avoids importing @sentry/react synchronously.
 */
export class SentryErrorBoundary extends Component<
  {
    children: ReactNode;
    fallback?: ReactNode;
  },
  { hasError: boolean; error: Error | null }
> {
  constructor(props: { children: ReactNode; fallback?: ReactNode }) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    if (sentryModule) {
      sentryModule.captureException(error, {
        contexts: { react: { componentStack: info.componentStack } },
      });
    } else {
      console.error("Uncaught error:", error, info);
    }
  }

  render() {
    if (this.state.hasError) {
      return this.props.fallback ?? null;
    }
    return this.props.children;
  }
}

/**
 * Lightweight capture helper for imperative error reporting outside React
 * component boundaries (e.g. API catch blocks, async utilities).
 *
 * When Sentry is configured (VITE_SENTRY_DSN), delegates to captureException
 * so the error shows up in the Sentry dashboard with stack + breadcrumbs.
 * Otherwise falls back to console.error — preserving parity with the error
 * boundary's own fallback and avoiding bundling the SDK on the cold path.
 */
export function captureError(error: unknown, context?: Record<string, unknown>): void {
  if (!import.meta.env.VITE_SENTRY_DSN) {
    console.error(error);
    return;
  }
  // sentryModule is populated asynchronously by initSentry's dynamic import.
  // If captureError runs before that resolves (e.g. during initial boot),
  // fall back to console.error rather than silently dropping the signal.
  if (!sentryModule) {
    console.error(error, context ?? {});
    return;
  }
  const err = error instanceof Error ? error : new Error(String(error));
  if (context) {
    sentryModule.captureException(err, { contexts: { data: context } });
  } else {
    sentryModule.captureException(err);
  }
}
