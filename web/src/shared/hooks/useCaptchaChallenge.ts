import { useCallback, useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getCaptcha, getCaptchaStatus, type CaptchaState } from "@/shared/lib/auth-captcha-shim";

const SERVER_GENERATED_TYPES = new Set(["default", "go_captcha"]);

/**
 * Organization-scoped human-check challenge for signup and verification-code
 * delivery. Unlike the login-coupled useCaptchaFlow, this hook only needs the
 * organization slug/id: the server decides whether a captcha provider is
 * enabled (captcha-status.required) and which type renders. The collected
 * `token` plus `captchaId` are submitted alongside the guarded action so the
 * backend EnforceCaptcha gate can verify them.
 */
export function useCaptchaChallenge(organizationId: string, enabled = true) {
  const [state, setState] = useState<CaptchaState | null>(null);
  const [token, setToken] = useState("");
  const [seed, setSeed] = useState(0);

  const statusQuery = useQuery({
    queryKey: ["captcha-challenge-status", organizationId],
    enabled: enabled && Boolean(organizationId),
    queryFn: () => getCaptchaStatus({ organizationId }),
  });

  const required = statusQuery.data?.required === true;
  const providerType = statusQuery.data?.providerType ?? undefined;
  const siteKey = statusQuery.data?.siteKey ?? undefined;

  const reload = useCallback(async () => {
    setToken("");
    if (!required || !organizationId) {
      setState(null);
      return;
    }
    // Third-party widgets render client-side from the public site key; only
    // server-generated challenges need a fetched image (the factory rejects
    // generate for third-party types).
    if (providerType && !SERVER_GENERATED_TYPES.has(providerType)) {
      setState({ type: providerType, clientId: siteKey ?? null });
      setSeed((value) => value + 1);
      return;
    }
    const captcha = await getCaptcha({ organizationId });
    setState({ ...captcha, type: captcha.type ?? providerType ?? null, clientId: siteKey ?? null });
    setSeed((value) => value + 1);
  }, [required, organizationId, providerType, siteKey]);

  useEffect(() => {
    async function syncChallenge() {
      await reload();
    }
    void syncChallenge();
  }, [reload]);

  return {
    required,
    providerType,
    state,
    seed,
    token,
    setToken,
    reload,
    captchaId: state?.captchaId ?? "",
  };
}
