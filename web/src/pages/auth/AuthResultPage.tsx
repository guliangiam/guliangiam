import { Button, Result } from "antd";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { safeRedirectUrl } from "@/shared/lib/url";

export default function AuthResultPage() {
  const { t } = useTranslation("auth");
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [countdown, setCountdown] = useState(5);

  const status = searchParams.get("status") || "success";
  const message = searchParams.get("message") || "";
  const rawRedirect = searchParams.get("redirect") || "/";
  const redirectUrl = safeRedirectUrl(rawRedirect);

  const resultConfig: Record<
    string,
    { title: string; subTitle: string; status: "success" | "error" | "warning" | "info" }
  > = {
    success: {
      title: t("authResult.successTitle"),
      subTitle: message || t("authResult.successSubTitle"),
      status: "success",
    },
    error: {
      title: t("authResult.errorTitle"),
      subTitle: message || t("authResult.errorSubTitle"),
      status: "error",
    },
    denied: {
      title: t("authResult.deniedTitle"),
      subTitle: message || t("authResult.deniedSubTitle"),
      status: "warning",
    },
    cancelled: {
      title: t("authResult.cancelledTitle"),
      subTitle: message || t("authResult.cancelledSubTitle"),
      status: "info",
    },
  };

  const config = resultConfig[status] || resultConfig.success;

  useEffect(() => {
    if (status === "success" && redirectUrl) {
      const timer = setInterval(() => {
        setCountdown((prev) => {
          if (prev <= 1) {
            clearInterval(timer);
            navigate(redirectUrl, { replace: true });
            return 0;
          }
          return prev - 1;
        });
      }, 1000);
      return () => clearInterval(timer);
    }
  }, [status, redirectUrl, navigate]);

  return (
    <div className="gl-auth-background">
      <div className="gl-auth-panel" style={{ maxWidth: 520, minHeight: "auto" }}>
        <div className="gl-auth-form" style={{ alignItems: "center", justifyContent: "center" }}>
          <Result
            status={config.status}
            title={config.title}
            subTitle={config.subTitle}
            extra={[
              status === "success" && redirectUrl !== "/" ? (
                <Button
                  type="primary"
                  key="redirect"
                  onClick={() => navigate(redirectUrl, { replace: true })}
                >
                  {t("authResult.continue")} ({countdown}s)
                </Button>
              ) : null,
              <Button key="home" onClick={() => navigate("/", { replace: true })}>
                {t("authResult.goToDashboard")}
              </Button>,
              status !== "success" && (
                <Button
                  type="primary"
                  key="retry"
                  onClick={() => navigate("/login", { replace: true })}
                >
                  {t("authResult.tryAgain")}
                </Button>
              ),
            ].filter(Boolean)}
          />
        </div>
      </div>
    </div>
  );
}
