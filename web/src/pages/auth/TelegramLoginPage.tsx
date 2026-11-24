import { useEffect, useMemo, useState } from "react";
import { Alert, Card, Skeleton, Typography, message } from "antd";
import { useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { getProviders } from "@/shared/api/iam/provider-extended";
import { getQueryParamsFromState } from "@/shared/lib/provider-auth";
import type { ProviderItem } from "@/shared/types/domain";
import { localizedError } from "@/shared/lib/server-error";

type TelegramState = {
  authUrl: string;
  providerName: string;
};

function resolveTelegramState(search: string): TelegramState | null {
  const params = new URLSearchParams(search);
  const state = params.get("state");
  if (!state) {
    return null;
  }

  try {
    const innerParams = new URLSearchParams(getQueryParamsFromState(state));
    const providerName = innerParams.get("provider") ?? "";
    if (!providerName) {
      return null;
    }

    return {
      authUrl: `${window.location.origin}/callback?state=${encodeURIComponent(state)}`,
      providerName,
    };
  } catch {
    return null;
  }
}

export default function TelegramLoginPage() {
  const location = useLocation();
  const { t } = useTranslation("auth");
  const telegramState = useMemo(() => resolveTelegramState(location.search), [location.search]);
  const [botUsername, setBotUsername] = useState<string | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // A missing ?state= parameter is derived rather than pushed into state, so the
  // component never needs an effect just to report it.
  const error = telegramState ? loadError : t("telegramMissingState");
  const loading = telegramState ? isLoading : false;

  useEffect(() => {
    if (!telegramState) {
      return;
    }

    let cancelled = false;

    void (async () => {
      try {
        const providers = await getProviders("admin");
        if (cancelled) {
          return;
        }

        const telegramProvider = providers.find(
          (provider: ProviderItem) => provider.name === telegramState.providerName,
        );

        if (!telegramProvider?.clientId) {
          setLoadError(t("telegramProviderNotFound"));
          return;
        }

        setBotUsername(telegramProvider.clientId);
      } catch (requestError) {
        if (cancelled) {
          return;
        }

        const errorMessage = localizedError(requestError, t, "telegramProviderLoadFailed");
        setLoadError(errorMessage);
        message.error(errorMessage);
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [telegramState, t]);

  useEffect(() => {
    if (!botUsername || !telegramState?.authUrl) {
      return;
    }

    const existingScript = document.querySelector(
      'script[src="https://telegram.org/js/telegram-widget.js"]',
    );
    if (existingScript) {
      existingScript.remove();
    }

    const script = document.createElement("script");
    script.src = "https://telegram.org/js/telegram-widget.js";
    script.setAttribute("data-telegram-login", botUsername);
    script.setAttribute("data-size", "large");
    script.setAttribute("data-auth-url", telegramState.authUrl);
    script.setAttribute("data-request-access", "write");
    script.async = true;

    const container = document.getElementById("telegram-login-container");
    if (!container) {
      return;
    }

    container.innerHTML = "";
    container.appendChild(script);

    return () => {
      script.remove();
    };
  }, [botUsername, telegramState]);

  return (
    <div className="login-content" style={{ margin: "auto" }}>
      <div style={{ marginBottom: 10, textAlign: "center" }}>
        <Card
          style={{
            width: 400,
            maxWidth: "calc(100vw - 32px)",
            margin: "0 auto",
            marginTop: 100,
          }}
          title={t("telegramTitle")}
        >
          <div style={{ textAlign: "center", padding: 20 }}>
            {loading ? (
              <Skeleton active paragraph={{ rows: 3 }} />
            ) : error ? (
              <Alert type="error" showIcon title={error} />
            ) : (
              <>
                <Typography.Paragraph>{t("telegramPrompt")}</Typography.Paragraph>
                <div
                  id="telegram-login-container"
                  style={{
                    display: "flex",
                    justifyContent: "center",
                    marginTop: 20,
                    minHeight: 40,
                  }}
                />
              </>
            )}
          </div>
        </Card>
      </div>
    </div>
  );
}
