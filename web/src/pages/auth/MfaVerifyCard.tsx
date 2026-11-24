import { useMemo, useState } from "react";
import { Alert, App, Button, Checkbox, Form, Input, Select, Space, Typography } from "antd";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { sendVerificationCode } from "@/shared/lib/auth-verification-shim";
import { CaptchaChallengeField } from "@/shared/ui/CaptchaChallengeField";
import { useCaptchaChallenge } from "@/shared/hooks/useCaptchaChallenge";
import type { MfaPropsItem } from "@/shared/types/domain";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
} from "@/shared/lib/server-error";

type VerifyFormValues = {
  mfaType: string;
  passcode?: string;
  recoveryCode?: string;
  enableMfaRemember?: boolean;
};

type MfaVerifyCardProps = {
  mfaProps: MfaPropsItem[];
  organizationId?: string;
  /** 登录入口所属应用；用于把发码路由到该应用绑定的短信/邮件提供商。 */
  applicationId?: string;
  challengeState?: string;
  onBack: () => void;
  onVerify: (values: VerifyFormValues) => Promise<void>;
};

function getMethodLabel(mfaType: string | null | undefined, t: (key: string) => string) {
  switch (mfaType) {
    case "totp":
    case "app":
      return t("mfa.methodAuthenticatorApp");
    case "email":
      return t("mfa.methodEmail");
    case "sms":
      return t("mfa.methodSms");
    case "radius":
      return t("mfa.methodRadius");
    case "push":
      return t("mfa.methodPush");
    default:
      return mfaType || t("mfa.methodUnknown");
  }
}

