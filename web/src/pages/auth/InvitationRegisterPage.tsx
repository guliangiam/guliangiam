import { App, Button, Form, Input, Typography } from "antd";
import { useMutation } from "@tanstack/react-query";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { verifyInvitation, type VerifyInvitationResult } from "@/shared/api/iam/invitation";
import { DEFAULT_PASSWORD_POLICY } from "@/shared/api/iam/auth";
import { signup, type SignupPayload } from "@/shared/api/iam/auth-signup";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
} from "@/shared/lib/server-error";

export default function InvitationRegisterPage() {
  const { t } = useTranslation("auth");
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [verifyForm] = Form.useForm();
  const [registerForm] = Form.useForm();
  const [step, setStep] = useState(0);
  const [invitationInfo, setInvitationInfo] = useState<VerifyInvitationResult | null>(null);

  // Pre-fill code from URL query param.
  const prefillCode = searchParams.get("code") || "";
  // 接受链接携带组织 slug（/register/invitation?code=...&organization=...）。
  const urlOrganizationSlug = searchParams.get("organization") || "";

  const verifyMutation = useMutation({
    mutationFn: (code: string) => verifyInvitation(code),
    onSuccess: (data) => {
      setInvitationInfo(data);
      setStep(1);
    },
    onError: (error) => {
      if (isHttpError(error)) {
        applyServerFieldErrors(verifyForm, error, buildServerFieldResolver(verifyForm), t);
      }
      message.error(localizedError(error, t, "invitationRegister.invalidCode"));
    },
  });

  // 链接带 code 时自动预检邀请码，无效即提示并停留在输入步骤。
  const autoVerifiedRef = useRef(false);
  useEffect(() => {
    if (prefillCode && !autoVerifiedRef.current) {
      autoVerifiedRef.current = true;
      verifyMutation.mutate(prefillCode);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [prefillCode]);

  const registerMutation = useMutation({
    mutationFn: (payload: SignupPayload) => signup(payload),
    onSuccess: () => {
      message.success(t("invitationRegister.success"));
      navigate("/login", { replace: true });
    },
    onError: (error) => {
      if (isHttpError(error)) {
        applyServerFieldErrors(registerForm, error, buildServerFieldResolver(registerForm), t);
      }
      message.error(localizedError(error, t, "invitationRegister.registerFailed"));
    },
  });

  const onVerifyCode = (values: { code: string }) => {
    verifyMutation.mutate(values.code);
  };

  const onRegister = (values: { username: string; password: string; confirmPassword: string }) => {
    if (values.password !== values.confirmPassword) {
      message.error(t("invitationRegister.passwordMismatch"));
      return;
    }
    // 组织来源：URL 参数优先，其次 verify 返回的 organizationSlug；两者皆无则无法注册。
    const organizationSlug = urlOrganizationSlug || invitationInfo?.organizationSlug || "";
    if (!organizationSlug) {
      message.error(t("invitationRegister.organizationMissing"));
      return;
    }
    registerMutation.mutate({
      organizationSlug,
      username: values.username,
      password: values.password,
      email: invitationInfo?.email || "",
      invitationCode: verifyForm.getFieldValue("code"),
    });
  };

  return (
    <div className="gl-auth-background">
      <div className="gl-auth-panel" style={{ maxWidth: 480, minHeight: "auto" }}>
        <div className="gl-auth-form">
          <div>
            <h1 className="gl-auth-form__title">{t("invitationRegister.title")}</h1>
            <p className="gl-auth-form__subtitle">{t("invitationRegister.subtitle")}</p>
          </div>

          {step === 0 && (
            <Form
              form={verifyForm}
              layout="vertical"
              scrollToFirstError
              onFinish={onVerifyCode}
              initialValues={{ code: prefillCode }}
            >
              <Form.Item
                name="code"
                label={t("invitationRegister.codeLabel")}
                rules={[{ required: true, message: t("auth:enterCode") }]}
              >
                <Input placeholder={t("invitationRegister.codePlaceholder")} />
              </Form.Item>
              <Form.Item>
                <Button type="primary" htmlType="submit" loading={verifyMutation.isPending} block>
                  {t("invitationRegister.verifyButton")}
                </Button>
              </Form.Item>
            </Form>
          )}

          {step === 1 && invitationInfo && (
            <>
              <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
                {t("invitationRegister.registerSubtitle")}
              </Typography.Paragraph>
              <Form form={registerForm} layout="vertical" scrollToFirstError onFinish={onRegister}>
                <Form.Item
                  name="username"
                  label={t("invitationRegister.username")}
                  rules={[{ required: true, message: t("auth:enterUsername") }]}
                >
                  <Input
                    placeholder={t("invitationRegister.chooseUsername")}
                    autoComplete="username"
                  />
                </Form.Item>
                <Form.Item
                  name="password"
                  label={t("password")}
                  rules={[
                    { required: true, message: t("auth:enterPassword") },
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
                  label={t("invitationRegister.confirmPassword")}
                  dependencies={["password"]}
                  rules={[
                    { required: true, message: t("auth:enterConfirmPassword") },
                    ({ getFieldValue }) => ({
                      validator(_, value) {
                        if (!value || getFieldValue("password") === value) {
                          return Promise.resolve();
                        }
                        return Promise.reject(new Error(t("invitationRegister.passwordMismatch")));
                      },
                    }),
                  ]}
                >
                  <Input.Password autoComplete="new-password" />
                </Form.Item>
                <Form.Item>
                  <Button
                    type="primary"
                    htmlType="submit"
                    loading={registerMutation.isPending}
                    block
                  >
                    {t("invitationRegister.registerTitle")}
                  </Button>
                </Form.Item>
              </Form>
            </>
          )}

          <div style={{ textAlign: "center" }}>
            {t("invitationRegister.alreadyHaveAccount")}{" "}
            <Button type="link" onClick={() => navigate("/login", { replace: true })}>
              {t("signIn")}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
