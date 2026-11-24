import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Result, Button, Skeleton } from "antd";
import { useTranslation } from "react-i18next";
import { socialLogin, getAuthAccountContext } from "@/shared/api/iam/auth";
import { getProviders } from "@/shared/api/iam/provider-extended";
import { useSessionStore } from "@/features/session/session-store";
import { queryKeys } from "@/shared/api/query-keys";
import { localizedError } from "@/shared/lib/server-error";

export default function SocialCallbackPage() {
  const { t } = useTranslation("auth");
  const navigate = useNavigate();
  const { providerType } = useParams<{ providerType: string }>();
  const [searchParams] = useSearchParams();
  const [loginError, setLoginError] = useState<string | null>(null);
  const [isProcessing, setIsProcessing] = useState(true);
  const setSession = useSessionStore((state) => state.setSession);

  const code = searchParams.get("code") || "";
  const state = searchParams.get("state") || undefined;
  const applicationId = searchParams.get("application") || "app-built-in";

  const providersQuery = useQuery({
    queryKey: queryKeys.providers.all,
    queryFn: () => getProviders(),
    // The callback cannot make progress without code + state + providerType;
    // skip the provider fetch instead of burning a request on a doomed flow.
    enabled: Boolean(code && state && providerType),
    // A failed provider read is terminal on this page: the error result offers
    // an explicit retry (back to /login), so an automatic retry loop would only
    // hide the failure from the operator.
    retry: false,
  });

  // Every pre-flight failure (missing callback parameters, provider list not
  // loaded, unknown provider) is derived from its inputs instead of being pushed
  // into state by the effect, which keeps the UI correct on the first render.
  const preflightError = useMemo(() => {
    if (!code || !state || !providerType) return t("common:socialCallback.missingParams");
    if (providersQuery.isError) return t("common:socialCallback.providerLoadFailed");
    if (providersQuery.isLoading) return null;
    const providers = providersQuery.data ?? [];
    const found = providers.some((p) => p.type === providerType || p.name === providerType);
    if (!found) {
      return t("common:socialCallback.providerNotFound", {
        provider: providerType,
      });
    }
    return null;
  }, [
    code,
    state,
    providerType,
    providersQuery.data,
    providersQuery.isError,
    providersQuery.isLoading,
    t,
  ]);

  useEffect(() => {
    if (preflightError || providersQuery.isLoading) {
      return;
    }

    const providers = providersQuery.data ?? [];
    const matchedProvider = providers.find(
      (p) => p.type === providerType || p.name === providerType,
    );

    if (!matchedProvider) {
      return;
    }

    let cancelled = false;

    async function processLogin() {
      try {
        const codeVerifier = state
          ? (window.sessionStorage.getItem(`pkce_verifier_${state}`) ?? undefined)
          : undefined;
        const result = await socialLogin({
          providerId: matchedProvider!.id,
          code,
          state,
          codeVerifier,
          applicationId,
        });

        if (cancelled) return;

        if (result.status === "success") {
          // Refresh session from server and update store
          try {
            const sessionContext = await getAuthAccountContext();
            setSession({
              account: sessionContext.account,
              organization: sessionContext.organization,
              passwordPolicy: sessionContext.passwordPolicy,
              sessionPolicy: sessionContext.sessionPolicy,
              needUpdatePassword: sessionContext.needUpdatePassword,
            });
          } catch {
            // Fallback: use the result directly
            setSession({
              account: result.account,
              organization: result.organization,
              passwordPolicy: result.passwordPolicy,
              sessionPolicy: result.sessionPolicy,
              needUpdatePassword: result.needUpdatePassword,
            });
          }
          navigate("/", { replace: true });
        } else if (result.status === "mfa_required") {
          navigate("/login", {
            replace: true,
            state: { challengeId: result.challengeId },
          });
        } else if (result.status === "required_mfa_setup") {
          navigate("/mfa/setup", { replace: true });
        } else {
          // 未知 status 不能静默落空（否则 isProcessing 结束后页面空白卡死）：
          // 兜底渲染错误态，复用既有 loginFailed 文案保持 14 语言键齐全。
          setLoginError(
            t("common:socialCallback.loginFailed", "Social login failed. Please try again."),
          );
        }
      } catch (err) {
        if (cancelled) return;
        setLoginError(localizedError(err, t, "common:socialCallback.loginFailed"));
      } finally {
        if (!cancelled) {
          setIsProcessing(false);
        }
      }
    }

    processLogin();

    return () => {
      cancelled = true;
    };
  }, [
    code,
    providerType,
    state,
    applicationId,
    providersQuery.data,
    providersQuery.isLoading,
    preflightError,
    navigate,
    setSession,
    t,
  ]);

  const error = preflightError ?? loginError;
  const processing = preflightError ? false : isProcessing;

  if (processing) {
    return (
      <div
        className="gl-empty-state"
        style={{
          minHeight: "100vh",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <Skeleton active paragraph={{ rows: 4 }} title={false} />
      </div>
    );
  }

  if (error) {
    return (
      <div
        className="gl-empty-state"
        style={{
          minHeight: "100vh",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <Result
          status="error"
          title={t("common:socialCallback.failed")}
          subTitle={error}
          extra={[
            <Button
              key="retry"
              type="primary"
              onClick={() => navigate("/login", { replace: true })}
            >
              {t("common:socialCallback.tryAgain")}
            </Button>,
            <Button key="home" onClick={() => navigate("/", { replace: true })}>
              {t("common:socialCallback.goHome")}
            </Button>,
          ]}
        />
      </div>
    );
  }

  return null;
}
