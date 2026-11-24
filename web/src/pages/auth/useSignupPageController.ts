import { useCallback, useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { App, Form } from "antd";
import { signup, type SignupPayload } from "@/shared/api/iam/auth-signup";
import { verifyInvitation } from "@/shared/api/iam/invitation";
import { resolveApplication } from "@/shared/api/iam/application";
import { sendVerificationCode } from "@/shared/lib/auth-verification-shim";
import {
  applyServerFieldErrors,
  serverErrorMessage,
  serverFieldLeaf,
} from "@/shared/lib/server-error";
import { getDefaultSignupItems } from "@/shared/lib/signup-field-config";
import {
  DEFAULT_APPLICATION_NAME,
  getFormPositionStyles,
  getIsSplitLayout,
  getOuterBackground,
} from "@/shared/lib/auth-signin-items";
import { useCaptchaChallenge } from "@/shared/hooks/useCaptchaChallenge";
import { usePublishDiscoveredOrg } from "@/features/theme/discovered-org-store";

export { getDefaultSignupItems, getFieldConfig } from "@/shared/lib/signup-field-config";

export function getPasswordStrength(password: string): number {
  if (!password) return 0;
  let score = 0;
  if (password.length >= 8) score += 25;
  if (password.length >= 12) score += 10;
  if (/[a-z]/.test(password)) score += 15;
  if (/[A-Z]/.test(password)) score += 15;
  if (/\d/.test(password)) score += 15;
  if (/[^a-zA-Z0-9]/.test(password)) score += 20;
  return Math.min(100, score);
}
export function getPasswordStrengthColor(strength: number): string {
  if (strength < 30) return "var(--gl-color-error)";
  if (strength < 60) return "var(--gl-color-warning)";
  return "var(--gl-color-success)";
}

export function useSignupPageController() {
  const { t } = useTranslation();
  const { t: ta } = useTranslation("auth");
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { organizationName } = useParams<{ organizationName: string }>();
  const [form] = Form.useForm();
  const [agreedToTerms, setAgreedToTerms] = useState(false);
  const [agreementModalOpen, setAgreementModalOpen] = useState(false);
  const passwordValue = Form.useWatch("password", form) ?? "";
  const passwordStrength = getPasswordStrength(passwordValue);
  // 验证码收件人来自注册表单已填的 email/phone；两者皆空时禁用发码按钮。
  const emailValue = Form.useWatch("email", form) ?? "";
  const phoneValue = Form.useWatch("phone", form) ?? "";
  const canSendVerificationCode = Boolean(emailValue.trim() || phoneValue.trim());

  const organizationSlug =
    searchParams.get("organization") ||
    (organizationName ? decodeURIComponent(organizationName) : null) ||
    "built-in";
  const signupAppId = searchParams.get("application") || "";
  const isOAuthFlow = location.pathname.includes("/oauth/authorize");
  // 邀请链接可直达通用注册页：?code=<邀请码>，先 verify 预检再随注册请求携带。
  const invitationCode = searchParams.get("code") || "";

  const invitationQuery = useQuery({
    queryKey: ["invitation-verify", invitationCode],
    queryFn: () => verifyInvitation(invitationCode),
    enabled: Boolean(invitationCode),
    retry: false,
    staleTime: 30_000,
  });
  const invitationChecking = Boolean(invitationCode) && invitationQuery.isPending;
  const invitationInvalid = Boolean(invitationCode) && invitationQuery.isError;

  // 匿名公开发现：resolve 返回公开投影（含界面定制字段），管理面 by-ID 接口
  // 对未登录调用方 401。无 ?application= 时解析组织默认应用，定制同样生效；
  // 应用不存在时 resolveApplication 返回 null，页面回退平台默认样式与注册项。
  const { data: application } = useQuery({
    queryKey: ["signup-application", organizationSlug, signupAppId],
    queryFn: () => resolveApplication(organizationSlug, signupAppId || DEFAULT_APPLICATION_NAME),
    retry: false,
    staleTime: 30_000,
  });

  // 人机验证：由组织启用的验证码供应商裁决（captcha-status.required），
  // 注册提交携带 token/captchaId 供后端 EnforceCaptcha 校验。
  const captcha = useCaptchaChallenge(organizationSlug);

  // 组织主题匿名跟随：注册页与登录页同一链路（公开投影 theme）。
  usePublishDiscoveredOrg(organizationSlug);

  const signupItems = useMemo(() => {
    if (application?.signupItems && application.signupItems.length > 0) {
      return application.signupItems.filter((item) => item.visible !== false);
    }
    return getDefaultSignupItems();
  }, [application]);

  // 与登录页同一套壳呈现规则（auth-signin-items 共享 helper），保证应用界面
  // 定制在登录/注册两页无缝一致。signupHtml 是注册页的整页覆盖入口。
  const outerBackground = getOuterBackground(application?.formBackgroundUrl);
  const formPositionStyles = getFormPositionStyles(application?.formOffset);
  const isSplitLayout = getIsSplitLayout(application?.formOffset);
  const customSignupHtml = application?.signupHtml ?? "";
  const signupDisabled = application?.enableSignup === false;

  const agreementItem = useMemo(
    () => signupItems.find((item) => item.name === "Agreement"),
    [signupItems],
  );
  const formItems = useMemo(
    () => signupItems.filter((item) => item.name !== "Agreement" && item.name !== "Captcha"),
    [signupItems],
  );

  // 人机验证的可达性由两段配置共同裁决（此前只读组织供应商，应用的 Captcha
  // 注册项是死配置）：
  //   1. 组织已启用验证码供应商（captcha.required）→ 否则无组件可渲染；
  //   2. 应用「界面定制 → 注册项」未把 Captcha 项设为不可见（visible=false 视为
  //      管理员对该应用显式关闭）。
  // 注意：signupItems 已按 visible 过滤，不能用它判断"被关闭"；必须读原始配置。
  const captchaItemVisible = useMemo(() => {
    const raw = application?.signupItems;
    if (!raw || raw.length === 0) return true;
    const item = raw.find((entry) => entry.name === "Captcha");
    if (!item) return true;
    return item.visible !== false;
  }, [application]);
  const captchaRequired = captcha.required && captchaItemVisible;

  const resolveSignupField = (field: string) => {
    const leaf = serverFieldLeaf(field);
    const targets: Record<string, { name: string; label: string }> = {
      username: { name: "username", label: ta("username") },
      email: { name: "email", label: ta("email") },
      phone: { name: "phone", label: ta("phone") },
      password: { name: "password", label: ta("password") },
      confirmPassword: { name: "confirmPassword", label: ta("confirmPassword") },
      verificationCode: { name: "verificationCode", label: ta("verificationCode") },
    };
    return targets[leaf];
  };

  const signupMutation = useMutation({
    mutationFn: (payload: SignupPayload) => signup(payload),
    onSuccess: () => {
      message.success(t("auth:signupSuccess"));
      if (isOAuthFlow) navigate(`/login/oauth/authorize${location.search}`, { replace: true });
      else navigate("/login", { replace: true });
    },
    onError: (error) => {
      void captcha.reload();
      if (applyServerFieldErrors(form, error, resolveSignupField, t)) return;
      message.error(serverErrorMessage(error, t, "auth:signupFailed"));
    },
  });

  // 注册验证码发码：与后端 resolveVerificationType 同序（email 优先，其次 phone），
  // 经 auth-verification-shim 走 POST /api/v1/auth/verification-code/send。错误就地
  // 提示并吞掉拒绝（与 ResetContactModal 一致），避免未处理 rejection；按钮在收件人
  // 为空时由 sendVerificationCodeDisabled 禁用。
  const handleSendVerificationCode = useCallback(async () => {
    const email = ((form.getFieldValue("email") as string | undefined) ?? "").trim();
    const phone = ((form.getFieldValue("phone") as string | undefined) ?? "").trim();
    const countryCode = (form.getFieldValue("countryCode") as string | undefined) ?? "";
    const dest = email || phone;
    if (!dest) return;
    try {
      await sendVerificationCode({
        organizationId: organizationSlug,
        applicationId: application?.id,
        dest,
        type: email ? "email" : "phone",
        countryCode: email ? undefined : countryCode,
        invalidPhoneMessage: t("auth:enterValidPhone"),
        // 组织启用图形验证码时，发码同样要过 captcha 门禁；不传会在后端被拒。
        captchaToken: captchaRequired ? captcha.token : undefined,
        captchaId: captchaRequired ? captcha.captchaId : undefined,
      });
      message.success(t("auth:codeSent"));
      // 图形验证码 token 单次有效：发码成功后立即换一张，避免重复提交被拒。
      void captcha.reload();
    } catch (error) {
      void captcha.reload();
      message.error(serverErrorMessage(error, t, "auth:sendCodeFailed"));
    }
  }, [application, captcha, captchaRequired, form, message, organizationSlug, t]);

  const onFinish = (values: Record<string, string>) => {
    if (invitationChecking || invitationInvalid) return;
    if (captchaRequired && !captcha.token) {
      message.error(ta("completeCaptchaFirst"));
      return;
    }
    signupMutation.mutate({
      organizationSlug,
      username: values.username,
      password: values.password,
      email: values.email,
      phone: values.phone,
      signupAppId,
      verificationCode: values.verificationCode,
      invitationCode: invitationCode || undefined,
      captchaToken: captchaRequired ? captcha.token : undefined,
      captchaId: captchaRequired ? captcha.captchaId : undefined,
    });
  };

  // 未勾选同意即点击注册时，先弹出《使用条款》《隐私政策》确认弹窗，而非直接提交。
  const isAgreed = () => form.getFieldValue("agreement") === true || agreedToTerms;

  const handleRegisterClick = () => {
    if (invitationChecking || invitationInvalid) return;
    if (!isAgreed()) {
      setAgreementModalOpen(true);
      return;
    }
    form.submit();
  };

  const handleAgreeAndContinue = () => {
    form.setFieldValue("agreement", true);
    setAgreedToTerms(true);
    setAgreementModalOpen(false);
    form.submit();
  };

  const strengthLabel =
    passwordStrength < 30
      ? t("auth:passwordWeak")
      : passwordStrength < 60
        ? t("auth:passwordFair")
        : t("auth:passwordStrong");

  return {
    // i18n
    t,
    ta,
    navigate,
    form,
    // state
    agreedToTerms,
    setAgreedToTerms,
    agreementModalOpen,
    setAgreementModalOpen,
    passwordValue,
    passwordStrength,
    strengthLabel,
    // derived
    organizationSlug,
    signupAppId,
    isOAuthFlow,
    application,
    signupItems,
    formItems,
    agreementItem,
    // shell presentation（与登录页共享规则）
    outerBackground,
    formPositionStyles,
    isSplitLayout,
    customSignupHtml,
    signupDisabled,
    // mutation
    signupMutation,
    // captcha
    captchaRequired,
    captchaState: captcha.state,
    captchaSeed: captcha.seed,
    captchaToken: captcha.token,
    setCaptchaToken: captcha.setToken,
    reloadCaptcha: captcha.reload,
    // invitation
    invitationCode,
    invitationInfo: invitationQuery.data,
    invitationChecking,
    invitationInvalid,
    // handler
    onFinish,
    handleRegisterClick,
    handleAgreeAndContinue,
    handleSendVerificationCode,
    sendVerificationCodeDisabled: !canSendVerificationCode,
  };
}
