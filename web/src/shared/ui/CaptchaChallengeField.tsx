import { Button, Input, Space, Typography } from "antd";
import { useTranslation } from "react-i18next";
import type { CaptchaStateLike } from "@/pages/auth/LoginSigninItems";
import { GoCaptchaInteraction } from "@/shared/ui/GoCaptchaInteraction";
import { CaptchaWidget } from "@/pages/auth/CaptchaWidget";

type CaptchaChallengeFieldProps = {
  state: CaptchaStateLike | null;
  seed: number;
  token: string;
  onChange: (token: string) => void;
  reload: () => void;
  label?: string;
};

/**
 * Controlled human-check field shared by signup and verification-code delivery.
 * Renders the organization's configured provider: default image input,
 * go_captcha click/slide interaction, or a third-party widget. The collected
 * token is lifted to the caller (useCaptchaChallenge) for submission.
 */
export function CaptchaChallengeField({
  state,
  seed,
  token,
  onChange,
  reload,
  label,
}: CaptchaChallengeFieldProps) {
  const { t } = useTranslation("auth");
  if (!state) {
    return null;
  }

  const caption = label ?? t("humanCaptcha");

  if (state.type === "default") {
    return (
      <div style={{ marginBottom: 16 }}>
        <Typography.Text strong style={{ display: "block", marginBottom: 8 }}>
          {caption}
        </Typography.Text>
        <Input
          value={token}
          onChange={(event) => onChange(event.target.value)}
          placeholder={t("enterCaptcha")}
          suffix={
            state.captchaImage ? (
              <button
                type="button"
                style={{ border: 0, background: "transparent", padding: 0 }}
                onClick={reload}
              >
                <img
                  src={`data:image/png;base64,${state.captchaImage}`}
                  alt="captcha"
                  style={{ height: 30, cursor: "pointer", display: "block" }}
                />
              </button>
            ) : null
          }
        />
      </div>
    );
  }

  if (state.type === "go_captcha") {
    return (
      <div style={{ marginBottom: 16 }}>
        <Typography.Text strong style={{ display: "block", marginBottom: 8 }}>
          {caption}
        </Typography.Text>
        <GoCaptchaInteraction
          key={`${state.shape ?? "click"}-${seed}`}
          shape={state.shape}
          masterImage={state.captchaImage}
          subImage={state.captchaImageSub}
          value={token}
          onChange={onChange}
        />
        <Button size="small" type="link" style={{ paddingInline: 0 }} onClick={reload}>
          {t("reloadCaptcha")}
        </Button>
      </div>
    );
  }

  return (
    <div style={{ marginBottom: 16 }}>
      <Typography.Text strong style={{ display: "block", marginBottom: 8 }}>
        {caption}
      </Typography.Text>
      <Space orientation="vertical" style={{ width: "100%" }} size={12}>
        <CaptchaWidget
          key={`${state.type ?? "none"}-${seed}`}
          captchaType={state.type ?? "none"}
          subType={state.subType}
          siteKey={state.clientId}
          clientId2={state.clientId2}
          captchaConfig={state.captchaConfig}
          onChange={onChange}
        />
        <Space>
          <Button type="default" onClick={reload}>
            {t("reloadCaptcha")}
          </Button>
          {token ? (
            <Typography.Text type="secondary">{t("captchaTokenObtained")}</Typography.Text>
          ) : null}
        </Space>
      </Space>
    </div>
  );
}
