import { useCallback, useEffect, useRef, useState } from "react";
import { useSessionStore } from "@/features/session/session-store";

const DEFAULT_TIMEOUT_MINUTES = 30;
const MAX_WARNING_BEFORE_MS = 5 * 60 * 1000;
const CHECK_INTERVAL_MS = 30 * 1000; // Check every 30 seconds

type SessionTimeoutState = "active" | "warning" | "expired";

export function useSessionTimeout() {
  const account = useSessionStore((s) => s.account);
  const sessionPolicy = useSessionStore((s) => s.sessionPolicy);
  const timeoutMinutes = sessionPolicy?.idleTimeoutMinutes ?? DEFAULT_TIMEOUT_MINUTES;
  const inactivityTimeoutMs = timeoutMinutes * 60 * 1000;
  // Never let the warning window swallow the whole policy (short 5min orgs).
  const warningBeforeMs = Math.min(MAX_WARNING_BEFORE_MS, inactivityTimeoutMs / 2);
  const [state, setState] = useState<SessionTimeoutState>("active");
  // Seeded on mount (not during render) so no impure call happens while rendering.
  const lastActivityRef = useRef<number>(0);
  const timerRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined);
  const [minutesRemaining, setMinutesRemaining] = useState<number>(timeoutMinutes);

  const markActivity = useCallback(() => {
    lastActivityRef.current = Date.now();
    setMinutesRemaining(timeoutMinutes);
  }, [timeoutMinutes]);

  const recordActivity = useCallback(() => {
    markActivity();
    if (state === "warning") {
      setState("active");
    }
  }, [markActivity, state]);

  const extendSession = useCallback(() => {
    markActivity();
    setState("active");
  }, [markActivity]);

  const expireNow = useCallback(() => {
    setMinutesRemaining(0);
    setState("expired");
  }, []);

  // Track user activity
  useEffect(() => {
    if (!account) return;

    lastActivityRef.current = Date.now();

    const events = ["mousedown", "keydown", "scroll", "touchstart", "click"] as const;
    const handler = () => {
      lastActivityRef.current = Date.now();
    };

    for (const event of events) {
      document.addEventListener(event, handler, { passive: true });
    }

    return () => {
      for (const event of events) {
        document.removeEventListener(event, handler);
      }
    };
  }, [account]);

  // Check inactivity periodically
  useEffect(() => {
    if (!account) return;

    timerRef.current = setInterval(() => {
      const elapsed = Date.now() - lastActivityRef.current;
      const remaining = inactivityTimeoutMs - elapsed;
      setMinutesRemaining(Math.max(0, Math.round(remaining / 60000)));

      if (remaining <= 0) {
        setState("expired");
      } else if (remaining <= warningBeforeMs) {
        setState("warning");
      } else {
        setState("active");
      }
    }, CHECK_INTERVAL_MS);

    return () => {
      if (timerRef.current) {
        clearInterval(timerRef.current);
      }
    };
  }, [account, inactivityTimeoutMs, warningBeforeMs]);

  // Signed-out sessions are always "active"; deriving this avoids pushing the
  // signed-out case into state from an effect.
  const effectiveState: SessionTimeoutState = account ? state : "active";

  // Handle expired state - redirect to login
  useEffect(() => {
    if (effectiveState !== "expired") return;

    try {
      localStorage.removeItem("gl-session");
    } catch {
      // Ignore
    }

    const currentPath = window.location.pathname;
    const loginPath = currentPath.startsWith("/login")
      ? "/login"
      : `/login?from=${encodeURIComponent(currentPath)}&reason=session_expired`;
    window.location.href = loginPath;
  }, [effectiveState]);

  return {
    state: effectiveState,
    recordActivity,
    extendSession,
    expireNow,
    /** Minutes remaining before session expires (approximate) */
    minutesRemaining: account ? minutesRemaining : Infinity,
  };
}
