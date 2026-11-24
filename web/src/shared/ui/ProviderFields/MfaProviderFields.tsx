import { Alert, Form, Input } from "antd";
import type { FormInstance } from "antd";
import { useTranslation } from "react-i18next";

/**
 * Per-type MFA layouts, aligned with what the login runtime actually reads:
 * - RADIUS: resolveRadiusProvider takes the shared secret from clientSecret
 *   (native column) and host/port from providerUrl (scheme://host:port); the
 *   generic host/port/password columns are NOT consumed, so they are not
 *   rendered here. requireMessageAuthenticator lives in the auth-switch section.
 * - TOTP is built-in (per-user enrollment in security settings).
 * - SMS/Email MFA reuse the organization's SMS/Email messaging providers.
 * - Push MFA binds a Notification-category provider (EnablePushMfa rejects
 *   other categories), so a Push row here carries no credentials.
 */
export default function MfaProviderFields({ form }: { form?: FormInstance }) {
  const { t } = useTranslation("resource");
  const providerType = Form.useWatch("providerType", form);
  const type = String(providerType ?? "").toUpperCase();

  if (type === "RADIUS") {
    return (
      <>
        <Form.Item
          label={t("provider.mfa.radiusServer", "RADIUS Server")}
          name="providerUrl"
          extra={t(
            "provider.mfa.radiusServerHint",
            "Absolute URL with host and port, e.g. radius://radius.example.com:1812. The login runtime derives the RADIUS host and port from this value.",
          )}
          rules={[
            {
              required: true,
              message: t("provider.mfa.radiusServerRequired", "RADIUS server URL is required"),
            },
            {
              pattern: /^[a-z][a-z0-9+.-]*:\/\/\S+$/i,
              message: t(
                "provider.mfa.radiusServerFormat",
                "Use an absolute URL such as radius://radius.example.com:1812",
              ),
            },
          ]}
        >
          <Input placeholder="radius://radius.example.com:1812" />
        </Form.Item>
        <Form.Item
          label={t("provider.mfa.sharedSecret", "Shared Secret")}
          name="clientSecret"
          extra={t(
            "provider.mfa.sharedSecretHint",
            "RADIUS shared secret. Leave blank on edit to keep the stored value.",
          )}
        >
          <Input.Password placeholder="RADIUS shared secret" autoComplete="new-password" />
        </Form.Item>
      </>
    );
  }

  const hintKey =
    type === "PUSH"
      ? "provider.mfa.pushHint"
      : type === "SMS"
        ? "provider.mfa.smsHint"
        : type === "EMAIL"
          ? "provider.mfa.emailHint"
          : "provider.mfa.totpHint";
  const hintFallback =
    type === "PUSH"
      ? "Push MFA sends approval requests through a Notification-category provider. Create and configure it under the Notification category, then bind it in the user's security settings; no credentials are stored on this row."
      : type === "SMS"
        ? "SMS MFA codes are delivered by the organization's SMS providers. Configure vendors under the SMS category; no credentials are stored on this row."
        : type === "EMAIL"
          ? "Email MFA codes are delivered by the organization's Email providers. Configure vendors under the Email category; no credentials are stored on this row."
          : "TOTP is built in: users enroll an authenticator app in their security settings. No external provider credentials are needed for this row.";

  return (
    <Alert showIcon type="info" style={{ marginBottom: 16 }} title={t(hintKey, hintFallback)} />
  );
}