export function MfaVerifyCard({
  mfaProps,
  organizationId,
  applicationId,
  challengeState,
  onBack,
  onVerify,
}: MfaVerifyCardProps) {
  const { t } = useTranslation("auth");
  const { message } = App.useApp();
  const [form] = Form.useForm<VerifyFormValues>();
  const [useRecoveryCode, setUseRecoveryCode] = useState(false);
  const captcha = useCaptchaChallenge(organizationId ?? "", Boolean(organizationId));

  const preferredMfaType =
    mfaProps.find((item) => item.isPreferred)?.mfaType || mfaProps[0]?.mfaType;
  const selectedMfaType = Form.useWatch("mfaType", form) || preferredMfaType || "app";
  const requiresPasscode = selectedMfaType !== "push";
  const selectedMfa = useMemo(
    () => mfaProps.find((item) => item.mfaType === selectedMfaType) || mfaProps[0] || null,
    [mfaProps, selectedMfaType],
  );

  const sendCodeMutation = useMutation({
    mutationFn: async () => {
      if (!organizationId) {
        throw new Error(t("mfaMethodNotSupported"));
      }
      if (!selectedMfa?.secret || !selectedMfa?.mfaType) {
        throw new Error(t("mfaNoTarget"));
      }
      if (captcha.required && !captcha.token) {
        throw new Error(t("completeCaptchaFirst"));
      }

      const verificationType = selectedMfa.mfaType === "email" ? "email" : "phone";
      await sendVerificationCode({
        organizationId,
        applicationId,
        dest: selectedMfa.secret,
        type: verificationType,
        // Backend returns ISO 3166-1 alpha-2 (new rows) or a legacy dial prefix;
        // the shim converts either form to E.164 before sending.
        countryCode: selectedMfa.countryCode ?? "",
        invalidPhoneMessage: t("enterValidPhone"),
        captchaToken: captcha.required ? captcha.token : undefined,
        captchaId: captcha.required ? captcha.captchaId : undefined,
      });
    },
    onSuccess: () => {
      message.success(t("codeSent"));
      void captcha.reload();
    },
    onError: (error) => {
      void captcha.reload();
      message.error(localizedError(error, t, "sendCodeFailed"));
    },
  });

  const verifyMutation = useMutation({
    mutationFn: async (values: VerifyFormValues) => {
      await onVerify(values);
    },
    onError: (error) => {
      if (isHttpError(error)) {
        applyServerFieldErrors(form, error, buildServerFieldResolver<VerifyFormValues>(form), t);
      }
      message.error(localizedError(error, t, "mfaVerifyFailed"));
    },
  });

  return (
    <Space orientation="vertical" size={16} style={{ width: "100%" }}>
      <div>
        <Typography.Title level={4} style={{ marginBottom: 8 }}>
          {t("mfa.title")}
        </Typography.Title>
        <Typography.Paragraph style={{ marginBottom: 0 }}>{t("mfaTitle")}</Typography.Paragraph>
      </div>

      <Form
        form={form}
        layout="vertical"
        initialValues={{
          mfaType: preferredMfaType,
          enableMfaRemember: false,
        }}
        onFinish={(values) => {
          verifyMutation.mutate(values);
        }}
      >
        <Form.Item
          label={t("mfaMethod")}
          name="mfaType"
          rules={[{ required: true, message: t("selectMfaMethod") }]}
        >
          <Select
            options={mfaProps.map((item) => ({
              label: getMethodLabel(item.mfaType, t),
              value: item.mfaType || "unknown",
            }))}
          />
        </Form.Item>

        {selectedMfaType === "email" || selectedMfaType === "sms" ? (
          <Alert
            showIcon
            type="info"
            style={{ marginBottom: 16 }}
            title={t("currentTarget", { target: selectedMfa?.secret || "-" })}
            action={
              <Button
                size="small"
                loading={sendCodeMutation.isPending}
                onClick={() => sendCodeMutation.mutate()}
              >
                {t("sendMfaCode")}
              </Button>
            }
          />
        ) : null}

        {captcha.required && (selectedMfaType === "email" || selectedMfaType === "sms") ? (
          <CaptchaChallengeField
            state={captcha.state}
            seed={captcha.seed}
            token={captcha.token}
            onChange={captcha.setToken}
            reload={captcha.reload}
          />
        ) : null}

        {selectedMfaType === "push" ? (
          <Alert
            showIcon
            type="info"
            style={{ marginBottom: 16 }}
            title={
              challengeState === "push_pending"
                ? t("pushSentToDevice", { target: selectedMfa?.secret || "-" })
                : t("pushWillSend", { target: selectedMfa?.secret || "-" })
            }
            description={
              challengeState === "push_pending"
                ? t("approveOnDeviceThenVerify")
                : t("clickToVerifyWillSendPush")
            }
          />
        ) : null}

        {!useRecoveryCode ? (
          <Form.Item
            label={t("verificationCode")}
            name="passcode"
            rules={requiresPasscode ? [{ required: true, message: t("enterCode") }] : []}
          >
            <Input
              placeholder={
                selectedMfaType === "radius"
                  ? t("enterRadiusPassword")
                  : selectedMfaType === "push"
                    ? t("noCodeNeededPush")
                    : t("enterCodeMfa")
              }
              disabled={!requiresPasscode}
            />
          </Form.Item>
        ) : (
          <Form.Item
            label={t("mfa.recoveryCode")}
            name="recoveryCode"
            rules={[{ required: true, message: t("enterRecoveryCode") }]}
          >
            <Input />
          </Form.Item>
        )}

        <Form.Item name="enableMfaRemember" valuePropName="checked">
          <Checkbox>
            {t("rememberDevice", { hours: selectedMfa?.mfaRememberInHours ?? 0 })}
          </Checkbox>
        </Form.Item>

        <Space orientation="vertical" style={{ width: "100%" }}>
          <Button type="primary" htmlType="submit" block loading={verifyMutation.isPending}>
            {t("verifyAndSignIn")}
          </Button>
          <Button block onClick={() => setUseRecoveryCode((value) => !value)}>
            {useRecoveryCode ? t("useRegularMfa") : t("useRecoveryCode")}
          </Button>
          <Button block onClick={onBack}>
            {t("backToLoginForm")}
          </Button>
        </Space>
      </Form>
    </Space>
  );
}
