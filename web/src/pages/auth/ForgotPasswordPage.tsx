import { App, Button, Form, Input, Steps, Typography } from "antd";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { DEFAULT_PASSWORD_POLICY } from "@/shared/api/iam/auth";
import { resetPassword, type ResetPasswordPayload } from "@/shared/api/iam/auth-signup";
import { loadOwnedApplicationDetail } from "@/shared/lib/auth-application-loader";
import { sendVerificationCode } from "@/shared/lib/auth-verification-shim";
import {
  applyServerFieldErrors,
  serverErrorMessage,
  serverFieldLeaf,
} from "@/shared/lib/server-error";
import { CountryCodeSelect } from "@/shared/ui/CountryCodeSelect";
import { CaptchaChallengeField } from "@/shared/ui/CaptchaChallengeField";
import { SendCodeInput } from "@/shared/ui/SendCodeInput";
import { useCaptchaChallenge } from "@/shared/hooks/useCaptchaChallenge";

// 仅用于区分「应用 UUID」和「应用名/slug」；不能限定版本号，因为仓库的应用
// 主键是 UUIDv7（如 01a0ea76-…-7000-…），限定 v1–v5 会把合法 ID 误判为名称。
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export default function ForgotPasswordPage() {
  const { t } = useTranslation("auth");
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [identifyForm] = Form.useForm();
  const [resetForm] = Form.useForm();
  const [step, setStep] = useState(0);
  const [username, setUsername] = useState("");

  const organizationSlug = searchParams.get("organization") || "built-in";
  // 登录页的忘记密码链接历史上携带的是应用名（如 app-built-in），而验证码
  // 发送接口的 applicationId 需要应用 UUID 才能命中应用级提供商；这里对非
  // UUID 做一次公开解析，解析失败则退回组织级提供商（不影响发码）。
  const applicationParam = searchParams.get("application") || undefined;
  const applicationIdFromUrl =
    applicationParam && UUID_PATTERN.test(applicationParam) ? applicationParam : undefined;
  const applicationLookup = useQuery({
    queryKey: ["forgot-password-application", organizationSlug, applicationParam],
    queryFn: () => loadOwnedApplicationDetail(organizationSlug, applicationParam as string),
    enabled: Boolean(applicationParam) && !applicationIdFromUrl,
    retry: false,
    staleTime: 5 * 60_000,
  });
  const applicationId = applicationIdFromUrl ?? applicationLookup.data?.id ?? undefined;
  const captcha = useCaptchaChallenge(organizationSlug);
  const identifyUsername = Form.useWatch<string | undefined>("username", identifyForm);
  // 不含 @ 的输入按手机号处理：本地号码必须配国家码才能合成 E.164。
  const isPhoneRecovery = Boolean(identifyUsername) && !identifyUsername!.includes("@");

  const resolveIdentifyField = (field: string) => {
    if (serverFieldLeaf(field) !== "username") return undefined;
    return { name: "username", label: t("forgotPassword.usernameOrEmail") };
  };

  const resolveResetField = (field: string) => {
    const leaf = serverFieldLeaf(field);
    if (leaf === "verificationCode") {
      return { name: "verificationCode", label: t("forgotPassword.verificationCode") };
    }
    if (leaf === "newPassword" || leaf === "password") {
      return { name: "newPassword", label: t("forgotPassword.newPassword") };
    }
    return undefined;
  };

  const sendCodeMutation = useMutation({
    mutationFn: async (values: { username: string; countryCode?: string }) => {
      const dest = values.username;
      const type = dest.includes("@") ? "email" : "phone";
      return sendVerificationCode({
        organizationId: organizationSlug,
        applicationId,
        dest,
        type,
        countryCode: values.countryCode,
        invalidPhoneMessage: t("enterValidPhone"),
        captchaToken: captcha.required ? captcha.token : undefined,
        captchaId: captcha.required ? captcha.captchaId : undefined,
      });
    },
    onSuccess: () => {
      message.success(t("forgotPassword.codeSentToContact"));
      void captcha.reload();
    },
    onError: (error) => {
      void captcha.reload();
      if (step === 0 && applyServerFieldErrors(identifyForm, error, resolveIdentifyField, t)) {
        return;
      }
      message.error(serverErrorMessage(error, t, "sendCodeFailed"));
    },
  });

  const resetMutation = useMutation({
    mutationFn: (payload: ResetPasswordPayload) => resetPassword(payload),
    onSuccess: () => {
      message.success(t("forgotPassword.resetSuccess"));
      navigate("/login", { replace: true });
    },
    onError: (error) => {
      if (applyServerFieldErrors(resetForm, error, resolveResetField, t)) return;
      message.error(serverErrorMessage(error, t, "forgotPassword.resetFailed"));
    },
  });

  const onIdentify = (values: { username: string; countryCode?: string }) => {
    if (captcha.required && !captcha.token) {
      message.error(t("completeCaptchaFirst"));
      return;
    }
    setUsername(values.username);
    sendCodeMutation.mutate(values, {
      onSuccess: () => {
        setStep(1);
      },
    });
  };

  const handleResendCode = async () => {
    if (!username) return;
    if (captcha.required && !captcha.token) {
      message.error(t("completeCaptchaFirst"));
      return;
    }
    try {
      await sendCodeMutation.mutateAsync({
        username,
        countryCode: identifyForm.getFieldValue("countryCode") as string | undefined,
      });
    } catch {
      // sendCodeMutation.onError 已弹出错误提示；吞掉拒绝避免未处理 rejection
      // （与 ResetContactModal 发码同口径）。
    }
  };

  const onReset = (values: {
    verificationCode: string;
    newPassword: string;
    confirmPassword: string;
  }) => {
    if (captcha.required && !captcha.token) {
      message.error(t("completeCaptchaFirst"));
      return;
    }
    if (values.newPassword !== values.confirmPassword) {
      resetForm.setFields([
        { name: "confirmPassword", errors: [t("forgotPassword.passwordMismatch")] },
      ]);
      return;
    }
    resetMutation.mutate({
      organizationSlug,
      username,
      verificationCode: values.verificationCode,
      newPassword: values.newPassword,
      // 重置提交与发码同口径：组织启用图形验证码时必须带上本轮 token，
      // 否则后端会以 400 拒绝（前端已在提交前做空 token 拦截）。
      captchaToken: captcha.required ? captcha.token : undefined,
      captchaId: captcha.required ? captcha.captchaId : undefined,
    });
  };

  // 未登录无法（也不应）探测管理端用户列表取脱敏联系方式；只给通用提示。
  const contactHint = t("forgotPassword.codeSentToDefault");

  return (
    <div className="gl-auth-background">
      <div className="gl-auth-panel" style={{ maxWidth: 480, minHeight: "auto" }}>
        <div className="gl-auth-form">
          <div>
            <h1 className="gl-auth-form__title">{t("forgotPassword.title")}</h1>
            <p className="gl-auth-form__subtitle">
              {t("forgotPassword.organization")} <strong>{organizationSlug}</strong>
            </p>
          </div>

          <Steps
            current={step}
            size="small"
            style={{ marginBottom: 24 }}
            items={[
              { title: t("forgotPassword.stepIdentify") },
              { title: t("forgotPassword.stepReset") },
            ]}
          />

          {step === 0 && (
            <Form form={identifyForm} layout="vertical" scrollToFirstError onFinish={onIdentify}>
              <Form.Item
                name="username"
                label={t("forgotPassword.usernameOrEmail")}
                rules={[{ required: true, message: t("auth:enterUsername") }]}
              >
                <Input
                  placeholder={t("forgotPassword.enterUsernameOrEmail")}
                  autoComplete="username"
                />
              </Form.Item>
              {isPhoneRecovery && (
                <Form.Item
                  name="countryCode"
                  label={t("countryCode")}
                  rules={[{ required: true, message: t("selectCountryCode") }]}
                >
                  <CountryCodeSelect ariaLabel={t("countryCode")} style={{ width: "100%" }} />
                </Form.Item>
              )}
              {captcha.required && (
                <CaptchaChallengeField
                  state={captcha.state}
                  seed={captcha.seed}
                  token={captcha.token}
                  onChange={captcha.setToken}
                  reload={captcha.reload}
                />
              )}
              <Form.Item>
                <Button
                  type="primary"
                  htmlType="submit"
                  loading={sendCodeMutation.isPending}
                  disabled={sendCodeMutation.isPending}
                  block
                >
                  {t("forgotPassword.sendVerificationCode")}
                </Button>
              </Form.Item>
            </Form>
          )}

          {step === 1 && (
            <>
              <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
                {contactHint}
              </Typography.Paragraph>
              <Form form={resetForm} layout="vertical" scrollToFirstError onFinish={onReset}>
                <Form.Item
                  name="verificationCode"
                  label={t("forgotPassword.verificationCode")}
                  rules={[{ required: true, message: t("auth:enterCode") }]}
                >
                  <SendCodeInput
                    onSendCode={handleResendCode}
                    placeholder={t("forgotPassword.enterReceivedCode")}
                    sendButtonText={t("forgotPassword.resendCode")}
                    autoComplete="one-time-code"
                  />
                </Form.Item>
                {captcha.required && (
                  // 重置提交与重发验证码共用本轮人机验证：发码成功后 token 会被
                  // 消费并刷新，提交时再校验一次（后端 ResetPassword 已强制校验）。
                  <Form.Item extra={t("completeCaptchaFirst")}>
                    <CaptchaChallengeField
                      state={captcha.state}
                      seed={captcha.seed}
                      token={captcha.token}
                      onChange={captcha.setToken}
                      reload={captcha.reload}
                    />
                  </Form.Item>
                )}
                <Form.Item
                  name="newPassword"
                  label={t("forgotPassword.newPassword")}
                  rules={[
                    { required: true, message: t("auth:enterNewPassword") },
                    {
                      validator: (_, value) =>
                        Array.from(value ?? "").length >= DEFAULT_PASSWORD_POLICY.minLength
                          ? Promise.resolve()
                          : Promise.reject(
                              new Error(
                                t("common:form.minLength", {
                                  min: DEFAULT_PASSWORD_POLICY.minLength,
                                }),
                              ),
                            ),
                    },
                  ]}
                >
                  <Input.Password autoComplete="new-password" />
                </Form.Item>
                <Form.Item
                  name="confirmPassword"
                  label={t("forgotPassword.confirmNewPassword")}
                  dependencies={["newPassword"]}
                  rules={[
                    { required: true, message: t("auth:enterConfirmPassword") },
                    ({ getFieldValue }) => ({
                      validator(_, value) {
                        if (!value || getFieldValue("newPassword") === value) {
                          return Promise.resolve();
                        }
                        return Promise.reject(new Error(t("forgotPassword.passwordMismatch")));
                      },
                    }),
                  ]}
                >
                  <Input.Password autoComplete="new-password" />
                </Form.Item>
                {/* 重置提交前必须再解一次人机验证：后端 ResetPassword 会强制校验
                    captchaToken，且发码时消费掉的 token 不能复用（见上方 CaptchaChallengeField）。 */}
                <Form.Item>
                  <Button
                    type="primary"
                    htmlType="submit"
                    loading={resetMutation.isPending}
                    disabled={resetMutation.isPending}
                    block
                  >
                    {t("forgotPassword.resetButton")}
                  </Button>
                </Form.Item>
              </Form>
            </>
          )}

          <div style={{ textAlign: "center" }}>
            {t("forgotPassword.rememberPassword")}{" "}
            <Button type="link" onClick={() => navigate("/login", { replace: true })}>
              {t("signIn")}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
